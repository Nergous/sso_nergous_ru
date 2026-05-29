package service

import (
	"context"

	"sso/internal/modules/audit"
	"sso/internal/modules/identity/internal/domain"
	"sso/internal/kernel/actor"
)

// CreateUserInput is the parsed CreateUserRequest. Field validation is
// expected upstream (proto-level buf.validate); this layer only assembles
// a User aggregate and persists it.
type CreateUserInput struct {
	Email       string
	Username    string
	DisplayName string
	AvatarURL   string // empty = absent (proto3 optional)
	Locale      string
	Timezone    string
}

// CreateUser provisions a new identity record. Server generates id, etag,
// timestamps; status defaults to ACTIVE. Returns ErrUserAlreadyExists on
// uniqueness collision (email or username).
func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*domain.User, error) {
	a, err := actor.Require(ctx)
	if err != nil {
		return nil, err
	}
	id, err := domain.NewUserID()
	if err != nil {
		return nil, err
	}

	user := domain.NewUser(domain.NewUserParams{
		ID:          id,
		Email:       in.Email,
		Username:    in.Username,
		DisplayName: in.DisplayName,
		AvatarURL:   in.AvatarURL,
		Locale:      in.Locale,
		Timezone:    in.Timezone,
		Now:         s.now().UTC(),
	})

	aud := audit.BaseFromActor(a, audit.EventTypeIdentityCreateUser)
	aud.SubjectType = audit.SubjectTypeUser
	aud.SubjectID = id.String()

	if err := s.repo.Create(ctx, user); err != nil {
		out, reason := classifyError(err)
		s.auditor.Emit(ctx, withOutcome(aud, out, reason))
		return nil, err
	}

	s.auditor.Success(ctx, aud)
	return user, nil
}
