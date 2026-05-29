package grpcadapter

import (
	"time"

	"sso/internal/platform/crypto/jwt"
	sessiondom "sso/internal/modules/session"

	ssoauthv1 "github.com/Nergous/sso_protos/gen/go/sso/auth/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ----------------------------------------------------------------------------
// Request-side helpers — normalise oneofs into plain Go values
// ----------------------------------------------------------------------------
//
// The proto contract uses oneof groups for app_target (app_id | app_slug) and
// identifier (email | username). Generated getters return the empty string
// for the un-set branch, so we expose the canonical fields by name. Exactly
// one of each pair is non-empty for a well-formed request — the proto-level
// buf.validate.oneof.required option enforces presence; the handler is
// responsible for surfacing VALIDATION_FAILED on the (currently impossible
// but defensively handled) all-empty case.
//
// Helpers use small named interfaces rather than depending on concrete
// generated request types — the same shapes are reused by LoginRequest,
// ResetPasswordWithRecoveryCodeRequest, and ServiceAccountAuthRequest.

type appTargeter interface {
	GetAppId() string
	GetAppSlug() string
}

type identifierBearer interface {
	GetEmail() string
	GetUsername() string
}

// appTarget unpacks the (app_id, app_slug) oneof. Either field is the
// canonical addressing form for the target app; the use-case decides how
// to resolve a slug into an AppID.
type appTarget struct {
	AppID   string // UUID, empty when app_slug is the set branch
	AppSlug string // canonical slug, empty when app_id is the set branch
}

func appTargetFromProto(req appTargeter) appTarget {
	return appTarget{
		AppID:   req.GetAppId(),
		AppSlug: req.GetAppSlug(),
	}
}

// identifier unpacks the (email, username) oneof.
type identifier struct {
	Email    string
	Username string
}

func identifierFromProto(req identifierBearer) identifier {
	return identifier{
		Email:    req.GetEmail(),
		Username: req.GetUsername(),
	}
}

// deviceFromProto unpacks the client-supplied DeviceInfo. ip_address is
// intentionally ignored — the handler MUST substitute the gRPC peer
// address (anti-spoofing). user_agent and device_name pass through
// verbatim (the proto-level validators bound their length).
func deviceFromProto(d *ssoauthv1.DeviceInfo) (userAgent, deviceName string) {
	if d == nil {
		return "", ""
	}
	return d.GetUserAgent(), d.GetDeviceName()
}

// ----------------------------------------------------------------------------
// Response-side helpers — render domain / use-case values onto the wire
// ----------------------------------------------------------------------------

// authTokensToProto packs a credential-bundle into the canonical
// AuthTokens message returned by Login (single-factor), Refresh,
// CompleteMfaChallenge, ChangePassword, ResetPasswordWithRecoveryCode,
// and AuthenticateServiceAccount.
//
// All six callers materialise the same fields; the helper keeps the
// field ordering in lockstep with the proto so a future field
// (e.g. id_token) lands in exactly one place.
func authTokensToProto(
	accessToken, refreshToken, subjectID, sessionID string,
	accessExpiresAt, refreshExpiresAt time.Time,
) *ssoauthv1.AuthTokens {
	return &ssoauthv1.AuthTokens{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		SubjectId:             subjectID,
		SessionId:             sessionID,
		AccessTokenExpiresAt:  timestamppb.New(accessExpiresAt),
		RefreshTokenExpiresAt: timestamppb.New(refreshExpiresAt),
	}
}

// sessionInfoToProto renders a domain.Session as one row of
// ListSessionsResponse.sessions. The caller passes its own session id
// from the verified-claims actor so the mapper stays free of ctx /
// actor imports — the "is_current" flag is the only piece of UI state
// that needs that comparison.
//
// AppId is left empty for now: the Session aggregate does not carry the
// originating app id (the JWT claim does). When ListSessions becomes
// app-aware the Session schema gains an `app_id` column and the
// mapper picks it up. Device.DeviceName is left empty for the same
// reason (not persisted in Stage 1).
func sessionInfoToProto(s *sessiondom.Session, callerSessionID string) *ssoauthv1.SessionInfo {
	return &ssoauthv1.SessionInfo{
		SessionId:  s.ID().String(),
		CreatedAt:  timestamppb.New(s.IssuedAt()),
		LastSeenAt: timestamppb.New(s.LastSeenAt()),
		ExpiresAt:  timestamppb.New(s.ExpiresAt()),
		Device: &ssoauthv1.DeviceInfo{
			UserAgent: s.UserAgent,
			IpAddress: s.IpAddress,
		},
		IsCurrent: s.ID().String() == callerSessionID,
	}
}

// tokenSubjectTypeToProto maps the JWT-layer subject kind onto the
// ValidateTokenResponse wire enum. UNSPECIFIED is never produced by a
// well-formed verified token — it is reserved for the zero value and
// surfaces only on the (currently impossible) "valid signature, missing
// subject_type" path.
func tokenSubjectTypeToProto(t jwt.SubjectType) ssoauthv1.TokenSubjectType {
	switch t {
	case jwt.SubjectTypeUser:
		return ssoauthv1.TokenSubjectType_TOKEN_SUBJECT_TYPE_USER
	case jwt.SubjectTypeServiceAccount:
		return ssoauthv1.TokenSubjectType_TOKEN_SUBJECT_TYPE_SERVICE_ACCOUNT
	}
	return ssoauthv1.TokenSubjectType_TOKEN_SUBJECT_TYPE_UNSPECIFIED
}

// recoveryCodesToProto packs a freshly-minted batch into the wire
// response. Codes are shown exactly once; the server retains only the
// salted hashes (use-case responsibility) and cannot re-display them.
func recoveryCodesToProto(codes []string, generatedAt time.Time) *ssoauthv1.RecoveryCodesResponse {
	return &ssoauthv1.RecoveryCodesResponse{
		Codes:       codes,
		GeneratedAt: timestamppb.New(generatedAt),
	}
}
