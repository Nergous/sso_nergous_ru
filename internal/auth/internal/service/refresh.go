package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/audit"
	"sso/internal/identity"
	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/session"

	"github.com/google/uuid"
)

// RefreshInput carries the plaintext refresh token presented by the
// client. UserAgent / IpAddress are intentionally absent: per Stage 1
// design the session's attribution fields are immutable after Login
// (a roaming user keeps their session, no field updates).
type RefreshInput struct {
	RefreshToken string
}

// RefreshOutput is the use-case's view of a successful rotation. The
// new refresh-token plaintext is returned to the client once; the
// server only retains its SHA-256 hash on the session row.
type RefreshOutput struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionID        string
	SubjectID        string
}

// Refresh rotates the supplied refresh token and mints a fresh access
// token, sliding the refresh window forward (capped at the session's
// absolute hard-cap, which is NEVER extended).
//
// All token-failure paths collapse to ErrInvalidToken (per the
// "INVALID_TOKEN" reason fusion in session/errors.go); BLOCKED user
// surfaces ErrUserBlocked; a detected replay surfaces
// ErrRefreshTokenReused AND revokes every active session for the user
// (OAuth2 BCP §4.13 token-theft response).
func (s *Service) Refresh(ctx context.Context, in RefreshInput) (*RefreshOutput, error) {
	if in.RefreshToken == "" {
		return nil, &validation.Error{Field: "refresh_token", Reason: "required"}
	}

	now := s.now().UTC()
	oldHash := s.tokenGen.Hash(in.RefreshToken)

	aud := audit.NewAuditParams{
		EventType: audit.EventTypeAuthRefresh,
		ActorType: audit.ActorTypeAnonymous,
	}

	// 1. Lookup session by hash. Unknown hash → invalid token (could be
	//    forged, already-rotated, or never-issued — all the same to us).
	sess, err := s.sessions.GetByRefreshHash(ctx, oldHash)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonInvalidToken)
			return nil, ErrInvalidToken
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: get session: %w", err)
	}
	aud.SubjectType = audit.SubjectTypeSession
	aud.SubjectID = sess.ID().String()

	// 2. Session-state checks. No defensive revoke on already-bad rows:
	//    revoked is already revoked, and expired rows TTL out on their
	//    own. We only revoke proactively on user-state failures below.
	if sess.IsRevoked() || sess.IsAbsoluteExpired(now) || sess.IsRefreshExpired(now) {
		s.auditor.Fail(ctx, aud, audit.ReasonInvalidToken)
		return nil, ErrInvalidToken
	}

	// 3. Lookup user. The session row was written by Login with a valid
	//    UUID, so ParseUserID realistically can't fail; defensive wrap
	//    keeps us out of NPE territory if it ever does.
	userID, err := identity.ParseUserID(string(sess.UserID()))
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: parse user id from session: %w", err)
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			s.revokeSessionBestEffort(ctx, sess, now)
			s.auditor.Fail(ctx, aud, audit.ReasonUserNotFound)
			return nil, ErrInvalidToken
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: get user: %w", err)
	}

	// 4. User-state checks. DELETED → invalid token (anti-enumeration);
	//    BLOCKED surfaces. Both revoke the session so a later
	//    unblock/restore doesn't resurrect the old refresh chain.
	switch user.Status() {
	case identity.UserStatusDeleted:
		s.revokeSessionBestEffort(ctx, sess, now)
		s.auditor.Deny(ctx, aud, audit.ReasonUserDeleted)
		return nil, ErrInvalidToken
	case identity.UserStatusBlocked:
		s.revokeSessionBestEffort(ctx, sess, now)
		s.auditor.Deny(ctx, aud, audit.ReasonUserBlocked)
		return nil, ErrUserBlocked
	}

	// 5. Mint the new refresh token.
	newPlain, newHash, err := s.tokenGen.Generate()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: gen new refresh token: %w", err)
	}

	// 6. Slide the refresh window. Hard-cap is never extended.
	newRefreshExpiresAt := now.Add(s.refreshRotationTTL)
	if newRefreshExpiresAt.After(sess.ExpiresAt()) {
		newRefreshExpiresAt = sess.ExpiresAt()
	}

	// 7. Aggregate mutation. Already-revoked is rejected here too —
	//    redundant given step 2, but the invariant lives at the
	//    aggregate so we honour its return.
	if err := sess.RotateRefresh(newHash, now, newRefreshExpiresAt); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: rotate aggregate: %w", err)
	}

	// 8. Conditional persist. The repository's UPDATE pins oldHash; if
	//    another rotation slipped in first, it surfaces
	//    ErrRefreshTokenReused — token theft signal. Per OAuth2 BCP
	//    §4.13 we revoke every active session for the user and surface
	//    the dedicated error so the gRPC layer can pick a distinct
	//    reason on the wire.
	if err := s.sessions.Rotate(ctx, sess, oldHash); err != nil {
		if errors.Is(err, session.ErrRefreshTokenReused) {
			s.log.WarnContext(ctx, "auth: refresh-token replay detected; revoking all user sessions",
				"session_id", sess.ID().String(),
				"user_id", user.ID().String(),
			)
			if revErr := s.sessions.RevokeAllForUser(ctx, sess.UserID(), now); revErr != nil {
				s.log.ErrorContext(ctx, "auth: revoke-all on replay failed",
					"user_id", user.ID().String(),
					"err", revErr,
				)
			}
			s.auditor.Fail(ctx, aud, audit.ReasonRefreshTokenReused)
			return nil, ErrRefreshTokenReused
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: rotate session: %w", err)
	}

	// 9. Mint JTI and sign the new access token.
	jti, err := uuid.NewV7()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: new jti: %w", err)
	}
	access, err := s.signer.Sign(jwt.Claims{
		Subject:     user.ID().String(),
		SubjectType: jwt.SubjectTypeUser,
		SessionID:   sess.ID().String(),
		JTI:         jti.String(),
	})
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return nil, fmt.Errorf("refresh: sign access token: %w", err)
	}

	s.auditor.Success(ctx, aud)

	return &RefreshOutput{
		AccessToken:      access,
		AccessExpiresAt:  now.Add(s.accessTTL),
		RefreshToken:     newPlain,
		RefreshExpiresAt: newRefreshExpiresAt,
		SessionID:        sess.ID().String(),
		SubjectID:        string(userID),
	}, nil
}
