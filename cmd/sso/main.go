package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"sso/internal/app"
	"sso/internal/config"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
)

const (
	envLocal = "local"
	envProd  = "prod"
)

var (
	version = "0.0.0"
	commit  = "none"
	date    = "unknown"
)

func main() {
	checkUpdate()
	fmt.Printf("Version: %s\nCommit: %s\nBuild date: %s\n", version, commit, date)
	cfg := config.MustLoad()

	log := setupLogger(cfg.Env)

	log.Info("starting app",
		slog.String("env", cfg.Env),
		slog.String("storage_path", cfg.StoragePath),
		slog.Int("port", cfg.GRPC.Port))

	application := app.New(log, cfg.GRPC.Port, cfg.StoragePath, cfg.TokenTTL)

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

func checkUpdate() {
	v, err := semver.Parse(version)
	if err != nil {
		fmt.Println(err)
		return
	}
	latest, err := selfupdate.UpdateSelf(v, "nergous/sso_nergous_ru")
	if err != nil {
		fmt.Println(err)
		return
	}
	if latest.Version.Equals(v) {
		fmt.Println("Обновлено до версии: ", latest.Version)
	} else {
		fmt.Println("Вы используете последнюю версию")
	}
}
