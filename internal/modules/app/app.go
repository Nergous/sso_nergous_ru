// Package app is the public API of the app bounded context.
//
// External callers interact with the module through these surfaces:
//
//	app.New(Deps)    wires the module (module.go)
//	app.Service      application-layer admin RPCs (service.go re-exports)
//	app.Repository   full persistence contract (consumed by auth/access)
//	app.AppReader    narrow read-only API for cross-module lookups
//
// The type aliases below let other modules program against
// app.App / app.AppID etc. instead of importing the internal domain
// package directly. The internal package stays unreachable thanks to
// Go's "internal/" protection.
package app

import (
	"context"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/kernel/etag"
)

type (
	App              = domain.App
	AppID            = domain.AppID
	AppStatus        = domain.AppStatus
	AppPatch         = domain.AppPatch
	NewAppParams     = domain.NewAppParams
	RestoreAppParams = domain.RestoreAppParams
	ListOrderBy      = domain.ListOrderBy
	ListQuery        = domain.ListQuery
	ListResult       = domain.ListResult
	PageCursor       = domain.PageCursor
	Etag             = etag.Etag
)

// Status enum re-exports. Numeric values match the proto enum
// (sso.app.v1.AppStatus minus UNSPECIFIED).
const (
	AppStatusActive      = domain.AppStatusActive
	AppStatusDisabled    = domain.AppStatusDisabled
	AppStatusMaintenance = domain.AppStatusMaintenance
)

// ListOrderBy enum re-exports.
const (
	OrderByUnspecified   = domain.OrderByUnspecified
	OrderByCreatedAtDesc = domain.OrderByCreatedAtDesc
	OrderByCreatedAtAsc  = domain.OrderByCreatedAtAsc
	OrderByAppIDDesc     = domain.OrderByAppIDDesc
	OrderByAppIDAsc      = domain.OrderByAppIDAsc
	OrderByNameDesc      = domain.OrderByNameDesc
	OrderByNameAsc       = domain.OrderByNameAsc
)

// ID constructors / parsers re-exported as package-level variables so
// call sites read app.NewAppID() / app.ParseAppID().
var (
	NewAppID   = domain.NewAppID
	ParseAppID = domain.ParseAppID
	NewApp     = domain.NewApp
	RestoreApp = domain.RestoreApp
)

// ----------------------------------------------------------------------------
// Sentinel errors. External consumers test for them with errors.Is.
// ----------------------------------------------------------------------------

var (
	ErrAppNotFound      = domain.ErrAppNotFound
	ErrAppAlreadyExists = domain.ErrAppAlreadyExists
	ErrEtagMismatch     = domain.ErrEtagMismatch
	ErrAppDisabled      = domain.ErrAppDisabled
	ErrAppInMaintenance = domain.ErrAppInMaintenance
)

// ----------------------------------------------------------------------------
// Repository — full persistence contract.
//
// Re-exported so cross-module orchestrators (auth.Login resolves the
// target app by id, access enforces app-existence preconditions) can
// program against app.Repository without breaching internal/. The
// MariaDB adapter inside this module satisfies it; bootstrap surfaces
// the concrete value via Module.Repository().
// ----------------------------------------------------------------------------

type Repository = domain.Repository

// ----------------------------------------------------------------------------
// AppReader — narrow read-only surface used by sibling modules that
// only need GetByID lookups (auth.Login, access existence checks).
// Repository remains the right choice when writes are needed.
//
// The MariaDB Repository in internal/mariadb satisfies this interface
// (compile-time checked in module.go).
// ----------------------------------------------------------------------------

type AppReader interface {
	GetByID(ctx context.Context, id AppID) (*App, error)
}
