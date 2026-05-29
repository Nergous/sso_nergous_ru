package mariadb

import (
	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/app/internal/mariadb/dbgen"
	"sso/internal/kernel/etag"
)

// dbgenToDomain hydrates a domain.App from a freshly-scanned sqlc row.
// Trusted-row path (no validation) via RestoreApp.
func dbgenToDomain(a dbgen.App) *domain.App {
	return domain.RestoreApp(domain.RestoreAppParams{
		ID:        domain.AppID(a.ID),
		Name:      a.Name,
		Slug:      a.Slug,
		Link:      a.Link,
		Status:    domain.AppStatus(a.Status),
		Etag:      etag.Etag(a.Etag),
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	})
}

func toCreateParams(a *domain.App) dbgen.CreateAppParams {
	return dbgen.CreateAppParams{
		ID:        a.ID().String(),
		Name:      a.Name,
		Slug:      a.Slug(),
		Link:      a.Link,
		Status:    uint8(a.Status()),
		Etag:      a.Etag().String(),
		CreatedAt: a.CreatedAt(),
		UpdatedAt: a.UpdatedAt(),
	}
}

func toUpdateParams(a *domain.App) dbgen.UpdateAppParams {
	return dbgen.UpdateAppParams{
		Name:      a.Name,
		Link:      a.Link,
		Status:    uint8(a.Status()),
		Etag:      a.Etag().String(),
		UpdatedAt: a.UpdatedAt(),
		ID:        a.ID().String(),
	}
}

// toUpdateWithEtagParams — Etag_2 is sqlc's positional name for the second
// occurrence of `etag = ?` (in the WHERE clause).
func toUpdateWithEtagParams(a *domain.App, expectedEtag etag.Etag) dbgen.UpdateAppWithEtagParams {
	return dbgen.UpdateAppWithEtagParams{
		Name:      a.Name,
		Link:      a.Link,
		Status:    uint8(a.Status()),
		Etag:      a.Etag().String(),
		UpdatedAt: a.UpdatedAt(),
		ID:        a.ID().String(),
		Etag_2:    expectedEtag.String(),
	}
}
