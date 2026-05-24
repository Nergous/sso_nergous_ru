package domain

import (
	"context"
	"sso/internal/app"
	"sso/internal/kernel/etag"
	"time"
)

type ListOrderBy uint8

const (
	OrderByUnspecified   ListOrderBy = 0
	OrderByCreatedAtDesc ListOrderBy = 1
	OrderByCreatedAtAsc  ListOrderBy = 2
	OrderByRoleIDDesc    ListOrderBy = 3
	OrderByRoleIDAsc     ListOrderBy = 4
	OrderByNameDesc      ListOrderBy = 5
	OrderByNameAsc       ListOrderBy = 6
)

type PageCursor struct {
	CreatedAt time.Time
	RoleID    RoleID
	Name      string
}

type ListQuery struct {
	// AppID scopes the listing to a single app. Required — roles are
	// always listed per app per the proto contract.
	AppID    app.AppID
	PageSize int
	After    *PageCursor
	Search   string
	Statuses []RoleStatus
	OrderBy  ListOrderBy
}

type ListResult struct {
	Roles      []*Role
	NextCursor *PageCursor
	TotalSize  *int
}

type Repository interface {
	Create(ctx context.Context, r *Role) error
	GetByID(ctx context.Context, id RoleID) (*Role, error)
	List(ctx context.Context, q ListQuery) (ListResult, error)
	Update(ctx context.Context, r *Role, expectedEtag etag.Etag) error
	Delete(ctx context.Context, id RoleID, expectedEtag etag.Etag) error
}
