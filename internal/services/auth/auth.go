package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"sso/internal/domain/models"
	"sso/internal/storage"
	"sso/internal/storage/sqlite"
	jwt_sso "sso/lib/jwt"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
)

type Auth struct {
	log             *slog.Logger
	usrSaver        UserSaver
	usrProvider     UserProvider
	appProvider     AppProvider
	tokenTTL        time.Duration
	refreshTTL      time.Duration
	refreshProvider RefreshTokenProvider
}

type RefreshTokenProvider interface {
	SaveRefreshToken(ctx context.Context, token string, userID int64, appID int32, expiresAt time.Time) error
	GetRefreshToken(ctx context.Context, token string) (userID int64, appID int32, expiresAt time.Time, err error)
	DeleteRefreshToken(ctx context.Context, token string) error
}

type UserSaver interface {
	SaveUser(
		ctx context.Context,
		email string,
		passHash []byte,
		steamURL string,
		pathToPhoto string,
	) (uid int64, err error)
	UpdateUser(
		ctx context.Context,
		user sqlite.UpdateModel,
	) error
}

type UserProvider interface {
	User(ctx context.Context, email string) (models.User, error)
	UserByID(ctx context.Context, id int64) (models.User, error)
	IsAdmin(ctx context.Context, userID int64) (bool, error)
	GetAllUsers(ctx context.Context) ([]models.User, error)
}

type AppProvider interface {
	App(ctx context.Context, appID int32) (models.App, error)
}

// New returns a new instance of the Auth service
func New(
	log *slog.Logger,
	usrSaver UserSaver,
	usrProvider UserProvider,
	appProvider AppProvider,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	refreshProvider RefreshTokenProvider,
) *Auth {
	return &Auth{
		log:             log,
		usrSaver:        usrSaver,
		usrProvider:     usrProvider,
		appProvider:     appProvider,
		tokenTTL:        tokenTTL,
		refreshTTL:      refreshTTL,
		refreshProvider: refreshProvider,
	}
}

