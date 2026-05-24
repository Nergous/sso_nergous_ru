// Package audit re-exports the application-layer Service together
// with the typed Input/Output structs declared in internal/service.
package audit

import "sso/internal/audit/internal/service"

// Service exposes the audit read use-cases (GetAuditEvent /
// ListAuditEvents).
type Service = service.Service

// Authorization plug-in. Bootstrap (or tests) supply a real
// implementation; internal/service ships AlwaysDeny / AlwaysAllow for
// dev and migration scaffolding.
type (
	AuditAuthorizer       = service.AuditAuthorizer
	AlwaysDenyAuthorizer  = service.AlwaysDenyAuthorizer
	AlwaysAllowAuthorizer = service.AlwaysAllowAuthorizer
)

// Input / Output type aliases.
type (
	ListAuditEventsInput  = service.ListAuditEventsInput
	AuditFiltersInput     = service.AuditFiltersInput
	ListAuditEventsOutput = service.ListAuditEventsOutput
)

// ErrPermissionDenied is surfaced by the Service when the authorizer
// rejects the caller.
var ErrPermissionDenied = service.ErrPermissionDenied
