package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/app"
	"sso/internal/audit"
	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/passwordhash"
	sadom "sso/internal/serviceaccount"

	"github.com/google/uuid"
)

// AuthenticateServiceAccountInput is the use-case payload for the OAuth2
// client_credentials grant. AppID xor AppSlug must be set; the
// mapper.appTargetFromProto helper at the gRPC boundary takes care of
// splitting the proto oneof.
//
// AppSlug is currently rejected with a validation error — the app
// repository has only GetByID. When GetBySlug lands, the slug branch
// resolves like a normal id lookup; no other code in this file changes.
type AuthenticateServiceAccountInput struct {
	ServiceAccountID string
	ClientSecret     string
	AppID            string
	AppSlug          string
	IpAddress        string
	UserAgent        string
}

// AuthenticateServiceAccountOutput is the use-case's view of a successful
// backend-to-backend authentication.
//
// Service accounts are session-less by construction: there is no
// refresh_token, no session row, no LastSeenAt bookkeeping. SAs that
// need a "new" access_token re-authenticate from scratch — the cost
// is one bcrypt-compare, comparable to a refresh-with-rotation. The
// gRPC mapper packs Access* / SubjectID into AuthTokens with the
// refresh fields left unset.
type AuthenticateServiceAccountOutput struct {
	AccessToken     string
	AccessExpiresAt time.Time
	SubjectID       string // serviceAccount.ID().String()
}

// AuthenticateServiceAccount exchanges (service_account_id, client_secret,
// app_target) for a short-lived access token (OAuth2 client_credentials
// grant). The issued token has subject_type=SERVICE_ACCOUNT; downstream
// authorization (CheckPermission, role grants) treats it identically to
// a user token apart from the session-less property.
//
// Error policy:
//   - missing / disabled / maintenance app surfaces natively (NOT_FOUND
//     and FAILED_PRECONDITION on the wire). Unlike user Login, app
//     identity is NOT collapsed into invalid-credentials — backend
//     callers need wiring-debug signal, and app_id enumeration over
//     UUIDs is impractical.
//   - missing service account AND wrong secret collapse to
//     ErrServiceAccountInvalidCredentials → INVALID_CLIENT_CREDENTIALS
//     on the wire. Backend identities deserve the same anti-enumeration
//     guarantee as user credentials.
//   - disabled service account surfaces natively.
func (s *Service) AuthenticateServiceAccount(
	ctx context.Context, in AuthenticateServiceAccountInput,
) (AuthenticateServiceAccountOutput, error) {
	if err := validateServiceAccountAuthInput(in); err != nil {
		return AuthenticateServiceAccountOutput{}, err
	}

	aud := audit.NewAuditParams{
		EventType: audit.EventTypeAuthAuthenticateServiceAccount,
		ActorType: audit.ActorTypeAnonymous,
		IpAddress: in.IpAddress,
		UserAgent: in.UserAgent,
	}

	// 1. Resolve target app.
	appID, err := app.ParseAppID(in.AppID)
	if err != nil {
		return AuthenticateServiceAccountOutput{}, err
	}
	aud.AppID = appID.String()

	a, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		if errors.Is(err, app.ErrAppNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonAppNotFound)
		} else {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		return AuthenticateServiceAccountOutput{}, err // ErrAppNotFound → NOT_FOUND at the handler
	}
	switch a.Status() {
	case app.AppStatusDisabled:
		s.auditor.Deny(ctx, aud, audit.ReasonAppDisabled)
		return AuthenticateServiceAccountOutput{}, app.ErrAppDisabled
	case app.AppStatusMaintenance:
		s.auditor.Deny(ctx, aud, audit.ReasonAppInMaintenance)
		return AuthenticateServiceAccountOutput{}, app.ErrAppInMaintenance
	}

	// 2. Resolve the service account. Missing → collapse to
	//    invalid-credentials (anti-enumeration).
	saID, err := sadom.ParseServiceAccountID(in.ServiceAccountID)
	if err != nil {
		return AuthenticateServiceAccountOutput{}, err
	}
	sa, err := s.serviceAccounts.GetByID(ctx, saID)
	if err != nil {
		if errors.Is(err, sadom.ErrServiceAccountNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonServiceAccountNotFound)
			return AuthenticateServiceAccountOutput{}, sadom.ErrServiceAccountInvalidCredentials
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return AuthenticateServiceAccountOutput{}, fmt.Errorf("auth sa: get account: %w", err)
	}
	aud.SubjectType = audit.SubjectTypeServiceAccount
	aud.SubjectID = sa.ID().String()

	if sa.Status() == sadom.ServiceAccountDisabled {
		s.auditor.Deny(ctx, aud, audit.ReasonServiceAccountDisabled)
		return AuthenticateServiceAccountOutput{}, sadom.ErrServiceAccountDisabled
	}

	// 3. Verify client secret. bcrypt-compare cost ~50–100 ms at cost=12;
	//    rate-limiting at the gRPC interceptor protects against brute force.
	if err := passwordhash.Compare(sa.SecretHash(), in.ClientSecret); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInvalidClientCredentials)
		return AuthenticateServiceAccountOutput{}, sadom.ErrServiceAccountInvalidCredentials
	}

	// 4. Mint the access token. No session, no refresh — SAs re-auth
	//    from scratch when the access token expires.
	jti, err := uuid.NewV7()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return AuthenticateServiceAccountOutput{}, fmt.Errorf("auth sa: new jti: %w", err)
	}
	access, err := s.signer.Sign(jwt.Claims{
		Subject:     sa.ID().String(),
		SubjectType: jwt.SubjectTypeServiceAccount,
		// SessionID intentionally empty; the verifier path keys off
		// SubjectType=SERVICE_ACCOUNT to skip the session lookup
		// (see grpcauth.Interceptor and usecase Validate).
		JTI: jti.String(),
	})
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return AuthenticateServiceAccountOutput{}, fmt.Errorf("auth sa: sign access token: %w", err)
	}

	s.auditor.Success(ctx, aud)

	now := s.now().UTC()
	return AuthenticateServiceAccountOutput{
		AccessToken:     access,
		AccessExpiresAt: now.Add(s.accessTTL),
		SubjectID:       sa.ID().String(),
	}, nil
}

// validateServiceAccountAuthInput enforces the basic shape contract
// before any I/O. The full UUID parse for service_account_id and app_id
// happens further down — the proto-level validators already cover the
// shape, this is the defensive re-check (same convention as Login).
func validateServiceAccountAuthInput(in AuthenticateServiceAccountInput) error {
	if in.ServiceAccountID == "" {
		return &validation.Error{Field: "service_account_id", Reason: "required"}
	}
	if in.ClientSecret == "" {
		return &validation.Error{Field: "client_secret", Reason: "required"}
	}
	if in.AppID == "" && in.AppSlug == "" {
		return &validation.Error{Field: "app_target", Reason: "required"}
	}
	if in.AppID == "" && in.AppSlug != "" {
		// The repo has only GetByID; surface a deterministic validation
		// error rather than silently dropping the request.
		return &validation.Error{
			Field:  "app_slug",
			Reason: "app_slug authentication is not yet supported; use app_id",
		}
	}
	return nil
}
