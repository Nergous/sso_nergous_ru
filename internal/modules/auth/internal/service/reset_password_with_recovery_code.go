package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/modules/app"
	"sso/internal/modules/audit"
	"sso/internal/modules/identity"
	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/passwordhash"
	"sso/internal/modules/recoverycode"
	"sso/internal/modules/session"

	"github.com/google/uuid"
)

// ResetPasswordWithRecoveryCodeInput is what the gRPC handler hands
// over after unpacking the proto. The (Email, Username) and
// (AppID, AppSlug) pairs are oneof — exactly one of each is non-empty.
// UserAgent / IpAddress / DeviceName attribute the new session that
// gets minted on success.
type ResetPasswordWithRecoveryCodeInput struct {
	Email        string
	Username     string
	AppID        string
	AppSlug      string
	RecoveryCode string
	NewPassword  string
	UserAgent    string
	IpAddress    string
	DeviceName   string
}

// ResetPasswordWithRecoveryCodeOutput mirrors ChangePasswordOutput: the
// credentials backing the freshly-issued session. Proto returns
// AuthTokens, so no User aggregate is exposed.
type ResetPasswordWithRecoveryCodeOutput struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionID        string
	SubjectID        string
}

// ResetPasswordWithRecoveryCode performs the unauthenticated password
// reset flow:
//
//  1. Resolve (identifier, app) — same anti-enumeration discipline as
//     Login for the app side; the user side surfaces NOT_FOUND when
//     no identifier matches (proto-permitted, mitigated by rate-limit).
//  2. Consume a code from the user's active batch — the conditional
//     UPDATE at the repo layer is the single-use guard against parallel
//     consume of the same code.
//  3. Hash the new password and persist it.
//  4. Revoke every existing session (forced log-out everywhere).
//  5. Mint a fresh session + token pair for the supplied app and
//     return it inline — same wire shape as Login.
//
// On every "won't reset" path the use-case never leaks whether the code
// itself was wrong vs. whether the user has no active batch — both
// collapse to ErrRecoveryCodeInvalid (anti-enumeration).
func (s *Service) ResetPasswordWithRecoveryCode(
	ctx context.Context, in ResetPasswordWithRecoveryCodeInput,
) (ResetPasswordWithRecoveryCodeOutput, error) {
	if err := validateResetPasswordInput(in); err != nil {
		return ResetPasswordWithRecoveryCodeOutput{}, err
	}

	aud := audit.NewAuditParams{
		EventType: audit.EventTypeAuthResetPasswordWithRecoveryCode,
		ActorType: audit.ActorTypeAnonymous,
		IpAddress: in.IpAddress,
		UserAgent: in.UserAgent,
	}

	// Resolve the app first — like Login, missing/disabled apps short-
	// circuit before we touch the user record (no point looking the
	// user up if we have nowhere to issue a session for).
	a, err := s.resolveApp(ctx, in.AppID, in.AppSlug)
	if err != nil {
		if errors.Is(err, app.ErrAppNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonAppNotFound)
		} else {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		return ResetPasswordWithRecoveryCodeOutput{}, err
	}
	aud.AppID = a.ID().String()

	if a.Status() != app.AppStatusActive {
		switch a.Status() {
		case app.AppStatusDisabled:
			s.auditor.Deny(ctx, aud, audit.ReasonAppDisabled)
		case app.AppStatusMaintenance:
			s.auditor.Deny(ctx, aud, audit.ReasonAppInMaintenance)
		default:
			s.auditor.Deny(ctx, aud, audit.ReasonInternal)
		}
		return ResetPasswordWithRecoveryCodeOutput{}, app.ErrAppDisabled
	}

	user, err := s.lookupUser(ctx, in.Email, in.Username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonUserNotFound)
		} else {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		// NOT_FOUND is permitted by the proto here (unlike Login).
		// Rate-limit on this RPC is the mitigation against user
		// enumeration (§7).
		return ResetPasswordWithRecoveryCodeOutput{}, err
	}
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = user.ID().String()

	switch user.Status() {
	case identity.UserStatusDeleted:
		s.auditor.Deny(ctx, aud, audit.ReasonUserDeleted)
		return ResetPasswordWithRecoveryCodeOutput{}, ErrUserDeleted
	case identity.UserStatusBlocked:
		s.auditor.Deny(ctx, aud, audit.ReasonUserBlocked)
		return ResetPasswordWithRecoveryCodeOutput{}, ErrUserBlocked
	}

	// Look up the active batch. "No batch" and "wrong code" collapse to
	// the same wire reason — the response gives no signal about whether
	// the user ever generated codes.
	batch, err := s.recoveryCodes.GetActiveBatchByUser(ctx, recoverycode.UserID(user.ID().String()))
	if err != nil {
		if errors.Is(err, recoverycode.ErrBatchNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonRecoveryCodeInvalid)
			return ResetPasswordWithRecoveryCodeOutput{}, recoverycode.ErrRecoveryCodeInvalid
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: get active batch: %w", err)
	}

	now := s.now().UTC()
	hash := s.recoveryGen.Hash(in.RecoveryCode)
	if err := s.recoveryCodes.ConsumeCode(ctx, batch.ID(), hash, now); err != nil {
		// ErrRecoveryCodeInvalid → surface as-is; anything else is internal.
		if errors.Is(err, recoverycode.ErrRecoveryCodeInvalid) {
			s.auditor.Fail(ctx, aud, audit.ReasonRecoveryCodeInvalid)
			return ResetPasswordWithRecoveryCodeOutput{}, err
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: consume code: %w", err)
	}

	newHash, err := passwordhash.Hash(in.NewPassword, s.bcryptCost)
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: hash new password: %w", err)
	}
	// Snapshot the pre-bump etag — UpdateUserPasswordWithEtag uses it
	// as the optimistic-lock key. Same rationale as ChangePassword.
	preEtag := user.Etag()
	if err := user.SetPassword(newHash, now); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: set password: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, user, preEtag); err != nil {
		if errors.Is(err, identity.ErrEtagMismatch) {
			s.auditor.Fail(ctx, aud, audit.ReasonEtagMismatch)
		} else {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: persist password: %w", err)
	}

	if err := s.sessions.RevokeAllForUser(ctx, session.UserID(user.ID().String()), now); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: revoke sessions: %w", err)
	}

	// ----- mint a fresh session + tokens (mirrors login.go steps 6-10) ------

	sessionID, err := session.NewSessionID()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: new session id: %w", err)
	}
	jti, err := uuid.NewV7()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: new jti: %w", err)
	}
	refreshPlain, refreshHash, err := s.tokenGen.Generate()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: gen refresh token: %w", err)
	}

	sessionExpiresAt := now.Add(s.refreshTTL)
	refreshExpiresAt := now.Add(s.refreshRotationTTL)
	if refreshExpiresAt.After(sessionExpiresAt) {
		refreshExpiresAt = sessionExpiresAt
	}

	sess := session.NewSession(session.NewSessionParams{
		ID:                    sessionID,
		UserID:                session.UserID(user.ID().String()),
		RefreshTokenHash:      refreshHash,
		UserAgent:             in.UserAgent,
		IpAddress:             in.IpAddress,
		Now:                   now,
		ExpiresAt:             sessionExpiresAt,
		RefreshTokenExpiresAt: refreshExpiresAt,
	})
	if err := s.sessions.Create(ctx, sess); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: create session: %w", err)
	}

	access, err := s.signer.Sign(jwt.Claims{
		Subject:     user.ID().String(),
		SubjectType: jwt.SubjectTypeUser,
		SessionID:   sess.ID().String(),
		JTI:         jti.String(),
	})
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ResetPasswordWithRecoveryCodeOutput{}, fmt.Errorf("reset password: sign access token: %w", err)
	}

	s.auditor.Success(ctx, aud)

	return ResetPasswordWithRecoveryCodeOutput{
		AccessToken:      access,
		AccessExpiresAt:  now.Add(s.accessTTL),
		RefreshToken:     refreshPlain,
		RefreshExpiresAt: refreshExpiresAt,
		SessionID:        sess.ID().String(),
		SubjectID:        user.ID().String(),
	}, nil
}

