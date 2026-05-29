// Package bootstrap is the application composition root. It owns the
// dependency graph: open the DB, build module wirings, register their
// gRPC handlers on a configured server.
//
// cmd/sso/main.go remains responsible for process-lifecycle concerns
// (config/logger loading, signal handling, graceful shutdown). bootstrap
// owns "what we are running" and main owns "when".
//
// Modules already migrated to the per-bounded-context layout
// (identity / app / role / access) are wired via their public
// Module.New() constructors. Modules still in the legacy top-level
// layout (audit / auth / serviceaccount / session / recoverycode)
// continue to use their existing NewService / NewRepository
// constructors directly.
package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"sso/internal/kernel/actor"
	"sso/internal/modules/access"
	"sso/internal/modules/app"
	"sso/internal/modules/audit"
	"sso/internal/modules/auth"
	"sso/internal/modules/identity"
	"sso/internal/modules/recoverycode"
	"sso/internal/modules/role"
	"sso/internal/modules/serviceaccount"
	"sso/internal/modules/session"
	"sso/internal/platform/audit/authz"
	auditbus "sso/internal/platform/audit/bus"
	"sso/internal/platform/config"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/randtoken"
	recoverygen "sso/internal/platform/crypto/recoverycode"
	grpcauth "sso/internal/platform/grpc/auth"
	grpcserver "sso/internal/platform/grpc/server"
	"sso/internal/platform/httpserver"
	"sso/internal/platform/mariadb"
	"sso/internal/platform/ratelimit"

	ssoauthv1 "github.com/Nergous/sso_protos/gen/go/sso/auth/v1"
)

// App is the assembled application. Run blocks until every listener stops
// (cleanly or by error); Stop signals shutdown and releases resources in
// reverse order.
type App struct {
	log               *slog.Logger
	db                *sql.DB
	grpcServer        *grpcserver.Server
	httpServer        *httpserver.Server // nil when cfg.HTTP.Enabled=false
	dbShutdownTimeout time.Duration
	httpShutdownTO    time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{} // closed when Run returns
}

