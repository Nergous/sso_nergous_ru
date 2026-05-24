// Package role exposes the wire-up for the role bounded context.
// bootstrap.New constructs a single *role.Module and pulls everything
// else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the RolesService handler
//	mod.Repository()                // full persistence contract for access
//	mod.Service()                   // full admin Service (rarely needed)
//
// The constructor owns the internal dependency graph (db → repo →
// service → handler). Anything outside this file should not need to
// import internal/* — the public surface in role.go / service.go
// covers normal use.
package role

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"sso/internal/audit"
	grpcadapter "sso/internal/role/internal/grpc"
	"sso/internal/role/internal/mariadb"
	"sso/internal/role/internal/service"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink role uses to record write operations
// (CreateRole, UpdateRole, lifecycle transitions, ...).
//
// It is a type alias over audit.Emitter — role does not extend the
// contract, it merely documents that audit is an outgoing dependency.
// bootstrap supplies the concrete emitter via Deps.
type Emitter = audit.Emitter

// Deps lists everything role needs from its host.
//
// DB     — connection pool, owned upstream (bootstrap closes it).
// Log    — structured logger; required.
// Clock  — optional; defaults to time.Now when nil.
// Audit  — optional; defaults to audit.NopEmitter when nil.
type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Clock func() time.Time
	Audit Emitter
}

// Module is the assembled role bounded context. Construct with New;
// callers should not zero-init this struct directly.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("role: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("role: log is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	repo := mariadb.NewRepository(d.DB)

	// Compile-time check: the MariaDB repository satisfies the public
	// Repository surface. If GetByID/Create/Update/Delete/List drift,
	// the build breaks here.
	var _ Repository = repo

	svc := service.NewService(d.Log, repo, d.Clock, d.Audit)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the RolesService handlers to the supplied
// gRPC server.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service.
func (m *Module) Service() *service.Service { return m.service }

// Repository returns the full persistence contract. access consumes
// this for role-active / role-in-app precondition checks and Role
// aggregate hydration in ListUserRoles.
func (m *Module) Repository() Repository { return m.repo }
