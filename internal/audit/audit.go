// Package audit is the public API of the audit bounded context
// (append-only security event log).
//
// External callers interact with the module through these surfaces:
//
//	audit.New(Deps)    wires the module (module.go)
//	audit.Service      application-layer read RPCs (service.go re-exports)
//	audit.Repository   persistence contract (consumed by auditbus)
//	audit.Emitter      publication surface (every use-case across the
//	                   project records events through this)
//
// The type aliases below let other modules program against
// audit.Audit / audit.EventType etc. instead of importing the internal
// domain package directly. The internal package stays unreachable
// thanks to Go's "internal/" protection.
package audit

import "sso/internal/audit/internal/domain"

// ----------------------------------------------------------------------------
// Type aliases — re-export of the internal domain types.
// ----------------------------------------------------------------------------

type (
	Audit              = domain.Audit
	AuditID            = domain.AuditID
	ActorID            = domain.ActorID
	ActorType          = domain.ActorType
	AuditOutcome       = domain.AuditOutcome
	SubjectID          = domain.SubjectID
	SubjectType        = domain.SubjectType
	AppID              = domain.AppID
	EventType          = domain.EventType
	NewAuditParams     = domain.NewAuditParams
	RestoreAuditParams = domain.RestoreAuditParams
	AuditFilters       = domain.AuditFilters
	ListQuery          = domain.ListQuery
	ListResult         = domain.ListResult
	PageCursor         = domain.PageCursor
	Repository         = domain.Repository
	Emitter            = domain.Emitter
	NopEmitter         = domain.NopEmitter
)

// Maximum-length / cardinality constants for metadata, reasons,
// user-agent. Re-exported so platform/auditbus and other helpers can
// reach the same caps without touching internal/domain.
const (
	MetadataMaxEntries  = domain.MetadataMaxEntries
	MetadataKeyMaxLen   = domain.MetadataKeyMaxLen
	MetadataValueMaxLen = domain.MetadataValueMaxLen
	UserAgentMaxLen     = domain.UserAgentMaxLen
	ReasonMaxLen        = domain.ReasonMaxLen
)

// ----------------------------------------------------------------------------
// ActorType enum
// ----------------------------------------------------------------------------

const (
	ActorTypeUnknown   = domain.ActorTypeUnknown
	ActorTypeUser      = domain.ActorTypeUser
	ActorTypeService   = domain.ActorTypeService
	ActorTypeSystem    = domain.ActorTypeSystem
	ActorTypeAnonymous = domain.ActorTypeAnonymous
)

// ----------------------------------------------------------------------------
// AuditOutcome enum
// ----------------------------------------------------------------------------

const (
	OutcomeUnknown = domain.OutcomeUnknown
	OutcomeSuccess = domain.OutcomeSuccess
	OutcomeFailure = domain.OutcomeFailure
	OutcomeDenied  = domain.OutcomeDenied
)

// ----------------------------------------------------------------------------
// SubjectType enum
// ----------------------------------------------------------------------------

const (
	SubjectTypeUnknown        = domain.SubjectTypeUnknown
	SubjectTypeUser           = domain.SubjectTypeUser
	SubjectTypeRole           = domain.SubjectTypeRole
	SubjectTypeApp            = domain.SubjectTypeApp
	SubjectTypeSession        = domain.SubjectTypeSession
	SubjectTypeRoleAssignment = domain.SubjectTypeRoleAssignment
	SubjectTypeServiceAccount = domain.SubjectTypeServiceAccount
)

// ----------------------------------------------------------------------------
// EventType enum
// ----------------------------------------------------------------------------

