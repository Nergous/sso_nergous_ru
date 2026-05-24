package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestRequestID_Reuse(t *testing.T) {
	const incoming = "test-incoming-id"
	var got string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		// The middleware also rewrites the request header so the
		// gateway header-matcher forwards the same ID to gRPC.
		if r.Header.Get(requestIDHeader) != incoming {
			t.Fatalf("request header not propagated: got %q", r.Header.Get(requestIDHeader))
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(requestIDHeader, incoming)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got != incoming {
		t.Fatalf("ctx id mismatch: got %q want %q", got, incoming)
	}
	if rec.Header().Get(requestIDHeader) != incoming {
		t.Fatalf("response header mismatch: got %q want %q", rec.Header().Get(requestIDHeader), incoming)
	}
}

func TestRequestID_Generate(t *testing.T) {
	var got string
	h := requestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// 16 random bytes hex-encoded = 32 lowercase hex chars.
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(got) {
		t.Fatalf("generated id has wrong shape: %q", got)
	}
	if rec.Header().Get(requestIDHeader) != got {
		t.Fatalf("response header mismatch: got %q want %q", rec.Header().Get(requestIDHeader), got)
	}
}

func TestRecover_PanicReturns500(t *testing.T) {
	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	h := recoverMiddleware(log)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/explode", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(buf.String(), "panic in http handler") {
		t.Fatalf("expected panic log entry, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "/explode") {
		t.Fatalf("expected path in log entry, got: %s", buf.String())
	}
}

func TestLogging_FieldsPresent(t *testing.T) {
	buf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	chain := requestIDMiddleware(loggingMiddleware(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(w, "tea")
	})))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/teapot", nil))

	logLine := buf.String()
	for _, want := range []string{
		`"method":"POST"`,
		`"path":"/teapot"`,
		`"status":418`,
		`"bytes":3`,
		`"request_id":"`,
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log missing %q\nfull line: %s", want, logLine)
		}
	}
}
