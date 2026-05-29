package domain

import "errors"

var (
	// ErrAssignmentNotFound is surfaced internally by the repository when
	// a Get-by-(user,role) miss happens; the use-case layer rarely
	// propagates it directly because Remove is idempotent.
	ErrAssignmentNotFound = errors.New("access: assignment not found")

	// ErrUserNotFound — the user_id refers to no row in identity.users.
	ErrUserNotFound = errors.New("access: user not found")

	// ErrRoleNotFound — the role_id refers to no row in roles.
	ErrRoleNotFound = errors.New("access: role not found")

	// ErrAppNotFound — the app_id refers to no row in apps.
	ErrAppNotFound = errors.New("access: app not found")

	// ErrRoleDisabled — the role exists but is DISABLED. Granting a
	// disabled role fails with this; removing it does not.
	ErrRoleDisabled = errors.New("access: role is disabled")

	// ErrRoleNotInApp — bulk operation referenced a role whose app_id
	// does not match the requested app. Cross-app bulk ops are rejected
	// wholesale (no partial success).
	ErrRoleNotInApp = errors.New("access: role does not belong to app")

	// ErrUserNotEligible — the user is BLOCKED or DELETED; assignments
	// must not be granted in that state. Existing assignments stay; they
	// just don't contribute to CheckPermission.
	ErrUserNotEligible = errors.New("access: user is blocked or deleted")
)
