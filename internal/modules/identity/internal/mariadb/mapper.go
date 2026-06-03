package mariadb

import (
	"time"

	"sso/internal/kernel/dbutil"
	"sso/internal/kernel/etag"
	"sso/internal/modules/identity/internal/domain"
	"sso/internal/modules/identity/internal/mariadb/dbgen"
)

// dbgenToDomain hydrates a domain.User from a freshly-scanned sqlc row.
// The row was produced by code in this package, so it skips re-validation
// via RestoreUser.
func dbgenToDomain(u dbgen.User) *domain.User {
	avatar := ""
	if u.AvatarUrl.Valid {
		avatar = u.AvatarUrl.String
	}
	var lastLogin time.Time
	if u.LastLoginAt.Valid {
		lastLogin = u.LastLoginAt.Time
	}
	var passwordHash []byte
	if u.PasswordHash.Valid {
		passwordHash = []byte(u.PasswordHash.String)
	}

	var lockoutUntil time.Time
	if u.LockoutUntil.Valid {
		lockoutUntil = u.LockoutUntil.Time
	}

	return domain.RestoreUser(domain.RestoreUserParams{
		ID:                  domain.UserID(u.ID),
		Email:               u.Email,
		Username:            u.Username,
		DisplayName:         u.DisplayName,
		PasswordHash:        passwordHash,
		AvatarURL:           avatar,
		Locale:              u.Locale,
		Timezone:            u.Timezone,
		Status:              domain.UserStatus(u.Status),
		Etag:                etag.Etag(u.Etag),
		CreatedAt:           u.CreatedAt,
		UpdatedAt:           u.UpdatedAt,
		LastLoginAt:         lastLogin,
		FailedLoginAttempts: int(u.FailedLoginAttempts),
		LockoutUntil:        lockoutUntil,
	})
}

// toCreateParams flattens a domain.User into the sqlc CreateUser arg shape.
func toCreateParams(u *domain.User) dbgen.CreateUserParams {
	return dbgen.CreateUserParams{
		ID:           u.ID().String(),
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: dbutil.BytesToNullString(u.PasswordHash()),
		AvatarUrl:    dbutil.StringToNullString(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		CreatedAt:    u.CreatedAt(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  dbutil.TimeToNullTime(u.LastLoginAt),
	}
}

// toUpdateParams flattens a domain.User into the sqlc UpdateUser arg shape
// (no etag in WHERE — unconditional update).
func toUpdateParams(u *domain.User) dbgen.UpdateUserParams {
	return dbgen.UpdateUserParams{
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: dbutil.BytesToNullString(u.PasswordHash()),
		AvatarUrl:    dbutil.StringToNullString(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  dbutil.TimeToNullTime(u.LastLoginAt),
		ID:           u.ID().String(),
	}
}

// toUpdateWithEtagParams flattens a domain.User + expected etag into the
// sqlc UpdateUserWithEtag arg shape. Etag_2 is sqlc's positional name for
// the second occurrence of `etag = ?` (in the WHERE clause).
func toUpdateWithEtagParams(u *domain.User, expectedEtag etag.Etag) dbgen.UpdateUserWithEtagParams {
	return dbgen.UpdateUserWithEtagParams{
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: dbutil.BytesToNullString(u.PasswordHash()),
		AvatarUrl:    dbutil.StringToNullString(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  dbutil.TimeToNullTime(u.LastLoginAt),
		ID:           u.ID().String(),
		Etag_2:       expectedEtag.String(),
	}
}

func toLockUserParams(id domain.UserID, until time.Time) dbgen.LockUserParams {
	return dbgen.LockUserParams{
		LockoutUntil: dbutil.TimeToNullTime(until),
		ID:           id.String(),
	}
}

// toUpdatePasswordWithEtagParams flattens a domain.User + expected etag
// into the dedicated UpdateUserPasswordWithEtag arg shape. Used by the
// auth use-case for ChangePassword / ResetPasswordWithRecoveryCode —
// touches only password_hash + etag + updated_at, leaving the rest of
// the row alone.
func toUpdatePasswordWithEtagParams(u *domain.User, expectedEtag etag.Etag) dbgen.UpdateUserPasswordWithEtagParams {
	return dbgen.UpdateUserPasswordWithEtagParams{
		PasswordHash: dbutil.BytesToNullString(u.PasswordHash()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		ID:           u.ID().String(),
		Etag_2:       expectedEtag.String(),
	}
}

func toUpdateLastLoginAtParams(id domain.UserID, now time.Time) dbgen.UpdateUserLastLoginAtParams {
	return dbgen.UpdateUserLastLoginAtParams{
		LastLoginAt: dbutil.TimeToNullTime(now),
		ID:          id.String(),
	}
}
