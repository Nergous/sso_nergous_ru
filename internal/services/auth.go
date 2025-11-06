package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/storage/mariadb"
	jwt_sso "sso/lib/jwt"
	serr "sso/lib/serr"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInternal           = errors.New("internal error")
)

type AuthService struct {
	log        *slog.Logger
	storage    *mariadb.Storage
	tokenTTL   time.Duration
	refreshTTL time.Duration
	userR      *repositories.UserRepo
	appR       *repositories.AppRepo
	tokenR     *repositories.TokenRepo
}

// New returns a new instance of the Auth service
func NewAuthService(
	log *slog.Logger,
	storage *mariadb.Storage,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	UserR *repositories.UserRepo,
	AppR *repositories.AppRepo,
	TokenR *repositories.TokenRepo,
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
	ctx *context.Context,
	email string,
	password string,
	appId uint32,
) (accessToken string, refreshToken string, err error) {
	const op = "auth.Login"

	user, err := a.userR.GetUserByEmail(ctx, email)

	ok, _ := serr.Gerr(op, "user not found", "failed to get user", a.log, err)
	if !ok {
		return "", "", ErrInvalidCredentials
	}

	ok, _ = serr.LogFerr(
		bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password)),
		op,
		"invalid password",
		a.log,
	)
	if !ok {
		return "", "", ErrInvalidCredentials
	}

	app, err := a.appR.GetAppByID(ctx, appId)
	ok, err = serr.Gerr(op, "app not found", "failed to get app", a.log, err)
	if !ok {
		return "", "", err
	}

	isAdmin, err := a.appR.IsAdmin(ctx, user.ID, appId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			isAdmin = false
		} else {
			return "", "", err
		}
	}

	accessToken, err = jwt_sso.NewAccessToken(
		user.ID,
		user.Email,
		isAdmin,
		app.ID,
		app.Secret,
		a.tokenTTL,
	)

	ok, err = serr.LogFerr(err, op, "failed to generate token", a.log)
	if !ok {
		return "", "", err
	}

	refreshToken, err = jwt_sso.NewRefreshToken()
	ok, err = serr.LogFerr(err, op, "failed to generate refresh token", a.log)
	if !ok {
		return "", "", err
	}

	expiresAt := time.Now().Add(a.refreshTTL)

	err = a.tokenR.DeleteRefreshTokenByIDs(ctx, user.ID, appId)
	if err != nil {
		return "", "", err
	}

	_, err = a.tokenR.CreateRefreshToken(ctx, &models.RefreshToken{
		Token:     refreshToken,
		UserID:    user.ID,
		AppID:     appId,
		ExpiresAt: expiresAt,
	})

	ok, err = serr.Gerr(op, "failed to save refresh token", "failed to save refresh token", a.log, err)
	if !ok {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (a *AuthService) Logout(ctx *context.Context, refreshToken string) error {
	const op = "auth.Logout"
	if refreshToken == "" {
		return errors.New("refresh token is required")
	}

	err := a.tokenR.DeleteRefreshToken(ctx, refreshToken)

	ok, err := serr.Gerr(op, "refresh token not found", "failed to delete refresh token", a.log, err)
	if !ok {
		return err
	}

	return nil
}

func (a *AuthService) RegisterNewUser(
	ctx *context.Context,
	email string,
	password string,
	steamURL string,
	pathToPhoto string,
) (id uint32, err error) {
	const op = "auth.RegisterNewUser"

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	ok, err := serr.LogFerr(err, op, "failed to generate password hash", a.log)
	if !ok {
		return 0, err
	}

	id, err = a.userR.CreateUser(ctx, &models.User{
		Email:       email,
		PassHash:    string(passHash),
		SteamURL:    steamURL,
		PathToPhoto: pathToPhoto,
	})

	ok, err = serr.Gerr(op, "failed to create user", "failed to create user", a.log, err)

	if !ok {
		return 0, err
	}

	return id, nil
}

func (a *AuthService) ValidateToken(
	ctx *context.Context,
	tokenStr string,
) (userID uint32, isValid bool, err error) {
	const op = "auth.ValidateToken"
	parser := jwt.NewParser()
	claims := jwt.MapClaims{}

	_, _, err = parser.ParseUnverified(tokenStr, &claims)
	ok, err := serr.LogFerr(err, op, "failed to validate token", a.log)
	if !ok {
		return 0, false, err
	}

	appIDFloat, ok := claims["app_id"].(float64)
	if !ok {
		return 0, false, serr.Ferr(op, "failed to validate")
	}

	appID := uint32(appIDFloat)

	app, err := a.appR.GetAppByID(ctx, appID)
	ok, err = serr.Gerr(op, "app not found", "failed to get app", a.log, err)
	if !ok {
		return 0, false, err
	}

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, serr.Ferr(op, "invalid token")
		}
		return []byte(app.Secret), nil
	})
	if err != nil || !token.Valid {
		return 0, false, serr.Ferr(op, "failed to validate")
	}

	claims, ok = token.Claims.(jwt.MapClaims)

	if !ok {
		return 0, false, serr.Ferr(op, "failed to validate")
	}

	uidFloat, ok := claims["uid"].(float64)
	if !ok {
		return 0, false, serr.Ferr(op, "failed to validate")
	}

	if expFloat, ok := claims["exp"].(float64); ok {
		if int64(expFloat) < time.Now().Unix() {
			return 0, false, serr.Ferr(op, "token expired")
		}
	}

	return uint32(uidFloat), true, nil
}

