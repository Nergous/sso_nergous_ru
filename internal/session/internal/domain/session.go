// Package domain holds the Session aggregate for the session bounded
// context (refresh-token-backed login sessions issued by AuthService).
//
// A Session is the server-side bookkeeping for one (user, device) pair:
// it carries the hash of the currently valid refresh token, the
// absolute hard-cap (expires_at — established at Login and never
// extended), and the sliding refresh window
// (refresh_token_expires_at — extended on each successful Refresh up
// to the hard-cap). Revocation is one-way: once revoked_at is set, the
// session never authenticates again.
package domain

import (
	"fmt"
	"time"

	"sso/internal/kernel/validation"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// Cross-context UUID handles
// ----------------------------------------------------------------------------
//
// SessionID is owned by this bounded context. UserID is a cross-context
// handle (the User aggregate is in internal/domain/identity); we
// declare it as a typed alias here to keep session free of an identity
// import.

type SessionID string
type UserID string

func NewSessionID() (SessionID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return SessionID(id.String()), nil
}

func ParseSessionID(s string) (SessionID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "session_id", Reason: "must be a valid UUID"}
	}
	return SessionID(s), nil
}

func ParseUserID(s string) (UserID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "user_id", Reason: "must be a valid UUID"}
	}
	return UserID(s), nil
}

func (s SessionID) String() string { return string(s) }
func (u UserID) String() string    { return string(u) }

// ----------------------------------------------------------------------------
// Session aggregate
// ----------------------------------------------------------------------------
//
// Field visibility split:
//
//   Unexported (only the aggregate itself can change them):
//     id, userID                — immutable after construction
//     issuedAt                  — immutable after construction
//     expiresAt                 — absolute hard-cap; set once at Login
//     refreshTokenHash          — rotated by RotateRefresh
//     refreshTokenExpiresAt     — slides on RotateRefresh, capped at expiresAt
//     lastSeenAt                — advanced by TouchLastSeen / RotateRefresh
//     revokedAt                 — zero = active; set once by Revoke (idempotent)
//
//   Exported (plain attribution data, set on Login, not mutated):
//     UserAgent, IPAddress
//
// Sessions are NOT version-tagged with an etag — there is no client-
// driven update path. The "concurrent rotation" race during Refresh is
// guarded at the repository layer via a conditional UPDATE that pins
// the previous refresh_token_hash (see repository.go).

type Session struct {
	id                    SessionID
	userID                UserID
	refreshTokenHash      []byte // SHA-256 (32 bytes)
	issuedAt              time.Time
	expiresAt             time.Time // absolute hard-cap, never extended
	refreshTokenExpiresAt time.Time // sliding window
	lastSeenAt            time.Time
	revokedAt             time.Time // zero = active

	UserAgent string
	IpAddress string
}

// NewSessionParams is what the Login use-case supplies. Both expiry
// timestamps are explicit so the policy (TTLs) lives in config / the
// use-case layer, not in the aggregate.
type NewSessionParams struct {
	ID                    SessionID
	UserID                UserID
	RefreshTokenHash      []byte
	UserAgent             string
	IpAddress             string
	Now                   time.Time
	ExpiresAt             time.Time // absolute hard-cap
	RefreshTokenExpiresAt time.Time // first sliding window
}

func NewSession(p NewSessionParams) *Session {
	return &Session{
		id:                    p.ID,
		userID:                p.UserID,
		refreshTokenHash:      p.RefreshTokenHash,
		issuedAt:              p.Now,
		expiresAt:             p.ExpiresAt,
		refreshTokenExpiresAt: p.RefreshTokenExpiresAt,
		lastSeenAt:            p.Now,
		UserAgent:             p.UserAgent,
		IpAddress:             p.IpAddress,
	}
}

// RestoreSessionParams carries the full row read back from the
// repository. Trusted (the row was written by NewSession or a mutator
// earlier); no validation.
type RestoreSessionParams struct {
	ID                    SessionID
	UserID                UserID
	RefreshTokenHash      []byte
	UserAgent             string
	IpAddress             string
	IssuedAt              time.Time
	ExpiresAt             time.Time
	RefreshTokenExpiresAt time.Time
	LastSeenAt            time.Time
	RevokedAt             time.Time // zero = not revoked
}

