package httpserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHealthz_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	healthzHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

func TestReadyz_NilReadiness_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	readyzHandler(discardLogger(), nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
}

func TestReadyz_OK(t *testing.T) {
	readiness := func(ctx context.Context) error { return nil }

	rec := httptest.NewRecorder()
	readyzHandler(discardLogger(), readiness).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "ready") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

func TestReadyz_Fail(t *testing.T) {
	readiness := func(ctx context.Context) error { return errors.New("db down") }

	rec := httptest.NewRecorder()
	readyzHandler(discardLogger(), readiness).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "db down") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

func TestMetricsStub_OK(t *testing.T) {
	rec := httptest.NewRecorder()
	metricsStubHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	if !strings.HasPrefix(rec.Body.String(), "#") {
		t.Fatalf("expected prometheus comment, got: %q", rec.Body.String())
	}
}
