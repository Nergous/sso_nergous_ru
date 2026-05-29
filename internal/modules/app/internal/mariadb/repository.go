// Package app is the MariaDB implementation of
// internal/domain/app.Repository. Same shape as the identity adapter.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/app/internal/mariadb/dbgen"
	"sso/internal/kernel/dbutil"
	"sso/internal/kernel/etag"
)

type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

// Compile-time check.
var _ domain.Repository = (*Repository)(nil)

// ----------------------------------------------------------------------------
// Create
// ----------------------------------------------------------------------------

func (r *Repository) Create(ctx context.Context, a *domain.App) error {
	if err := r.q.CreateApp(ctx, toCreateParams(a)); err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrAppAlreadyExists
		}
		return fmt.Errorf("app repo: create: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// GetByID
// ----------------------------------------------------------------------------

func (r *Repository) GetByID(ctx context.Context, id domain.AppID) (*domain.App, error) {
	row, err := r.q.GetAppByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAppNotFound
		}
		return nil, fmt.Errorf("app repo: get_by_id: %w", err)
	}
	return dbgenToDomain(row), nil
}

// ----------------------------------------------------------------------------
// Update
// ----------------------------------------------------------------------------

func (r *Repository) Update(ctx context.Context, a *domain.App, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.UpdateApp(ctx, toUpdateParams(a))
	} else {
		res, err = r.q.UpdateAppWithEtag(ctx, toUpdateWithEtagParams(a, expectedEtag))
	}
	if err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrAppAlreadyExists
		}
		return fmt.Errorf("app repo: update: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("app repo: update: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountAppByID(ctx, a.ID().String())
		},
		domain.ErrAppNotFound, domain.ErrEtagMismatch)
}

// ----------------------------------------------------------------------------
// Delete
// ----------------------------------------------------------------------------

func (r *Repository) Delete(ctx context.Context, id domain.AppID, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.DeleteApp(ctx, id.String())
	} else {
		res, err = r.q.DeleteAppWithEtag(ctx, dbgen.DeleteAppWithEtagParams{
			ID:   id.String(),
			Etag: expectedEtag.String(),
		})
	}
	if err != nil {
		return fmt.Errorf("app repo: delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("app repo: delete: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountAppByID(ctx, id.String())
		},
		domain.ErrAppNotFound, domain.ErrEtagMismatch)
}
