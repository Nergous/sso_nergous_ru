package auditx

import (
	"errors"

	audit "sso/internal/audit/internal/domain"
	"sso/internal/kernel/validation"
)

// OutcomeReason pairs an audit outcome with its reason code. Used as the
// value type in classification maps that drive per-service Classify
// calls.
type OutcomeReason struct {
	Outcome audit.AuditOutcome
	Reason  string
}

// Fail constructs an OutcomeReason with audit.OutcomeFailure. Use for
// operational errors (not-found, etag mismatch, internal).
func Fail(reason string) OutcomeReason {
	return OutcomeReason{Outcome: audit.OutcomeFailure, Reason: reason}
}

// Deny constructs an OutcomeReason with audit.OutcomeDenied. Use for
// policy-style rejections (role disabled, user blocked, app in
// maintenance, RBAC failure).
func Deny(reason string) OutcomeReason {
	return OutcomeReason{Outcome: audit.OutcomeDenied, Reason: reason}
}

// Classify maps a domain/use-case error to an (Outcome, Reason) pair.
//
// Lookup order:
//  1. *validation.Error → (OutcomeFailure, ReasonValidationFailed). Tested
//     first so a per-service map does not have to list it explicitly.
//  2. The first errors.Is match against a key in m → the entry's pair.
//  3. Fallback → (OutcomeFailure, ReasonInternal).
//
// Map iteration order is undefined; entries must be mutually exclusive
// under errors.Is (in practice they always are — wrapped sentinels do
// not collide).
func Classify(err error, m map[error]OutcomeReason) (audit.AuditOutcome, string) {
	var verr *validation.Error
	if errors.As(err, &verr) {
		return audit.OutcomeFailure, audit.ReasonValidationFailed
	}
	for sentinel, pair := range m {
		if errors.Is(err, sentinel) {
			return pair.Outcome, pair.Reason
		}
	}
	return audit.OutcomeFailure, audit.ReasonInternal
}

// WithOutcome stamps outcome+reason on params without mutating the
// caller's value — returns a fresh struct ready for Emit. Used at the
// failure branch of every mutating use-case after Classify has selected
// the right pair.
func WithOutcome(p audit.NewAuditParams, out audit.AuditOutcome, reason string) audit.NewAuditParams {
	p.Outcome = out
	p.Reason = reason
	return p
}
