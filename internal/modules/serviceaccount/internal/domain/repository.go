package domain

import (
	"context"
	"sso/internal/kernel/etag"
	"time"
)

type ListOrderBy uint8

// Numbering mirrors sso.serviceaccount.v1.ListServiceAccountsOrderBy
// minus UNSPECIFIED. Don't reorder without checking the proto enum.
const (
	OrderByUnspecified             ListOrderBy = 0
	OrderByCreatedAtDesc           ListOrderBy = 1
	OrderByCreatedAtAsc            ListOrderBy = 2
	OrderByServiceAccountIDDesc    ListOrderBy = 3
	OrderByServiceAccountIDAsc     ListOrderBy = 4
	OrderByNameDesc                ListOrderBy = 5
	OrderByNameAsc                 ListOrderBy = 6
	OrderByLastAuthenticatedAtDesc ListOrderBy = 7
	OrderByLastAuthenticatedAtAsc  ListOrderBy = 8
)

type PageCursor struct {
	CreatedAt        time.Time
	ServiceAccountID ServiceAccountID
	Name             string
}

type ListQuery struct {
	PageSize int
	After    *PageCursor
	Search   string
	Statuses []ServiceAccountStatus
	OrderBy  ListOrderBy
}

type ListResult struct {
	ServiceAccounts []*ServiceAccount
	NextCursor      *PageCursor
	TotalSize       *int
}

type Repository interface {
	Create(ctx context.Context, f *ServiceAccount) error
	GetByID(ctx context.Context, id ServiceAccountID) (*ServiceAccount, error)
	List(ctx context.Context, q ListQuery) (ListResult, error)
	Update(ctx context.Context, f *ServiceAccount, expectedEtag etag.Etag) error
	Delete(ctx context.Context, id ServiceAccountID, expectedEtag etag.Etag) error
}
