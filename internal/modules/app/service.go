// Package app re-exports the application-layer Service together with
// the typed Input/Output structs declared in internal/service.
//
// External code (the gRPC handler in internal/grpc, admin tooling,
// integration tests) programs against app.Service and
// app.CreateAppInput — never against internal/service directly. The
// internal package stays unreachable thanks to Go's "internal/"
// protection; these aliases are the only way through.
package app

import "sso/internal/modules/app/internal/service"

// Service is the use-case orchestrator. Methods correspond 1-to-1 to
// the AppService RPCs and are grouped by intent across files in
// internal/service: create.go, get.go, update.go, delete.go.
type Service = service.Service

// Input / Output type aliases. One per RPC; the names match the methods
// on Service.
type (
	CreateAppInput            = service.CreateAppInput
	ListAppsInput             = service.ListAppsInput
	ListAppsOutput            = service.ListAppsOutput
	UpdateAppInput            = service.UpdateAppInput
	DisableAppInput           = service.DisableAppInput
	EnableAppInput            = service.EnableAppInput
	EnterMaintenanceModeInput = service.EnterMaintenanceModeInput
	ExitMaintenanceModeInput  = service.ExitMaintenanceModeInput
	PermanentlyDeleteAppInput = service.PermanentlyDeleteAppInput
)

// EtagWildcard is the wire-level sentinel meaning "skip optimistic
// concurrency check" — matches the proto contract on RPCs that accept
// "*" in the etag field. Exposed here so callers can build inputs
// without importing internal/service or audit/auditx.
const EtagWildcard = service.EtagWildcard
