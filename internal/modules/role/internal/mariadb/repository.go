// Package role is the MariaDB implementation of
// internal/domain/role.Repository.
//
// One wrinkle compared to identity / app: the role aggregate carries a
// permission set stored in a separate role_permissions table, so writes
// (Create / Update) wrap the row INSERT/UPDATE and the permission rows
// in a single transaction. Reads (GetByID) issue two queries — the row
// plus the permission list — and the mapper assembles the aggregate.
//
// Dynamic ListRoles lives in the sibling list.go file (sqlc cannot
// template variable WHERE / ORDER BY economically).
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"sso/internal/kernel/dbutil"
	"sso/internal/kernel/etag"
	"sso/internal/modules/role/internal/domain"
	"sso/internal/modules/role/internal/mariadb/dbgen"
)

// Repository persists role aggregates in MariaDB.
type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

// NewRepository wires a Repository to an existing *sql.DB. The pool is
// owned upstream; the repository never closes it.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

// Compile-time check.
var _ domain.Repository = (*Repository)(nil)

// ----------------------------------------------------------------------------
// Create
// ----------------------------------------------------------------------------
//
// Atomic: the role row and all permission rows go in a single transaction,
// so a failure on any permission insert rolls back the role row too —
// no orphan roles.

func (r *Repository) Create(ctx context.Context, role *domain.Role) error {
	return r.inTx(ctx, func(q *dbgen.Queries) error {
		if err := q.CreateRole(ctx, toCreateParams(role)); err != nil {
			if dbutil.IsDuplicateEntry(err) {
				return domain.ErrRoleAlreadyExists
			}
			return fmt.Errorf("role repo: create: %w", err)
		}
		for _, p := range role.Permissions() {
			if err := q.InsertRolePermission(ctx, toInsertPermissionParams(role.ID(), p)); err != nil {
				return fmt.Errorf("role repo: create: insert permission: %w", err)
			}
		}
		return nil
	})
}

// ----------------------------------------------------------------------------
// GetByID
// ----------------------------------------------------------------------------

func (r *Repository) GetByID(ctx context.Context, id domain.RoleID) (*domain.Role, error) {
	row, err := r.q.GetRoleByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, fmt.Errorf("role repo: get_by_id: %w", err)
	}

	// GetRolePermissions is :many — an empty result is (empty slice, nil),
	// not sql.ErrNoRows. A role with zero permissions is valid; we just
	// hand back an empty slice.
	perms, err := r.q.GetRolePermissions(ctx, id.String())
	if err != nil {
		return nil, fmt.Errorf("role repo: get_by_id: permissions: %w", err)
	}
	return dbgenToDomain(row, perms), nil
}

// ----------------------------------------------------------------------------
// Update
// ----------------------------------------------------------------------------
//
// Atomic: row update + (optional) permission rewrite happen in one tx.
//
// To avoid pointless DELETE+INSERT churn on lifecycle transitions
// (Disable/Enable do not touch permissions), we read the current set
// inside the same tx and only rewrite if it differs from
// role.Permissions(). Both sides are sorted (the DB query has
// ORDER BY permission, domain canonicalises in NewRole/ApplyPatch),
// so the comparison is a direct slices.Equal.

func (r *Repository) Update(ctx context.Context, role *domain.Role, expectedEtag etag.Etag) error {
	return r.inTx(ctx, func(q *dbgen.Queries) error {
		var (
			res sql.Result
			err error
		)
		if expectedEtag == "" {
			res, err = q.UpdateRole(ctx, toUpdateParams(role))
		} else {
			res, err = q.UpdateRoleWithEtag(ctx, toUpdateWithEtagParams(role, expectedEtag))
		}
		if err != nil {
			if dbutil.IsDuplicateEntry(err) {
				return domain.ErrRoleAlreadyExists
			}
			return fmt.Errorf("role repo: update: %w", err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("role repo: update: rows_affected: %w", err)
		}
		if rows != 1 {
			return dbutil.Discriminate(ctx, expectedEtag,
				func(ctx context.Context) (int64, error) {
					return q.CountRoleByID(ctx, role.ID().String())
				},
				domain.ErrRoleNotFound, domain.ErrEtagMismatch)
		}

		existing, err := q.GetRolePermissions(ctx, role.ID().String())
		if err != nil {
			return fmt.Errorf("role repo: update: load permissions: %w", err)
		}
		desired := role.Permissions()
		if slices.Equal(existing, desired) {
			return nil
		}
		if err := q.DeleteRolePermissions(ctx, role.ID().String()); err != nil {
			return fmt.Errorf("role repo: update: clear permissions: %w", err)
		}
		for _, p := range desired {
			if err := q.InsertRolePermission(ctx, toInsertPermissionParams(role.ID(), p)); err != nil {
				return fmt.Errorf("role repo: update: insert permission: %w", err)
			}
		}
		return nil
	})
}

// ----------------------------------------------------------------------------
// Delete
// ----------------------------------------------------------------------------
//
// role_permissions has FOREIGN KEY ... ON DELETE CASCADE on role_id (in
// the schema), so deleting the parent row clears the join table for free.
// No transaction needed — single statement.

func (r *Repository) Delete(ctx context.Context, id domain.RoleID, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.DeleteRole(ctx, id.String())
	} else {
		res, err = r.q.DeleteRoleWithEtag(ctx, dbgen.DeleteRoleWithEtagParams{
			ID:   id.String(),
			Etag: expectedEtag.String(),
		})
	}
	if err != nil {
		return fmt.Errorf("role repo: delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("role repo: delete: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountRoleByID(ctx, id.String())
		},
		domain.ErrRoleNotFound, domain.ErrEtagMismatch)
}

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

// inTx is a thin wrapper over dbutil.InTx that gives fn a tx-bound
// *dbgen.Queries instead of the raw *sql.Tx — keeps the per-method call
// sites in this file readable.
func (r *Repository) inTx(ctx context.Context, fn func(*dbgen.Queries) error) error {
	return dbutil.InTx(ctx, r.db, func(tx *sql.Tx) error {
		return fn(dbgen.New(tx))
	})
}
