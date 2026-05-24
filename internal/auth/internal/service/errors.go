package service

import "errors"

var (
	// ErrInvalidCredentials covers every "the supplied (email|username,
	// password) pair will not authenticate" path used by Login: no such
	// user, wrong password, no password on file, soft-deleted user,
	// missing/disabled app. The handler maps to UNAUTHENTICATED with
	// reason INVALID_CREDENTIALS — never reveal which sub-condition
	// tripped (anti-enumeration).
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrUserBlocked is the one auth failure surfaced to the client,
	// because the legitimate user needs to know their account is on
	// hold (vs. believing they typed the password wrong). Maps to
	// PERMISSION_DENIED with reason USER_BLOCKED.
	ErrUserBlocked = errors.New("auth: user blocked")

	// ErrUserDeleted is currently unused on the wire (deleted users
	// collapse to ErrInvalidCredentials in Login/Refresh for anti-
	// enumeration). Kept for ChangePassword/ResetPassword paths that
	// legitimately need to surface the lifecycle state.
	ErrUserDeleted = errors.New("auth: user deleted")

	// ErrInvalidToken covers every "the supplied token is no longer
	// valid" path used by Refresh and ValidateToken: unknown refresh
	// hash, revoked session, absolute or sliding deadline passed,
	// associated user vanished. The handler maps to UNAUTHENTICATED
	// with reason INVALID_TOKEN — token-failure modes are intentionally
	// fused into one client-facing reason for security (see
	// session/errors.go).
	ErrInvalidToken = errors.New("auth: invalid token")

	// ErrRefreshTokenReused is the dedicated signal for "this refresh
	// token was already rotated" (OAuth2 BCP §4.13). Distinct from
	// ErrInvalidToken so the gRPC layer can map it to its own reason
	// (REFRESH_TOKEN_REUSED) and the client UI can warn the user that
	// their session may have been compromised. Whenever this fires the
	// use-case ALSO revokes every active session for the user.
	ErrRefreshTokenReused = errors.New("auth: refresh token reused")

	// ErrPasswordMismatch is the ChangePassword-only "old_password did
	// not match" sentinel. Distinct from ErrInvalidCredentials because
	// the caller is already authenticated via access_token — the wire
	// reason (PASSWORD_MISMATCH) carries that nuance for rate-limit and
	// UX purposes. Also returned when the caller has no password on
	// file (admin-created account): without a real comparison target,
	// "mismatch" is the only honest answer and matches what the user
	// sees on a wrong password.
	ErrPasswordMismatch = errors.New("auth: password mismatch")
)
