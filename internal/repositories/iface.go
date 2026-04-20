package repositories

import (
	"context"

	"sso/internal/models"
)

type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByID(ctx context.Context, id uint32) (models.User, error)
	GetAllUsers(ctx context.Context) ([]models.User, error)
	CreateUser(ctx context.Context, user *models.User) (uint32, error)
	UpdateUser(ctx context.Context, user models.User) error
	DeleteUser(ctx context.Context, id uint32) error
}

type AppRepository interface {
	GetAppByID(ctx context.Context, id uint32) (*models.App, error)
	GetAllApps(ctx context.Context) ([]models.App, error)
	CreateApp(ctx context.Context, app *models.App) (uint32, error)
	UpdateApp(ctx context.Context, app *models.App) error
	DeleteApp(ctx context.Context, id uint32) error
	ChangeStatusApp(ctx context.Context, id uint32) error
	IsAdmin(ctx context.Context, userID, appID uint32) (bool, error)
	AddAdmin(ctx context.Context, admin *models.Admin) error
	RemoveAdmin(ctx context.Context, userID, appID uint32) error
	GetAllUsersForApp(ctx context.Context, appID uint32) ([]models.AppUser, error)
}

type TokenRepository interface {
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) (*models.RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (models.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteRefreshTokenByIDs(ctx context.Context, userID, appID uint32) error
	GetUserByRefreshToken(ctx context.Context, token string) (models.User, error)
}

var (
	_ UserRepository  = (*UserRepo)(nil)
	_ AppRepository   = (*AppRepo)(nil)
	_ TokenRepository = (*TokenRepo)(nil)
)
