// Package identity re-exports the application-layer Service together
// with the typed Input/Output structs declared in internal/service.
//
// External code (the gRPC handler in internal/grpcadapter, the auth
// module's admin paths, integration tests) programs against
// identity.Service and identity.CreateUserInput — never against
// internal/service directly. The internal package stays unreachable
// thanks to Go's "internal/" protection; these aliases are the only
// way through.
package identity

import "sso/internal/modules/identity/internal/service"

// Service is the use-case orchestrator. Methods correspond 1-to-1 to
// the IdentityService RPCs and are grouped by intent across files in
// internal/service: create.go, get.go, update.go, delete.go.
type Service = service.Service

// Input / Output type aliases. One per RPC; the names match the methods
// on Service.
type (
	CreateUserInput            = service.CreateUserInput
	ListUsersInput             = service.ListUsersInput
	ListUsersOutput            = service.ListUsersOutput
	UpdateUserInput            = service.UpdateUserInput
	DisableUserInput           = service.DisableUserInput
	EnableUserInput            = service.EnableUserInput
	SoftDeleteUserInput        = service.SoftDeleteUserInput
	PermanentlyDeleteUserInput = service.PermanentlyDeleteUserInput
)

// EtagWildcard is the wire-level sentinel meaning "skip optimistic
// concurrency check" — matches the proto contract on RPCs that accept
// "*" in the etag field. Exposed here so callers can build inputs
// without importing internal/service or audit/auditx.
const EtagWildcard = service.EtagWildcard
