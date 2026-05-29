package auditbus

import (
	"context"
	"log/slog"
	"sso/internal/modules/audit"
)

type SyncEmitter struct {
	repo audit.Repository
	log  *slog.Logger
}

func NewSyncEmitter(r audit.Repository, log *slog.Logger) *SyncEmitter {
	return &SyncEmitter{repo: r, log: log}
}

func (e *SyncEmitter) Emit(ctx context.Context, audit *audit.Audit) error {
	sanitized := Sanitize(audit)
	if err := e.repo.Create(ctx, sanitized); err != nil {
		e.log.Warn(
			"audit.emit",
			slog.String("event_type", audit.EventType().String()),
			slog.String("actor_id", audit.ActorID().String()),
			slog.String("outcome", audit.Outcome().String()),
			slog.String("error", err.Error()),
		)
		return nil
	}
	return nil
}
