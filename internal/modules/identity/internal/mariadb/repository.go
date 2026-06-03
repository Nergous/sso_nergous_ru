// Package mariadb is the MariaDB implementation of the identity module's
// domain.Repository.
//
// Static per-row queries (Create/GetByID/Update/Delete + the existence check
// used to discriminate ErrUserNotFound vs ErrEtagMismatch) go through sqlc-
// generated code in dbgen. The dynamic Repository.List query — variable
// WHERE / ORDER BY / keyset cursor — stays hand-written in list.go, using
// the same *sql.DB that backs the sqlc Queries.
package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"sso/internal/kernel/dbutil"
	"sso/internal/kernel/etag"
	"sso/internal/modules/identity/internal/domain"
	"sso/internal/modules/identity/internal/mariadb/dbgen"
)

// Repository persists identity aggregates in MariaDB.
type Repository struct {
	db *sql.DB
	q  *dbgen.Queries
}

// NewRepository wires a Repository to an existing *sql.DB. The pool is owned
// upstream (bootstrap); the repository never closes it.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: dbgen.New(db)}
}

// Compile-time check.
var _ domain.Repository = (*Repository)(nil)

// ----------------------------------------------------------------------------
// Create
// ----------------------------------------------------------------------------

func (r *Repository) Create(ctx context.Context, u *domain.User) error {
	if err := r.q.CreateUser(ctx, toCreateParams(u)); err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("identity repo: create: %w", err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// GetByID
// ----------------------------------------------------------------------------

func (r *Repository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("identity repo: get_by_id: %w", err)
	}
	return dbgenToDomain(row), nil
}

// ----------------------------------------------------------------------------
// GetByEmail / GetByUsername
// ----------------------------------------------------------------------------
//
// These two are not on the domain.Repository interface — they exist for
// the auth use-case (Login resolves a credential by its email or
// username). Auth declares its own narrow interface at the point of use
// and the concrete *Repository here satisfies it via duck-typing.

// GetByEmail returns the user with the given email. Email lookups are
// unique (uk_users_email).
func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("identity repo: get_by_email: %w", err)
	}
	return dbgenToDomain(row), nil
}

// GetByUsername returns the user with the given username. Username
// lookups are unique (uk_users_username).
func (r *Repository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("identity repo: get_by_username: %w", err)
	}
	return dbgenToDomain(row), nil
}

func (r *Repository) GetFailedLoginAttempts(ctx context.Context, id domain.UserID) (int, error) {
	count, err := r.q.GetFailedLoginAttempts(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domain.ErrUserNotFound
		}
		return 0, fmt.Errorf("identity repo: get_failed_login_attempts: %w", err)
	}
	return int(count), nil
}

// ----------------------------------------------------------------------------
// Update
// ----------------------------------------------------------------------------

func (r *Repository) Update(ctx context.Context, u *domain.User, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.UpdateUser(ctx, toUpdateParams(u))
	} else {
		res, err = r.q.UpdateUserWithEtag(ctx, toUpdateWithEtagParams(u, expectedEtag))
	}
	if err != nil {
		if dbutil.IsDuplicateEntry(err) {
			return domain.ErrUserAlreadyExists
		}
		return fmt.Errorf("identity repo: update: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: update: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountUserByID(ctx, u.ID().String())
		},
		domain.ErrUserNotFound, domain.ErrEtagMismatch)
}

// ----------------------------------------------------------------------------
// UpdatePassword
// ----------------------------------------------------------------------------
//
// Dedicated narrow write-path for password rotation (ChangePassword,
// ResetPasswordWithRecoveryCode). Uses the
// UpdateUserPasswordWithEtag SQL: only password_hash + etag +
// updated_at change, the rest of the row is left untouched.
//
// expectedEtag is the etag observed BEFORE the caller invoked
// SetPassword on the aggregate (i.e. the on-disk etag the use-case is
// racing against). The aggregate's CURRENT etag (post-SetPassword) is
// what gets persisted.
//
// Wildcard ("") is intentionally NOT supported here — password changes
// must be a deliberate, optimistic-locked operation; the auth
// use-cases always have a concrete etag in hand.
func (r *Repository) UpdatePassword(ctx context.Context, u *domain.User, expectedEtag etag.Etag) error {
	if expectedEtag == "" {
		return fmt.Errorf("identity repo: update_password: expected etag is required")
	}
	res, err := r.q.UpdateUserPasswordWithEtag(ctx, toUpdatePasswordWithEtagParams(u, expectedEtag))
	if err != nil {
		return fmt.Errorf("identity repo: update_password: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: update_password: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountUserByID(ctx, u.ID().String())
		},
		domain.ErrUserNotFound, domain.ErrEtagMismatch)
}

func (r *Repository) UpdateLastLoginAt(ctx context.Context, id domain.UserID, now time.Time) error {
	res, err := r.q.UpdateUserLastLoginAt(ctx, toUpdateLastLoginAtParams(id, now))
	if err != nil {
		return fmt.Errorf("identity repo: update_last_login: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: update_last_login: rows_affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *Repository) IncrementFailedLogins(ctx context.Context, id domain.UserID) error {
	res, err := r.q.IncrementFailedLogins(ctx, id.String())
	if err != nil {
		return fmt.Errorf("identity repo: increment_failed_logins: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: increment_failed_logins: rows_affected: %w", err)
	}
	if rows == 0 {
		return nil
	}
	return nil
}

func (r *Repository) LockUser(ctx context.Context, id domain.UserID, until time.Time) error {
	res, err := r.q.LockUser(ctx, toLockUserParams(id, until))
	if err != nil {
		return fmt.Errorf("identity repo: lock_user: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: lock_user: rows_affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *Repository) ResetLoginFailures(ctx context.Context, id domain.UserID) error {
	res, err := r.q.ResetLoginFailures(ctx, id.String())
	if err != nil {
		return fmt.Errorf("identity repo: reset_login_failures: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: reset_login_failures: rows_affected: %w", err)
	}
	if rows == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// ----------------------------------------------------------------------------
// Delete
// ----------------------------------------------------------------------------

func (r *Repository) Delete(ctx context.Context, id domain.UserID, expectedEtag etag.Etag) error {
	var (
		res sql.Result
		err error
	)
	if expectedEtag == "" {
		res, err = r.q.DeleteUser(ctx, id.String())
	} else {
		res, err = r.q.DeleteUserWithEtag(ctx, dbgen.DeleteUserWithEtagParams{
			ID:   id.String(),
			Etag: expectedEtag.String(),
		})
	}
	if err != nil {
		return fmt.Errorf("identity repo: delete: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("identity repo: delete: rows_affected: %w", err)
	}
	if rows == 1 {
		return nil
	}
	return dbutil.Discriminate(ctx, expectedEtag,
		func(ctx context.Context) (int64, error) {
			return r.q.CountUserByID(ctx, id.String())
		},
		domain.ErrUserNotFound, domain.ErrEtagMismatch)
}