// New builds the application graph. On error every partially-acquired
// resource is released before returning, so callers do not need to clean
// up on failure.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger) (*App, error) {
	db, err := mariadb.Open(ctx, cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: open database: %w", err)
	}

	// ----- audit ------------------------------------------------------------
	auditModule, err := audit.New(audit.Deps{
		DB:    db,
		Log:   log,
		Authz: audit.AlwaysDenyAuthorizer{},
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire audit: %w", err)
	}
	// Audit emission is feature-gated: production runs with SyncEmitter
	// (write-through to audit_events); dev / tests can flip enabled=false
	// to skip the round-trip without touching every use-case wiring.
	var auditEmitter audit.Emitter = audit.NopEmitter{}
	if cfg.Audit.Enabled {
		auditEmitter = auditbus.NewSyncEmitter(auditModule.Repository(), log)
	}

	// ----- identity / app / role (new module layout) ------------------------
	identityModule, err := identity.New(identity.Deps{
		DB:    db,
		Log:   log,
		Clock: time.Now,
		Audit: auditEmitter,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire identity: %w", err)
	}

	appModule, err := app.New(app.Deps{
		DB:    db,
		Log:   log,
		Clock: time.Now,
		Audit: auditEmitter,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire app: %w", err)
	}

	roleModule, err := role.New(role.Deps{
		DB:    db,
		Log:   log,
		Clock: time.Now,
		Audit: auditEmitter,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire role: %w", err)
	}

	// access needs three sibling repositories for its cross-context
	// precondition checks (role active, user eligible, app exists).
	accessModule, err := access.New(access.Deps{
		DB:    db,
		Log:   log,
		Clock: time.Now,
		Audit: auditEmitter,
		Users: identityModule.Repository(),
		Roles: roleModule.Repository(),
		Apps:  appModule.Repository(),
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire access: %w", err)
	}

	// Late-bind the real audit authorizer now that access is ready.
	// The audit module was created above with AlwaysDenyAuthorizer to
	// break the dep cycle (audit emitter is consumed by access, but
	// the audit authz consumes access.Service).
	auditModule.SetAuthorizer(authz.New(accessModule.Service(), db, log))

	// ----- serviceaccount ---------------------------------------------------
	saModule, err := serviceaccount.New(serviceaccount.Deps{
		DB:    db,
		Log:   log,
		Clock: time.Now,
		Audit: auditEmitter,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire serviceaccount: %w", err)
	}
	saRepo := saModule.Repository()

	// ----- auth wiring ------------------------------------------------------
	//
	// JWT keys load synchronously: without them no token can be signed or
	// verified, so a load failure has to abort startup before the gRPC
	// listener comes up. A partial bootstrap that serves requests with no
	// auth would be far worse than a hard exit.
	sessionModule, err := session.New(session.Deps{DB: db, Log: log})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire session: %w", err)
	}
	sessionRepo := sessionModule.Repository()

	recoveryModule, err := recoverycode.New(recoverycode.Deps{DB: db, Log: log})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire recoverycode: %w", err)
	}
	recoveryCodeRepo := recoveryModule.Repository()

	priv, err := jwt.LoadPrivateKeyPEM(cfg.Auth.JWT.PrivateKeyPath)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: load jwt private key: %w", err)
	}
	pub, err := jwt.LoadPublicKeyPEM(cfg.Auth.JWT.PublicKeyPath)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: load jwt public key: %w", err)
	}
	signer := jwt.NewEd25519Signer(priv, cfg.Auth.JWT.Issuer, cfg.Auth.JWT.AccessTTL)
	verifier := jwt.NewEd25519Verifier(pub, cfg.Auth.JWT.Issuer)

	authModule, err := auth.New(auth.Deps{
		Log:                log,
		Users:              identityModule.Repository(),
		Sessions:           sessionRepo,
		ServiceAccounts:    saRepo,
		Apps:               appModule.Repository(),
		RecoveryCodes:      recoveryCodeRepo,
		Signer:             signer,
		Verifier:           verifier,
		TokenGen:           randtoken.Generator{},
		RecoveryGen:        recoverygen.New(),
		Clock:              time.Now,
		AccessTTL:          cfg.Auth.JWT.AccessTTL,
		RefreshTTL:         cfg.Auth.Session.RefreshTTL,
		RefreshRotationTTL: cfg.Auth.Session.RefreshRotationTTL,
		BcryptCost:         cfg.Auth.Bcrypt.Cost,
		Audit:              auditEmitter,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: wire auth: %w", err)
	}

	// Public method whitelist: AuthService's own public RPCs plus the
	// transport-level surfaces clients hit before authenticating.
	// grpc.health.* covers k8s liveness/readiness probes; reflection
	// (both v1 and v1alpha) is gated by config but always whitelisted
	// here so a server with reflection enabled doesn't reject discovery
	// for lack of a token.
	publicRPCs := append([]string{}, auth.PublicRPCs...)
	publicRPCs = append(publicRPCs,
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo",
		"/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo",
	)
	authInterceptor := grpcauth.NewInterceptor(verifier, sessionRepo, log, publicRPCs)

	// Rate-limit interceptor. Disabled in config → nil → server.New
	// skips it. Extractors stay in bootstrap because they know proto
	// types; the ratelimit package itself is proto-free by design.
	var rateLimitUnary grpc.UnaryServerInterceptor
	if cfg.RateLimit.Enabled {
		rl := buildRateLimiter(cfg.RateLimit)
		rl.Start(ctx)
		rateLimitUnary = rl.Unary()
	}

	srv, err := grpcserver.New(cfg.GRPC, log, authInterceptor.Unary(), rateLimitUnary,
		identityModule.RegisterServer,
		appModule.RegisterServer,
		roleModule.RegisterServer,
		saModule.RegisterServer,
		accessModule.RegisterServer,
		authModule.RegisterServer,
		auditModule.RegisterServer,
	)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap: build grpc server: %w", err)
	}

	// HTTP listener (grpc-gateway) is optional: cfg.HTTP.Enabled=false
	// keeps the process gRPC-only, no HTTP port bound.
	var httpSrv *httpserver.Server
	if cfg.HTTP.Enabled {
		// Gateway dials gRPC over loopback. cfg.GRPC.Host may be "0.0.0.0";
		// rewrite to 127.0.0.1 so we don't accidentally route through an
		// external interface when an admin binds gRPC widely.
		gatewayTarget := loopbackTarget(&cfg.GRPC)
		httpSrv, err = httpserver.New(ctx, httpserver.Deps{
			Cfg:        cfg.HTTP,
			Log:        log,
			GRPCTarget: gatewayTarget,
			Readiness: func(probeCtx context.Context) error {
				return db.PingContext(probeCtx)
			},
		})
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("bootstrap: build http server: %w", err)
		}
	}

	return &App{
		log:               log,
		db:                db,
		grpcServer:        srv,
		httpServer:        httpSrv,
		dbShutdownTimeout: cfg.Database.ShutdownTimeout,
		httpShutdownTO:    cfg.HTTP.ShutdownTimeout,
		stopCh:            make(chan struct{}),
		doneCh:            make(chan struct{}),
	}, nil
}

