package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/audit"
	"sso/internal/identity"
	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/jwt"
	"sso/internal/platform/crypto/passwordhash"
	"sso/internal/session"

	"github.com/google/uuid"
)

// ChangePasswordInput is what the gRPC handler hands over. UserID is the
// caller's own subject id read from the verified-claims actor — the proto
// contract guarantees a caller can only change their own password.
// UserAgent / IpAddress / DeviceName are attribution for the freshly-
// minted session; for ChangePassword they originate from gRPC peer/
// metadata (not the request body — ChangePasswordRequest has no
// DeviceInfo).
type ChangePasswordInput struct {
	UserID      string
	OldPassword string
	NewPassword string
	UserAgent   string
	IpAddress   string
	DeviceName  string
}

// ChangePasswordOutput is the use-case's view of a successful password
// change: the credential bundle for the newly-issued session. The proto
// response is AuthTokens (no embedded User), so this struct deliberately
// omits the User aggregate that LoginOutput carries.
type ChangePasswordOutput struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	SessionID        string
	SubjectID        string
}

// ChangePassword swaps the caller's password and re-establishes their
// session from scratch. Per the proto contract: every existing session
// (including the one that made this call) is revoked, then a fresh
// (access, refresh) pair is issued inline. The client MUST replace its
// tokens.
//
// Distinct sentinels:
//   - ErrPasswordMismatch  — old_password did not match (PASSWORD_MISMATCH).
//   - ErrUserBlocked       — caller's account is BLOCKED (surface, not anti-enum).
//   - ErrUserDeleted       — caller's account is DELETED.
//   - ErrInvalidToken      — caller's user record vanished between token
//     issuance and this call (rare; treat as if the token went stale).
//
// "new_password equals old_password" surfaces as a validation.Error on
// the "new_password" field — the gRPC layer renders it as
// INVALID_ARGUMENT with a BadRequest detail.
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) (ChangePasswordOutput, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return ChangePasswordOutput{}, err
	}
	if err := validateChangePasswordInput(in); err != nil {
		return ChangePasswordOutput{}, err
	}

	userID, err := identity.ParseUserID(in.UserID)
	if err != nil {
		return ChangePasswordOutput{}, err
	}

	aud := audit.BaseFromActor(a, audit.EventTypeAuthChangePassword)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = userID.String()

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			// Token was valid at the interceptor but the user vanished
			// before this call landed. Surface as INVALID_TOKEN — the
			// client's session is effectively dead.
			s.auditor.Fail(ctx, aud, audit.ReasonInvalidToken)
			return ChangePasswordOutput{}, ErrInvalidToken
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: get user: %w", err)
	}

	switch user.Status() {
	case identity.UserStatusDeleted:
		s.auditor.Deny(ctx, aud, audit.ReasonUserDeleted)
		return ChangePasswordOutput{}, ErrUserDeleted
	case identity.UserStatusBlocked:
		s.auditor.Deny(ctx, aud, audit.ReasonUserBlocked)
		return ChangePasswordOutput{}, ErrUserBlocked
	}

	// No password on file collapses to PASSWORD_MISMATCH: the only honest
	// answer when there's nothing to compare against, and indistinguishable
	// on the wire from a genuinely-wrong old password (anti-enumeration of
	// admin-created accounts).
	if !user.HasPassword() {
		s.auditor.Fail(ctx, aud, audit.ReasonPasswordMismatch)
		return ChangePasswordOutput{}, ErrPasswordMismatch
	}
	if err := passwordhash.Compare(user.PasswordHash(), in.OldPassword); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonPasswordMismatch)
		return ChangePasswordOutput{}, ErrPasswordMismatch
	}

	newHash, err := passwordhash.Hash(in.NewPassword, s.bcryptCost)
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: hash new password: %w", err)
	}

	now := s.now().UTC()
	// Snapshot the on-disk etag BEFORE SetPassword bumps the aggregate's
	// version. The repository's UpdateUserPasswordWithEtag pins this
	// value in the WHERE clause; passing the post-bump etag would always
	// miss its own row.
	preEtag := user.Etag()
	if err := user.SetPassword(newHash, now); err != nil {
		// SetPassword rejects DELETED (already filtered above) and an
		// empty hash (we just produced one); reaching here is a
		// programming error, so wrap and let it surface as internal.
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: set password: %w", err)
	}

	// A concurrent IdentityService.UpdateUser bumping the etag between
	// our GetByID and this UPDATE would surface as ErrEtagMismatch and
	// bubble up as internal — rare, and a retry from the client is a
	// sensible response.
	if err := s.users.UpdatePassword(ctx, user, preEtag); err != nil {
		if errors.Is(err, identity.ErrEtagMismatch) {
			s.auditor.Fail(ctx, aud, audit.ReasonEtagMismatch)
		} else {
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		return ChangePasswordOutput{}, fmt.Errorf("change password: persist password: %w", err)
	}

	// Revoke every existing session for the user, including the one that
	// made this call. The fresh session minted below is the only one
	// left active after this point.
	if err := s.sessions.RevokeAllForUser(ctx, session.UserID(user.ID().String()), now); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: revoke sessions: %w", err)
	}

	// ----- mint a fresh session + token pair (mirrors login.go steps 6-10) ---

	sessionID, err := session.NewSessionID()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: new session id: %w", err)
	}
	jti, err := uuid.NewV7()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: new jti: %w", err)
	}
	refreshPlain, refreshHash, err := s.tokenGen.Generate()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: gen refresh token: %w", err)
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
		return ChangePasswordOutput{}, fmt.Errorf("change password: create session: %w", err)
	}

	access, err := s.signer.Sign(jwt.Claims{
		Subject:     user.ID().String(),
		SubjectType: jwt.SubjectTypeUser,
		SessionID:   sess.ID().String(),
		JTI:         jti.String(),
	})
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return ChangePasswordOutput{}, fmt.Errorf("change password: sign access token: %w", err)
	}

	s.auditor.Success(ctx, aud)

	return ChangePasswordOutput{
		AccessToken:      access,
		AccessExpiresAt:  now.Add(s.accessTTL),
		RefreshToken:     refreshPlain,
		RefreshExpiresAt: refreshExpiresAt,
		SessionID:        sess.ID().String(),
		SubjectID:        user.ID().String(),
	}, nil
}

// validateChangePasswordInput re-checks the invariants the gRPC layer
// already enforces (protovalidate on length, actor presence on UserID),
// so the use-case stays self-defensive against any future caller.
//
// "new == old" is checked here because it is a semantic rule, not a
// length / format rule — protovalidate cannot express it.
func validateChangePasswordInput(in ChangePasswordInput) error {
	if in.UserID == "" {
		return &validation.Error{Field: "user_id", Reason: "required"}
	}
	if in.OldPassword == "" {
		return &validation.Error{Field: "old_password", Reason: "required"}
	}
	if in.NewPassword == "" {
		return &validation.Error{Field: "new_password", Reason: "required"}
	}
	if in.NewPassword == in.OldPassword {
		return &validation.Error{
			Field:  "new_password",
			Reason: "must differ from old_password",
		}
	}
	return nil
}
