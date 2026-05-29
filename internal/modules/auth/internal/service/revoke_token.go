package service

import (
	"context"
	"errors"
	"fmt"

	"sso/internal/modules/audit"
	"sso/internal/modules/session"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
)

// RevokeTokenInput carries the caller and the refresh-token plaintext
// they want to invalidate. CallerUserID is the actor's own subject id
// from the verified-claims context; RefreshToken is the value the
// client holds locally (the server only stored its SHA-256 hash).
type RevokeTokenInput struct {
	CallerUserID string
	RefreshToken string
}

// RevokeToken invalidates the session backing a refresh-token chain.
// All access_tokens derived from that session stop validating on the
// next ValidateToken pass. Idempotent.
//
// Lookup is by hash, the same path Refresh uses; that means:
//
//   - Unknown hash (forged value, already-rotated, never-issued) is
//     treated as success — there is nothing to revoke. Surfacing
//     NOT_FOUND would leak which refresh-tokens have been seen.
//   - A real hit whose session belongs to a different user is rejected
//     with ErrSessionNotOwned (PERMISSION_DENIED on the wire). Without
//     this, a holder of any valid access_token could revoke arbitrary
//     other users' sessions by guessing refresh-tokens.
//   - An already-revoked session is a no-op.
func (s *Service) RevokeToken(ctx context.Context, in RevokeTokenInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	if in.RefreshToken == "" {
		return &validation.Error{Field: "refresh_token", Reason: "required"}
	}

	hash := s.tokenGen.Hash(in.RefreshToken)

	aud := audit.BaseFromActor(a, audit.EventTypeAuthRevokeToken)

	sess, err := s.sessions.GetByRefreshHash(ctx, hash)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			// Idempotent no-op. SubjectID stays empty (we never resolved a
			// session), so subjectType also stays unset — audit.NewAudit
			// allows this combination.
			s.auditor.Success(ctx, aud)
			return nil
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("revoke token: get session: %w", err)
	}
	aud.SubjectType = audit.SubjectTypeSession
	aud.SubjectID = sess.ID().String()

	if sess.UserID().String() != in.CallerUserID {
		s.auditor.Deny(ctx, aud, audit.ReasonPermissionDenied)
		return session.ErrSessionNotOwned
	}

	if sess.IsRevoked() {
		s.auditor.Success(ctx, aud) // idempotent no-op
		return nil
	}

	sess.Revoke(s.now().UTC())
	if err := s.sessions.Update(ctx, sess); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("revoke token: persist revocation: %w", err)
	}

	s.auditor.Success(ctx, aud)
	return nil
}
