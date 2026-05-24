// Package identity exposes the wire-up for the identity bounded
// context. bootstrap.New constructs a single *identity.Module and
// pulls everything else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the IdentityService handler
//	mod.Repository()                // full persistence contract for auth
//	mod.UserReader()                // narrow read-only surface for access
//	mod.Service()                   // full admin Service (rarely needed)
//
// The constructor owns the internal dependency graph (db → repo →
// service → handler). Anything outside this file should not need to
// import internal/* — the public surface in identity.go / service.go
// covers normal use.
package identity

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"sso/internal/audit"
	grpcadapter "sso/internal/identity/internal/grpc"
	"sso/internal/identity/internal/mariadb"
	"sso/internal/identity/internal/service"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink identity uses to record write
// operations (CreateUser, UpdateUser, lifecycle transitions, ...).
//
// It is a type alias over audit.Emitter — identity does not extend
// the contract, it merely documents that audit is an outgoing
// dependency. bootstrap supplies the concrete emitter via Deps.
type Emitter = audit.Emitter

// Deps lists everything identity needs from its host.
//
// DB     — connection pool, owned upstream (bootstrap closes it).
// Log    — structured logger; required.
// Clock  — optional; defaults to time.Now when nil.
// Audit  — optional; defaults to audit.NopEmitter when nil (events are
//
//	dropped). bootstrap supplies a real emitter in production.
type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Clock func() time.Time
	Audit Emitter
}

// Module is the assembled identity bounded context. Construct with New;
// callers should not zero-init this struct directly.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
//
// Ordering: repository (talks to MariaDB) is built first, then the
// application Service (talks to the repository plus the audit
// emitter), then the gRPC handler (talks to the Service). The same
// repository value satisfies UserReader and the wider Repository
// surface — compile-time checks just below guarantee the public
// contracts stay in sync with the repository's method set.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("identity: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("identity: log is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	repo := mariadb.NewRepository(d.DB)

	// Compile-time checks: the MariaDB repository satisfies both public
	// contracts. If GetByID/GetByEmail/GetByUsername drift, the build
	// breaks here — not at the first cross-module call.
	var _ UserReader = repo
	var _ Repository = repo

	svc := service.NewService(d.Log, repo, d.Clock, d.Audit)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the IdentityService handlers to the supplied
// gRPC server. bootstrap passes this method as a registrar callback to
// grpcserver.New.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service. Most callers don't
// need this — the gRPC handler in this module already routes the
// public RPCs. Useful for admin tooling that wants to bypass the
// transport layer (seed-admin CLI, integration tests).
func (m *Module) Service() *service.Service { return m.service }

// Repository returns the full persistence contract. auth consumes this
// to power Register / Login / ChangePassword / Refresh and other write
// paths that the narrow UserReader does not cover.
func (m *Module) Repository() Repository { return m.repo }

// UserReader returns the narrow read-only surface. access fetches the
// actor user for RoleAssignment lookups through this.
func (m *Module) UserReader() UserReader { return m.repo }
