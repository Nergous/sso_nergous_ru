// Package access exposes the wire-up for the access bounded context
// (role assignments and authorization decisions). bootstrap.New
// constructs a single *access.Module and pulls everything else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the AccessService handler
//	mod.Service()                   // application-layer service
//
// The constructor owns the internal dependency graph (db → repo →
// service → handler). Cross-context cooperation: access pulls
// identity.Repository, role.Repository, and app.Repository for
// precondition checks (role active, user not blocked/deleted, role
// belongs to app, etc.). The access aggregate itself stays free of
// those imports — only its use-case layer reaches across.
package access

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	grpcadapter "sso/internal/modules/access/internal/grpc"
	"sso/internal/modules/access/internal/mariadb"
	"sso/internal/modules/access/internal/service"
	"sso/internal/modules/app"
	"sso/internal/modules/audit"
	"sso/internal/modules/identity"
	"sso/internal/modules/role"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink access uses to record write
// operations (GrantRoleToUser, BulkGrantRoles, ...).
type Emitter = audit.Emitter

// Deps lists everything access needs from its host. Beyond the usual
// DB / Log / Clock / Audit, access needs the three sibling
// repositories — supplied by the sibling Module.Repository() getters
// in bootstrap.
type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Clock func() time.Time
	Audit Emitter

	Users identity.Repository
	Roles role.Repository
	Apps  app.Repository
}

// Module is the assembled access bounded context. Construct with New;
// callers should not zero-init this struct directly.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("access: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("access: log is required")
	}
	if d.Users == nil {
		return nil, fmt.Errorf("access: users repository is required")
	}
	if d.Roles == nil {
		return nil, fmt.Errorf("access: roles repository is required")
	}
	if d.Apps == nil {
		return nil, fmt.Errorf("access: apps repository is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	repo := mariadb.NewRepository(d.DB)

	var _ Repository = repo

	svc := service.NewService(d.Log, repo, d.Users, d.Roles, d.Apps, d.Clock, d.Audit)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the AccessService handlers to the supplied
// gRPC server.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service.
func (m *Module) Service() *service.Service { return m.service }