const (
	EventTypeUnknown = domain.EventTypeUnknown

	EventTypeIdentityCreateUser            = domain.EventTypeIdentityCreateUser
	EventTypeIdentityGetUser               = domain.EventTypeIdentityGetUser
	EventTypeIdentityListUsers             = domain.EventTypeIdentityListUsers
	EventTypeIdentityUpdateUser            = domain.EventTypeIdentityUpdateUser
	EventTypeIdentityDisableUser           = domain.EventTypeIdentityDisableUser
	EventTypeIdentityEnableUser            = domain.EventTypeIdentityEnableUser
	EventTypeIdentitySoftDeleteUser        = domain.EventTypeIdentitySoftDeleteUser
	EventTypeIdentityPermanentlyDeleteUser = domain.EventTypeIdentityPermanentlyDeleteUser

	EventTypeAppCreateApp            = domain.EventTypeAppCreateApp
	EventTypeAppGetApp               = domain.EventTypeAppGetApp
	EventTypeAppListApps             = domain.EventTypeAppListApps
	EventTypeAppUpdateApp            = domain.EventTypeAppUpdateApp
	EventTypeAppDisableApp           = domain.EventTypeAppDisableApp
	EventTypeAppEnableApp            = domain.EventTypeAppEnableApp
	EventTypeAppEnterMaintenanceMode = domain.EventTypeAppEnterMaintenanceMode
	EventTypeAppExitMaintenanceMode  = domain.EventTypeAppExitMaintenanceMode
	EventTypeAppPermanentlyDeleteApp = domain.EventTypeAppPermanentlyDeleteApp

	EventTypeRoleCreateRole            = domain.EventTypeRoleCreateRole
	EventTypeRoleGetRole               = domain.EventTypeRoleGetRole
	EventTypeRoleListRoles             = domain.EventTypeRoleListRoles
	EventTypeRoleUpdateRole            = domain.EventTypeRoleUpdateRole
	EventTypeRoleDisableRole           = domain.EventTypeRoleDisableRole
	EventTypeRoleEnableRole            = domain.EventTypeRoleEnableRole
	EventTypeRolePermanentlyDeleteRole = domain.EventTypeRolePermanentlyDeleteRole

	EventTypeServiceAccountCreateServiceAccount            = domain.EventTypeServiceAccountCreateServiceAccount
	EventTypeServiceAccountGetServiceAccount               = domain.EventTypeServiceAccountGetServiceAccount
	EventTypeServiceAccountListServiceAccount              = domain.EventTypeServiceAccountListServiceAccount
	EventTypeServiceAccountUpdateServiceAccount            = domain.EventTypeServiceAccountUpdateServiceAccount
	EventTypeServiceAccountRotateCredentials               = domain.EventTypeServiceAccountRotateCredentials
	EventTypeServiceAccountDisableServiceAccount           = domain.EventTypeServiceAccountDisableServiceAccount
	EventTypeServiceAccountEnableServiceAccount            = domain.EventTypeServiceAccountEnableServiceAccount
	EventTypeServiceAccountPermanentlyDeleteServiceAccount = domain.EventTypeServiceAccountPermanentlyDeleteServiceAccount

	EventTypeAccessHasRoleInApp         = domain.EventTypeAccessHasRoleInApp
	EventTypeAccessListUserRoles        = domain.EventTypeAccessListUserRoles
	EventTypeAccessCheckPermission      = domain.EventTypeAccessCheckPermission
	EventTypeAccessBatchCheckPermission = domain.EventTypeAccessBatchCheckPermission
	EventTypeAccessGrantRoleToUser      = domain.EventTypeAccessGrantRoleToUser
	EventTypeAccessRemoveRoleFromUser   = domain.EventTypeAccessRemoveRoleFromUser
	EventTypeAccessBulkGrantRoles       = domain.EventTypeAccessBulkGrantRoles
	EventTypeAccessBulkRemoveRoles      = domain.EventTypeAccessBulkRemoveRoles

	EventTypeAuthRegister                      = domain.EventTypeAuthRegister
	EventTypeAuthLogin                         = domain.EventTypeAuthLogin
	EventTypeAuthLogout                        = domain.EventTypeAuthLogout
	EventTypeAuthRefresh                       = domain.EventTypeAuthRefresh
	EventTypeAuthValidateToken                 = domain.EventTypeAuthValidateToken
	EventTypeAuthChangePassword                = domain.EventTypeAuthChangePassword
	EventTypeAuthListSessions                  = domain.EventTypeAuthListSessions
	EventTypeAuthRevokeSession                 = domain.EventTypeAuthRevokeSession
	EventTypeAuthRevokeAllSessions             = domain.EventTypeAuthRevokeAllSessions
	EventTypeAuthRevokeToken                   = domain.EventTypeAuthRevokeToken
	EventTypeAuthGenerateRecoveryCodes         = domain.EventTypeAuthGenerateRecoveryCodes
	EventTypeAuthResetPasswordWithRecoveryCode = domain.EventTypeAuthResetPasswordWithRecoveryCode
	EventTypeAuthAuthenticateServiceAccount    = domain.EventTypeAuthAuthenticateServiceAccount
)

