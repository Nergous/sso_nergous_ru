// Package recoverycode exposes the wire-up for the recoverycode
// bounded context.
//
//	mod.Repository()    persistence contract, consumed by auth
//
// There is no Service or gRPC handler here — recovery-code operations
// are surfaced through AuthService (GenerateRecoveryCodes /
// ResetPasswordWithRecoveryCode). The Module exists only to own the
// repository.
package recoverycode

import (
	"database/sql"
	"fmt"
	"log/slog"

	"sso/internal/recoverycode/internal/mariadb"
)

// Deps lists everything recoverycode needs from its host.
type Deps struct {
	DB  *sql.DB
	Log *slog.Logger
}

// Module is the assembled recoverycode bounded context.
type Module struct {
	repo *mariadb.Repository
}

// New wires the module from its dependencies.
func New(d Deps) (*Module, error) {
	if d.DB == nil {
		return nil, fmt.Errorf("recoverycode: db is required")
	}
	if d.Log == nil {
		return nil, fmt.Errorf("recoverycode: log is required")
	}

	repo := mariadb.NewRepository(d.DB)

	var _ Repository = repo

	return &Module{repo: repo}, nil
}

// Repository returns the persistence contract.
func (m *Module) Repository() Repository { return m.repo }