// validateResetPasswordInput enforces the oneof constraints
// (exactly-one identifier, exactly-one app target) plus non-empty
// recovery_code / new_password. Length / pattern bounds land at the
// proto layer via protovalidate.
func validateResetPasswordInput(in ResetPasswordWithRecoveryCodeInput) error {
	hasEmail := in.Email != ""
	hasUsername := in.Username != ""
	if hasEmail == hasUsername {
		return &validation.Error{
			Field:  "email_or_username",
			Reason: "exactly one of email or username must be provided",
		}
	}
	hasAppID := in.AppID != ""
	hasAppSlug := in.AppSlug != ""
	if hasAppID == hasAppSlug {
		return &validation.Error{
			Field:  "app_target",
			Reason: "exactly one of app_id or app_slug must be provided",
		}
	}
	if in.RecoveryCode == "" {
		return &validation.Error{Field: "recovery_code", Reason: "required"}
	}
	if in.NewPassword == "" {
		return &validation.Error{Field: "new_password", Reason: "required"}
	}
	return nil
}

// resolveApp dispatches to the matching repo method based on which
// branch of the app_target oneof was populated. AppSlug is rejected
// here until app.Repository.GetBySlug lands — same compromise as
// AuthenticateServiceAccount (see service_account.go), and the call
// site is already gated by validateResetPasswordInput's exactly-one
// check.
func (s *Service) resolveApp(ctx context.Context, appID, appSlug string) (*app.App, error) {
	if appSlug != "" {
		return nil, &validation.Error{
			Field:  "app_slug",
			Reason: "app_slug resolution is not yet supported; use app_id",
		}
	}
	id, err := app.ParseAppID(appID)
	if err != nil {
		return nil, err
	}
	return s.apps.GetByID(ctx, id)
}
