package tests

import (
	"testing"
	"time"

	"sso/tests/suite"

	ssov1 "github.com/Nergous/sso_protos/gen/go/sso"
	"github.com/brianvoe/gofakeit/v6"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	emptyAppID = 0
	appID      = 1
	appSecret  = "test-secret"

	passDefaultLen = 10
)

func TestRegisterLogin_Login_HappyPath(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := randomFakePassword()

	respReg, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{Email: email, Password: password})
	require.NoError(t, err)
	assert.NotEmpty(t, respReg.GetUserId())

	respLogin, err := st.AuthClient.Login(ctx, &ssov1.LoginRequest{
		Email:    email,
		Password: password,
		AppId:    appID,
	})
	require.NoError(t, err)

	loginTime := time.Now()

	token := respLogin.GetToken()
	require.NotEmpty(t, token)

	tokenParsed, err := jwt.Parse(
		token,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(appSecret), nil
		})
	require.NoError(t, err)

	claims, ok := tokenParsed.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Equal(t, respReg.GetUserId(), int64(claims["uid"].(float64)))
	assert.Equal(t, email, claims["email"].(string))
	assert.Equal(t, appID, int(claims["app_id"].(float64)))

	const deltaSeconds = 5

	assert.InDelta(t, loginTime.Add(st.Cfg.TokenTTL).Unix(), claims["exp"].(float64), deltaSeconds)
}

func TestRegisterLogin_DuplicatedRegistration(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := randomFakePassword()

	respReg, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, respReg.GetUserId())

	respReg, err = st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    email,
		Password: password,
	})
	require.Error(t, err)
	assert.Empty(t, respReg.GetUserId())
	assert.ErrorContains(t, err, "user already exists")
}

func TestRegister_FailCases(t *testing.T) {
	ctx, st := suite.New(t)

	tests := []struct {
		name        string
		email       string
		password    string
		expectedErr string
	}{
		{
			name:        "Empty password",
			email:       gofakeit.Email(),
			password:    "",
			expectedErr: "password is required",
		},
		{
			name:        "Empty email",
			email:       "",
			password:    randomFakePassword(),
			expectedErr: "email is required",
		},
		{
			name:        "Empty email and password",
			email:       "",
			password:    "",
			expectedErr: "email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
				Email:    tt.email,
				Password: tt.password,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestLogin_FailCases(t *testing.T) {
	ctx, st := suite.New(t)

	email := gofakeit.Email()
	password := randomFakePassword()
	_, err := st.AuthClient.Register(ctx, &ssov1.RegisterRequest{
		Email:    gofakeit.Email(),
		Password: randomFakePassword(),
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		email       string
		password    string
		app_id      int32
		expectedErr string
	}{
		{
			name:        "Empty password",
			email:       email,
			password:    "",
			app_id:      appID,
			expectedErr: "password is required",
		},
		{
			name:        "Empty email",
			email:       "",
			password:    password,
			app_id:      appID,
			expectedErr: "email is required",
		},
		{
			name:        "Empty appID",
			email:       email,
			password:    password,
			app_id:      0,
			expectedErr: "app_id is required",
		},
		{
			name:        "Empty email and password",
			email:       "",
			password:    "",
			app_id:      appID,
			expectedErr: "email is required",
		},
		{
			name:        "Empty email and app_id",
			email:       "",
			password:    password,
			app_id:      emptyAppID,
			expectedErr: "email is required",
		},
		{
			name:        "Empty password and app_id",
			email:       email,
			password:    "",
			app_id:      emptyAppID,
			expectedErr: "password is required",
		},
		{
			name:        "Empty email, password and app_id",
			email:       "",
			password:    "",
			app_id:      emptyAppID,
			expectedErr: "email is required",
		},
		{
			name:        "Email not exists",
			email:       "Nergous6@yandex.ru",
			password:    password,
			app_id:      appID,
			expectedErr: "invalid credentials",
		},
		{
			name:        "App not exists",
			email:       email,
			password:    password,
			app_id:      1231,
			expectedErr: "invalid credentials",
		},
		{
			name:        "Wrong password",
			email:       email,
			password:    "12312",
			app_id:      appID,
			expectedErr: "invalid credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := st.AuthClient.Login(ctx, &ssov1.LoginRequest{
				Email:    tt.email,
				Password: tt.password,
				AppId:    tt.app_id,
			})
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func randomFakePassword() string {
	return gofakeit.Password(true, true, true, true, true, passDefaultLen)
}
