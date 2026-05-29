// Package serviceAccount is the MariaDB implementation of
// internal/domain/serviceAccount.Repository.
//
// Mirrors the identity / app implementation: per-row CRUD via sqlc,
// dynamic List in list.go, etag-aware Update/Delete with
// discriminateMissingOrMismatch when a 0-rows-affected write needs to
// be classified as ErrServiceAccountNotFound vs ErrEtagMismatch.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domain "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/kernel/dbutil"
	"sso/internal/kernel/etag"
	"sso/internal/modules/serviceaccount/internal/mariadb/dbgen"
)

type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

var _ domain.Repository = (*Repository)(nil)

// ----------------------------------------------------------------------------
// Create
// ----------------------------------------------------------------------------

func (r *Repository) Create(ctx context.Context, sa *domain.ServiceAccount) error {
	if err := r.q.CreateServiceAccount(ctx, toCreateParams(sa)); err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrServiceAccountAlreadyExists
		}
		return fmt.Errorf("service_account repo: create: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// GetByID
// ----------------------------------------------------------------------------

func (r *Repository) GetByID(ctx context.Context, id domain.ServiceAccountID) (*domain.ServiceAccount, error) {
	row, err := r.q.GetServiceAccountById(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrServiceAccountNotFound
		}
		return nil, fmt.Errorf("service_account repo: get_by_id: %w", err)
	}
	return dbgenToDomain(row), nil
}

// ----------------------------------------------------------------------------
// Update
// ----------------------------------------------------------------------------

func (r *Repository) Update(ctx context.Context, sa *domain.ServiceAccount, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.UpdateServiceAccount(ctx, toUpdateParams(sa))
	} else {
		res, err = r.q.UpdateServiceAccountWithEtag(ctx, toUpdateWithEtagParams(sa, expectedEtag))
	}
	if err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrServiceAccountAlreadyExists
		}
		return fmt.Errorf("service_account repo: update: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("service_account repo: update: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountServiceAccountByID(ctx, sa.ID().String())
		},
		domain.ErrServiceAccountNotFound, domain.ErrEtagMismatch)
}

// ----------------------------------------------------------------------------
// Delete
// ----------------------------------------------------------------------------

func (r *Repository) Delete(ctx context.Context, id domain.ServiceAccountID, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.DeleteServiceAccount(ctx, id.String())
	} else {
		res, err = r.q.DeleteServiceAccountWithEtag(ctx, dbgen.DeleteServiceAccountWithEtagParams{
			ID:   id.String(),
			Etag: expectedEtag.String(),
		})
	}
	if err != nil {
		return fmt.Errorf("service_account repo: delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("service_account repo: delete: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountServiceAccountByID(ctx, id.String())
		},
		domain.ErrServiceAccountNotFound, domain.ErrEtagMismatch)
}
