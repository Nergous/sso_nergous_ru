package app

import (
	"log/slog"
	"time"

	grpcapp "sso/internal/app/grpc"
	"sso/internal/services/auth"
	"sso/internal/storage/sqlite"
)

type App struct {
	GRPCServer *grpcapp.App
}

func New(
	log *slog.Logger,
	grpcPort int,
	storagePath string,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
) *App {
	storage, err := sqlite.New(storagePath)
	if err != nil {
		panic(err)
	}

	storage.StartCleanupRoutine(24 * time.Hour)
	log.Info("started refresh tokens cleanup routine", slog.String("interval", "24h"))

	authService := auth.New(log, storage, storage, storage, tokenTTL, refreshTTL, storage)

	grpcApp := grpcapp.New(log, grpcPort, authService)
	return &App{
		GRPCServer: grpcApp,
	}
}
