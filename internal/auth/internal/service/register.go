package service

import (
	"context"
	"errors"
	"fmt"

	"sso/internal/audit"
	"sso/internal/identity"
	"sso/internal/kernel/validation"
	"sso/internal/platform/crypto/passwordhash"
)

type RegisterInput struct {
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string
	Locale      string
	Timezone    string
	Password    string
	IpAddress   string
	UserAgent   string
}

func (s *Service) Register(ctx context.Context, r RegisterInput) (*identity.User, error) {
	passwordHash, err := passwordhash.Hash(r.Password, s.bcryptCost)
	if err != nil {
		return nil, err
	}

	// NewUser does not mint an id by itself — it accepts one through
	// NewUserParams. Forgetting this step persists the user with an
	// empty id, which downstream Login / Refresh / ChangePassword would
	// then sign into the JWT subject and rediscover at ParseUserID time
	// as a validation error.
	id, err := identity.NewUserID()
	if err != nil {
		return nil, fmt.Errorf("register: new user id: %w", err)
	}

	user := identity.NewUser(identity.NewUserParams{
		ID:           id,
		Email:        r.Email,
		Username:     r.Username,
		DisplayName:  r.DisplayName,
		AvatarURL:    r.AvatarURL,
		Locale:       r.Locale,
		Timezone:     r.Timezone,
		PasswordHash: passwordHash,
		Now:          s.now().UTC(),
	})

	aud := audit.NewAuditParams{
		EventType:   audit.EventTypeAuthRegister,
		ActorType:   audit.ActorTypeAnonymous,
		SubjectType: audit.SubjectTypeUser,
		SubjectID:   user.ID().String(),
		IpAddress:   r.IpAddress,
		UserAgent:   r.UserAgent,
	}

	if err := s.users.Create(ctx, user); err != nil {
		var verr *validation.Error
		switch {
		case errors.Is(err, identity.ErrUserAlreadyExists):
			s.auditor.Fail(ctx, aud, audit.ReasonUserAlreadyExists)
		case errors.As(err, &verr):
			s.auditor.Fail(ctx, aud, audit.ReasonValidationFailed)
		default:
			s.auditor.Fail(ctx, aud, audit.ReasonInternal)
		}
		return nil, fmt.Errorf("register: create user: %w", err)
	}

	s.auditor.Success(ctx, aud)
	return user, nil
}
