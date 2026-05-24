package service

import (
	"context"
	"errors"
	"fmt"

	"sso/internal/audit"
	"sso/internal/session"
	"sso/internal/kernel/actor"
)

// LogoutInput carries the session id of the caller, read by the
// handler from the verified-claims actor (NOT from the request body —
// Logout has no payload). Empty SessionID is rejected as a programming
// error: the handler must guarantee a populated value before reaching
// this layer.
type LogoutInput struct {
	SessionID string
}

// Logout revokes the caller's session. Idempotent: a missing or
// already-revoked session is not an error (proto contract: "Idempotent").
// Subsequent access_token presentations against this session will fail
// at the interceptor's IsActive check, regardless of the access-token's
// remaining lifetime (up to access_ttl after this call returns).
func (s *Service) Logout(ctx context.Context, in LogoutInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	sid, err := session.ParseSessionID(in.SessionID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAuthLogout)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = a.ID

	sess, err := s.sessions.GetByID(ctx, sid)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			s.auditor.Success(ctx, aud) // idempotent no-op
			return nil
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("logout: get session: %w", err)
	}

	if sess.IsRevoked() {
		s.auditor.Success(ctx, aud) // idempotent no-op
		return nil
	}

	sess.Revoke(s.now().UTC())
	if err := s.sessions.Update(ctx, sess); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("logout: persist revocation: %w", err)
	}

	s.auditor.Success(ctx, aud)
	return nil
}
