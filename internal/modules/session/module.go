// Package session exposes the wire-up for the session bounded context.
// bootstrap.New constructs a single *session.Module and pulls
// everything else off it:
//
//	mod.Repository()    persistence contract, consumed by auth and the
//	                    grpcauth interceptor
//
// There is no Service or gRPC handler in this module — session
// operations are surfaced through AuthService (Login / Refresh /
// Logout / Revoke*). The Module exists only to own the repository.
package session

import (
	"database/sql"
	"fmt"
	"log/slog"

	"sso/internal/modules/session/internal/mariadb"
)

// Deps lists everything session needs from its host.
type Deps struct {
	DB  *sql.DB
	Log *slog.Logger
}

// Module is the assembled session bounded context.
type Module struct {
	repo *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("session: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("session: log is required")
	}

	repo := mariadb.NewRepository(d.DB)

	var _ Repository = repo

	return &Module{repo: repo}, nil
}

// Repository returns the persistence contract.
func (m *Module) Repository() Repository { return m.repo }
