package domain

import "errors"

// Sentinel errors owned by the session bounded context. The auth
// use-case translates these into the proto's generic INVALID_TOKEN
// reason at the wire (see errors.proto: token-failure modes are
// intentionally fused into one client-facing reason for security).
var (
	// ErrSessionNotFound — no row matches the supplied id or refresh-
	// token hash.
	ErrSessionNotFound = errors.New("session: not found")

	// ErrSessionRevoked — the session was revoked (Logout, RevokeSession,
	// RevokeAllSessions, or implicit revocation triggered by suspected
	// refresh-token theft).
	ErrSessionRevoked = errors.New("session: revoked")

	// ErrSessionExpired — either the absolute hard-cap or the current
	// sliding refresh window has passed. Use-case decides which signal
	// to surface; both fold into the same wire-level reason.
	ErrSessionExpired = errors.New("session: expired")

	// ErrSessionNotOwned — the caller's subject does not match the
	// session's user. Surfaces on RevokeSession when the targeted id
	// belongs to a different user.
	ErrSessionNotOwned = errors.New("session: not owned by caller")

	// ErrRefreshTokenReused — a Refresh tried to rotate using a hash
	// that no longer matches the row (another rotation already happened
	// with the same starting state). Per OAuth2 token-theft guidance,
	// the use-case layer should treat this as "session compromised" and
	// revoke the entire chain.
	ErrRefreshTokenReused = errors.New("session: refresh token reused")
)
