package domain

import (
	"context"
	"sso/internal/kernel/etag"
	"time"
)

// ListOrderBy mirrors sso.identity.v1.ListUsersOrderBy. Domain-level enum so
// the repository never imports proto stubs.
type ListOrderBy uint8

const (
	OrderByUnspecified   ListOrderBy = 0 // server picks default (CreatedAtDesc)
	OrderByCreatedAtDesc ListOrderBy = 1
	OrderByCreatedAtAsc  ListOrderBy = 2
	OrderByUserIDDesc    ListOrderBy = 3
	OrderByUserIDAsc     ListOrderBy = 4
	OrderByUsernameDesc  ListOrderBy = 5
	OrderByUsernameAsc   ListOrderBy = 6
)

// PageCursor is a typed keyset cursor. The application layer is responsible
// for opaque-encoding it (base64 of JSON) before handing back to clients;
// the repository deals only with the typed form.
//
// Fields populated depend on OrderBy: CreatedAt/UserID for time-based orders,
// Username/UserID for username orders, etc. The repository advances "strictly
// after" the cursor in the active order; UserID is always the tie-breaker.
type PageCursor struct {
	CreatedAt time.Time
	UserID    UserID
	Username  string
}

// ListQuery is the input to Repository.List. Filters apply OR within a
// repeated field and AND across different fields. An empty Statuses slice
// excludes USER_STATUS_DELETED by default (proto contract).
type ListQuery struct {
	PageSize     int         // > 0; caller resolves "0 means default" upstream
	After        *PageCursor // nil = first page
	Search       string
	Emails       []string
	Usernames    []string
	DisplayNames []string
	Statuses     []UserStatus
	OrderBy      ListOrderBy
}

// ListResult is the output of Repository.List. NextCursor is nil on the last
// page; TotalSize is nil when the repository chose not to compute it
// (consistent with the optional total_size in the proto response).
type ListResult struct {
	Users      []*User
	NextCursor *PageCursor
	TotalSize  *int
}

// Repository abstracts persistence of User aggregates. Implementations live
// in internal/persistence/<driver>/identity.
//
// Error contract:
//
//	Create  → ErrUserAlreadyExists (email or username collision)
//	GetByID → ErrUserNotFound
//	Update  → ErrEtagMismatch (when expectedEtag != "" and stored etag differs)
//	          ErrUserNotFound (no row at all)
//	          ErrUserAlreadyExists (uniqueness collision on patched fields)
//	Delete  → ErrEtagMismatch / ErrUserNotFound (same semantics as Update)
//
// expectedEtag conventions:
//
//	""  unconditional (matches the AuthService/IdentityService "*" wildcard
//	    on the wire). Repository skips the etag comparison.
//	*   normal optimistic-lock check.
type Repository interface {
	Create(ctx context.Context, u *User) error
	GetByID(ctx context.Context, id UserID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context, q ListQuery) (ListResult, error)
	Update(ctx context.Context, u *User, expectedEtag etag.Etag) error
	UpdatePassword(ctx context.Context, u *User, expectedEtag etag.Etag) error
	UpdateLastLoginAt(ctx context.Context, id UserID, now time.Time) error
	Delete(ctx context.Context, id UserID, expectedEtag etag.Etag) error
}