func RestoreSession(p RestoreSessionParams) *Session {
	return &Session{
		id:                    p.ID,
		userID:                p.UserID,
		refreshTokenHash:      p.RefreshTokenHash,
		issuedAt:              p.IssuedAt,
		expiresAt:             p.ExpiresAt,
		refreshTokenExpiresAt: p.RefreshTokenExpiresAt,
		lastSeenAt:            p.LastSeenAt,
		revokedAt:             p.RevokedAt,
		UserAgent:             p.UserAgent,
		IpAddress:             p.IpAddress,
	}
}

// ----------------------------------------------------------------------------
// Accessors
// ----------------------------------------------------------------------------

func (s *Session) ID() SessionID                    { return s.id }
func (s *Session) UserID() UserID                   { return s.userID }
func (s *Session) RefreshTokenHash() []byte         { return s.refreshTokenHash }
func (s *Session) IssuedAt() time.Time              { return s.issuedAt }
func (s *Session) ExpiresAt() time.Time             { return s.expiresAt }
func (s *Session) RefreshTokenExpiresAt() time.Time { return s.refreshTokenExpiresAt }
func (s *Session) LastSeenAt() time.Time            { return s.lastSeenAt }

// RevokedAt returns the revocation timestamp; zero time means the
// session is still active. Callers should prefer IsRevoked for
// boolean checks.
func (s *Session) RevokedAt() time.Time { return s.revokedAt }

// ----------------------------------------------------------------------------
// State predicates
// ----------------------------------------------------------------------------

// IsRevoked reports whether Revoke has been called on this session.
// Revocation is one-way and idempotent.
func (s *Session) IsRevoked() bool { return !s.revokedAt.IsZero() }

// IsAbsoluteExpired reports whether the absolute hard-cap (expiresAt)
// has passed. Once true, no Refresh can revive the session — the user
// must Login again.
func (s *Session) IsAbsoluteExpired(now time.Time) bool {
	return !now.Before(s.expiresAt)
}

// IsRefreshExpired reports whether the current sliding refresh window
// has passed. The use-case layer compares this against `now` on each
// Refresh; once true, the user must Login again even if the absolute
// hard-cap has not yet fired.
func (s *Session) IsRefreshExpired(now time.Time) bool {
	return !now.Before(s.refreshTokenExpiresAt)
}

// IsActive returns true iff the session is neither revoked nor past
// any of its expiry deadlines. Convenience for ValidateToken / Refresh.
func (s *Session) IsActive(now time.Time) bool {
	return !s.IsRevoked() && !s.IsAbsoluteExpired(now) && !s.IsRefreshExpired(now)
}

// ----------------------------------------------------------------------------
// Mutators
// ----------------------------------------------------------------------------

// Revoke marks the session as revoked. Idempotent: subsequent calls do
// nothing (revoked_at is not bumped to "now" — the original revocation
// time is preserved for the audit trail).
func (s *Session) Revoke(now time.Time) {
	if s.IsRevoked() {
		return
	}
	s.revokedAt = now
}

// TouchLastSeen advances last_seen_at without rotating credentials.
// Currently called only after a successful ValidateToken to support
// "active sessions" UX in ListSessions; the absolute and refresh-window
// deadlines are NOT changed.
func (s *Session) TouchLastSeen(now time.Time) {
	if s.IsRevoked() {
		return
	}
	s.lastSeenAt = now
}

// RotateRefresh swaps the stored refresh-token hash and slides the
// refresh window. The new window is supplied by the caller (the
// use-case layer caps it against the absolute hard-cap so the sliding
// deadline never crosses ExpiresAt).
//
// Rejects rotation on a revoked session — the repository's conditional
// UPDATE is the primary guard against concurrent rotation, but a
// belt-and-braces check at the aggregate keeps the invariant local.
func (s *Session) RotateRefresh(newHash []byte, now time.Time, newRefreshExpiresAt time.Time) error {
	if s.IsRevoked() {
		return ErrSessionRevoked
	}
	s.refreshTokenHash = newHash
	s.refreshTokenExpiresAt = newRefreshExpiresAt
	s.lastSeenAt = now
	return nil
}
