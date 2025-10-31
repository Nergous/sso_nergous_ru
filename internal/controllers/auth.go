package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sso/internal/services"

	ssov1 "github.com/Nergous/sso_protos/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	emptyValue = 0
)

type AuthController struct {
	ssov1.UnimplementedAuthServer
	AuthS services.AuthService
}

func NewAuthController(auth *services.AuthService) *AuthController {
	return &AuthController{AuthS: *auth}
}

func RegisterAuth(gRPC *grpc.Server, authS services.AuthService) {
	ssov1.RegisterAuthServer(gRPC, &AuthController{AuthS: authS})
}

func (c *AuthController) Login(
	ctx context.Context,
	req *ssov1.LoginRequest,
) (*ssov1.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	email := req.GetEmail()
	password := req.GetPassword()
	appID := req.GetAppId()

	if err := validateLogin(email, password, appID); err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := c.AuthS.Login(&ctx, email, password, appID)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.LoginResponse{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func (c *AuthController) Logout(
	ctx context.Context,
	req *ssov1.LogoutRequest,
) (*ssov1.LogoutResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	refreshToken := req.GetToken()
	if refreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	err := c.AuthS.Logout(&ctx, refreshToken)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.LogoutResponse{}, nil
}

func (c *AuthController) Refresh(
	ctx context.Context,
	req *ssov1.RefreshRequest,
) (*ssov1.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	refreshToken := req.GetRefreshToken()
	if refreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	accessToken, newRefreshToken, err := c.AuthS.Refresh(&ctx, refreshToken)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (c *AuthController) Register(
	ctx context.Context,
	req *ssov1.RegisterRequest,
) (*ssov1.RegisterResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fmt.Println("=================")
	fmt.Println("REGISTERING")
	fmt.Println("=================")

	email := req.GetEmail()
	password := req.GetPassword()
	steamURL := req.GetSteamUrl()
	pathToPhoto := req.GetPathToPhoto()

	if err := validateRegister(email, password, steamURL, pathToPhoto); err != nil {
		return nil, err
	}

	userID, err := c.AuthS.RegisterNewUser(&ctx, email, password, steamURL, pathToPhoto)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.RegisterResponse{UserId: userID}, nil
}

func (c *AuthController) ValidateToken(
	ctx context.Context,
	req *ssov1.ValidateTokenRequest,
) (*ssov1.ValidateTokenResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	token := req.GetToken()

	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	userID, isValid, err := c.AuthS.ValidateToken(&ctx, token)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.ValidateTokenResponse{UserId: userID, Valid: isValid}, nil
}

func validateLogin(
	email string,
	password string,
	appId uint32,
) error {
	if email == "" {
		return status.Error(codes.InvalidArgument, "email is required")
	}

	if password == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}

	if appId == emptyValue {
		return status.Error(codes.InvalidArgument, "app_id is required")
	}
	return nil
}

func validateRegister(
	email string,
	password string,
	steamURL string,
	pathToPhoto string,
) error {
	if email == "" {
		return status.Error(codes.InvalidArgument, "email is required")
	}

	if password == "" {
		return status.Error(codes.InvalidArgument, "password is required")
	}

	if steamURL == "" {
		return status.Error(codes.InvalidArgument, "steam url is required")
	}

	if pathToPhoto == "" {
		return status.Error(codes.InvalidArgument, "photo is required")
	}

	return nil
}
