// Package app hosts the application-layer use-cases for the app bounded
// context. One method on Service per RPC in sso.app.v1.AppService, grouped
// across files:
//
//	service.go — Service struct + helpers shared by multiple use-cases
//	create.go  — CreateApp
//	get.go     — GetApp, ListApps (and the page-cursor codec)
//	update.go  — UpdateApp, EnableApp, DisableApp, EnterMaintenanceMode,
//	             ExitMaintenanceMode, buildPatch
//	delete.go  — PermanentlyDeleteApp
//
// The service is a thin orchestrator: it parses inputs into typed values,
// calls App mutators on aggregates loaded through Repository, and returns
// domain Apps / errors. gRPC mapping happens one layer above.
//
// The directory is internal/usecase/app and the package is `app`.
// Importers commonly alias as `appsvc` to avoid shadowing the
// bootstrap.App type at the call site.
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"sso/internal/app/internal/domain"
	"sso/internal/audit"
	"sso/internal/audit/auditx"
	"sso/internal/kernel/actor"
)

// Service exposes the app use-cases. now is injected for testability;
// production wiring uses time.Now (see bootstrap).
type Service struct {
	repo    domain.Repository
	now     func() time.Time
	auditor auditx.Auditor
}

// NewService constructs the service. now must not be nil.
func NewService(log *slog.Logger, repo domain.Repository, now func() time.Time, emitter audit.Emitter) *Service {
	return &Service{repo: repo, now: now, auditor: auditx.New(log, emitter)}
}

// EtagWildcard re-exports auditx.EtagWildcard so existing call sites
// (including tests) need no migration. New code should reference the
// auditx constant directly.
const EtagWildcard = auditx.EtagWildcard

// errReasonMap maps app-domain sentinels to their audit (Outcome,
// Reason) pair. auditx.Classify handles *validation.Error and the
// default fallback; this table only enumerates per-domain sentinels.
// App has no policy-level rejections (state-machine refusals like
// ErrAppDisabled are surfaced as Failure so the gRPC layer can choose
// the right code per RPC).
var errReasonMap = map[error]auditx.OutcomeReason{
	domain.ErrAppNotFound:      auditx.Fail(audit.ReasonAppNotFound),
	domain.ErrAppAlreadyExists: auditx.Fail(audit.ReasonAppAlreadyExists),
	domain.ErrEtagMismatch:     auditx.Fail(audit.ReasonEtagMismatch),
	domain.ErrAppDisabled:      auditx.Fail(audit.ReasonAppDisabled),
	domain.ErrAppInMaintenance: auditx.Fail(audit.ReasonAppInMaintenance),
}

// classifyError is the per-package thin wrapper around auditx.Classify.
func classifyError(err error) (audit.AuditOutcome, string) {
	return auditx.Classify(err, errReasonMap)
}

// withOutcome is a thin alias over auditx.WithOutcome.
func withOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	return auditx.WithOutcome(p, out, reason)
}

// lifecycleEmit is the shared scaffold for Disable/Enable/EnterMaintenance/
// ExitMaintenance with audit emission: load → mutate → save → emit. Mutate
// may return a domain error from a state-machine rejection (e.g.
// ErrAppDisabled when EnterMaintenance is invoked on a DISABLED app),
// which is audited as a failure with the matching reason.
//
// AllowMissing + NotFound returns nil without an emit: nothing happened,
// nothing to record. All other paths emit exactly once.
func (s *Service) lifecycleEmit(
	ctx context.Context,
	rawID string,
	allowMissing bool,
	eventType audit.EventType,
	mutate func(*domain.App) error,
) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := domain.ParseAppID(rawID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, eventType)
	aud.SubjectType = audit.SubjectTypeApp
	aud.SubjectID = id.String()
	aud.AppID = id.String()

	target, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if allowMissing && errors.Is(err, domain.ErrAppNotFound) {
			return nil
		}
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	if err := mutate(target); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	if err := s.repo.Update(ctx, target, ""); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
