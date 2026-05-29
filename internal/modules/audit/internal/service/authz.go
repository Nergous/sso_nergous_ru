package service

import "context"

// AuditAuthorizer answers the single question the audit-read path
// needs: "may this caller read the audit log?".
//
// The interface deliberately abstracts the permission-evaluation
// strategy. Two production-ready impls are foreseen:
//
//   - access-backed: caller has role `sso.admin.audit` in app
//     `sso-admin` (depends on the seed-admin migration coming in §6.4
//     of TODO.md — until then it always denies because the app does
//     not exist yet).
//   - claims-based: AUDIT_READ scope on the access-token (future).
//
// Until §6.4 lands we wire AlwaysDenyAuthorizer in bootstrap, which
// keeps the gRPC handler reachable but returns PERMISSION_DENIED for
// every call — a safe default for an unwired admin surface.
type AuditAuthorizer interface {
	CanReadAudit(ctx context.Context) (bool, error)
}

// AlwaysDenyAuthorizer is the safe default: no caller can read the
// audit log. Replaced once `sso-admin` is seeded (§6.4).
type AlwaysDenyAuthorizer struct{}

func (AlwaysDenyAuthorizer) CanReadAudit(context.Context) (bool, error) { return false, nil }

// AlwaysAllowAuthorizer is for tests / dev only. Never wire in prod.
type AlwaysAllowAuthorizer struct{}

func (AlwaysAllowAuthorizer) CanReadAudit(context.Context) (bool, error) { return true, nil }
