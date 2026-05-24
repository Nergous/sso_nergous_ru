package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sso/internal/platform/config"
)

// New constructs a slog.Logger from LogConfig + Env. The returned logger has
// "service" and "env" attached as global attributes so every record carries
// service identity. Errors here are unrecoverable (bad sink, bad level) and
// surface up to main.
func New(cfg config.LogConfig, env config.Env) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	out, err := openSink(cfg.Sink, cfg.Path)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(out, opts)
	case "text":
		handler = slog.NewTextHandler(out, opts)
	default:
		return nil, fmt.Errorf("logger: unknown format %q", cfg.Format)
	}

	return slog.New(handler).With(
		slog.String("service", "sso"),
		slog.String("env", string(env)),
	), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("logger: unknown level %q", s)
}

func openSink(sink, path string) (io.Writer, error) {
	switch sink {
	case "stdout":
		return os.Stdout, nil
	case "file":
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("logger: open %q: %w", path, err)
		}
		return f, nil
	}
	return nil, fmt.Errorf("logger: unknown sink %q", sink)
}
