package auth

import (
	"context"
	"errors"

	"sso/internal/services/auth"

	ssov1 "github.com/Nergous/sso_protos/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	emptyValue = 0
)

type Auth interface {
	Login(
		ctx context.Context,
		email string,
		password string,
		appId int32,
	) (token string, err error)
	RegisterNewUser(
		ctx context.Context,
		email string,
		password string,
		steamURL string,
		pathToPhoto string,
	) (userID int64, err error)
	IsAdmin(
		ctx context.Context,
		userID int64,
	) (isAdmin bool, err error)
	ValidateToken(
		ctx context.Context,
		token string,
	) (userID int64, isValid bool, err error)
	UserInfo(
		ctx context.Context,
		userID int64,
	) (email, steamURL, pathToPhoto string, err error)
}

type serverAPI struct {
	ssov1.UnimplementedAuthServer
	auth Auth
}

func Register(gRPC *grpc.Server, auth Auth) {
	ssov1.RegisterAuthServer(gRPC, &serverAPI{auth: auth})
}

func (s *serverAPI) Login(
	ctx context.Context,
	req *ssov1.LoginRequest,
) (*ssov1.LoginResponse, error) {
	email := req.GetEmail()
	password := req.GetPassword()
	appID := req.GetAppId()

	if err := validateLogin(email, password, appID); err != nil {
		return nil, err
	}

	token, err := s.auth.Login(ctx, email, password, appID)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, status.Error(codes.InvalidArgument, "invalid credentials")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.LoginResponse{Token: token}, nil
}

func (s *serverAPI) Register(
	ctx context.Context,
	req *ssov1.RegisterRequest,
) (*ssov1.RegisterResponse, error) {
	email := req.GetEmail()
	password := req.GetPassword()
	steamURL := req.GetSteamUrl()
	pathToPhoto := req.GetPathToPhoto()

	if err := validateRegister(email, password, steamURL, pathToPhoto); err != nil {
		return nil, err
	}

	userID, err := s.auth.RegisterNewUser(ctx, email, password, steamURL, pathToPhoto)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			return nil, status.Error(codes.AlreadyExists, "user already exists")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.RegisterResponse{UserId: userID}, nil
}

func (s *serverAPI) IsAdmin(
	ctx context.Context,
	req *ssov1.IsAdminRequest,
) (*ssov1.IsAdminResponse, error) {
	userID := req.GetUserId()

	if userID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	isAdmin, err := s.auth.IsAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.IsAdminResponse{IsAdmin: isAdmin}, nil
}

func (s *serverAPI) ValidateToken(
	ctx context.Context,
	req *ssov1.ValidateTokenRequest,
) (*ssov1.ValidateTokenResponse, error) {
	token := req.GetToken()

	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "token is required")
	}

	userID, isValid, err := s.auth.ValidateToken(ctx, token)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.ValidateTokenResponse{UserId: userID, Valid: isValid}, nil
}

func (s *serverAPI) UserInfo(
	ctx context.Context,
	req *ssov1.UserInfoRequest,
) (*ssov1.UserInfoResponse, error) {
	userID := req.GetUserId()

	if userID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	email, steamURL, pathToPhoto, err := s.auth.UserInfo(ctx, userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.UserInfoResponse{Email: email, SteamUrl: steamURL, PathToPhoto: pathToPhoto}, nil
}

func validateLogin(
	email string,
	password string,
	appId int32,
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
