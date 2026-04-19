package testutil

import (
	"io"
	"log/slog"
)

// NewTestLogger returns a slog.Logger that discards all output — use in tests
// so logs don't pollute `go test -v` output.
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
