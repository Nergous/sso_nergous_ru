package mariadb

import (
	"database/sql"
	"time"

	"sso/internal/identity/internal/domain"
	"sso/internal/identity/internal/mariadb/dbgen"
	"sso/internal/kernel/etag"
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

	return domain.RestoreUser(domain.RestoreUserParams{
		ID:           domain.UserID(u.ID),
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: passwordHash,
		AvatarURL:    avatar,
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       domain.UserStatus(u.Status),
		Etag:         etag.Etag(u.Etag),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		LastLoginAt:  lastLogin,
	})
}

// toCreateParams flattens a domain.User into the sqlc CreateUser arg shape.
func toCreateParams(u *domain.User) dbgen.CreateUserParams {
	return dbgen.CreateUserParams{
		ID:           u.ID().String(),
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: passwordHashToDB(u.PasswordHash()),
		AvatarUrl:    avatarToDB(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		CreatedAt:    u.CreatedAt(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  lastLoginToDB(u.LastLoginAt),
	}
}

// toUpdateParams flattens a domain.User into the sqlc UpdateUser arg shape
// (no etag in WHERE — unconditional update).
func toUpdateParams(u *domain.User) dbgen.UpdateUserParams {
	return dbgen.UpdateUserParams{
		Email:        u.Email,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: passwordHashToDB(u.PasswordHash()),
		AvatarUrl:    avatarToDB(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  lastLoginToDB(u.LastLoginAt),
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
		PasswordHash: passwordHashToDB(u.PasswordHash()),
		AvatarUrl:    avatarToDB(u.AvatarURL),
		Locale:       u.Locale,
		Timezone:     u.Timezone,
		Status:       uint8(u.Status()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		LastLoginAt:  lastLoginToDB(u.LastLoginAt),
		ID:           u.ID().String(),
		Etag_2:       expectedEtag.String(),
	}
}

// avatarToDB maps the empty domain value to SQL NULL — canonical "absent"
// for a proto3 optional field.
func avatarToDB(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// lastLoginToDB lifts the never-logged-in zero time to SQL NULL.
func lastLoginToDB(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// passwordHashToDB lifts an empty/nil hash to SQL NULL — a user with no
// password set on file. sqlc maps the nullable VARBINARY column to
// sql.NullString; the bcrypt output (`$2a$12$...`) is plain ASCII so the
// string round-trip is byte-exact.
func passwordHashToDB(h []byte) sql.NullString {
	if len(h) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(h), Valid: true}
}

// toUpdatePasswordWithEtagParams flattens a domain.User + expected etag
// into the dedicated UpdateUserPasswordWithEtag arg shape. Used by the
// auth use-case for ChangePassword / ResetPasswordWithRecoveryCode —
// touches only password_hash + etag + updated_at, leaving the rest of
// the row alone.
func toUpdatePasswordWithEtagParams(u *domain.User, expectedEtag etag.Etag) dbgen.UpdateUserPasswordWithEtagParams {
	return dbgen.UpdateUserPasswordWithEtagParams{
		PasswordHash: passwordHashToDB(u.PasswordHash()),
		Etag:         u.Etag().String(),
		UpdatedAt:    u.UpdatedAt(),
		ID:           u.ID().String(),
		Etag_2:       expectedEtag.String(),
	}
}

func toUpdateLastLoginAtParams(id domain.UserID, now time.Time) dbgen.UpdateUserLastLoginAtParams {
	return dbgen.UpdateUserLastLoginAtParams{
		LastLoginAt: lastLoginToDB(now),
		ID:          id.String(),
	}
}
