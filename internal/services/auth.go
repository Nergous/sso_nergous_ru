package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/storage/mariadb"
	jwt_sso "sso/lib/jwt"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	log        *slog.Logger
	storage    *mariadb.Storage
	tokenTTL   time.Duration
	refreshTTL time.Duration
	userR      repositories.UserRepository
	appR       repositories.AppRepository
	tokenR     repositories.TokenRepository
}

// NewAuthService returns a new instance of the Auth service.
func NewAuthService(
	log *slog.Logger,
	storage *mariadb.Storage,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	UserR repositories.UserRepository,
	AppR repositories.AppRepository,
	TokenR repositories.TokenRepository,
) *AuthService {
	return &AuthService{
		log:        log,
		storage:    storage,
		tokenTTL:   tokenTTL,
		refreshTTL: refreshTTL,
		userR:      UserR,
		appR:       AppR,
		tokenR:     TokenR,
	}
}

func (a *AuthService) Login(
	ctx context.Context,
	email, password string,
	appID uint32,
) (accessToken, refreshToken string, err error) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op))

	user, err := a.userR.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		return "", "", domain.ErrInvalidCredentials
	}
	if err != nil {
		log.Error("failed to get user", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password)); err != nil {
		return "", "", domain.ErrInvalidCredentials
	}

	app, err := a.appR.GetAppByID(ctx, appID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	isAdmin, err := a.appR.IsAdmin(ctx, user.ID, appID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	accessToken, err = jwt_sso.NewAccessToken(user.ID, user.Email, isAdmin, app.ID, app.Secret, a.tokenTTL)
	if err != nil {
		log.Error("failed to sign access token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	refreshToken, err = jwt_sso.NewRefreshToken()
	if err != nil {
		log.Error("failed to generate refresh token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := a.tokenR.DeleteRefreshTokenByIDs(ctx, user.ID, appID); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	if _, err := a.tokenR.CreateRefreshToken(ctx, &models.RefreshToken{
		Token:     refreshToken,
		UserID:    user.ID,
		AppID:     appID,
		ExpiresAt: time.Now().Add(a.refreshTTL),
	}); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	return accessToken, refreshToken, nil
}

func (a *AuthService) Logout(ctx context.Context, refreshToken string) error {
	const op = "auth.Logout"
	if refreshToken == "" {
		return domain.ErrValidationFailed
	}

	if err := a.tokenR.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *AuthService) RegisterNewUser(
	ctx context.Context,
	email, password, steamURL, pathToPhoto string,
) (uint32, error) {
	const op = "auth.RegisterNewUser"
	log := a.log.With(slog.String("op", op))

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", slog.Any("err", err))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.userR.CreateUser(ctx, &models.User{
		Email:       email,
		PassHash:    string(passHash),
		SteamURL:    steamURL,
		PathToPhoto: pathToPhoto,
	})
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	return id, nil
}

func (a *AuthService) ValidateToken(
	ctx context.Context,
	tokenStr string,
) (userID uint32, isValid bool, err error) {
	const op = "auth.ValidateToken"

	parser := jwt.NewParser()
	claims := jwt.MapClaims{}

	if _, _, err := parser.ParseUnverified(tokenStr, &claims); err != nil {
		return 0, false, domain.ErrInvalidToken
	}

	appIDFloat, ok := claims["app_id"].(float64)
	if !ok {
		return 0, false, domain.ErrInvalidToken
	}
	appID := uint32(appIDFloat)

	app, err := a.appR.GetAppByID(ctx, appID)
	if err != nil {
		if errors.Is(err, domain.ErrAppNotFound) {
			return 0, false, domain.ErrInvalidToken
		}
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrInvalidToken
		}
		return []byte(app.Secret), nil
	})
	if err != nil || !token.Valid {
		return 0, false, domain.ErrInvalidToken
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, false, domain.ErrInvalidToken
	}

	uidFloat, ok := mc["uid"].(float64)
	if !ok {
		return 0, false, domain.ErrInvalidToken
	}

	if expFloat, ok := mc["exp"].(float64); ok {
		if int64(expFloat) < time.Now().Unix() {
			return 0, false, domain.ErrTokenExpired
		}
	}

	return uint32(uidFloat), true, nil
}

func (a *AuthService) Refresh(
	ctx context.Context,
	refreshToken string,
) (accessToken, refreshTokenNew string, err error) {
	const op = "auth.Refresh"
	log := a.log.With(slog.String("op", op))

	rTkn, err := a.tokenR.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if time.Now().After(rTkn.ExpiresAt) {
		if err := a.tokenR.DeleteRefreshToken(ctx, refreshToken); err != nil {
			log.Warn("failed to delete expired refresh token", slog.Any("err", err))
		}
		return "", "", domain.ErrTokenExpired
	}

	user, err := a.userR.GetUserByID(ctx, rTkn.UserID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	isAdmin, err := a.appR.IsAdmin(ctx, user.ID, rTkn.AppID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	app, err := a.appR.GetAppByID(ctx, rTkn.AppID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	accessToken, err = jwt_sso.NewAccessToken(user.ID, user.Email, isAdmin, app.ID, app.Secret, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate access token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	newRefreshToken, err := jwt_sso.NewRefreshToken()
	if err != nil {
		log.Error("failed to generate refresh token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if _, err := a.tokenR.CreateRefreshToken(ctx, &models.RefreshToken{
		Token:     newRefreshToken,
		UserID:    user.ID,
		AppID:     app.ID,
		ExpiresAt: time.Now().Add(a.refreshTTL),
	}); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := a.tokenR.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	return accessToken, newRefreshToken, nil
}
