package services_test

import (
	"context"
	"testing"
	"time"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/testutil"

	"github.com/stretchr/testify/require"
)

type testSuite struct {
	svc    *services.AuthService
	userR  *repositories.UserRepo
	appR   *repositories.AppRepo
	tokenR *repositories.TokenRepo
}

func newSuite(t *testing.T, refreshTTL time.Duration) *testSuite {
	t.Helper()
	storage := testutil.NewTestStorage(t)

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, refreshTTL, userR, appR, tokenR)
	return &testSuite{svc, userR, appR, tokenR}
}

func (s *testSuite) seedApp(t *testing.T) uint32 {
	t.Helper()
	id, err := s.appR.CreateApp(context.Background(), &models.App{
		Name: "test-app", Secret: "super-secret", Link: "https://example.com", IsEnabled: true,
	})
	require.NoError(t, err)
	return id
}

func TestAuthService_RegisterThenLogin_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)

	uid, err := s.svc.RegisterNewUser(ctx, "alice@example.com", "correcthorse", "https://s.com/id/a", "a.png")
	require.NoError(t, err)
	require.NotZero(t, uid)

	access, refresh, err := s.svc.Login(ctx, "alice@example.com", "correcthorse", appID)
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)
}

func TestAuthService_Login_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "bob@example.com", "rightpass", "https://s.com", "p.png")
	require.NoError(t, err)

	_, _, err = s.svc.Login(ctx, "bob@example.com", "wrongpass", appID)
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthService_Login_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	s := newSuite(t, time.Hour)
	appID := s.seedApp(t)
	_, _, err := s.svc.Login(context.Background(), "ghost@example.com", "whatever", appID)
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)
}

func TestAuthService_Refresh_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "carol@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "carol@example.com", "pw123456", appID)
	require.NoError(t, err)

	access2, refresh2, err := s.svc.Refresh(ctx, refresh)
	require.NoError(t, err)
	require.NotEmpty(t, access2)
	require.NotEqual(t, refresh, refresh2)
}

func TestAuthService_Refresh_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	s := newSuite(t, time.Nanosecond)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "dave@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "dave@example.com", "pw123456", appID)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, _, err = s.svc.Refresh(ctx, refresh)
	require.ErrorIs(t, err, domain.ErrTokenExpired)
}

func TestAuthService_Logout_DeletesRefreshToken(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "eve@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "eve@example.com", "pw123456", appID)
	require.NoError(t, err)

	require.NoError(t, s.svc.Logout(ctx, refresh))

	_, _, err = s.svc.Refresh(ctx, refresh)
	require.Error(t, err)
}

func TestAuthService_ValidateToken_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	uid, err := s.svc.RegisterNewUser(ctx, "frank@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	access, _, err := s.svc.Login(ctx, "frank@example.com", "pw123456", appID)
	require.NoError(t, err)

	got, valid, err := s.svc.ValidateToken(ctx, access)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, uid, got)
}

func TestAuthService_ValidateToken_GarbageToken_Errors(t *testing.T) {
	s := newSuite(t, time.Hour)
	_, _, err := s.svc.ValidateToken(context.Background(), "not-a-jwt")
	require.Error(t, err)
}

func TestAuthService_RegisterDuplicateEmail_Fails(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()

	_, err := s.svc.RegisterNewUser(ctx, "dup@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, err = s.svc.RegisterNewUser(ctx, "dup@example.com", "otherpw1", "https://s.com", "p2.png")
	require.Error(t, err, "duplicate email should fail")
}