func (a *AuthService) Refresh(
	ctx *context.Context,
	refreshToken string,
) (accessToken string, refreshTokenNew string, err error) {
	const op = "auth.Refresh"

	rTkn, err := a.tokenR.GetRefreshToken(ctx, refreshToken)
	ok, err := serr.Gerr(op, "refresh token not found", "failed to get refresh token", a.log, err)
	if !ok {
		return "", "", err
	}

	if time.Now().After(rTkn.ExpiresAt) {
		err = a.tokenR.DeleteRefreshToken(ctx, refreshToken)

		ok, err := serr.Gerr(op, "refresh token not found", "failed to delete refresh token", a.log, err)
		if !ok {
			return "", "", err
		}
	}

	user, err := a.userR.GetUserByID(ctx, rTkn.UserID)
	ok, err = serr.Gerr(op, "user not found", "failed to get user", a.log, err)
	if !ok {
		return "", "", err
	}

	isAdmin, err := a.appR.IsAdmin(ctx, user.ID, rTkn.AppID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			isAdmin = false
		} else {
			return "", "", err
		}
	}

	app, err := a.appR.GetAppByID(ctx, rTkn.AppID)
	ok, err = serr.Gerr(op, "app not found", "failed to get app", a.log, err)
	if !ok {
		return "", "", err
	}

	accessToken, err = jwt_sso.NewAccessToken(
		user.ID,
		user.Email,
		isAdmin,
		app.ID,
		app.Secret,
		a.tokenTTL,
	)
	ok, err = serr.LogFerr(err, op, "failed to generate access token", a.log)
	if !ok {
		return "", "", err
	}

	newRefreshToken, err := jwt_sso.NewRefreshToken()
	ok, err = serr.LogFerr(err, op, "failed to generate refresh token", a.log)
	if !ok {
		return "", "", err
	}

	newExpiresAt := time.Now().Add(a.refreshTTL)

	_, err = a.tokenR.CreateRefreshToken(ctx, &models.RefreshToken{
		Token:     newRefreshToken,
		UserID:    user.ID,
		AppID:     app.ID,
		ExpiresAt: newExpiresAt,
	})
	ok, err = serr.LogFerr(err, op, "failed to create refresh token", a.log)
	if !ok {
		return "", "", err
	}

	err = a.tokenR.DeleteRefreshToken(ctx, refreshToken)
	ok, err = serr.Gerr(op, "refresh token not found", "failed to delete refresh token", a.log, err)
	if !ok {
		return "", "", err
	}

	return accessToken, newRefreshToken, nil
}
