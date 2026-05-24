// Package identity is the public API of the identity bounded context.
//
// External callers interact with the module through these surfaces:
//
//	identity.New(Deps)    wires the module (module.go)
//	identity.Service      application-layer admin RPCs (service.go re-exports)
//	identity.Repository   full persistence contract (consumed by auth and
//	                      cross-module orchestrators that need write access)
//	identity.UserReader   narrow read-only API for cross-module credential
//	                      lookups — access (this file)
//
// The type aliases below let other modules program against
// identity.User / identity.UserID etc. instead of importing the internal
// domain package directly. The internal package stays unreachable thanks
// to Go's "internal/" protection.
package identity

import (
	"context"

	"sso/internal/identity/internal/domain"
	"sso/internal/kernel/etag"
)

// ----------------------------------------------------------------------------
// Type aliases — re-export of the internal domain types.
//
// Aliases (type X = Y) make identity.User and domain.User the same type
// at the type-checker level — no conversion is needed when passing values
// across the boundary.
// ----------------------------------------------------------------------------

type (
	User              = domain.User
	UserID            = domain.UserID
	UserStatus        = domain.UserStatus
	UserPatch         = domain.UserPatch
	NewUserParams     = domain.NewUserParams
	RestoreUserParams = domain.RestoreUserParams
	ListOrderBy       = domain.ListOrderBy
	ListQuery         = domain.ListQuery
	ListResult        = domain.ListResult
	PageCursor        = domain.PageCursor
	Etag              = etag.Etag
)

// Status enum re-exports. Numeric values match the proto enum
// (sso.identity.v1.UserStatus minus UNSPECIFIED).
const (
	UserStatusActive  = domain.UserStatusActive
	UserStatusBlocked = domain.UserStatusBlocked
	UserStatusDeleted = domain.UserStatusDeleted
)

// ListOrderBy enum re-exports.
const (
	OrderByUnspecified   = domain.OrderByUnspecified
	OrderByCreatedAtDesc = domain.OrderByCreatedAtDesc
	OrderByCreatedAtAsc  = domain.OrderByCreatedAtAsc
	OrderByUserIDDesc    = domain.OrderByUserIDDesc
	OrderByUserIDAsc     = domain.OrderByUserIDAsc
	OrderByUsernameDesc  = domain.OrderByUsernameDesc
	OrderByUsernameAsc   = domain.OrderByUsernameAsc
)

// ID and etag constructors / parsers re-exported as package-level
// variables so call sites read identity.NewUserID() / identity.ParseEtag().
var (
	NewUserID   = domain.NewUserID
	ParseUserID = domain.ParseUserID
	NewUser     = domain.NewUser
	RestoreUser = domain.RestoreUser
)

// ----------------------------------------------------------------------------
// Sentinel errors. External consumers test for them with errors.Is.
// ----------------------------------------------------------------------------

var (
	ErrUserNotFound        = domain.ErrUserNotFound
	ErrUserAlreadyExists   = domain.ErrUserAlreadyExists
	ErrEtagMismatch        = domain.ErrEtagMismatch
	ErrUserDeleted         = domain.ErrUserDeleted
	ErrUserNotDeleted      = domain.ErrUserNotDeleted
	ErrInvalidPasswordHash = domain.ErrInvalidPasswordHash
)

// ----------------------------------------------------------------------------
// Repository — full persistence contract.
//
// Re-exported so cross-module orchestrators (auth.Service needs Create
// for Register, UpdatePassword for ChangePassword, etc.) can program
// against identity.Repository without breaching internal/. The MariaDB
// adapter inside this module satisfies it; bootstrap surfaces the
// concrete value via Module.Repository().
// ----------------------------------------------------------------------------

type Repository = domain.Repository

// ----------------------------------------------------------------------------
// UserReader — narrow read-only surface used by sibling modules.
//
// access uses GetByID when materialising a RoleAssignment's target user.
// UserReader keeps that cross-module contract minimal — Repository
// remains the right surface when writes are needed.
//
// The MariaDB Repository in internal/mariadb satisfies this interface
// (compile-time checked in module.go).
// ----------------------------------------------------------------------------

type UserReader interface {
	GetByID(ctx context.Context, id UserID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
}
