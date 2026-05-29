// Package service hosts the application-layer use-cases for the identity
// bounded context. One method on Service per RPC in
// sso.domain.v1.IdentityService, grouped by intent across files:
//
//	service.go — Service struct + helpers shared by multiple use-cases
//	create.go  — CreateUser
//	get.go     — GetUser, ListUsers (and the page-cursor codec)
//	update.go  — UpdateUser, DisableUser, EnableUser, buildPatch
//	delete.go  — SoftDeleteUser, PermanentlyDeleteUser
//
// The service is a thin orchestrator: it parses inputs into typed values,
// calls User mutators on aggregates loaded through Repository, and returns
// domain Users / errors. gRPC mapping happens one layer above.
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"sso/internal/modules/audit"
	"sso/internal/modules/audit/auditx"
	"sso/internal/modules/identity/internal/domain"
	"sso/internal/kernel/actor"
)

// Service exposes the identity use-cases. now is injected for testability;
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

// errReasonMap maps identity-domain sentinels to their audit (Outcome,
// Reason) pair. auditx.Classify handles *validation.Error and the
// default fallback; this table only enumerates per-domain sentinels.
// Identity has no policy-level rejections — every recognised error is a
// Failure.
var errReasonMap = map[error]auditx.OutcomeReason{
	domain.ErrUserNotFound:      auditx.Fail(audit.ReasonUserNotFound),
	domain.ErrUserAlreadyExists: auditx.Fail(audit.ReasonUserAlreadyExists),
	domain.ErrUserDeleted:       auditx.Fail(audit.ReasonUserDeleted),
	domain.ErrUserNotDeleted:    auditx.Fail(audit.ReasonUserNotDeleted),
	domain.ErrEtagMismatch:      auditx.Fail(audit.ReasonEtagMismatch),
}

// classifyError is the per-package thin wrapper around auditx.Classify.
// Kept as a one-liner so the dozen call sites in this package read
// `classifyError(err)` instead of repeating the table reference.
func classifyError(err error) (audit.AuditOutcome, string) {
	return auditx.Classify(err, errReasonMap)
}

// lifecycleEmit is the shared scaffold for Disable/Enable/SoftDelete with
// audit emission: load → mutate → save → emit. Mutate may return a domain
// error (e.g. ErrUserDeleted from Disable/Enable on a deleted user) which
// is audited as a failure via classifyError.
//
// AllowMissing + NotFound returns nil without an emit: nothing happened,
// nothing to record. All other paths emit exactly once.
func (s *Service) lifecycleEmit(
	ctx context.Context,
	rawID string,
	allowMissing bool,
	eventType audit.EventType,
	mutate func(*domain.User) error,
) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}
	id, err := domain.ParseUserID(rawID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, eventType)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = id.String()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if allowMissing && errors.Is(err, domain.ErrUserNotFound) {
			return nil
		}
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	if err := mutate(user); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	if err := s.repo.Update(ctx, user, ""); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}

// withOutcome is a thin alias over auditx.WithOutcome so the call sites
// in this package keep reading `withOutcome(aud, out, reason)`.
func withOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	return auditx.WithOutcome(p, out, reason)
}
