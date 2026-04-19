package services_test

import (
	"context"
	"testing"
	"time"

	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/testutil"

	"github.com/stretchr/testify/require"
)

func newAuthTestSuite(t *testing.T) (*services.AuthService, *repositories.UserRepo, *repositories.AppRepo, func()) {
	t.Helper()
	storage, cleanup := testutil.NewTestStorage(t)

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, time.Hour, userR, appR, tokenR)

	return svc, userR, appR, cleanup
}

func TestAuthService_RegisterThenLogin_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	// seed an app
	appID, err := appR.CreateApp(ctx, &models.App{
		Name:   "test-app",
		Secret: "super-secret",
		Link:   "https://example.com",
	})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "alice@example.com", "correcthorse", "https://steamcommunity.com/id/alice", "alice.png")
	require.NoError(t, err)
	require.NotZero(t, userID)

	accessToken, refreshToken, err := svc.Login(ctx, "alice@example.com", "correcthorse", appID)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
}

func TestAuthService_Login_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "bob@example.com", "rightpass", "https://s.com", "p.png")
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, "bob@example.com", "wrongpass", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Login_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, "ghost@example.com", "whatever", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Refresh_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "carol@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "carol@example.com", "pw123456", appID)
	require.NoError(t, err)

	newAccess, newRefresh, err := svc.Refresh(ctx, refreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, newAccess)
	require.NotEmpty(t, newRefresh)
	require.NotEqual(t, refreshToken, newRefresh)
}

func TestAuthService_Refresh_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	// Use a 1ns refresh TTL so the token is born expired.
	storage, cleanup := testutil.NewTestStorage(t)
	defer cleanup()

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)
	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, time.Nanosecond, userR, appR, tokenR)

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "dave@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "dave@example.com", "pw123456", appID)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, _, err = svc.Refresh(ctx, refreshToken)
	require.ErrorIs(t, err, services.ErrTokenExpired)
}

func TestAuthService_Logout_DeletesRefreshToken(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "eve@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "eve@example.com", "pw123456", appID)
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, refreshToken))

	// Using the refresh token after logout should fail.
	_, _, err = svc.Refresh(ctx, refreshToken)
	require.Error(t, err)
}

func TestAuthService_ValidateToken_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "frank@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	accessToken, _, err := svc.Login(ctx, "frank@example.com", "pw123456", appID)
	require.NoError(t, err)

	gotUserID, valid, err := svc.ValidateToken(ctx, accessToken)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, userID, gotUserID)
}

func TestAuthService_ValidateToken_GarbageToken_Errors(t *testing.T) {
	svc, _, _, cleanup := newAuthTestSuite(t)
	defer cleanup()

	_, _, err := svc.ValidateToken(context.Background(), "not-a-jwt")
	require.Error(t, err)
}
