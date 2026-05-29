package domain

import (
	"errors"
)

// Sentinel errors. Use errors.Is to test.
var (
	// ErrAppNotFound — repository or use-case looked up an app_id that
	// does not exist (or has been hard-deleted).
	ErrAppNotFound = errors.New("app: not found")

	// ErrAppAlreadyExists — uniqueness constraint tripped on name or slug
	// during Create or Update. The colliding field is intentionally NOT
	// surfaced beyond the error reason.
	ErrAppAlreadyExists = errors.New("app: already exists")

	// ErrEtagMismatch — optimistic concurrency check failed: the supplied
	// expected etag does not match the current server-side etag.
	ErrEtagMismatch = errors.New("app: etag mismatch")

	// ErrAppDisabled — operation rejected because the target app is in
	// DISABLED state and the requested transition does not allow it
	// (e.g. EnterMaintenance from a disabled app).
	ErrAppDisabled = errors.New("app: is disabled")

	// ErrAppInMaintenance — operation rejected because the target app is
	// in MAINTENANCE state and the requested transition does not allow it
	// (e.g. Enable from a maintenance app — use ExitMaintenance instead).
	ErrAppInMaintenance = errors.New("app: is in maintenance")
)
