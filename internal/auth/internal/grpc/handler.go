// Package auth is the gRPC adapter for sso.auth.v1.AuthService.
//
// Methods are added one RPC at a time as the use-case surface grows.
// The embedded UnimplementedAuthServiceServer keeps the package
// buildable against new proto versions: any RPC not yet wired surfaces
// as codes.Unimplemented on the wire, not a compile error.
package grpcadapter

import (
	"context"
	"log/slog"

	authsvc "sso/internal/auth/internal/service"
	"sso/internal/identity"
	"sso/internal/kernel/actor"
	grpcauth "sso/internal/platform/grpc/auth"

	ssoauthv1 "github.com/Nergous/sso_protos/gen/go/sso/auth/v1"
	ssoidentityv1 "github.com/Nergous/sso_protos/gen/go/sso/identity/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// userToProto reuses the canonical wire format from the identity adapter.
// Importing identitygrpc here is fine — both packages are gRPC adapters
// and share the proto dependency surface (cf. access reusing
// rolegrpc.RoleToProto).
var userToProto = identity.UserToProto

// actorID extracts the caller's subject id from ctx (populated by the
// grpcauth interceptor). Returns "" for public RPCs where no token was
// presented — the auth-service surface treats that as "no actor on file".
func actorID(ctx context.Context) string {
	if a, ok := actor.From(ctx); ok {
		return a.ID
	}
	return ""
}

type Handler struct {
	ssoauthv1.UnimplementedAuthServiceServer

	svc *authsvc.Service
	log *slog.Logger
}

func NewHandler(svc *authsvc.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// RegisterServer wires the handler onto a gRPC server. Named
// RegisterServer (not Register) because Register is a proto-defined
// RPC on AuthService — the two would collide on the same receiver.
func (h *Handler) RegisterServer(s *grpc.Server) {
	ssoauthv1.RegisterAuthServiceServer(s, h)
}

// ----------------------------------------------------------------------------
// Register — global user creation. No tokens issued.
// ----------------------------------------------------------------------------
//
// Per the proto contract (sso.auth.v1.AuthService.Register), the response
// is the freshly-created sso.identity.v1.User — AIP-133: Create RPCs
// return the created resource. Token issuance happens in a subsequent
// Login call.
//
// User-enumeration mitigation: on ALREADY_EXISTS the server MUST NOT
// disclose which specific field (email vs username) collided. The
// identity domain emits a single ErrUserAlreadyExists sentinel; the
// error mapper folds it into ERROR_REASON_USER_ALREADY_EXISTS without
// any per-field detail.
//
// AvatarURL / Locale / Timezone are absent from the proto RegisterRequest
// by design — registration is intentionally minimal. Users update those
// fields later via IdentityService.UpdateUser.
func (h *Handler) Register(ctx context.Context, req *ssoauthv1.RegisterRequest) (*ssoidentityv1.User, error) {
	user, err := h.svc.Register(ctx, authsvc.RegisterInput{
		Email:       req.GetEmail(),
		Username:    req.GetUsername(),
		DisplayName: req.GetDisplayName(),
		Password:    req.GetPassword(),
		IpAddress:   grpcauth.PeerIP(ctx),
		UserAgent:   grpcauth.UserAgentFromCtx(ctx),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return userToProto(user), nil
}

func (h *Handler) Login(ctx context.Context, req *ssoauthv1.LoginRequest) (*ssoauthv1.LoginResponse, error) {
	app := appTargetFromProto(req)
	id := identifierFromProto(req)
	userAgent, deviceName := deviceFromProto(req.GetDevice())

	out, err := h.svc.Login(ctx, authsvc.LoginInput{
		AppID:      app.AppID,
		Email:      id.Email,
		Username:   id.Username,
		Password:   req.GetPassword(),
		UserAgent:  userAgent,
		IpAddress:  grpcauth.PeerIP(ctx),
		DeviceName: deviceName,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoauthv1.LoginResponse{
		Tokens: authTokensToProto(
			out.AccessToken,
			out.RefreshToken,
			out.User.ID().String(),
			out.SessionID,
			out.AccessExpiresAt,
			out.RefreshExpiresAt,
		),
	}, nil
}

func (h *Handler) Refresh(ctx context.Context, req *ssoauthv1.RefreshRequest) (*ssoauthv1.AuthTokens, error) {
	out, err := h.svc.Refresh(ctx, authsvc.RefreshInput{
		RefreshToken: req.GetRefreshToken(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return authTokensToProto(
		out.AccessToken,
		out.RefreshToken,
		out.SubjectID,
		out.SessionID,
		out.AccessExpiresAt,
		out.RefreshExpiresAt,
	), nil
}

func (h *Handler) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	a, ok := actor.From(ctx)
	if !ok || a.SessionID == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if err := h.svc.Logout(ctx, authsvc.LogoutInput{
		SessionID: a.SessionID,
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// ChangePassword — swap caller's password, revoke everything, re-issue tokens.
// ----------------------------------------------------------------------------
//
// Per the proto contract (sso.auth.v1.AuthService.ChangePassword): on
// success every existing session for the caller is revoked (including
// the one that made this call) and a freshly-minted (access_token,
// refresh_token) pair is returned inline. The client MUST replace its
// tokens.
//
// Service-account tokens are rejected at this layer: SAs are session-
// less and have no human-set password. Surfacing UNAUTHENTICATED keeps
// the policy boundary at the wire without leaking implementation detail.
//
// The new session has no DeviceInfo on the wire (ChangePasswordRequest
// carries none — unlike LoginRequest). UserAgent / DeviceName are left
// empty; the IP comes from the gRPC peer the same way Login derives it.
// In ListSessions the new row will display with an empty device label.
func (h *Handler) ChangePassword(
	ctx context.Context, req *ssoauthv1.ChangePasswordRequest,
) (*ssoauthv1.AuthTokens, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	out, err := h.svc.ChangePassword(ctx, authsvc.ChangePasswordInput{
		UserID:      a.ID,
		OldPassword: req.GetOldPassword(),
		NewPassword: req.GetNewPassword(),
		IpAddress:   grpcauth.PeerIP(ctx),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return authTokensToProto(
		out.AccessToken,
		out.RefreshToken,
		out.SubjectID,
		out.SessionID,
		out.AccessExpiresAt,
		out.RefreshExpiresAt,
	), nil
}

// ----------------------------------------------------------------------------
// ListSessions — caller's currently-active sessions, paginated.
// ----------------------------------------------------------------------------
//
// Service-account tokens are rejected: SAs are session-less and have
// nothing to list. The caller's own session id is forwarded to the
// mapper so the `is_current` flag can be stamped on the matching row.
func (h *Handler) ListSessions(
	ctx context.Context, req *ssoauthv1.ListSessionsRequest,
) (*ssoauthv1.ListSessionsResponse, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	out, err := h.svc.ListSessions(ctx, authsvc.ListSessionsInput{
		UserID:    a.ID,
		PageSize:  req.GetPageSize(),
		PageToken: req.GetPageToken(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}

	infos := make([]*ssoauthv1.SessionInfo, 0, len(out.Sessions))
	for _, sess := range out.Sessions {
		infos = append(infos, sessionInfoToProto(sess, a.SessionID))
	}
	totalSize := int32(out.TotalSize)
	return &ssoauthv1.ListSessionsResponse{
		Sessions:      infos,
		NextPageToken: out.NextPageToken,
		TotalSize:     &totalSize,
	}, nil
}

// ----------------------------------------------------------------------------
// RevokeSession — terminate one of the caller's own sessions.
// ----------------------------------------------------------------------------
//
// Ownership check lives in the use-case: a session that belongs to a
// different user surfaces as ErrSessionNotOwned → PERMISSION_DENIED,
// distinct from NOT_FOUND. Idempotent on already-revoked rows.
func (h *Handler) RevokeSession(
	ctx context.Context, req *ssoauthv1.RevokeSessionRequest,
) (*emptypb.Empty, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if err := h.svc.RevokeSession(ctx, authsvc.RevokeSessionInput{
		CallerUserID: a.ID,
		SessionID:    req.GetSessionId(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// RevokeToken — invalidate a refresh-token chain belonging to the caller.
// ----------------------------------------------------------------------------
//
// Twin of RevokeSession: same outcome (session marked revoked), addressed
// by the refresh-token plaintext the client holds instead of the
// session_id from ListSessions. Idempotent on unknown / already-revoked
// tokens; cross-user attempts surface as PERMISSION_DENIED.
//
// Service-account tokens have no refresh chain — rejected outright.
func (h *Handler) RevokeToken(
	ctx context.Context, req *ssoauthv1.RevokeTokenRequest,
) (*emptypb.Empty, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if err := h.svc.RevokeToken(ctx, authsvc.RevokeTokenInput{
		CallerUserID: a.ID,
		RefreshToken: req.GetRefreshToken(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// RevokeAllSessions — bulk log-out, optionally sparing the current session.
// ----------------------------------------------------------------------------
//
// except_current=false (default) revokes every session including the
// one that made this call — the client's tokens stop validating on
// the next ValidateToken pass and the user has to Login again.
// except_current=true keeps the calling session alive for the
// "log out everywhere except here" UX. The current session id is read
// from the verified-claims actor (the request body does not carry it).
func (h *Handler) RevokeAllSessions(
	ctx context.Context, req *ssoauthv1.RevokeAllSessionsRequest,
) (*emptypb.Empty, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	if err := h.svc.RevokeAllSessions(ctx, authsvc.RevokeAllSessionsInput{
		CallerUserID:     a.ID,
		CurrentSessionID: a.SessionID,
		ExceptCurrent:    req.GetExceptCurrent(),
	}); err != nil {
		return nil, toGRPCError(err)
	}
	return &emptypb.Empty{}, nil
}

// ----------------------------------------------------------------------------
// GenerateRecoveryCodes — issue a fresh batch of 10 one-time codes.
// ----------------------------------------------------------------------------
//
// Authenticated. The previous batch (if any) is revoked atomically at
// the use-case level before the new one is written. Plaintext is shown
// to the user exactly once; the server retains only SHA-256 digests.
//
// Service-account tokens are rejected: SAs are session-less identities
// for backend-to-backend traffic and have no human-recoverable password
// path to fall back on.
func (h *Handler) GenerateRecoveryCodes(
	ctx context.Context, _ *emptypb.Empty,
) (*ssoauthv1.RecoveryCodesResponse, error) {
	a, ok := actor.From(ctx)
	if !ok || !a.IsUser() {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}

	out, err := h.svc.GenerateRecoveryCodes(ctx, authsvc.GenerateRecoveryCodesInput{
		UserID: a.ID,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return recoveryCodesToProto(out.Codes, out.GeneratedAt), nil
}

// ----------------------------------------------------------------------------
// ResetPasswordWithRecoveryCode — unauthenticated password reset.
// ----------------------------------------------------------------------------
//
// PUBLIC RPC: no access_token in metadata. The (identifier, recovery
// code) pair carries proof-of-identity. On success the use-case:
//
//   - consumes the code (single-use, conditional UPDATE),
//   - replaces the password hash,
//   - revokes every existing session,
//   - mints a fresh session/token pair for the supplied app and
//     returns it inline — same shape as Login.
//
// Rate-limit is required server-side per the proto contract; not in
// place yet (§7). For now the use-case is the only barrier between
// brute force and the password store.
func (h *Handler) ResetPasswordWithRecoveryCode(
	ctx context.Context, req *ssoauthv1.ResetPasswordWithRecoveryCodeRequest,
) (*ssoauthv1.AuthTokens, error) {
	app := appTargetFromProto(req)
	id := identifierFromProto(req)

	out, err := h.svc.ResetPasswordWithRecoveryCode(ctx, authsvc.ResetPasswordWithRecoveryCodeInput{
		Email:        id.Email,
		Username:     id.Username,
		AppID:        app.AppID,
		AppSlug:      app.AppSlug,
		RecoveryCode: req.GetRecoveryCode(),
		NewPassword:  req.GetNewPassword(),
		IpAddress:    grpcauth.PeerIP(ctx),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return authTokensToProto(
		out.AccessToken,
		out.RefreshToken,
		out.SubjectID,
		out.SessionID,
		out.AccessExpiresAt,
		out.RefreshExpiresAt,
	), nil
}

// ----------------------------------------------------------------------------
// ValidateToken — introspect an access token. PUBLIC RPC.
// ----------------------------------------------------------------------------
//
// The grpcauth interceptor passes ValidateToken through without
// verifying anything (it is whitelisted as public). The use-case
// performs the same session-state check the interceptor would on a
// private RPC, so a revoked session surfaces immediately instead of
// waiting up to access_ttl for the JWT to expire.
//
// On any "won't validate" path (bad signature, expired, revoked,
// vanished user) the use-case returns ErrInvalidToken, which the
// error mapper folds into UNAUTHENTICATED + INVALID_TOKEN. Per the
// proto contract, the response body is populated only for tokens that
// are currently valid.
func (h *Handler) ValidateToken(
	ctx context.Context, req *ssoauthv1.ValidateTokenRequest,
) (*ssoauthv1.ValidateTokenResponse, error) {
	out, err := h.svc.Validate(ctx, authsvc.ValidateInput{
		AccessToken: req.GetAccessToken(),
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoauthv1.ValidateTokenResponse{
		SubjectId:   out.SubjectID,
		SubjectType: tokenSubjectTypeToProto(out.SubjectType),
		SessionId:   out.SessionID,
		ExpiresAt:   timestamppb.New(out.ExpiresAt),
		AppId:       out.AppID,
	}, nil
}

// ----------------------------------------------------------------------------
// AuthenticateServiceAccount — OAuth2 client_credentials grant. PUBLIC RPC.
// ----------------------------------------------------------------------------
//
// Service accounts are session-less by construction: no refresh token,
// no session row, no LastSeenAt bookkeeping. The returned AuthTokens
// carries access_token + subject_id + access_token_expires_at; the
// refresh_token / session_id / refresh_token_expires_at fields stay at
// their proto3 zero values (empty / nil), and clients re-authenticate
// from scratch when the access token expires.
//
// authTokensToProto is NOT reused here: it stamps refresh fields with
// the supplied values, and passing time.Time{} produces a "year-1"
// Timestamp on the wire which is semantically misleading. Direct
// construction makes the absence explicit.
func (h *Handler) AuthenticateServiceAccount(
	ctx context.Context, req *ssoauthv1.ServiceAccountAuthRequest,
) (*ssoauthv1.AuthTokens, error) {
	app := appTargetFromProto(req)

	out, err := h.svc.AuthenticateServiceAccount(ctx, authsvc.AuthenticateServiceAccountInput{
		ServiceAccountID: req.GetServiceAccountId(),
		ClientSecret:     req.GetClientSecret(),
		AppID:            app.AppID,
		AppSlug:          app.AppSlug,
	})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &ssoauthv1.AuthTokens{
		AccessToken:          out.AccessToken,
		SubjectId:            out.SubjectID,
		AccessTokenExpiresAt: timestamppb.New(out.AccessExpiresAt),
	}, nil
}
