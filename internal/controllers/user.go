package controllers

import (
	"context"
	"time"

	"sso/internal/services"

	ssov1 "github.com/Nergous/sso_protos/gen/go/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserController struct {
	ssov1.UnimplementedUserServer
	UserS services.UserService
}

func NewUserController(usrS *services.UserService) *UserController {
	return &UserController{UserS: *usrS}
}

func RegisterUser(gRPC *grpc.Server, usrS services.UserService) {
	ssov1.RegisterUserServer(gRPC, &UserController{UserS: usrS})
}

func (c *UserController) UserInfo(
	ctx context.Context,
	req *ssov1.UserInfoRequest,
) (*ssov1.UserInfoResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetUserId()

	if userID == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	email, steamURL, pathToPhoto, err := c.UserS.UserInfo(&ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.UserInfoResponse{Email: email, SteamUrl: steamURL, PathToPhoto: pathToPhoto}, nil
}

func (c *UserController) GetAllUsers(
	ctx context.Context,
	req *ssov1.GetAllUsersRequest,
) (*ssov1.GetAllUsersResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	users, err := c.UserS.GetAllUsers(&ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var usersModels []*ssov1.UserModel
	for _, user := range users {
		usersModels = append(usersModels, &ssov1.UserModel{
			Id:          user.ID,
			Email:       user.Email,
			SteamUrl:    user.SteamURL,
			PathToPhoto: user.PathToPhoto,
		})
	}

	return &ssov1.GetAllUsersResponse{Users: usersModels}, nil
}

func (c *UserController) UpdateUser(
	ctx context.Context,
	req *ssov1.UpdateUserRequest,
) (*ssov1.UpdateUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetId()
	email := req.GetEmail()
	password := req.GetPassword()
	steamURL := req.GetSteamUrl()
	pathToPhoto := req.GetPathToPhoto()

	if userID == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	updateModel := &services.UpdateModel{
		ID:          userID,
		Email:       email,
		Password:    password,
		SteamURL:    steamURL,
		PathToPhoto: pathToPhoto,
	}
	err := c.UserS.UpdateUser(&ctx, updateModel)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.UpdateUserResponse{}, nil
}

func (c *UserController) DeleteUser(ctx context.Context, req *ssov1.DeleteUserRequest) (*ssov1.DeleteUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetId()

	if userID == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	err := c.UserS.DeleteUser(&ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.DeleteUserResponse{}, nil
}
