// Package serviceaccount exposes the wire-up for the service-account
// bounded context. bootstrap.New constructs a single
// *serviceaccount.Module and pulls everything else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the ServiceAccountService handler
//	mod.Repository()                // persistence contract for auth
//	mod.Service()                   // full admin Service (rarely needed)
package serviceaccount

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"sso/internal/audit"
	grpcadapter "sso/internal/serviceaccount/internal/grpc"
	"sso/internal/serviceaccount/internal/mariadb"
	"sso/internal/serviceaccount/internal/service"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink the module uses to record write
// operations. Type alias over audit.Emitter — keeps the wiring
// contract local to this module.
type Emitter = audit.Emitter

// Deps lists everything serviceaccount needs from its host.
type Deps struct {
	DB    *sql.DB
	Log   *slog.Logger
	Clock func() time.Time
	Audit Emitter
}

// Module is the assembled service-account bounded context.
type Module struct {
	service *service.Service
	handler *grpcadapter.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("serviceaccount: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("serviceaccount: log is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	repo := mariadb.NewRepository(d.DB)

	var _ Repository = repo

	svc := service.NewService(d.Log, repo, d.Clock, d.Audit)
	h := grpcadapter.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the ServiceAccountService handlers to the
// supplied gRPC server.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service.
func (m *Module) Service() *service.Service { return m.service }

// Repository returns the persistence contract. auth consumes this for
// AuthenticateServiceAccount.
func (m *Module) Repository() Repository { return m.repo }
