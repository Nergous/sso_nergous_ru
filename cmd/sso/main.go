package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sso/internal/app"
	"sso/internal/config"
)

const (
	envLocal = "local"
	envProd  = "prod"
)

func main() {
	cfg := config.MustLoad()
	log := setupLogger(cfg.Env)

	log.Info("starting app",
		slog.String("env", cfg.Env),
		slog.Int("port", cfg.GRPC.Port))

	application := app.New(log, cfg.GRPC.Port, cfg.DB.GetDSN(), cfg.TokenTTL, cfg.RefreshTTL, cfg.DefaultSecret)

	go application.GRPCServer.MustRun()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	application.GRPCServer.Stop()
	log.Info("app stopped")
}

func setupLogger(env string) *slog.Logger {
	var log *slog.Logger

	switch env {
	case envLocal:
		log = slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}),
		)
	case envProd:
		log = slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}),
		)
	}

	return log
}
