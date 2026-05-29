// Package serviceAccount hosts the application-layer use-cases for the
// service-account bounded context. Mirrors usecase/role:
//
//	service.go — Service struct + helpers
//	create.go  — CreateServiceAccount
//	get.go     — GetServiceAccount, ListServiceAccounts (page-cursor codec)
//	update.go  — UpdateServiceAccount, DisableServiceAccount, EnableServiceAccount
//	delete.go  — PermanentlyDeleteServiceAccount
//
// Importers should alias as `sasvc` to avoid confusion with the
// `serviceAccount` domain package.
package service

import (
	"log/slog"
	"time"

	"sso/internal/modules/audit"
	"sso/internal/modules/audit/auditx"
	serviceAccount "sso/internal/modules/serviceaccount/internal/domain"
)

type Service struct {
	repo    serviceAccount.Repository
	now     func() time.Time
	auditor auditx.Auditor
}

func NewService(log *slog.Logger, repo serviceAccount.Repository, now func() time.Time, emitter audit.Emitter) *Service {
	return &Service{repo: repo, now: now, auditor: auditx.New(log, emitter)}
}

// EtagWildcard re-exports auditx.EtagWildcard so existing call sites
// (including tests) need no migration. New code should reference the
// auditx constant directly.
const EtagWildcard = auditx.EtagWildcard

// errReasonMap maps service-account sentinels to their audit (Outcome,
// Reason) pair. auditx.Classify handles *validation.Error and the
// default fallback; this table only enumerates per-domain sentinels.
var errReasonMap = map[error]auditx.OutcomeReason{
	serviceAccount.ErrServiceAccountNotFound:      auditx.Fail(audit.ReasonServiceAccountNotFound),
	serviceAccount.ErrServiceAccountAlreadyExists: auditx.Fail(audit.ReasonServiceAccountAlreadyExists),
	serviceAccount.ErrEtagMismatch:                auditx.Fail(audit.ReasonEtagMismatch),
}

// classifyError is the per-package thin wrapper around auditx.Classify.
func classifyError(err error) (audit.AuditOutcome, string) {
	return auditx.Classify(err, errReasonMap)
}

// withOutcome is a thin alias over auditx.WithOutcome.
func withOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	return auditx.WithOutcome(p, out, reason)
}
