package service

import (
	"context"
	"errors"
	"fmt"

	"sso/internal/audit"
	"sso/internal/session"
	"sso/internal/kernel/actor"
)

// RevokeSessionInput carries the (caller, target) pair. CallerUserID is
// the actor's own subject id from the verified-claims context; SessionID
// is the target read from the request body.
type RevokeSessionInput struct {
	CallerUserID string
	SessionID    string
}

// RevokeSession revokes a session owned by the caller. Per the proto
// contract:
//
//   - Idempotent: a missing or already-revoked session is not an error.
//   - PERMISSION_DENIED when the session belongs to a different user
//     (ErrSessionNotOwned).
//   - NOT_FOUND only fires after ownership has been (vacuously) confirmed
//     — we surface ErrSessionNotFound for "session never existed" to
//     give the client a useful signal, distinct from the idempotent
//     "session existed and is now revoked" path.
//
// Implementation note: the proto says idempotent, but the error table
// also lists NOT_FOUND. We resolve the tension by treating "already
// revoked" as idempotent (no error, no-op) and "never existed" as
// NOT_FOUND — same shape Logout uses for its own session and
// IdentityService.PermanentlyDelete uses for hard-deletes.
func (s *Service) RevokeSession(ctx context.Context, in RevokeSessionInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	sid, err := session.ParseSessionID(in.SessionID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAuthRevokeSession)
	aud.SubjectType = audit.SubjectTypeSession
	aud.SubjectID = sid.String()

	sess, err := s.sessions.GetByID(ctx, sid)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonSessionNotFound)
			return session.ErrSessionNotFound
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("revoke session: get session: %w", err)
	}

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
		return fmt.Errorf("revoke session: persist revocation: %w", err)
	}

	s.auditor.Success(ctx, aud)
	return nil
}