// buildRateLimiter wires the in-memory rate-limit interceptor from
// config-driven policies and a hard-coded routing table that knows which
// proto-level extractor produces the key for each defended RPC.
//
// Keeping the extractors here (and not inside the ratelimit package)
// preserves the package's proto-freedom — ratelimit operates on opaque
// (ctx, req any) values and never imports a generated message type.
func buildRateLimiter(cfg config.RateLimitConfig) *ratelimit.Interceptor {
	idle := func(p config.Policy) time.Duration {
		// Default idle eviction: twice the time to refill the bucket
		// from empty to burst. Floored at 1m so a low-rps policy still
		// evicts within a reasonable window.
		d := time.Duration(2 * float64(p.Burst) / p.Rps * float64(time.Second))
		if d < time.Minute {
			d = time.Minute
		}
		return d
	}
	toPolicy := func(name ratelimit.PolicyName, p config.Policy) ratelimit.Policy {
		return ratelimit.Policy{
			Name:      name,
			RPS:       p.Rps,
			Burst:     p.Burst,
			IdleEvict: idle(p),
		}
	}

	policies := map[ratelimit.PolicyName]ratelimit.Policy{
		ratelimit.LoginPerIP:            toPolicy(ratelimit.LoginPerIP, cfg.Policies.LoginPerIP),
		ratelimit.LoginPerUsername:      toPolicy(ratelimit.LoginPerUsername, cfg.Policies.LoginPerUsername),
		ratelimit.ResetPerIP:            toPolicy(ratelimit.ResetPerIP, cfg.Policies.ResetPerIP),
		ratelimit.ResetPerEmail:         toPolicy(ratelimit.ResetPerEmail, cfg.Policies.ResetPerEmail),
		ratelimit.ServiceAuthPerClient:  toPolicy(ratelimit.ServiceAuthPerClient, cfg.Policies.ServiceAuthPerClient),
		ratelimit.ChangePasswordPerUser: toPolicy(ratelimit.ChangePasswordPerUser, cfg.Policies.ChangePasswordPerUser),
	}

	bindings := map[string][]ratelimit.MethodLimit{
		"/sso.auth.v1.AuthService/Login": {
			{Policy: ratelimit.LoginPerIP, Extractor: extractPeerIP},
			{Policy: ratelimit.LoginPerUsername, Extractor: extractLoginIdentifier},
		},
		"/sso.auth.v1.AuthService/ResetPasswordWithRecoveryCode": {
			{Policy: ratelimit.ResetPerIP, Extractor: extractPeerIP},
			{Policy: ratelimit.ResetPerEmail, Extractor: extractResetIdentifier},
		},
		"/sso.auth.v1.AuthService/AuthenticateServiceAccount": {
			{Policy: ratelimit.ServiceAuthPerClient, Extractor: extractServiceAccountID},
		},
		"/sso.auth.v1.AuthService/ChangePassword": {
			{Policy: ratelimit.ChangePasswordPerUser, Extractor: extractActorID},
		},
	}

	return ratelimit.New(policies, bindings, cfg.CleanupInterval)
}

// extractPeerIP keys on the gRPC peer IP. ok=false means the peer has
// no Addr (in-process test), in which case the policy is skipped rather
// than failing the request.
func extractPeerIP(ctx context.Context, _ any) (ratelimit.Key, bool) {
	ip := grpcauth.PeerIP(ctx)
	if ip == "" {
		return "", false
	}
	return ratelimit.Key(ip), true
}

// extractLoginIdentifier pulls the login key from LoginRequest payload
// (email if present, else username). Lower-cased and trimmed so the
// same identity hashes to one bucket regardless of caller casing.
func extractLoginIdentifier(_ context.Context, req any) (ratelimit.Key, bool) {
	r, ok := req.(*ssoauthv1.LoginRequest)
	if !ok {
		return "", false
	}
	return normalizedIdentifier(r.GetEmail(), r.GetUsername())
}

func extractResetIdentifier(_ context.Context, req any) (ratelimit.Key, bool) {
	r, ok := req.(*ssoauthv1.ResetPasswordWithRecoveryCodeRequest)
	if !ok {
		return "", false
	}
	return normalizedIdentifier(r.GetEmail(), r.GetUsername())
}

