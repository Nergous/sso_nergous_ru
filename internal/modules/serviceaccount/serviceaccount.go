// Package serviceaccount is the public API of the service-account
// bounded context. External callers interact with the module through:
//
//	serviceaccount.New(Deps)    wires the module (module.go)
//	serviceaccount.Service      application-layer admin RPCs (service.go)
//	serviceaccount.Repository   persistence contract (consumed by auth)
//
// The type aliases below let other modules program against
// serviceaccount.ServiceAccount / serviceaccount.ID etc. instead of
// importing the internal domain package directly.
package serviceaccount

import (
	"sso/internal/kernel/etag"
	"sso/internal/modules/serviceaccount/internal/domain"
)

type (
	ServiceAccount              = domain.ServiceAccount
	ServiceAccountID            = domain.ServiceAccountID
	ServiceAccountStatus        = domain.ServiceAccountStatus
	ServiceAccountPatch         = domain.ServiceAccountPatch
	NewServiceAccountParams     = domain.NewServiceAccountParams
	RestoreServiceAccountParams = domain.RestoreServiceAccountParams
	ListOrderBy                 = domain.ListOrderBy
	ListQuery                   = domain.ListQuery
	ListResult                  = domain.ListResult
	PageCursor                  = domain.PageCursor
	Repository                  = domain.Repository
	Etag                        = etag.Etag
)

// Status enum re-exports.
const (
	ServiceAccountActive   = domain.ServiceAccountActive
	ServiceAccountDisabled = domain.ServiceAccountDisabled
)

// ListOrderBy enum re-exports.
const (
	OrderByUnspecified             = domain.OrderByUnspecified
	OrderByCreatedAtDesc           = domain.OrderByCreatedAtDesc
	OrderByCreatedAtAsc            = domain.OrderByCreatedAtAsc
	OrderByServiceAccountIDDesc    = domain.OrderByServiceAccountIDDesc
	OrderByServiceAccountIDAsc     = domain.OrderByServiceAccountIDAsc
	OrderByNameDesc                = domain.OrderByNameDesc
	OrderByNameAsc                 = domain.OrderByNameAsc
	OrderByLastAuthenticatedAtDesc = domain.OrderByLastAuthenticatedAtDesc
	OrderByLastAuthenticatedAtAsc  = domain.OrderByLastAuthenticatedAtAsc
)

// ID constructors / parsers re-exported as package-level variables.
var (
	NewServiceAccountID     = domain.NewServiceAccountID
	ParseServiceAccountID   = domain.ParseServiceAccountID
	NewServiceAccount       = domain.NewServiceAccount
	RestoreServiceAccount   = domain.RestoreServiceAccount
)

// Sentinel errors. External consumers test for them with errors.Is.
var (
	ErrServiceAccountNotFound           = domain.ErrServiceAccountNotFound
	ErrServiceAccountAlreadyExists      = domain.ErrServiceAccountAlreadyExists
	ErrServiceAccountDisabled           = domain.ErrServiceAccountDisabled
	ErrServiceAccountInvalidCredentials = domain.ErrServiceAccountInvalidCredentials
	ErrEtagMismatch                     = domain.ErrEtagMismatch
)
