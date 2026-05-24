// Package auth exposes the wire-up for the auth bounded context.
// bootstrap.New constructs a single *auth.Module and pulls everything
// else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the AuthService handler
//	mod.Service()                   // application-layer service (rare)
//
// auth has no Repository of its own — it orchestrates across identity /
// session / recoverycode / app / serviceaccount. Deps lists every
// upstream repository (supplied by sibling Module.Repository() getters
// in bootstrap) plus the JWT signing material and bcrypt cost.
package auth

import (
	"fmt"
	"log/slog"
	"time"

	"sso/internal/app"
	"sso/internal/audit"
	grpcadapter "sso/internal/auth/internal/grpc"
	"sso/internal/auth/internal/service"
	"sso/internal/identity"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/randtoken"
	recoverygen "sso/internal/platform/crypto/recoverycode"
	"sso/internal/recoverycode"
	"sso/internal/serviceaccount"
	"sso/internal/session"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink auth uses to record write
// operations. Type alias over audit.Emitter — keeps the wiring
// contract local to this module.
type Emitter = audit.Emitter

// Deps lists everything auth needs from its host.
type Deps struct {
	Log *slog.Logger

	Users           identity.Repository
	Sessions        session.Repository
	ServiceAccounts serviceaccount.Repository
	Apps            app.Repository
	RecoveryCodes   recoverycode.Repository

	Signer   jwt.Signer
	Verifier jwt.Verifier

	TokenGen    randtoken.Generator
	RecoveryGen recoverygen.Generator

	Clock func() time.Time

	AccessTTL          time.Duration
	RefreshTTL         time.Duration
	RefreshRotationTTL time.Duration

	BcryptCost int

	Audit Emitter
}

// Module is the assembled auth bounded context.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
}

// New wires the module from its dependencies. Every upstream
// repository is required — nil would cause a use-case to NPE at first
// call, so reject at construction.
func New(d Deps) (*Module, error) {
	if d.Log == nil {
		return nil, fmt.Errorf("auth: log is required")
	}
	if d.Users == nil {
		return nil, fmt.Errorf("auth: users repository is required")
	}
	if d.Sessions == nil {
		return nil, fmt.Errorf("auth: sessions repository is required")
	}
	if d.ServiceAccounts == nil {
		return nil, fmt.Errorf("auth: service-accounts repository is required")
	}
	if d.Apps == nil {
		return nil, fmt.Errorf("auth: apps repository is required")
	}
	if d.RecoveryCodes == nil {
		return nil, fmt.Errorf("auth: recovery-codes repository is required")
	}
	if d.Signer == nil {
		return nil, fmt.Errorf("auth: jwt signer is required")
	}
	if d.Verifier == nil {
		return nil, fmt.Errorf("auth: jwt verifier is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	svc := service.NewService(
		d.Log,
		d.Users, d.Sessions, d.ServiceAccounts, d.Apps, d.RecoveryCodes,
		d.Signer, d.Verifier,
		d.TokenGen, d.RecoveryGen,
		d.Clock,
		d.AccessTTL, d.RefreshTTL, d.RefreshRotationTTL,
		d.BcryptCost,
		d.Audit,
	)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{service: svc, handler: h}, nil
}

// RegisterServer attaches the AuthService handlers to the supplied
// gRPC server.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service.
func (m *Module) Service() *service.Service { return m.service }
