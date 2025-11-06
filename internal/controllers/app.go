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

type AppController struct {
	ssov1.UnimplementedAppServer
	AppS          services.AppService
	DefaultSecret string
}

func NewAppController(appS *services.AppService, defaultSecret string) *AppController {
	return &AppController{AppS: *appS, DefaultSecret: defaultSecret}
}

func RegisterApp(gRPC *grpc.Server, appS services.AppService, defaultSecret string) {
	ssov1.RegisterAppServer(gRPC, &AppController{AppS: appS, DefaultSecret: defaultSecret})
}

func (c *AppController) GetApp(ctx context.Context, req *ssov1.GetAppRequest) (*ssov1.GetAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	appID := req.GetId()

	if appID == 0 {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	app, err := c.AppS.GetApp(&ctx, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	appModel := ssov1.AppModel{
		Id:        app.ID,
		Name:      app.Name,
		Link:      app.Link,
		Isenabled: app.IsEnabled,
	}

	return &ssov1.GetAppResponse{App: &appModel}, nil
}

func (c *AppController) GetAllApps(ctx context.Context, req *ssov1.GetAllAppsRequest) (*ssov1.GetAllAppsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	apps, err := c.AppS.GetAllApps(&ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	appModels := make([]*ssov1.AppModel, len(apps))
	for i, app := range apps {
		appModels[i] = &ssov1.AppModel{
			Id:        app.ID,
			Name:      app.Name,
			Link:      app.Link,
			Isenabled: app.IsEnabled,
		}
	}

	return &ssov1.GetAllAppsResponse{Apps: appModels}, nil
}

func (c *AppController) CreateApp(ctx context.Context, req *ssov1.CreateAppRequest) (*ssov1.CreateAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	name := req.GetName()
	link := req.GetLink()

	if name == "" || link == "" {
		return nil, status.Error(codes.InvalidArgument, "name and link are required")
	}
	defaultSecret := c.DefaultSecret
	appID, err := c.AppS.CreateApp(&ctx, name, link, defaultSecret)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	appModel := ssov1.AppModel{
		Id:        appID,
		Name:      name,
		Link:      link,
		Isenabled: true,
	}

	return &ssov1.CreateAppResponse{App: &appModel}, nil
}

func (c *AppController) UpdateApp(ctx context.Context, req *ssov1.UpdateAppRequest) (*ssov1.UpdateAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	appID := req.GetId()
	name := req.GetName()
	link := req.GetLink()

	if appID == 0 || name == "" || link == "" {
		return nil, status.Error(codes.InvalidArgument, "app_id, name and link are required")
	}

	err := c.AppS.UpdateApp(&ctx, appID, name, link)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.UpdateAppResponse{}, nil
}

func (c *AppController) DeleteApp(ctx context.Context, req *ssov1.DeleteAppRequest) (*ssov1.DeleteAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	appID := req.GetId()

	if appID == 0 {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	err := c.AppS.DeleteApp(&ctx, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.DeleteAppResponse{}, nil
}

func (c *AppController) ChangeStatusApp(ctx context.Context, req *ssov1.ChangeStatusAppRequest) (*ssov1.ChangeStatusAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	appID := req.GetId()

	if appID == 0 {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	err := c.AppS.ChangeStatusApp(&ctx, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.ChangeStatusAppResponse{}, nil
}

func (c *AppController) AddAdmin(ctx context.Context, req *ssov1.AddAdminRequest) (*ssov1.AddAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetUserId()
	appID := req.GetAppId()

	if userID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	err := c.AppS.AddAdmin(&ctx, userID, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.AddAdminResponse{}, nil
}

func (c *AppController) RemoveAdmin(ctx context.Context, req *ssov1.RemoveAdminRequest) (*ssov1.RemoveAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetUserId()
	appID := req.GetAppId()

	if userID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	err := c.AppS.RemoveAdmin(&ctx, userID, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.RemoveAdminResponse{}, nil
}

func (c *AppController) IsAdmin(
	ctx context.Context,
	req *ssov1.IsAdminRequest,
) (*ssov1.IsAdminResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	userID := req.GetUserId()
	appID := req.GetAppId()

	if userID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	isAdmin, err := c.AppS.IsAdmin(&ctx, userID, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &ssov1.IsAdminResponse{IsAdmin: isAdmin}, nil
}

func (C *AppController) GetAllUsersForApp(ctx context.Context, req *ssov1.GetAllUsersForAppRequest) (*ssov1.GetAllUsersForAppResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	appID := req.GetAppId()

	if appID == emptyValue {
		return nil, status.Error(codes.InvalidArgument, "app_id is required")
	}

	users, err := C.AppS.GetAllUsersForApp(&ctx, appID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var appUser []*ssov1.AppUser
	for _, user := range users {
		appUser = append(appUser, &ssov1.AppUser{
			Id:          user.ID,
			Email:       user.Email,
			SteamUrl:    user.SteamURL,
			PathToPhoto: user.PathToPhoto,
			IsAdmin:     user.IsAdmin,
		})
	}

	return &ssov1.GetAllUsersForAppResponse{Users: appUser}, nil
}
