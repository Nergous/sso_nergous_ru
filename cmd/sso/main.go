package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"sso/internal/bootstrap"
	"sso/internal/platform/config"
	"sso/internal/platform/logger"
	"syscall"

	"github.com/joho/godotenv"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "configPath", "", "path to config yaml file")
	flag.StringVar(&configPath, "cp", "", "path to config yaml file (shortcut)")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	resolvedPath, err := config.FetchPath(configPath)
	if err != nil {
		return err
	}

	_ = godotenv.Load()

	cfg, err := config.Load(resolvedPath)
	if err != nil {
		return err
	}

	lg, err := logger.New(cfg.Log, cfg.Env)
	if err != nil {
		return err
	}

	app, err := bootstrap.New(ctx, cfg, lg)
	if err != nil {
		return err
	}

	lg.Info("starting sso",
		slog.String("address", cfg.GRPC.Address()),
		buildInfo(),
	)

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		if err := app.Run(); err != nil && !errors.Is(err, net.ErrClosed) {
			errChan <- err
		}
	}()

	var runErr error
	select {
	case err, ok := <-errChan:
		if ok {
			lg.Error("server failed", slog.Any("error", err))
			runErr = err
		} else {
			lg.Info("server exited cleanly")
		}
	case <-ctx.Done():
		lg.Info("shutdown initiated")
		app.Stop()
	}

	lg.Info("sso stopped")
	return runErr
}

// buildInfo collects VCS metadata embedded by the Go toolchain (-buildvcs=true,
// the default for `go build` inside a git repo). Under `go run` the vcs.* keys
// are absent, so the group degrades to just go="goX.Y.Z".
func buildInfo() slog.Attr {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return slog.Group("build", slog.String("status", "unavailable"))
	}
	args := []any{slog.String("go", info.GoVersion)}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			args = append(args, slog.String("commit", s.Value))
		case "vcs.modified":
			args = append(args, slog.String("modified", s.Value))
		case "vcs.time":
			args = append(args, slog.String("vcs_time", s.Value))
		}
	}
	return slog.Group("build", args...)
}
