// Package auditx hosts the small cross-cutting helpers that every
// use-case package re-uses: an audit Emitter wrapper that swallows the
// best-effort errors, the wire constant for "etag wildcard" plus the
// matching parser, and the canonical "clamp page_size" routine.
//
// Anything that lives here was once duplicated across identity / role /
// app / access / serviceAccount / auth (and the read-side audit
// service). The package has no domain knowledge of its own — it is a
// thin seam between the orchestrating Service structs and the
// domain/audit + kernel layers.
package auditx

import (
	"context"
	"log/slog"

	audit "sso/internal/audit/internal/domain"
)

// Auditor wraps the (log, emitter) pair every Service needs to record an
// audit event. The Emit / Success / Fail / Deny methods are best-effort:
// a build or transport error is logged at warn level and swallowed, so
// audit failures cannot block a use-case from returning its primary
// result.
//
// The value is intentionally small (two pointers) and safe to copy.
// Services typically hold one as a struct field and call through it
// directly: `s.auditor.Fail(ctx, aud, audit.ReasonInternal)`.
type Auditor struct {
	log     *slog.Logger
	emitter audit.Emitter
}

// New constructs an Auditor. Neither arg may be nil — passing nil for
// the emitter is a wiring bug that would surface as a panic at first
// Emit, so reject it at construction.
func New(log *slog.Logger, emitter audit.Emitter) Auditor {
	return Auditor{log: log, emitter: emitter}
}

// Emit builds and dispatches an audit event. Build failures (invalid
// params) and transport failures are logged at warn and swallowed.
func (a Auditor) Emit(ctx context.Context, p audit.NewAuditParams) {
	ev, err := audit.NewAudit(p)
	if err != nil {
		a.log.WarnContext(ctx, "audit build", "err", err)
		return
	}
	if err := a.emitter.Emit(ctx, ev); err != nil {
		a.log.WarnContext(ctx, "audit emit", "err", err)
	}
}

// Success stamps OutcomeSuccess on p (Reason stays empty — NewAudit
// rejects a non-empty reason on a success outcome) and emits.
func (a Auditor) Success(ctx context.Context, p audit.NewAuditParams) {
	p.Outcome = audit.OutcomeSuccess
	p.Reason = ""
	a.Emit(ctx, p)
}

// Fail stamps OutcomeFailure + reason on p and emits. Used for
// operational failures: NotFound, validation, internal etc.
func (a Auditor) Fail(ctx context.Context, p audit.NewAuditParams, reason string) {
	p.Outcome = audit.OutcomeFailure
	p.Reason = reason
	a.Emit(ctx, p)
}

// Deny stamps OutcomeDenied + reason on p and emits. Used for policy
// rejections: role disabled, user blocked, app in maintenance, etc.
func (a Auditor) Deny(ctx context.Context, p audit.NewAuditParams, reason string) {
	p.Outcome = audit.OutcomeDenied
	p.Reason = reason
	a.Emit(ctx, p)
}
