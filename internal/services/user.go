package services

import (
	"context"
	"log/slog"

	"sso/internal/models"
	"sso/internal/repositories"
	serr "sso/lib/serr"

	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	log   *slog.Logger
	userR *repositories.UserRepo
}

func NewUserService(
	log *slog.Logger,
	UserR *repositories.UserRepo,
) *UserService {
	return &UserService{
		log:   log,
		userR: UserR,
	}
}

func (a *UserService) UserInfo(
	ctx *context.Context,
	userID uint32,
) (email, steamURL, pathToPhoto string, err error) {
	const op = "auth.UserInfo"
	log := a.log.With(
		slog.String("op", op),
		slog.Any("user_id", userID),
	)

	usr, err := a.userR.GetUserByID(ctx, userID)
	ok, err := serr.Gerr(op, "user not found", "failed to get user", log, err)
	if !ok {
		return "", "", "", err
	}

	return usr.Email, usr.SteamURL, usr.PathToPhoto, nil
}

func (a *UserService) GetAllUsers(ctx *context.Context) ([]models.User, error) {
	const op = "auth.GetUsers"

	users, err := a.userR.GetAllUsers(ctx)

	ok, err := serr.Gerr(op, "users not found", "failed to get users", a.log, err)
	if !ok {
		return nil, err
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

func (a *UserService) UpdateUser(ctx *context.Context, user *UpdateModel) error {
	const op = "auth.UpdateUser"

	var passHash []byte

	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		ok, err := serr.LogFerr(err, op, "failed to generate password hash", a.log)
		if !ok {
			return err
		}
		passHash = hashedPassword
	}

	var err error
	if passHash == nil {
		err = a.userR.UpdateUser(ctx, models.User{
			ID:          user.ID,
			Email:       user.Email,
			SteamURL:    user.SteamURL,
			PathToPhoto: user.PathToPhoto,
		})
	} else {
		err = a.userR.UpdateUser(ctx, models.User{
			ID:          user.ID,
			Email:       user.Email,
			PassHash:    string(passHash),
			SteamURL:    user.SteamURL,
			PathToPhoto: user.PathToPhoto,
		})
	}

	ok, err := serr.Gerr(op, "failed to update user", "failed to update user", a.log, err)
	if !ok {
		return err
	}

	return nil
}

func (a *UserService) DeleteUser(ctx *context.Context, userID uint32) error {
	const op = "auth.DeleteUser"

	err := a.userR.DeleteUser(ctx, userID)
	ok, err := serr.Gerr(op, "failed to delete user", "failed to delete user", a.log, err)
	if !ok {
		return err
	}
	return nil
}
