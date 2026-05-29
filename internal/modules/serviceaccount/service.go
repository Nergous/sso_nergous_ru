// Package serviceaccount re-exports the application-layer Service
// together with the typed Input/Output structs declared in
// internal/service.
package serviceaccount

import "sso/internal/modules/serviceaccount/internal/service"

// Service is the use-case orchestrator. Methods correspond 1-to-1 to
// the ServiceAccountService RPCs and are grouped by intent across
// files in internal/service: create.go, get.go, update.go, delete.go,
// rotate.go, secret.go.
type Service = service.Service

// Input / Output type aliases. One per RPC; the names match the
// methods on Service.
type (
	CreateServiceAccountInput            = service.CreateServiceAccountInput
	CreateServiceAccountOutput           = service.CreateServiceAccountOutput
	ListServiceAccountsInput             = service.ListServiceAccountsInput
	ListServiceAccountsOutput            = service.ListServiceAccountsOutput
	UpdateServiceAccountInput            = service.UpdateServiceAccountInput
	DisableServiceAccountInput           = service.DisableServiceAccountInput
	EnableServiceAccountInput            = service.EnableServiceAccountInput
	RotateCredentialsInput               = service.RotateCredentialsInput
	RotateCredentialsOutput              = service.RotateCredentialsOutput
	PermanentlyDeleteServiceAccountInput = service.PermanentlyDeleteServiceAccountInput
)

// EtagWildcard is the wire-level sentinel meaning "skip optimistic
// concurrency check" — matches the proto contract on RPCs that accept
// "*" in the etag field.
const EtagWildcard = service.EtagWildcard
