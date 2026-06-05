// Package logger builds a configured *slog.Logger from the application's
// LogConfig. It selects the minimum level, the output sink (stdout, stderr, or
// a file), and the record format (JSON or text), and attaches the service name
// and environment as global attributes so every record is self-describing.
//
// New returns an io.Closer alongside the logger; the caller must Close it on
// shutdown to release a file sink (it is a no-op for the stdout and stderr
// sinks). Configuration errors — an unknown level, format, or sink — are
// returned rather than logged, since at construction time there is no logger
// yet to report them.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sso/internal/platform/config"
	"sync"
)

// noopCloser is the io.Closer used for the stdout and stderr sinks, which are
// shared process streams and must not be closed.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// onceCloser wraps a sink's io.Closer so Close runs at most once, making it
// idempotent and safe for repeated or concurrent calls. The first call's
// result is remembered and returned to every caller.
type onceCloser struct {
	once sync.Once
	c    io.Closer
	err  error
}

func (o *onceCloser) Close() error {
	o.once.Do(func() { o.err = o.c.Close() })
	return o.err
}

// New constructs a *slog.Logger from cfg, tagging every record with the given
// appName ("service") and env. It also returns an io.Closer the caller must
// Close on shutdown to release a file sink; the closer is idempotent and a
// no-op for the stdout and stderr sinks.
//
// Pure configuration (level, format) is validated before the sink is opened,
// so an invalid format never leaks an opened file. Remaining errors — an
// unknown level, format, or sink — are unrecoverable and surface up to main,
// which has no logger yet to report them.
func New(cfg config.LogConfig, env config.Env, appName config.AppName) (*slog.Logger, io.Closer, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}

	newHandler, err := parseFormat(cfg.Format)
	if err != nil {
		return nil, nil, err
	}

	out, closer, err := openSink(cfg.Sink, cfg.Path)
	if err != nil {
		return nil, nil, err
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	handler := newHandler(out, opts)
	logger := slog.New(handler).With("service", appName, "env", env)

	return logger, &onceCloser{c: closer}, nil
}

// parseLevel maps a configured level name to its slog.Level, erroring on an
// unknown value.
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

// parseFormat maps a configured format name to a slog.Handler constructor. It
// validates the format without acquiring any resource, so New can reject a bad
// format before opening the sink.
func parseFormat(s string) (func(io.Writer, *slog.HandlerOptions) slog.Handler, error) {
	switch s {
	case "json":
		return func(w io.Writer, o *slog.HandlerOptions) slog.Handler {
			return slog.NewJSONHandler(w, o)
		}, nil
	case "text":
		return func(w io.Writer, o *slog.HandlerOptions) slog.Handler {
			return slog.NewTextHandler(w, o)
		}, nil
	}
	return nil, fmt.Errorf("logger: unknown format %q", s)
}

// openSink resolves the configured sink to a writer and its closer. "stdout"
// and "stderr" return the shared stream paired with a no-op closer; "file"
// opens path for appending (creating it with mode 0640 if absent) and returns
// it as both writer and closer. An unknown sink is an error.
func openSink(sink, path string) (io.Writer, io.Closer, error) {
	switch sink {
	case "stdout":
		return os.Stdout, noopCloser{}, nil
	case "stderr":
		return os.Stderr, noopCloser{}, nil
	case "file":
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
		if err != nil {
			return nil, nil, fmt.Errorf("logger: open %q: %w", path, err)
		}
		return f, f, nil
	}
	return nil, nil, fmt.Errorf("logger: unknown sink %q", sink)
}
