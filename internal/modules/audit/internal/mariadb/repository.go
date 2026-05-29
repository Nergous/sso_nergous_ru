// Package audit is the MariaDB implementation of
// internal/domain/audit.Repository.
//
// The aggregate is append-only: Create / GetByID / List only — no
// Update / Delete contract. See internal/domain/audit/repository.go for
// the interface, error contract and ordering semantics.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domain "sso/internal/modules/audit/internal/domain"
	"sso/internal/modules/audit/internal/mariadb/dbgen"
)

type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

var _ domain.Repository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, a *domain.Audit) error {
	params, err := toCreateParams(a)
	if err != nil {
		return fmt.Errorf("audit repo: create: %w", err)
	}
	if err := r.q.CreateAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("audit repo: create: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id domain.AuditID) (*domain.Audit, error) {
	row, err := r.q.GetAuditEventByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAuditNotFound
		}
		return nil, fmt.Errorf("audit repo: get_by_id: %w", err)
	}
	a, err := dbgenToDomain(row)
	if err != nil {
		return nil, fmt.Errorf("audit repo: get_by_id: %w", err)
	}
	return a, nil
}
