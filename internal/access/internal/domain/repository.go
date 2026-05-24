package domain

import (
	"context"
	"time"
)

type ListOrderBy uint8

// Numbering mirrors sso.access.v1.ListUserRolesOrderBy minus UNSPECIFIED.
const (
	OrderByUnspecified   ListOrderBy = 0
	OrderByGrantedAtDesc ListOrderBy = 1
	OrderByGrantedAtAsc  ListOrderBy = 2
	OrderByRoleIDDesc    ListOrderBy = 3
	OrderByRoleIDAsc     ListOrderBy = 4
)

// PageCursor is the keyset for ListUserRoles. Both fields are populated
// for time-based orders; only RoleID is used for id-based orders.
type PageCursor struct {
	GrantedAt time.Time
	RoleID    RoleID
}

type ListUserRolesQuery struct {
	UserID   UserID
	AppID    AppID
	PageSize int
	After    *PageCursor
	OrderBy  ListOrderBy
}

// ListUserRolesRow carries the joined (assignment + role-id) per page
// row. The use-case fetches the actual Role aggregates from
// role.Repository afterwards — keeps access from importing the role
// domain package.
type ListUserRolesRow struct {
	RoleID    RoleID
	GrantedAt time.Time
}

type ListUserRolesResult struct {
	Rows       []ListUserRolesRow
	NextCursor *PageCursor
	TotalSize  *int
}

// PermissionRow is one (role_id, permission) tuple emitted by
// ListActivePermissions. The use-case uses these for wildcard
// expansion on CheckPermission and to populate matched_role_ids.
type PermissionRow struct {
	RoleID     RoleID
	Permission string
}

// Repository is the persistence contract for role assignments. CRUD
// here is intentionally narrow: assignments are immutable except for
// "exists / does-not-exist" — there is no Update surface.
type Repository interface {
	// Create inserts a new assignment. Returns (created=true, nil) on a
	// fresh insert; (created=false, nil) if a row with the same
	// (user_id, role_id) already existed (idempotent grant). Any other
	// error is propagated.
	Create(ctx context.Context, a *RoleAssignment) (created bool, err error)

	// Get returns the existing assignment or ErrAssignmentNotFound.
	Get(ctx context.Context, userID UserID, roleID RoleID) (*RoleAssignment, error)

	// Delete removes the assignment. Idempotent: returns (removed=false,
	// nil) when the row was not present.
	Delete(ctx context.Context, userID UserID, roleID RoleID) (removed bool, err error)

	// BulkCreate inserts the given assignments atomically. The returned
	// slice is positionally aligned with the input; createdMask[i] is
	// true when the assignment at index i was newly inserted (false on
	// idempotent re-grant).
	BulkCreate(ctx context.Context, assignments []*RoleAssignment) (createdMask []bool, err error)

	// BulkDelete removes the given (user, role) pairs atomically. All
	// pairs share the same userID; only role_ids vary.
	BulkDelete(ctx context.Context, userID UserID, roleIDs []RoleID) error

	// ListUserRoles paginates assignments for one (user, app). Returns
	// only role_ids + granted_at; the use-case loads full Role records
	// from the role.Repository.
	ListUserRoles(ctx context.Context, q ListUserRolesQuery) (ListUserRolesResult, error)

	// ListActivePermissions returns one row per (role_id, permission)
	// for ACTIVE roles assigned to the user in the target app. DISABLED
	// roles are filtered server-side at the JOIN. The use-case layer
	// performs wildcard matching against the requested permission.
	ListActivePermissions(ctx context.Context, userID UserID, appID AppID) ([]PermissionRow, error)
}
