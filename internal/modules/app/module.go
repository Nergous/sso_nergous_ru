// Package app exposes the wire-up for the app bounded context.
// bootstrap.New constructs a single *app.Module and pulls everything
// else off it:
//
//	mod.RegisterServer(grpcServer)  // attaches the AppService handler
//	mod.Repository()                // full persistence contract
//	mod.AppReader()                 // narrow read-only surface
//	mod.Service()                   // full admin Service (rarely needed)
//
// The constructor owns the internal dependency graph (db → repo →
// service → handler). Anything outside this file should not need to
// import internal/* — the public surface in app.go / service.go covers
// normal use.
package app

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	appgrpc "sso/internal/modules/app/internal/grpc"
	"sso/internal/modules/app/internal/mariadb"
	"sso/internal/modules/app/internal/service"
	"sso/internal/modules/audit"

	"google.golang.org/grpc"
)

// Emitter is the audit-event sink app uses to record write operations
// (CreateApp, UpdateApp, lifecycle transitions, ...).
//
// It is a type alias over audit.Emitter — app does not extend the
// contract, it merely documents that audit is an outgoing dependency.
// bootstrap supplies the concrete emitter via Deps.
type Emitter = audit.Emitter

// Deps lists everything app needs from its host.
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

// Module is the assembled app bounded context. Construct with New;
// callers should not zero-init this struct directly.
type Module struct {
	service *service.Service
	handler *appgrpc.Handler
	repo    *mariadb.Repository
}

// New wires the module from its dependencies.
//
// Ordering: repository (talks to MariaDB) is built first, then the
// application Service (talks to the repository plus the audit
// emitter), then the gRPC handler (talks to the Service). The same
// repository value satisfies AppReader and the wider Repository
// surface — compile-time checks just below guarantee the public
// contracts stay in sync with the repository's method set.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("app: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("app: log is required")
	}
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Audit == nil {
		d.Audit = audit.NopEmitter{}
	}

	repo := mariadb.NewRepository(d.DB)

	var _ AppReader = repo
	var _ Repository = repo

	svc := service.NewService(d.Log, repo, d.Clock, d.Audit)
	h := appgrpc.NewHandler(svc, d.Log)

	return &Module{
		service: svc,
		handler: h,
		repo:    repo,
	}, nil
}

// RegisterServer attaches the AppService handlers to the supplied gRPC
// server. bootstrap passes this method as a registrar callback to
// grpcserver.New.
func (m *Module) RegisterServer(s *grpc.Server) {
	m.handler.RegisterServer(s)
}

// Service returns the application-layer Service. Most callers don't
// need this — the gRPC handler in this module already routes the
// public RPCs.
func (m *Module) Service() *service.Service { return m.service }

// Repository returns the full persistence contract. auth and access
// consume this for cross-module reads and existence checks.
func (m *Module) Repository() Repository { return m.repo }

// AppReader returns the narrow read-only surface.
func (m *Module) AppReader() AppReader { return m.repo }
