// Package role hosts the application-layer use-cases for the role
// bounded context. One method on Service per RPC in
// sso.roles.v1.RolesService, grouped across files (mirrors identity /
// app):
//
//	service.go — Service struct + helpers shared by multiple use-cases
//	create.go  — CreateRole
//	get.go     — GetRole, ListRoles (and the page-cursor codec)
//	update.go  — UpdateRole, DisableRole, EnableRole, buildPatch
//	delete.go  — PermanentlyDeleteRole
//
// Importers should alias as `rolesvc` (or whatever fits the call site)
// to avoid confusion with the `role` domain package.
package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"sso/internal/modules/audit"
	"sso/internal/modules/audit/auditx"
	"sso/internal/kernel/actor"
	"sso/internal/modules/role/internal/domain"
)

// Service exposes the role use-cases. now is injected for testability;
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

// errReasonMap maps role-domain sentinels to their audit (Outcome,
// Reason) pair. auditx.Classify handles *validation.Error and the
// default fallback; this table only enumerates per-domain sentinels.
// Role has no policy-level rejections — every recognised error is a
// Failure.
var errReasonMap = map[error]auditx.OutcomeReason{
	domain.ErrRoleNotFound:      auditx.Fail(audit.ReasonRoleNotFound),
	domain.ErrRoleAlreadyExists: auditx.Fail(audit.ReasonRoleAlreadyExists),
	domain.ErrEtagMismatch:      auditx.Fail(audit.ReasonEtagMismatch),
}

// classifyError is the per-package thin wrapper around auditx.Classify.
func classifyError(err error) (audit.AuditOutcome, string) {
	return auditx.Classify(err, errReasonMap)
}

// withOutcome is a thin alias over auditx.WithOutcome.
func withOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	return auditx.WithOutcome(p, out, reason)
}

// lifecycleTransition is the shared scaffold for Disable/Enable: load →
// mutate → save (unconditional etag). AllowMissing converts
// ErrRoleNotFound into a successful no-op (AIP-135).
func (s *Service) lifecycleTransition(
	ctx context.Context,
	rawID string,
	allowMissing bool,
	eventType audit.EventType,
	mutate func(*domain.Role),
) error {
	a, err := actor.Require(ctx)
	if err != nil {
		return err
	}

	id, err := domain.ParseRoleID(rawID)
	if err != nil {
		return err
	}

	aud := audit.BaseFromActor(a, eventType)
	aud.SubjectType = audit.SubjectTypeRole
	aud.SubjectID = id.String()

	r, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if allowMissing && errors.Is(err, domain.ErrRoleNotFound) {
			return nil
		}
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	mutate(r)
	if err := s.repo.Update(ctx, r, ""); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return err
	}

	s.auditor.Success(ctx, aud)
	return nil
}
