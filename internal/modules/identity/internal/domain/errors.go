package domain

import (
	"errors"
)

// Sentinel errors. Use errors.Is to test.
var (
	// ErrUserNotFound — repository or use-case looked up a user_id that does
	// not exist (or has been hard-deleted).
	ErrUserNotFound = errors.New("identity: not found")

	// ErrUserAlreadyExists — a uniqueness constraint tripped on email or
	// username during Create. The colliding field is intentionally NOT
	// surfaced (user-enumeration mitigation, see
	// sso.common.v1.ERROR_REASON_USER_ALREADY_EXISTS).
	ErrUserAlreadyExists = errors.New("identity: already exists")

	// ErrEtagMismatch — optimistic concurrency check failed: the supplied
	// expected etag does not match the current server-side etag.
	ErrEtagMismatch = errors.New("identity: etag mismatch")

	// ErrUserDeleted — operation rejected because the target user is in
	// USER_STATUS_DELETED state (lifecycle rule).
	ErrUserDeleted = errors.New("identity: is deleted")

	// ErrUserNotDeleted — operation requires the user to be soft-deleted
	// first (currently: PermanentlyDelete).
	ErrUserNotDeleted = errors.New("identity: is not deleted")

	// ErrInvalidPasswordHash — SetPassword received an empty hash. The
	// auth use-case is expected to compute the bcrypt hash and pass a
	// non-empty value; an empty slice almost certainly indicates a
	// caller bug. Clearing credentials goes through ClearPassword.
	ErrInvalidPasswordHash = errors.New("identity: invalid password hash")
)
