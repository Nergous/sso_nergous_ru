package domain

type EventType uint16

const (
	EventTypeUnknown EventType = 0

	EventTypeIdentityCreateUser            EventType = 1
	EventTypeIdentityGetUser               EventType = 2
	EventTypeIdentityListUsers             EventType = 3
	EventTypeIdentityUpdateUser            EventType = 4
	EventTypeIdentityDisableUser           EventType = 5
	EventTypeIdentityEnableUser            EventType = 6
	EventTypeIdentitySoftDeleteUser        EventType = 7
	EventTypeIdentityPermanentlyDeleteUser EventType = 8
	// reserved for identity events 1 - 20

	EventTypeAppCreateApp            EventType = 21
	EventTypeAppGetApp               EventType = 22
	EventTypeAppListApps             EventType = 23
	EventTypeAppUpdateApp            EventType = 24
	EventTypeAppDisableApp           EventType = 25
	EventTypeAppEnableApp            EventType = 26
	EventTypeAppEnterMaintenanceMode EventType = 27
	EventTypeAppExitMaintenanceMode  EventType = 28
	EventTypeAppPermanentlyDeleteApp EventType = 29
	// reserved for app events 21 - 40

	EventTypeRoleCreateRole            EventType = 41
	EventTypeRoleGetRole               EventType = 42
	EventTypeRoleListRoles             EventType = 43
	EventTypeRoleUpdateRole            EventType = 44
	EventTypeRoleDisableRole           EventType = 45
	EventTypeRoleEnableRole            EventType = 46
	EventTypeRolePermanentlyDeleteRole EventType = 47
	// reserved for role events 41 - 60

	EventTypeServiceAccountCreateServiceAccount            EventType = 61
	EventTypeServiceAccountGetServiceAccount               EventType = 62
	EventTypeServiceAccountListServiceAccount              EventType = 63
	EventTypeServiceAccountUpdateServiceAccount            EventType = 64
	EventTypeServiceAccountRotateCredentials               EventType = 65
	EventTypeServiceAccountDisableServiceAccount           EventType = 66
	EventTypeServiceAccountEnableServiceAccount            EventType = 67
	EventTypeServiceAccountPermanentlyDeleteServiceAccount EventType = 68
	// reserved for SA events 61 - 80

	EventTypeAccessHasRoleInApp         EventType = 81
	EventTypeAccessListUserRoles        EventType = 82
	EventTypeAccessCheckPermission      EventType = 83
	EventTypeAccessBatchCheckPermission EventType = 84
	EventTypeAccessGrantRoleToUser      EventType = 85
	EventTypeAccessRemoveRoleFromUser   EventType = 86
	EventTypeAccessBulkGrantRoles       EventType = 87
	EventTypeAccessBulkRemoveRoles      EventType = 88
	// reserved for access events 81 - 100

	EventTypeAuthRegister                      EventType = 101
	EventTypeAuthLogin                         EventType = 102
	EventTypeAuthLogout                        EventType = 103
	EventTypeAuthRefresh                       EventType = 104
	EventTypeAuthValidateToken                 EventType = 105
	EventTypeAuthChangePassword                EventType = 106
	EventTypeAuthListSessions                  EventType = 107
	EventTypeAuthRevokeSession                 EventType = 108
	EventTypeAuthRevokeAllSessions             EventType = 109
	EventTypeAuthRevokeToken                   EventType = 110
	EventTypeAuthGenerateRecoveryCodes         EventType = 111
	EventTypeAuthResetPasswordWithRecoveryCode EventType = 112
	EventTypeAuthAuthenticateServiceAccount    EventType = 113
	// reserved for auth events 101 - 130
)

