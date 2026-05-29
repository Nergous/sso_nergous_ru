// Package audit exposes the wire-up for the audit bounded context.
// bootstrap.New constructs a single *audit.Module and pulls everything
// else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the AuditService handler
//	mod.Repository()                // persistence contract (consumed by auditbus)
//	mod.Service()                   // read-side Service (rare direct use)
package audit

import (
	"database/sql"
	"fmt"
	"log/slog"

	grpcadapter "sso/internal/modules/audit/internal/grpc"
	"sso/internal/modules/audit/internal/mariadb"
	"sso/internal/modules/audit/internal/service"

	"google.golang.org/grpc"
)

// Deps lists everything audit needs from its host.
//
// Authz is required — it gates every read RPC. bootstrap supplies
// AlwaysDenyAuthorizer until a real RBAC implementation lands.
type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Authz AuditAuthorizer
}

// Module is the assembled audit bounded context.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("audit: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("audit: log is required")
	}
	if d.Authz == nil {
		d.Authz = AlwaysDenyAuthorizer{}
	}

	repo := mariadb.NewRepository(d.DB)

	var _ Repository = repo

	svc := service.NewService(repo, d.Authz)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the AuditService handlers to the supplied
// gRPC server.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service.
func (m *Module) Service() *service.Service { return m.service }

// Repository returns the persistence contract. The platform-level
// auditbus emitter consumes this to write events synchronously.
func (m *Module) Repository() Repository { return m.repo }

func (m *Module) SetAuthorizer(a AuditAuthorizer) { m.service.SetAuthorizer(a) }