// ----------------------------------------------------------------------------
// Reason strings (audit.reason field values)
// ----------------------------------------------------------------------------

const (
	ReasonInternal                    = domain.ReasonInternal
	ReasonInvalidCredentials          = domain.ReasonInvalidCredentials
	ReasonInvalidToken                = domain.ReasonInvalidToken
	ReasonRefreshTokenReused          = domain.ReasonRefreshTokenReused
	ReasonSessionNotFound             = domain.ReasonSessionNotFound
	ReasonPasswordMismatch            = domain.ReasonPasswordMismatch
	ReasonValidationFailed            = domain.ReasonValidationFailed
	ReasonEtagMismatch                = domain.ReasonEtagMismatch
	ReasonUserAlreadyExists           = domain.ReasonUserAlreadyExists
	ReasonUserBlocked                 = domain.ReasonUserBlocked
	ReasonUserDeleted                 = domain.ReasonUserDeleted
	ReasonUserNotFound                = domain.ReasonUserNotFound
	ReasonUserNotDeleted              = domain.ReasonUserNotDeleted
	ReasonUserNotEligible             = domain.ReasonUserNotEligible
	ReasonAppNotFound                 = domain.ReasonAppNotFound
	ReasonAppAlreadyExists            = domain.ReasonAppAlreadyExists
	ReasonAppDisabled                 = domain.ReasonAppDisabled
	ReasonAppInMaintenance            = domain.ReasonAppInMaintenance
	ReasonRoleNotFound                = domain.ReasonRoleNotFound
	ReasonRoleAlreadyExists           = domain.ReasonRoleAlreadyExists
	ReasonRoleDisabled                = domain.ReasonRoleDisabled
	ReasonRoleNotInApp                = domain.ReasonRoleNotInApp
	ReasonPermissionDenied            = domain.ReasonPermissionDenied
	ReasonRecoveryCodeInvalid         = domain.ReasonRecoveryCodeInvalid
	ReasonServiceAccountNotFound      = domain.ReasonServiceAccountNotFound
	ReasonServiceAccountAlreadyExists = domain.ReasonServiceAccountAlreadyExists
	ReasonServiceAccountDisabled      = domain.ReasonServiceAccountDisabled
	ReasonInvalidClientCredentials    = domain.ReasonInvalidClientCredentials
	ReasonRateLimited                 = domain.ReasonRateLimited
)

// ID constructors / parsers re-exported as package-level variables.
var (
	NewAuditID         = domain.NewAuditID
	ParseAuditID       = domain.ParseAuditID
	ParseActorID       = domain.ParseActorID
	ParseSubjectID     = domain.ParseSubjectID
	ParseAppID         = domain.ParseAppID
	NewAudit           = domain.NewAudit
	RestoreAudit       = domain.RestoreAudit
	BaseFromActor     = domain.BaseFromActor
	MapActorKind       = domain.MapActorKind
	ParseEventTypeSlug = domain.ParseEventTypeSlug
)

// Sentinel errors. External consumers test for them with errors.Is.
var (
	ErrAuditNotFound      = domain.ErrAuditNotFound
	ErrInvalidEventType   = domain.ErrInvalidEventType
	ErrInvalidActorType   = domain.ErrInvalidActorType
	ErrInvalidSubjectType = domain.ErrInvalidSubjectType
	ErrInvalidOutcome     = domain.ErrInvalidOutcome
)
