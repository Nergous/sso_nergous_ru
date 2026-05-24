// Package access is the MariaDB implementation of
// internal/domain/access.Repository.
//
// Single-row mutations go through sqlc; bulk operations and the
// hand-written ListUserRoles live alongside in list.go (sqlc cannot
// template variable WHERE / ORDER BY economically).
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sso/internal/access/internal/domain"
	"sso/internal/access/internal/mariadb/dbgen"
	"sso/internal/kernel/dbutil"
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
// Create / Get / Delete
// ----------------------------------------------------------------------------

func (r *Repository) Create(ctx context.Context, a *domain.RoleAssignment) (bool, error) {
	err := r.q.CreateRoleAssignment(ctx, toCreateParams(a))
	if err == nil {
		return true, nil
	}
	if dbutil.IsDuplicateEntry(err) {
		// Idempotent: row already exists. Caller can decide whether to
		// fetch the existing record (BulkGrantRoles needs the original
		// granted_at to populate the response).
		return false, nil
	}
	return false, fmt.Errorf("access repo: create: %w", err)
}

func (r *Repository) Get(ctx context.Context, userID domain.UserID, roleID domain.RoleID) (*domain.RoleAssignment, error) {
	row, err := r.q.GetRoleAssignment(ctx, dbgen.GetRoleAssignmentParams{
		UserID: userID.String(),
		RoleID: roleID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("access repo: get: %w", err)
	}
	return dbgenToDomain(row), nil
}

func (r *Repository) Delete(ctx context.Context, userID domain.UserID, roleID domain.RoleID) (bool, error) {
	res, err := r.q.DeleteRoleAssignment(ctx, dbgen.DeleteRoleAssignmentParams{
		UserID: userID.String(),
		RoleID: roleID.String(),
	})
	if err != nil {
		return false, fmt.Errorf("access repo: delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("access repo: delete: rows_affected: %w", err)
	}
	return rows == 1, nil
}

// ----------------------------------------------------------------------------
// Bulk
// ----------------------------------------------------------------------------

// BulkCreate runs every insert inside a single transaction. Each insert
// is allowed to "succeed-as-no-op" on duplicate-key (idempotent grant);
// any other failure rolls the whole batch back, matching the proto
// "all-or-nothing on validation failure" guarantee.
func (r *Repository) BulkCreate(ctx context.Context, assignments []*domain.RoleAssignment) ([]bool, error) {
	mask := make([]bool, len(assignments))
	err := dbutil.InTx(ctx, r.db, func(tx *sql.Tx) error {
		q := dbgen.New(tx)
		for i, a := range assignments {
			err := q.CreateRoleAssignment(ctx, toCreateParams(a))
			switch {
			case err == nil:
				mask[i] = true
			case dbutil.IsDuplicateEntry(err):
				mask[i] = false
			default:
				return fmt.Errorf("access repo: bulk_create[%d]: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return mask, nil
}

// BulkDelete removes the (userID × roleIDs) pairs atomically. Missing
// pairs are silently ignored (idempotent remove) — only DB errors abort.
func (r *Repository) BulkDelete(ctx context.Context, userID domain.UserID, roleIDs []domain.RoleID) error {
	return dbutil.InTx(ctx, r.db, func(tx *sql.Tx) error {
		q := dbgen.New(tx)
		for _, rid := range roleIDs {
			if _, err := q.DeleteRoleAssignment(ctx, dbgen.DeleteRoleAssignmentParams{
				UserID: userID.String(),
				RoleID: rid.String(),
			}); err != nil {
				return fmt.Errorf("access repo: bulk_delete: %w", err)
			}
		}
		return nil
	})
}

// ----------------------------------------------------------------------------
// ListActivePermissions
// ----------------------------------------------------------------------------
//
// Status filter is parameterised — repository takes the active-status
// uint8 from the use-case (which knows the role.RoleStatus constant).
// Keeps access persistence package free of the role package import.

const activeRoleStatus uint8 = 1 // mirrors role.RoleStatusActive

func (r *Repository) ListActivePermissions(ctx context.Context, userID domain.UserID, appID domain.AppID) ([]domain.PermissionRow, error) {
	rows, err := r.q.ListActivePermissionsByUserApp(ctx, dbgen.ListActivePermissionsByUserAppParams{
		UserID: userID.String(),
		AppID:  appID.String(),
		Status: activeRoleStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("access repo: list_active_permissions: %w", err)
	}
	out := make([]domain.PermissionRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.PermissionRow{
			RoleID:     domain.RoleID(row.RoleID),
			Permission: row.Permission,
		})
	}
	return out, nil
}
