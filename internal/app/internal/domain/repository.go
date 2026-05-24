package domain

import (
	"context"
	"sso/internal/kernel/etag"
	"time"
)

// ListOrderBy mirrors sso.app.v1.ListAppsOrderBy. Domain-level enum so
// the repository never imports proto stubs.
type ListOrderBy uint8

const (
	OrderByUnspecified   ListOrderBy = 0 // server picks default (CreatedAtDesc)
	OrderByCreatedAtDesc ListOrderBy = 1
	OrderByCreatedAtAsc  ListOrderBy = 2
	OrderByAppIDDesc     ListOrderBy = 3
	OrderByAppIDAsc      ListOrderBy = 4
	OrderByNameDesc      ListOrderBy = 5
	OrderByNameAsc       ListOrderBy = 6
)

// PageCursor is a typed keyset cursor. Application layer encodes/decodes
// the opaque token; repository deals only with the typed form.
//
// Fields populated depend on OrderBy: CreatedAt/AppID for time-based
// orders, Name/AppID for name orders, AppID alone for id-only orders.
type PageCursor struct {
	CreatedAt time.Time
	AppID     AppID
	Name      string
}

// ListQuery is the input to Repository.List. An empty Statuses slice
// applies no status filter (default behaviour mirrors the proto:
// list everything matching other filters).
type ListQuery struct {
	PageSize int
	After    *PageCursor
	Search   string
	Statuses []AppStatus
	OrderBy  ListOrderBy
}

type ListResult struct {
	Apps       []*App
	NextCursor *PageCursor
	TotalSize  *int
}

// Repository abstracts persistence of App aggregates.
//
// Error contract:
//
//	Create  → ErrAppAlreadyExists (name or slug collision)
//	GetByID → ErrAppNotFound
//	Update  → ErrEtagMismatch (when expectedEtag != "" and stored etag differs)
//	          ErrAppNotFound (no row at all)
//	          ErrAppAlreadyExists (uniqueness collision on patched name)
//	Delete  → ErrEtagMismatch / ErrAppNotFound (same semantics as Update)
//
// expectedEtag conventions:
//
//	"" — unconditional (matches the wire-level "*" wildcard).
//	     Repository skips the etag comparison.
//	*  — normal optimistic-lock check.
type Repository interface {
	Create(ctx context.Context, a *App) error
	GetByID(ctx context.Context, id AppID) (*App, error)
	List(ctx context.Context, q ListQuery) (ListResult, error)
	Update(ctx context.Context, a *App, expectedEtag etag.Etag) error
	Delete(ctx context.Context, id AppID, expectedEtag etag.Etag) error
}
