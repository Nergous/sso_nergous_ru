// Package access is the public API of the access bounded context
// (role assignments and authorization decisions).
//
// External callers interact with the module through these surfaces:
//
//	access.New(Deps)    wires the module (module.go)
//	access.Service      application-layer RPCs (service.go re-exports)
//
// The type aliases below let other modules program against
// access.RoleAssignment / access.UserID etc. instead of importing the
// internal domain package directly. The internal package stays
// unreachable thanks to Go's "internal/" protection.
package access

import "sso/internal/modules/access/internal/domain"

type (
	UserID                  = domain.UserID
	RoleID                  = domain.RoleID
	AppID                   = domain.AppID
	ActorID                 = domain.ActorID
	RoleAssignment          = domain.RoleAssignment
	NewRoleAssignmentParams = domain.NewRoleAssignmentParams
	ListOrderBy             = domain.ListOrderBy
	PageCursor              = domain.PageCursor
	ListUserRolesQuery      = domain.ListUserRolesQuery
	ListUserRolesRow        = domain.ListUserRolesRow
	ListUserRolesResult     = domain.ListUserRolesResult
	PermissionRow           = domain.PermissionRow
)

// ListOrderBy enum re-exports.
const (
	OrderByUnspecified   = domain.OrderByUnspecified
	OrderByGrantedAtDesc = domain.OrderByGrantedAtDesc
	OrderByGrantedAtAsc  = domain.OrderByGrantedAtAsc
	OrderByRoleIDDesc    = domain.OrderByRoleIDDesc
	OrderByRoleIDAsc     = domain.OrderByRoleIDAsc
)

// ID parsers re-exported as package-level variables.
var (
	ParseUserID  = domain.ParseUserID
	ParseRoleID  = domain.ParseRoleID
	ParseAppID   = domain.ParseAppID
	ParseActorID = domain.ParseActorID
)

// ----------------------------------------------------------------------------
// Sentinel errors. External consumers test for them with errors.Is.
// ----------------------------------------------------------------------------

var (
	ErrAssignmentNotFound = domain.ErrAssignmentNotFound
	ErrUserNotFound       = domain.ErrUserNotFound
	ErrRoleNotFound       = domain.ErrRoleNotFound
	ErrAppNotFound        = domain.ErrAppNotFound
	ErrRoleDisabled       = domain.ErrRoleDisabled
	ErrRoleNotInApp       = domain.ErrRoleNotInApp
	ErrUserNotEligible    = domain.ErrUserNotEligible
)

// Repository is the persistence contract for role assignments. Exposed
// at the public surface for the unlikely case admin tooling needs to
// reach the store directly; normal consumers use Service.
type Repository = domain.Repository