func (e EventType) String() string {
	switch e {
	case EventTypeIdentityCreateUser:
		return "identity.create_user"
	case EventTypeIdentityGetUser:
		return "identity.get_user"
	case EventTypeIdentityListUsers:
		return "identity.list_users"
	case EventTypeIdentityUpdateUser:
		return "identity.update_user"
	case EventTypeIdentityDisableUser:
		return "identity.disable_user"
	case EventTypeIdentityEnableUser:
		return "identity.enable_user"
	case EventTypeIdentitySoftDeleteUser:
		return "identity.soft_delete_user"
	case EventTypeIdentityPermanentlyDeleteUser:
		return "identity.permanently_delete_user"

	case EventTypeAppCreateApp:
		return "app.create_app"
	case EventTypeAppGetApp:
		return "app.get_app"
	case EventTypeAppListApps:
		return "app.list_apps"
	case EventTypeAppUpdateApp:
		return "app.update_app"
	case EventTypeAppDisableApp:
		return "app.disable_app"
	case EventTypeAppEnableApp:
		return "app.enable_app"
	case EventTypeAppEnterMaintenanceMode:
		return "app.enter_maintenance_mode"
	case EventTypeAppExitMaintenanceMode:
		return "app.exit_maintenance_mode"
	case EventTypeAppPermanentlyDeleteApp:
		return "app.permanently_delete_app"

	case EventTypeRoleCreateRole:
		return "role.create_role"
	case EventTypeRoleGetRole:
		return "role.get_role"
	case EventTypeRoleListRoles:
		return "role.list_roles"
	case EventTypeRoleUpdateRole:
		return "role.update_role"
	case EventTypeRoleDisableRole:
		return "role.disable_role"
	case EventTypeRoleEnableRole:
		return "role.enable_role"
	case EventTypeRolePermanentlyDeleteRole:
		return "role.permanently_delete_role"

	case EventTypeServiceAccountCreateServiceAccount:
		return "service_account.create_service_account"
	case EventTypeServiceAccountGetServiceAccount:
		return "service_account.get_service_account"
	case EventTypeServiceAccountListServiceAccount:
		return "service_account.list_service_account"
	case EventTypeServiceAccountUpdateServiceAccount:
		return "service_account.update_service_account"
	case EventTypeServiceAccountRotateCredentials:
		return "service_account.rotate_credentials"
	case EventTypeServiceAccountDisableServiceAccount:
		return "service_account.disable_service_account"
	case EventTypeServiceAccountEnableServiceAccount:
		return "service_account.enable_service_account"
	case EventTypeServiceAccountPermanentlyDeleteServiceAccount:
		return "service_account.permanently_delete_service_account"

	case EventTypeAccessHasRoleInApp:
		return "access.has_role_in_app"
	case EventTypeAccessListUserRoles:
		return "access.list_user_roles"
	case EventTypeAccessCheckPermission:
		return "access.check_permission"
	case EventTypeAccessBatchCheckPermission:
		return "access.batch_check_permission"
	case EventTypeAccessGrantRoleToUser:
		return "access.grant_role_to_user"
	case EventTypeAccessRemoveRoleFromUser:
		return "access.remove_role_from_user"
	case EventTypeAccessBulkGrantRoles:
		return "access.bulk_grant_roles"
	case EventTypeAccessBulkRemoveRoles:
		return "access.bulk_remove_roles"

	case EventTypeAuthRegister:
		return "auth.register"
	case EventTypeAuthLogin:
		return "auth.login"
	case EventTypeAuthLogout:
		return "auth.logout"
	case EventTypeAuthRefresh:
		return "auth.refresh"
	case EventTypeAuthValidateToken:
		return "auth.validate_token"
	case EventTypeAuthChangePassword:
		return "auth.change_password"
	case EventTypeAuthListSessions:
		return "auth.list_sessions"
	case EventTypeAuthRevokeSession:
		return "auth.revoke_session"
	case EventTypeAuthRevokeAllSessions:
		return "auth.revoke_all_sessions"
	case EventTypeAuthRevokeToken:
		return "auth.revoke_token"
	case EventTypeAuthGenerateRecoveryCodes:
		return "auth.generate_recovery_codes"
	case EventTypeAuthResetPasswordWithRecoveryCode:
		return "auth.reset_password_with_recovery_code"
	case EventTypeAuthAuthenticateServiceAccount:
		return "auth.authenticate_service_account"

	default:
		return "unknown"
	}
}

// IsKnown reports whether e is one of the declared event types
// (i.e. not EventTypeUnknown and not a value outside the canonical set).
func (e EventType) IsKnown() bool {
	return e != EventTypeUnknown && e.String() != "unknown"
}

// eventTypeBySlug is the reverse of EventType.String, built once at
// package init from the canonical set. Used by the persistence layer to
// turn the stored slug back into a typed EventType.
var eventTypeBySlug = func() map[string]EventType {
	m := make(map[string]EventType, 64)
	for et := EventType(1); et < 200; et++ {
		s := et.String()
		if s == "unknown" {
			continue
		}
		m[s] = et
	}
	return m
}()

// ParseEventTypeSlug maps a stored event_type slug back to its typed
// EventType. Returns EventTypeUnknown for an unrecognised slug (e.g. an
// event recorded by a newer version of the code that this binary does
// not yet know about).
func ParseEventTypeSlug(slug string) EventType {
	return eventTypeBySlug[slug]
}
