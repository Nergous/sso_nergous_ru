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
	"sso/internal/modules/session"

	"github.com/google/uuid"
)

// LoginInput is what the gRPC handler hands over after unpacking the
// proto. IpAddress is the authoritative peer address (server-derived,
// never the client-claimed value).
type LoginInput struct {
	AppID      string
	Email      string // exactly one of Email/Username must be set
	Username   string
	Password   string // plaintext; bcrypt-compared against user.PasswordHash
	UserAgent  string
	IpAddress  string
	DeviceName string
}

// LoginOutput is the use-case's view of a successful login. RefreshToken
// is the plaintext value the client must store; the server only keeps
// the SHA-256 hash on the session row.
type LoginOutput struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string // plaintext, returned to client once
	RefreshExpiresAt time.Time
	SessionID        string
	User             *identity.User
}

// Login authenticates a (email|username, password) pair against an app
// and mints a fresh access+refresh pair backed by a new session row.
//
// Every "won't authenticate" path collapses to ErrInvalidCredentials so
// the response gives no enumeration signal. The single exception is
// USER_STATUS_BLOCKED, which surfaces as ErrUserBlocked — the legitimate
// owner of a blocked account needs to know why login fails.
func (s *Service) Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	// 1. Input validation. The gRPC handler also runs protovalidate, but
	//    we re-check here so the use-case is self-defensive against any
	//    future caller.
	if err := validateLoginInput(in); err != nil {
		return LoginOutput{}, err
	}

	// Audit base — populated as we discover the user; emitted on every
	// terminal path. Subject stays empty until the user lookup succeeds.
	aud := audit.NewAuditParams{
		EventType: audit.EventTypeAuthLogin,
		ActorType: audit.ActorTypeAnonymous,
		IpAddress: in.IpAddress,
		UserAgent: in.UserAgent,
	}

	// 2. Resolve the app. Both "missing" and "non-active" collapse to
	//    invalid credentials so we don't tell scanners which app slugs
	//    exist or are disabled.
	appID, err := app.ParseAppID(in.AppID)
	if err != nil {
		return LoginOutput{}, err
	}
	aud.AppID = appID.String()

	a, err := s.apps.GetByID(ctx, appID)
	if err != nil {
		if errors.Is(err, app.ErrAppNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonAppNotFound)
			return LoginOutput{}, ErrInvalidCredentials
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: get app: %w", err)
	}
	if a.Status() != app.AppStatusActive {
		switch a.Status() {
		case app.AppStatusDisabled:
			s.auditor.Deny(ctx, aud, audit.ReasonAppDisabled)
		case app.AppStatusMaintenance:
			s.auditor.Deny(ctx, aud, audit.ReasonAppInMaintenance)
		default:
			s.auditor.Deny(ctx, aud, audit.ReasonInternal)
		}
		return LoginOutput{}, ErrInvalidCredentials
	}

	// 3. Lookup user by email or username (whichever the client sent).
	user, err := s.lookupUser(ctx, in.Email, in.Username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			s.auditor.Fail(ctx, aud, audit.ReasonUserNotFound)
			return LoginOutput{}, ErrInvalidCredentials
		}
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: lookup user: %w", err)
	}
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = user.ID().String()

	// 4. State checks. DELETED → invalid credentials (anti-enumeration);
	//    BLOCKED is the one path we surface so the legitimate user can
	//    recognise their account is on hold.
	switch user.Status() {
	case identity.UserStatusDeleted:
		s.auditor.Deny(ctx, aud, audit.ReasonUserDeleted)
		return LoginOutput{}, ErrInvalidCredentials
	case identity.UserStatusBlocked:
		s.auditor.Deny(ctx, aud, audit.ReasonUserBlocked)
		return LoginOutput{}, ErrUserBlocked
	}

	// 5. Password check. No password on file ≡ wrong password from the
	//    client's perspective.
	if !user.HasPassword() {
		s.auditor.Fail(ctx, aud, audit.ReasonInvalidCredentials)
		return LoginOutput{}, ErrInvalidCredentials
	}
	if err := passwordhash.Compare(user.PasswordHash(), in.Password); err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonPasswordMismatch)
		return LoginOutput{}, ErrInvalidCredentials
	}

	// 6. Mint identifiers and the refresh token.
	sessionID, err := session.NewSessionID()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: new session id: %w", err)
	}
	jti, err := uuid.NewV7()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: new jti: %w", err)
	}
	refreshPlain, refreshHash, err := s.tokenGen.Generate()
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: gen refresh token: %w", err)
	}

	// 7. Compute the two expiry deadlines. The sliding window is capped
	//    against the absolute hard-cap — defensive; config validation
	//    already enforces refreshRotationTTL <= refreshTTL.
	now := s.now().UTC()
	sessionExpiresAt := now.Add(s.refreshTTL)
	refreshExpiresAt := now.Add(s.refreshRotationTTL)
	if refreshExpiresAt.After(sessionExpiresAt) {
		refreshExpiresAt = sessionExpiresAt
	}

	// 8. Persist the session.
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
		return LoginOutput{}, fmt.Errorf("login: create session: %w", err)
	}

	// 10. Sign the access token. Issuer/IssuedAt/ExpiresAt are stamped
	//     by the signer from its own configuration.
	access, err := s.signer.Sign(jwt.Claims{
		Subject:     user.ID().String(),
		SubjectType: jwt.SubjectTypeUser,
		SessionID:   sess.ID().String(),
		JTI:         jti.String(),
	})
	if err != nil {
		s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		return LoginOutput{}, fmt.Errorf("login: sign access token: %w", err)
	}

	err = s.users.UpdateLastLoginAt(ctx, user.ID(), now)

	s.auditor.Success(ctx, aud)

	return LoginOutput{
		AccessToken:      access,
		AccessExpiresAt:  now.Add(s.accessTTL),
		RefreshToken:     refreshPlain,
		RefreshExpiresAt: refreshExpiresAt,
		SessionID:        sess.ID().String(),
		User:             user,
	}, nil
}

// validateLoginInput enforces "exactly one of email/username", a
// non-empty password, and a non-empty app_id. The full UUID parse for
// app_id happens later (we need errors.Is-friendly handling there).
func validateLoginInput(in LoginInput) error {
	if in.AppID == "" {
		return &validation.Error{Field: "app_id", Reason: "required"}
	}
	hasEmail := in.Email != ""
	hasUsername := in.Username != ""
	if hasEmail == hasUsername {
		return &validation.Error{
			Field:  "email_or_username",
			Reason: "exactly one of email or username must be provided",
		}
	}
	if in.Password == "" {
		return &validation.Error{Field: "password", Reason: "required"}
	}
	return nil
}

// lookupUser dispatches to the email or username repo method based on
// which input field was populated. validateLoginInput guarantees
// exactly one is non-empty.
func (s *Service) lookupUser(ctx context.Context, email, username string) (*identity.User, error) {
	if email != "" {
		return s.users.GetByEmail(ctx, email)
	}
	return s.users.GetByUsername(ctx, username)
}
