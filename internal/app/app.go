package app

import (
	"log/slog"
	"time"

	grpcapp "sso/internal/app/grpc"
	"sso/internal/controllers"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/storage/mariadb"
)

type App struct {
	GRPCServer *grpcapp.App
}

func New(
	log *slog.Logger,
	grpcPort int,
	dsn string,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	defaultSecret string,
) *App {
	storage, err := mariadb.NewStorage(dsn)
	if err != nil {
		panic(err)
	}

	if err := storage.Migrate(); err != nil {
		panic(err)
	}

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	userS := services.NewUserService(log, userR)
	appS := services.NewAppService(log, appR)
	authS := services.NewAuthService(log, storage, tokenTTL, refreshTTL, userR, appR, tokenR)

	userC := controllers.NewUserController(userS)
	appC := controllers.NewAppController(appS, defaultSecret)
	authC := controllers.NewAuthController(authS)

	grpcApp := grpcapp.New(log, grpcPort, authC, userC, appC)
	return &App{
		GRPCServer: grpcApp,
	}
}
