package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/modules/session"
)

// ValidateInput carries the access token to introspect. Plaintext —
// the verifier owns signature + exp/iat/issuer checks; this use-case
// adds the session-state check that the grpcauth interceptor would
// otherwise apply on private RPCs.
type ValidateInput struct {
	AccessToken string
}

// ValidateOutput is the use-case's view of a valid token.
//
// SubjectType passes through as jwt.SubjectType so the gRPC layer
// stays the only place that knows the proto enum. SessionID is empty
// for service-account tokens (SAs are session-less by construction).
//
// AppID surfaces empty until the JWT claims gain an `app_id` field —
// the proto contract requires it in ValidateTokenResponse but the
// current jwt.Claims struct does not carry it yet. Wire-compliance
// works (string default ""); semantic-compliance lands when login.go
// and refresh.go start stamping AppID into the signing claims. The
// downstream handler reads it from this struct unchanged either way.
type ValidateOutput struct {
	SubjectID   string
	SubjectType jwt.SubjectType
	SessionID   string
	AppID       string
	ExpiresAt   time.Time
}

// Validate introspects an access token. Every "won't validate" path
// (bad signature, expired claims, revoked session, vanished user) is
// fused into ErrInvalidToken — wire-level reason fusion is documented
// in session/errors.go and the auth-handler errors map.
//
// PUBLIC RPC: the grpcauth interceptor passes ValidateToken through
// without verifying anything. This method performs the same
// session-state check that the interceptor would on a private RPC,
// so a revoked session surfaces immediately instead of waiting up to
// access_ttl for the JWT to expire.
//
// Service-account tokens skip the session lookup: SAs do not have a
// row in the sessions table, the JWT signature + expiry is the only
// check that applies.
func (s *Service) Validate(ctx context.Context, in ValidateInput) (ValidateOutput, error) {
	if in.AccessToken == "" {
		return ValidateOutput{}, &validation.Error{Field: "access_token", Reason: "required"}
	}

	claims, err := s.verifier.Verify(in.AccessToken)
	if err != nil {
		// signature mismatch, malformed JWT, wrong issuer, exp passed —
		// all collapse to a single client-facing reason.
		return ValidateOutput{}, ErrInvalidToken
	}

	switch claims.SubjectType {
	case jwt.SubjectTypeUser:
		// Resolve the session and enforce the same active-state contract
		// the interceptor applies on private RPCs.
		sid, perr := session.ParseSessionID(claims.SessionID)
		if perr != nil {
			// A user-typed token without a session_id is malformed —
			// treat as invalid token, not a validation error (the
			// caller did not supply session_id; the JWT did).
			return ValidateOutput{}, ErrInvalidToken
		}
		sess, gerr := s.sessions.GetByID(ctx, sid)
		if gerr != nil {
			if errors.Is(gerr, session.ErrSessionNotFound) {
				return ValidateOutput{}, ErrInvalidToken
			}
			return ValidateOutput{}, fmt.Errorf("validate: get session: %w", gerr)
		}
		if !sess.IsActive(s.now().UTC()) {
			return ValidateOutput{}, ErrInvalidToken
		}

	case jwt.SubjectTypeServiceAccount:
		// session-less; nothing to look up.

	default:
		// An unknown subject_type from a verified signature means the
		// issuer policy changed under us — fail closed.
		return ValidateOutput{}, ErrInvalidToken
	}

	return ValidateOutput{
		SubjectID:   claims.Subject,
		SubjectType: claims.SubjectType,
		SessionID:   claims.SessionID,
		// AppID stays empty until jwt.Claims carries it; see the
		// ValidateOutput.AppID doc comment for the migration path.
		ExpiresAt: claims.ExpiresAt,
	}, nil
}
