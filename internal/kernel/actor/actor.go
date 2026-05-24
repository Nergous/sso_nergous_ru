// Package actor exposes the authenticated principal carried through
// per-RPC context.
//
// The grpcauth interceptor populates an Actor after JWT verification;
// downstream code (use-cases, the audit emitter, the access service
// when stamping granted_by_user_id) reads it for authorization
// decisions and provenance. Public RPC handlers (Login, Register,
// Refresh, ValidateToken, ResetPasswordWithRecoveryCode,
// AuthenticateServiceAccount) run without an actor in context — From
// returns ok=false on those paths.
//
// kernel/actor sits at the bottom of the dependency tree: it must NOT
// import anything from internal/platform or internal/domain. The
// Kind string values are duplicated here intentionally (mirrored in
// platform/jwt.SubjectType*) so the actor contract stays independent
// of the JWT-specific representation — swapping the authenticator
// (e.g. to OAuth2/OIDC) only requires updating the interceptor's
// mapping, not every call site.
//
// Naming: `Actor.ID` / `Actor.Kind` deliberately avoid the JWT-style
// "Subject" prefix to keep them distinct from `audit.Subject*` (which
// names the object acted upon, not the actor).
package actor

import (
	"context"
	"errors"
)

// Kind enumerates the principal kinds that can authenticate against
// the service. Wire-level string values are stable.
type Kind string

const (
	KindUser           Kind = "user"
	KindServiceAccount Kind = "service_account"
)

func (k Kind) String() string { return string(k) }

// Actor is the authenticated principal for the current request plus
// the request-meta the interceptor lifts off the transport.
//
//   - ID: stringified UUIDv7 of the user or service account.
//   - Kind: USER or SERVICE_ACCOUNT.
//   - SessionID: server-side session id (UUIDv7) — present only for
//     user actors; empty for service-account JWTs, which are session-
//     less by construction.
//   - IpAddress: server-derived peer IP; empty in in-process tests.
//   - UserAgent: gRPC client's User-Agent header; empty when absent.
type Actor struct {
	ID        string
	Kind      Kind
	SessionID string
	IpAddress string
	UserAgent string
}

// IsUser reports whether the actor is a human user.
func (a Actor) IsUser() bool { return a.Kind == KindUser }

// IsServiceAccount reports whether the actor is a backend identity.
func (a Actor) IsServiceAccount() bool { return a.Kind == KindServiceAccount }

// ctxKey is the unexported type used as the context-value key. Using a
// dedicated type (not a string) avoids collisions with other packages
// that might use the same string for their own key.
type ctxKey struct{}

// ErrNoActor is returned by Require when the context carries no actor.
// In practice this means the handler ran behind the public-RPC bypass
// or the interceptor was misconfigured — usecases that hit this should
// treat it as a programming error, not a client-facing condition.
var ErrNoActor = errors.New("actor: no actor in context")

// Inject returns a derived context that carries a. Called exactly once
// per RPC by the grpcauth interceptor, after a successful Verify.
func Inject(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

// From returns the actor stored in ctx. ok=false on unauthenticated
// requests (public-RPC bypass) or when the interceptor was not wired.
//
// Callers that need to distinguish "anonymous" from "service account"
// should check ok first and then inspect Kind — never assume KindUser
// as a default.
func From(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(Actor)
	return a, ok
}

// Require returns the actor stored in ctx or ErrNoActor when the
// context is unauthenticated. Use it in usecases that the interceptor
// guarantees as authenticated — surfacing the error keeps the code
// panic-free and easy to test, while still failing loudly if the wiring
// regresses.
func Require(ctx context.Context) (Actor, error) {
	a, ok := From(ctx)
	if !ok {
		return Actor{}, ErrNoActor
	}
	return a, nil
}
