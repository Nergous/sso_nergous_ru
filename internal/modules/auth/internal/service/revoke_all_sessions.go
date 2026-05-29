package service

import (
	"context"
	"fmt"

	"sso/internal/modules/audit"
	"sso/internal/modules/identity"
	"sso/internal/modules/session"
	"sso/internal/kernel/actor"
)

// RevokeAllSessionsInput carries the caller and the toggle. CurrentSessionID
// is needed only when ExceptCurrent=true so we can skip the row that
// issued this call ("log out everywhere except here").
type RevokeAllSessionsInput struct {
	CallerUserID     string
	CurrentSessionID string
	ExceptCurrent    bool
}

// RevokeAllSessions terminates every session owned by the caller, with
// an optional "keep the current one alive" toggle.
//
// ExceptCurrent=false: bulk-revoke via the repository's RevokeAllForUser
// (one UPDATE). The caller's own access_token stops validating on the
// next ValidateToken pass.
//
// ExceptCurrent=true: iterate the user's sessions and revoke each
// active row except the current one (one UPDATE per row). The repo
// does not expose a bulk "revoke all except id" variant yet; with
// O(10) sessions per user the loop is fine, and the operation is rare
// (user-initiated from a settings page).
func (s *Service) RevokeAllSessions(ctx context.Context, in RevokeAllSessionsInput) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	userID, err := identity.ParseUserID(in.CallerUserID)
	if err != nil {
		return err
	}
	sessionUserID := session.UserID(userID.String())
	now := s.now().UTC()

	aud := audit.BaseFromActor(a, audit.EventTypeAuthRevokeAllSessions)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = userID.String()

	if !in.ExceptCurrent {
		if err := s.sessions.RevokeAllForUser(ctx, sessionUserID, now); err != nil {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
			return fmt.Errorf("revoke all sessions: %w", err)
		}
		s.auditor.Success(ctx, aud)
		return nil
	}

	// ExceptCurrent path. CurrentSessionID must be populated; the
	// handler reads it from the verified-claims actor and the
	// interceptor guarantees it for user-subject calls. A missing one
	// here is a programming error rather than a client-facing condition.
	all, err := s.sessions.ListByUser(ctx, sessionUserID)
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return fmt.Errorf("revoke all sessions: list by user: %w", err)
	}
	for _, sess := range all {
		if sess.IsRevoked() {
			continue
		}
		if sess.ID().String() == in.CurrentSessionID {
			continue
		}
		sess.Revoke(now)
		if err := s.sessions.Update(ctx, sess); err != nil {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
			return fmt.Errorf("revoke all sessions: persist revocation: %w", err)
		}
	}

	s.auditor.Success(ctx, aud)
	return nil
}
