// Package role is the public API of the role bounded context.
//
// External callers interact with the module through these surfaces:
//
//	role.New(Deps)    wires the module (module.go)
//	role.Service      application-layer admin RPCs (service.go re-exports)
//	role.Repository   full persistence contract (consumed by access)
//
// The type aliases below let other modules program against role.Role /
// role.RoleID etc. instead of importing the internal domain package
// directly. The internal package stays unreachable thanks to Go's
// "internal/" protection.
package role

import (
	"sso/internal/modules/app"
	"sso/internal/kernel/etag"
	"sso/internal/modules/role/internal/domain"
)

type (
	Role              = domain.Role
	RoleID            = domain.RoleID
	RoleStatus        = domain.RoleStatus
	RolePatch         = domain.RolePatch
	NewRoleParams     = domain.NewRoleParams
	RestoreRoleParams = domain.RestoreRoleParams
	ListOrderBy       = domain.ListOrderBy
	ListQuery         = domain.ListQuery
	ListResult        = domain.ListResult
	PageCursor        = domain.PageCursor
	Etag              = etag.Etag

	// AppID is a cross-context handle to app.AppID, re-exported here so
	// access's use-case layer can write role.AppID instead of pulling
	// the app package in just for the type.
	AppID = app.AppID
)

// Status enum re-exports. Numeric values match the proto enum
// (sso.roles.v1.RoleStatus minus UNSPECIFIED).
const (
	RoleStatusActive   = domain.RoleStatusActive
	RoleStatusDisabled = domain.RoleStatusDisabled
)

// ListOrderBy enum re-exports.
const (
	OrderByUnspecified   = domain.OrderByUnspecified
	OrderByCreatedAtDesc = domain.OrderByCreatedAtDesc
	OrderByCreatedAtAsc  = domain.OrderByCreatedAtAsc
	OrderByRoleIDDesc    = domain.OrderByRoleIDDesc
	OrderByRoleIDAsc     = domain.OrderByRoleIDAsc
	OrderByNameDesc      = domain.OrderByNameDesc
	OrderByNameAsc       = domain.OrderByNameAsc
)

// ID constructors / parsers re-exported as package-level variables.
var (
	NewRoleID   = domain.NewRoleID
	ParseRoleID = domain.ParseRoleID
	NewRole     = domain.NewRole
	RestoreRole = domain.RestoreRole
)

// ----------------------------------------------------------------------------
// Sentinel errors. External consumers test for them with errors.Is.
// ----------------------------------------------------------------------------

var (
	ErrRoleNotFound       = domain.ErrRoleNotFound
	ErrRoleAlreadyExists  = domain.ErrRoleAlreadyExists
	ErrEtagMismatch       = domain.ErrEtagMismatch
	ErrRoleDisabled       = domain.ErrRoleDisabled
	ErrRoleNotInApp       = domain.ErrRoleNotInApp
	ErrRoleHasAssignments = domain.ErrRoleHasAssignments
)

// ----------------------------------------------------------------------------
// Repository — full persistence contract.
//
// Re-exported so cross-module orchestrators (access enforces
// role-active / role-in-app preconditions and hydrates Role aggregates
// for ListUserRoles) can program against role.Repository without
// breaching internal/. The MariaDB adapter inside this module
// satisfies it; bootstrap surfaces the concrete value via
// Module.Repository().
// ----------------------------------------------------------------------------

type Repository = domain.Repository