func extractServiceAccountID(_ context.Context, req any) (ratelimit.Key, bool) {
	r, ok := req.(*ssoauthv1.ServiceAccountAuthRequest)
	if !ok {
		return "", false
	}
	id := strings.TrimSpace(r.GetServiceAccountId())
	if id == "" {
		return "", false
	}
	return ratelimit.Key(id), true
}

// extractActorID pulls the authenticated subject ID. For ChangePassword
// the auth interceptor has already injected the actor by the time this
// runs (it's a protected RPC), so missing actor here is a real failure
// of the upstream chain — we skip the policy rather than reject, since
// auth will reject the request itself.
func extractActorID(ctx context.Context, _ any) (ratelimit.Key, bool) {
	a, ok := actor.From(ctx)
	if !ok || a.ID == "" {
		return "", false
	}
	return ratelimit.Key(a.ID), true
}

func normalizedIdentifier(email, username string) (ratelimit.Key, bool) {
	id := strings.ToLower(strings.TrimSpace(email))
	if id == "" {
		id = strings.ToLower(strings.TrimSpace(username))
	}
	if id == "" {
		return "", false
	}
	return ratelimit.Key(id), true
}

// loopbackTarget picks the address the in-process gateway uses to dial the
// gRPC server. If gRPC is bound to a wildcard ("0.0.0.0" / "::"), we connect
// over the loopback interface anyway — the traffic should never leave the
// host even if the listener is publicly exposed.
func loopbackTarget(cfg *config.GRPCConfig) string {
	host := cfg.Host
	switch host {
	case "0.0.0.0", "":
		host = "127.0.0.1"
	case "::", "[::]":
		host = "[::1]"
	}
	return (&config.GRPCConfig{Host: host, Port: cfg.Port}).Address()
}

// Run starts every configured listener (gRPC always, HTTP if enabled) and
// blocks until they all exit. Each listener gets its own goroutine; if one
// fails the watcher triggers a coordinated shutdown of the others. Returns
// the first error reported by any goroutine, or nil on a clean stop.
func (a *App) Run() error {
	defer close(a.doneCh)
	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		err := a.grpcServer.Run()
		if err != nil {
			a.signalStop()
		}
		return err
	})

	if a.httpServer != nil {
		g.Go(func() error {
			err := a.httpServer.Run()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.signalStop()
				return err
			}
			return nil
		})
	}

	// Shutdown watcher: waits for Stop() (external SIGTERM/SIGINT path) or
	// for a listener failure (errgroup-cancelled ctx). HTTP is torn down
	// first — it is lighter and the gRPC server still needs to handle the
	// in-flight requests that came in through the gateway.
	g.Go(func() error {
		select {
		case <-a.stopCh:
		case <-ctx.Done():
		}
		if a.httpServer != nil {
			shctx, cancel := context.WithTimeout(context.Background(), a.httpShutdownTO)
			defer cancel()
			if err := a.httpServer.Stop(shctx); err != nil {
				a.log.Warn("bootstrap: http shutdown", slog.Any("error", err))
			}
		}
		a.grpcServer.Stop()
		return nil
	})

	return g.Wait()
}

// Stop signals every listener to shut down, waits for Run to return, and
// then closes the database pool. The wait ordering matters: gRPC handlers
// finishing their drain need the DB pool to stay open until they actually
// finish. Safe to call multiple times; the listener signal only fires once.
func (a *App) Stop() {
	a.signalStop()
	<-a.doneCh
	a.closeDB()
}

func (a *App) signalStop() {
	a.stopOnce.Do(func() { close(a.stopCh) })
}

// closeDB calls *sql.DB.Close in a goroutine and races it against a timer.
// In practice the gRPC server has already drained by this point so there
// are no in-flight queries to wait on; the watchdog only protects against
// a stuck connection that would otherwise hang the whole process.
func (a *App) closeDB() {
	done := make(chan error, 1)
	go func() { done <- a.db.Close() }()

	t := time.NewTimer(a.dbShutdownTimeout)
	defer t.Stop()

	select {
	case err := <-done:
		if err != nil {
			a.log.Warn("bootstrap: close database", slog.Any("error", err))
		}
	case <-t.C:
		a.log.Warn("bootstrap: close database timed out, abandoning",
			slog.Duration("timeout", a.dbShutdownTimeout))
	}
}