func (a *Auth) Refresh(
	ctx context.Context,
	refreshToken string,
) (accessToken string, refreshTokenNew string, err error) {
	const op = "auth.Refresh"

	log := a.log.With(slog.String("op", op))
	log.Info("refreshing token")

	userID, appID, expiresAt, err := a.refreshProvider.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, storage.ErrRefreshTokenNotFound) {
			log.Warn("refresh token not found")
			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}
		log.Error("failed to get refresh token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if time.Now().After(expiresAt) {
		// Удаляем просроченный токен
		_ = a.refreshProvider.DeleteRefreshToken(ctx, refreshToken)
		log.Warn("refresh token expired")
		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	user, err := a.usrProvider.UserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Warn("user not found")
			return "", "", fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}
		log.Error("failed to get user", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// 4. Получаем данные приложения
	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			log.Warn("app not found")
			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}
		log.Error("failed to get app", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// 5. Генерируем новый access токен
	accessToken, err = jwt_sso.NewToken(user, app, a.tokenTTL)
	if err != nil {
		log.Error("failed to generate access token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// 6. Генерируем новый refresh токен
	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		log.Error("failed to generate refresh token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// 7. Сохраняем новый refresh токен
	newExpiresAt := time.Now().Add(a.refreshTTL)
	err = a.refreshProvider.SaveRefreshToken(ctx, newRefreshToken, userID, appID, newExpiresAt)
	if err != nil {
		log.Error("failed to save refresh token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	// 8. Удаляем старый refresh токен (ротация токенов)
	err = a.refreshProvider.DeleteRefreshToken(ctx, refreshToken)
	if err != nil {
		log.Warn("failed to delete old refresh token", slog.String("error", err.Error()))
		// Не прерываем выполнение, т.к. новые токены уже сгенерированы
	}

	log.Info("tokens refreshed successfully")
	return accessToken, newRefreshToken, nil
}

func (a *Auth) Login(
	ctx context.Context,
	email string,
	password string,
	appId int32,
) (accessToken string, refreshToken string, err error) {
	const op = "auth.Login"

	log := a.log.With(
		slog.String("op", op),
	)

	log.Info("logging in", slog.String("email", email))

	user, err := a.usrProvider.User(ctx, email)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			a.log.Warn("user not found", slog.String("error", err.Error()))
			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}

		a.log.Error("failed to get user", slog.String("error", err.Error()))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		a.log.Warn("invalid password", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
	}

	app, err := a.appProvider.App(ctx, appId)
	if err != nil {
		if errors.Is(err, storage.ErrAppNotFound) {
			a.log.Warn("app not found", slog.String("error", err.Error()))
			return "", "", fmt.Errorf("%s: %w", op, ErrInvalidCredentials)
		}

		a.log.Error("failed to get app", slog.String("error", err.Error()))

		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	accessToken, err = jwt_sso.NewToken(user, app, a.tokenTTL)
	if err != nil {
		a.log.Error("failed to generate token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	refreshToken, err = generateRefreshToken()
	if err != nil {
		a.log.Error("failed to generate refresh token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	expiresAt := time.Now().Add(a.refreshTTL)
	err = a.refreshProvider.SaveRefreshToken(ctx, refreshToken, user.ID, appId, expiresAt)
	if err != nil {
		a.log.Error("failed to save refresh token", slog.String("error", err.Error()))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	return accessToken, refreshToken, nil
}

func (a *Auth) Logout(ctx context.Context, accessToken string) error {
	if accessToken == "" {
		return errors.New("access token is required")
	}

	err := a.refreshProvider.DeleteRefreshToken(ctx, accessToken)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}

	return nil
}

func generateRefreshToken() (string, error) {
	// Генерируем случайную строку для refresh токена
	// Можно использовать UUID или криптографически безопасную случайную строку
	const tokenLength = 32
	bytes := make([]byte, tokenLength)

	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

func (a *Auth) RegisterNewUser(
	ctx context.Context,
	email string,
	password string,
	steamURL string,
	pathToPhoto string,
) (userID int64, err error) {
	const op = "auth.RegisterNewUser"

	log := a.log.With(
		slog.String("op", op),
	)

	log.Info("registering new user", slog.String("email", email))

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to generate password hash", slog.String("error", err.Error()))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := a.usrSaver.SaveUser(ctx, email, passHash, steamURL, pathToPhoto)
	if err != nil {
		log.Error("failed to save user", slog.String("error", err.Error()))
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	log.Info("user registered", slog.Int64("id", id))
	return id, nil
}

func (a *Auth) IsAdmin(ctx context.Context, userID int64) (isAdmin bool, err error) {
	const op = "auth.IsAdmin"
	log := a.log.With(
		slog.String("op", op),
		slog.Int64("user_id", userID),
	)

	log.Info("checking if user is admin")

	isAdmin, err = a.usrProvider.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Warn("user not found", slog.String("error", err.Error()))
			return false, fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return isAdmin, nil
}

func (a *Auth) ValidateToken(ctx context.Context, tokenStr string) (userID int64, isValid bool, err error) {
	const op = "auth.ValidateToken"
	parser := jwt.NewParser()
	claims := jwt.MapClaims{}

	_, _, err = parser.ParseUnverified(tokenStr, &claims)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	appIDFloat, ok := claims["app_id"].(float64)
	if !ok {
		return 0, false, errors.New("missing app_id claim")
	}

	appID := int32(appIDFloat)

	app, err := a.appProvider.App(ctx, appID)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid token")
		}
		return []byte(app.Secret), nil
	})
	if err != nil || !token.Valid {
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	claims, ok = token.Claims.(jwt.MapClaims)

	if !ok {
		return 0, false, fmt.Errorf("%s: %w", op, err)
	}

	uidFloat, ok := claims["uid"].(float64)
	if !ok {
		return 0, false, errors.New("missing uid claim")
	}

	if expFloat, ok := claims["exp"].(float64); ok {
		if int64(expFloat) < time.Now().Unix() {
			return 0, false, fmt.Errorf("%s: %w", op, fmt.Errorf("token expired"))
		}
	}

	return int64(uidFloat), true, nil
}

func (a *Auth) UserInfo(
	ctx context.Context,
	userID int64,
) (email, steamURL, pathToPhoto string, err error) {
	const op = "auth.UserInfo"
	log := a.log.With(
		slog.String("op", op),
		slog.Int64("user_id", userID),
	)

	log.Info("getting user info")

	usr, err := a.usrProvider.UserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			log.Warn("user not found", slog.String("error", err.Error()))
			return "", "", "", fmt.Errorf("%s: %w", op, ErrUserNotFound)
		}
		return "", "", "", fmt.Errorf("%s: %w", op, err)
	}

	return usr.Email, usr.SteamURL, usr.PathToPhoto, nil
}

func (a *Auth) GetAllUsers(ctx context.Context) ([]models.User, error) {
	const op = "auth.GetUsers"
	users, err := a.usrProvider.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return users, nil
}

func (a *Auth) UpdateUser(ctx context.Context, user sqlite.UpdateModel) error {
	const op = "auth.UpdateUser"

	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		user.Password = string(hashedPassword)
	}

	err := a.usrSaver.UpdateUser(ctx, user)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
