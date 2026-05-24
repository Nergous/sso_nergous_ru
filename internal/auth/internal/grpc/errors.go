package grpcadapter

import (
	"errors"

	appdom "sso/internal/app"
	authsvc "sso/internal/auth/internal/service"
	identitydom "sso/internal/identity"
	"sso/internal/kernel/validation"
	grpcerr "sso/internal/platform/grpc/errors"
	recoverydom "sso/internal/recoverycode"
	sadom "sso/internal/serviceaccount"
	sessiondom "sso/internal/session"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCError maps every error that can bubble up from the auth use-case
// (including wrapped domain sentinels) onto a gRPC status + ErrorReason.
//
// Token-failure modes (invalid token, refresh-token reuse, expired or
// revoked session reachable via Refresh / ValidateToken) are intentionally
// fused into a single client-facing reason — INVALID_TOKEN — so the
// surface gives no enumeration signal. Server-side logs carry the
// distinction; the wire does not.
//
// Anti-enumeration on Login / AuthenticateServiceAccount: every
// "won't authenticate" path (missing app, disabled app, missing user,
// wrong password, no password on file, missing service account, wrong
// client secret) is already collapsed to ErrInvalidCredentials /
// ErrServiceAccountInvalidCredentials at the use-case layer, so this
// mapper only sees the fused sentinel.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// Field-level validation comes first: it's the only mapping that
	// attaches structured details (BadRequest) alongside the reason.
	var verr *validation.Error
	if errors.As(err, &verr) {
		return grpcerr.StatusWithValidation(verr)
	}

	switch {
	// ----- auth use-case sentinels ------------------------------------
	case errors.Is(err, authsvc.ErrInvalidCredentials):
		return grpcerr.StatusWithReason(codes.Unauthenticated,
			ssocommonv1.ErrorReason_ERROR_REASON_INVALID_CREDENTIALS, "invalid credentials")

	case errors.Is(err, authsvc.ErrInvalidToken),
		errors.Is(err, authsvc.ErrRefreshTokenReused):
		// RefreshTokenReused intentionally collapses to INVALID_TOKEN on
		// the wire — errors.proto has no dedicated reason, and the fused
		// reason preserves the anti-enumeration contract. The use-case
		// still emits a distinct sentinel so internal callers (logs,
		// metrics, OAuth2-BCP §4.13 session-revoke trigger) can tell
		// replay apart from generic expiry.
		return grpcerr.StatusWithReason(codes.Unauthenticated,
			ssocommonv1.ErrorReason_ERROR_REASON_INVALID_TOKEN, "invalid token")

	case errors.Is(err, authsvc.ErrUserBlocked):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_USER_BLOCKED, "user is blocked")

	case errors.Is(err, authsvc.ErrUserDeleted):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_USER_DELETED, "user is deleted")

	case errors.Is(err, recoverydom.ErrRecoveryCodeInvalid):
		// ResetPasswordWithRecoveryCode: covers "wrong code",
		// "already-used code", and "user has no active batch" — all
		// collapse to one wire reason so the response gives no
		// enumeration signal about whether codes were ever generated.
		return grpcerr.StatusWithReason(codes.Unauthenticated,
			ssocommonv1.ErrorReason_ERROR_REASON_RECOVERY_CODE_INVALID, "recovery code invalid")

	case errors.Is(err, authsvc.ErrPasswordMismatch):
		// ChangePassword: "old_password did not match" (and the
		// no-password-on-file collapse). Distinct from
		// INVALID_CREDENTIALS — the caller is already authenticated
		// via access_token, and the dedicated reason lets the client
		// drive rate-limit / UX off it specifically.
		return grpcerr.StatusWithReason(codes.Unauthenticated,
			ssocommonv1.ErrorReason_ERROR_REASON_PASSWORD_MISMATCH, "password mismatch")

	// ----- identity ---------------------------------------------------
	case errors.Is(err, identitydom.ErrUserAlreadyExists):
		// Register: never disclose which field (email vs username) collided.
		return grpcerr.StatusWithReason(codes.AlreadyExists,
			ssocommonv1.ErrorReason_ERROR_REASON_USER_ALREADY_EXISTS, "user already exists")

	case errors.Is(err, identitydom.ErrUserNotFound):
		// Surfaces on flows where the user genuinely needs to be addressable
		// (e.g. RevokeSession against a deleted user). Login/Refresh fold
		// this into ErrInvalidCredentials/ErrInvalidToken upstream.
		return grpcerr.StatusWithReason(codes.NotFound,
			ssocommonv1.ErrorReason_ERROR_REASON_USER_NOT_FOUND, "user not found")

	case errors.Is(err, identitydom.ErrUserDeleted):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_USER_DELETED, "user is deleted")

	// ----- session ----------------------------------------------------
	case errors.Is(err, sessiondom.ErrSessionNotFound):
		return grpcerr.StatusWithReason(codes.NotFound,
			ssocommonv1.ErrorReason_ERROR_REASON_SESSION_NOT_FOUND, "session not found")

	case errors.Is(err, sessiondom.ErrSessionNotOwned):
		return grpcerr.StatusWithReason(codes.PermissionDenied,
			ssocommonv1.ErrorReason_ERROR_REASON_SESSION_NOT_OWNED, "session not owned by caller")

	// ----- app --------------------------------------------------------
	// App-level errors arrive only on paths the use-case did NOT fuse
	// into INVALID_CREDENTIALS — currently service-account auth and
	// ResetPasswordWithRecoveryCode, where the caller explicitly
	// addresses an app by id/slug and a missing/disabled app is
	// informative, not an enumeration risk.
	case errors.Is(err, appdom.ErrAppNotFound):
		return grpcerr.StatusWithReason(codes.NotFound,
			ssocommonv1.ErrorReason_ERROR_REASON_APP_NOT_FOUND, "app not found")

	case errors.Is(err, appdom.ErrAppDisabled):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_APP_DISABLED, "app is disabled")

	case errors.Is(err, appdom.ErrAppInMaintenance):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_APP_IN_MAINTENANCE, "app is in maintenance")

	// ----- service accounts -------------------------------------------
	// "Account does not exist" and "secret does not match" collapse to
	// the same wire reason — backend identities deserve the same
	// anti-enumeration guarantee as user credentials.
	case errors.Is(err, sadom.ErrServiceAccountNotFound),
		errors.Is(err, sadom.ErrServiceAccountInvalidCredentials):
		return grpcerr.StatusWithReason(codes.Unauthenticated,
			ssocommonv1.ErrorReason_ERROR_REASON_INVALID_CLIENT_CREDENTIALS,
			"invalid client credentials")

	case errors.Is(err, sadom.ErrServiceAccountDisabled):
		return grpcerr.StatusWithReason(codes.FailedPrecondition,
			ssocommonv1.ErrorReason_ERROR_REASON_SERVICE_ACCOUNT_DISABLED,
			"service account is disabled")

	// ----- not yet surfaced -------------------------------------------
	// MFA sentinels (ErrMfaCodeInvalid, ErrMfaNotEnrolled,
	// ErrMfaAlreadyEnrolled, ErrMfaChallengeExpired,
	// ErrMfaChallengeConsumed, ErrRecoveryCodeInvalid) land with
	// Stage 4 — extend the switch when those packages exist.

	default:
		return status.Error(codes.Internal, "internal error")
	}
}
