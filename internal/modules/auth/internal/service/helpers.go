package service

import (
	"context"
	"time"

	"sso/internal/modules/session"
)

// revokeSessionBestEffort marks the session revoked in-aggregate and
// persists it. Errors are logged and swallowed — every caller of this
// helper is on a path where the request itself is already failing
// (deleted user, blocked user, replay-detected) and the credential
// error is what we owe the client. A failed persist leaves the row
// active until its TTL hits, which is a tolerable degradation.
func (s *Service) revokeSessionBestEffort(ctx context.Context, sess *session.Session, now time.Time) {
	sess.Revoke(now)
	if err := s.sessions.Update(ctx, sess); err != nil {
		s.log.WarnContext(ctx, "auth: revoke session best-effort: persist failed",
			"session_id", sess.ID().String(),
			"user_id", sess.UserID().String(),
			"err", err,
		)
	}
}
