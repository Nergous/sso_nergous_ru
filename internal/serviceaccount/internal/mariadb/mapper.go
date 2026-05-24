package mariadb

import (
	"database/sql"
	"time"

	domain "sso/internal/serviceaccount/internal/domain"
	"sso/internal/kernel/etag"
	"sso/internal/serviceaccount/internal/mariadb/dbgen"
)

func dbgenToDomain(r dbgen.ServiceAccount) *domain.ServiceAccount {
	var lastAuth time.Time
	if r.LastAuthenticatedAt.Valid {
		lastAuth = r.LastAuthenticatedAt.Time
	}
	return domain.RestoreServiceAccount(domain.RestoreServiceAccountParams{
		ID:                  domain.ServiceAccountID(r.ID),
		Name:                r.Name,
		Description:         r.Description,
		SecretHash:          r.ClientSecretHash,
		Status:              domain.ServiceAccountStatus(r.Status),
		Etag:                etag.Etag(r.Etag),
		CreatedAt:           r.CreatedAt,
		UpdatedAt:           r.UpdatedAt,
		LastAuthenticatedAt: lastAuth,
	})
}

func toCreateParams(s *domain.ServiceAccount) dbgen.CreateServiceAccountParams {
	return dbgen.CreateServiceAccountParams{
		ID:                  s.ID().String(),
		Name:                s.Name,
		Description:         s.Description,
		ClientSecretHash:    s.SecretHash(),
		Status:              uint8(s.Status()),
		Etag:                s.Etag().String(),
		CreatedAt:           s.CreatedAt(),
		UpdatedAt:           s.UpdatedAt(),
		LastAuthenticatedAt: lastAuthToDB(s.LastAuthenticatedAt),
	}
}

func toUpdateParams(s *domain.ServiceAccount) dbgen.UpdateServiceAccountParams {
	return dbgen.UpdateServiceAccountParams{
		Name:                s.Name,
		Description:         s.Description,
		ClientSecretHash:    s.SecretHash(),
		Status:              uint8(s.Status()),
		Etag:                s.Etag().String(),
		UpdatedAt:           s.UpdatedAt(),
		LastAuthenticatedAt: lastAuthToDB(s.LastAuthenticatedAt),
		ID:                  s.ID().String(),
	}
}

func toUpdateWithEtagParams(s *domain.ServiceAccount, expectedEtag etag.Etag) dbgen.UpdateServiceAccountWithEtagParams {
	return dbgen.UpdateServiceAccountWithEtagParams{
		Name:                s.Name,
		Description:         s.Description,
		ClientSecretHash:    s.SecretHash(),
		Status:              uint8(s.Status()),
		Etag:                s.Etag().String(),
		UpdatedAt:           s.UpdatedAt(),
		LastAuthenticatedAt: lastAuthToDB(s.LastAuthenticatedAt),
		ID:                  s.ID().String(),
		Etag_2:              expectedEtag.String(),
	}
}

// lastAuthToDB maps the zero time.Time to SQL NULL — service accounts
// that have never authenticated keep last_authenticated_at = NULL on disk.
func lastAuthToDB(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}
