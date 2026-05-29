package domain

import (
	"context"
	"time"
)

// Repository is the persistence contract for sessions.
//
// Concurrency model:
//   - Create is idempotent on (id) — id is server-generated UUIDv7 so
//     collisions are not expected.
//   - Update is unconditional. Used by Revoke and TouchLastSeen, where
//     the last writer wins is acceptable (revocation is monotone; the
//     last-seen timestamp is advisory).
//   - Rotate is conditional. It expects the caller to pin the previous
//     refresh-token hash; the repository performs an
//     UPDATE ... WHERE id = ? AND refresh_token_hash = ?. A 0-rows-
//     affected outcome means another rotation slipped in first — the
//     repository surfaces ErrRefreshTokenReused so the use-case layer
//     can treat it as a token-theft signal and revoke the whole
//     session per OAuth2 BCP.
type Repository interface {
	Create(ctx context.Context, s *Session) error

	// GetByID looks up a session by its server-side id. Used by
	// ValidateToken (the access-token claim carries session_id) and
	// RevokeSession.
	GetByID(ctx context.Context, id SessionID) (*Session, error)

	// GetByRefreshHash looks up a session by the SHA-256 hash of the
	// presented refresh token. Used exclusively by Refresh.
	GetByRefreshHash(ctx context.Context, hash []byte) (*Session, error)

	// Update writes the aggregate's current state unconditionally. Used
	// by Revoke and TouchLastSeen.
	Update(ctx context.Context, s *Session) error

	// Rotate atomically swaps the refresh-token hash. Returns
	// ErrRefreshTokenReused if the row's hash on disk does not match
	// expectedRefreshHash (= concurrent rotation, presumed token theft).
	Rotate(ctx context.Context, s *Session, expectedRefreshHash []byte) error

	// ListByUser returns the user's sessions ordered by issued_at DESC
	// (most recent first). Used by ListSessions in stage 2.
	ListByUser(ctx context.Context, userID UserID) ([]*Session, error)

	// RevokeAllForUser bulk-revokes every active session for the user.
	// Used by RevokeAllSessions and by ChangePassword (with the
	// "revoke other sessions" toggle) in stage 2.
	RevokeAllForUser(ctx context.Context, userID UserID, now time.Time) error
}
