package mariadb

import (
	"database/sql"
	"time"

	domain "sso/internal/modules/session/internal/domain"
	"sso/internal/modules/session/internal/mariadb/dbgen"
)

func dbgenToDomain(s dbgen.Session) *domain.Session {
	var userAgent string
	if s.UserAgent.Valid {
		userAgent = s.UserAgent.String
	}

	var ipAddress string
	if s.IpAddress.Valid {
		ipAddress = s.IpAddress.String
	}

	var revokedAt time.Time
	if s.RevokedAt.Valid {
		revokedAt = s.RevokedAt.Time
	}
	return domain.RestoreSession(domain.RestoreSessionParams{
		ID:                    domain.SessionID(s.ID),
		UserID:                domain.UserID(s.UserID),
		RefreshTokenHash:      s.RefreshTokenHash,
		UserAgent:             userAgent,
		IpAddress:             ipAddress,
		IssuedAt:              s.IssuedAt,
		ExpiresAt:             s.ExpiresAt,
		RefreshTokenExpiresAt: s.RefreshTokenExpiresAt,
		LastSeenAt:            s.LastSeenAt,
		RevokedAt:             revokedAt,
	})
}

func toCreateParams(s *domain.Session) dbgen.CreateSessionParams {
	return dbgen.CreateSessionParams{
		ID:                    s.ID().String(),
		UserID:                s.UserID().String(),
		RefreshTokenHash:      s.RefreshTokenHash(),
		UserAgent:             nullableString(s.UserAgent),
		IpAddress:             nullableString(s.IpAddress),
		IssuedAt:              s.IssuedAt(),
		ExpiresAt:             s.ExpiresAt(),
		RefreshTokenExpiresAt: s.RefreshTokenExpiresAt(),
		LastSeenAt:            s.LastSeenAt(),
		RevokedAt:             revokedAtToDB(s.RevokedAt()),
	}
}

func toUpdateParams(s *domain.Session) dbgen.UpdateSessionParams {
	return dbgen.UpdateSessionParams{
		ID:                    s.ID().String(),
		UserID:                s.UserID().String(),
		RefreshTokenHash:      s.RefreshTokenHash(),
		UserAgent:             nullableString(s.UserAgent),
		IpAddress:             nullableString(s.IpAddress),
		IssuedAt:              s.IssuedAt(),
		ExpiresAt:             s.ExpiresAt(),
		RefreshTokenExpiresAt: s.RefreshTokenExpiresAt(),
		LastSeenAt:            s.LastSeenAt(),
		RevokedAt:             revokedAtToDB(s.RevokedAt()),
	}
}

func toRotateParams(s *domain.Session, expectedRefreshHash []byte) dbgen.RotateSessionRefreshParams {
	return dbgen.RotateSessionRefreshParams{
		ID:                    s.ID().String(),
		RefreshTokenHash:      s.RefreshTokenHash(),
		RefreshTokenExpiresAt: s.RefreshTokenExpiresAt(),
		RefreshTokenHash_2:    expectedRefreshHash,
		LastSeenAt:            s.LastSeenAt(),
	}
}

func toRevokeParams(userID domain.UserID, now time.Time) dbgen.RevokeAllSessionsForUserParams {
	return dbgen.RevokeAllSessionsForUserParams{
		RevokedAt: revokedAtToDB(now),
		UserID:    userID.String(),
	}
}

// nullableString folds the use-case's empty-string convention onto SQL
// NULL: domain treats "" as "absent" for UserAgent / IpAddress (the
// columns are nullable in the schema, so we keep that distinction at the
// persistence boundary).
func nullableString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func revokedAtToDB(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
