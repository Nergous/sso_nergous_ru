// Package access re-exports the application-layer Service together
// with the typed Input/Output structs declared in internal/service.
package access

import "sso/internal/modules/access/internal/service"

// Service is the use-case orchestrator. Methods correspond 1-to-1 to
// the AccessService RPCs and are grouped by intent across files in
// internal/service: grant.go, remove.go, check.go, list.go.
type Service = service.Service

// Input / Output type aliases. One per RPC; the names match the
// methods on Service.
type (
	HasRoleInAppInput          = service.HasRoleInAppInput
	ListUserRolesInput         = service.ListUserRolesInput
	ListUserRolesOutput        = service.ListUserRolesOutput
	GrantRoleToUserInput       = service.GrantRoleToUserInput
	GrantRoleToUserOutput      = service.GrantRoleToUserOutput
	RemoveRoleFromUserInput    = service.RemoveRoleFromUserInput
	BulkGrantRolesInput        = service.BulkGrantRolesInput
	BulkGrantRolesOutput       = service.BulkGrantRolesOutput
	BulkRemoveRolesInput       = service.BulkRemoveRolesInput
	CheckPermissionInput       = service.CheckPermissionInput
	CheckPermissionOutput      = service.CheckPermissionOutput
	BatchCheckPermissionInput  = service.BatchCheckPermissionInput
	BatchCheckPermissionOutput = service.BatchCheckPermissionOutput
)
