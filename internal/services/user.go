package services

import (
	"context"
	"fmt"
	"log/slog"

	"sso/internal/models"
	"sso/internal/repositories"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	log   *slog.Logger
	userR repositories.UserRepository
}

func NewUserService(
	log *slog.Logger,
	UserR repositories.UserRepository,
) *UserService {
	return &UserService{
		log:   log,
		userR: UserR,
	}
}

func (a *UserService) UserInfo(
	ctx context.Context,
	userID uint32,
) (email, steamURL, pathToPhoto string, err error) {
	const op = "auth.UserInfo"

	usr, err := a.userR.GetUserByID(ctx, userID)
	if err != nil {
		return "", "", "", fmt.Errorf("%s: %w", op, err)
	}
	return usr.Email, usr.SteamURL, usr.PathToPhoto, nil
}

func (a *UserService) GetAllUsers(ctx context.Context) ([]models.User, error) {
	const op = "auth.GetUsers"

	users, err := a.userR.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return users, nil
}

type UpdateModel struct {
	ID          uint32
	Email       string
	Password    string
	SteamURL    string
	PathToPhoto string
}

func (a *UserService) UpdateUser(ctx context.Context, user *UpdateModel) error {
	const op = "auth.UpdateUser"
	log := a.log.With(slog.String("op", op))

	var passHash []byte

	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Error("failed to generate password hash", slog.Any("err", err))
			return fmt.Errorf("%s: %w", op, err)
		}
		passHash = hashedPassword
	}

	m := models.User{
		ID:          user.ID,
		Email:       user.Email,
		SteamURL:    user.SteamURL,
		PathToPhoto: user.PathToPhoto,
	}
	if passHash != nil {
		m.PassHash = string(passHash)
	}

	if err := a.userR.UpdateUser(ctx, m); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *UserService) DeleteUser(ctx context.Context, userID uint32) error {
	const op = "auth.DeleteUser"

	if err := a.userR.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
