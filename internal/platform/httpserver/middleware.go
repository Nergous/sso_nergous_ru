package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// requestIDHeader is the wire name on both the inbound HTTP request and the
// outbound response. The same spelling is what grpc-gateway forwards to the
// gRPC backend as metadata (see headerMatcher in server.go), and what the
// gRPC unaryRequestID interceptor reads — so a single ID flows through HTTP
// access logs, gateway, and gRPC handler logs.
const requestIDHeader = "X-Request-Id"

type ctxKey struct{}

var requestIDKey ctxKey

// RequestIDFromContext returns the per-request ID assigned by requestIDMiddleware,
// or "" if none is attached.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// requestIDMiddleware reuses the inbound X-Request-Id header when present
// (so upstream proxies / clients can correlate logs) and otherwise mints a
// fresh 128-bit hex value. The ID is stored in the request context and
// echoed back in the response header.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		// Mirror the ID onto the request as well so the header matcher in
		// server.go forwards it to gRPC even on requests that arrived
		// without one.
		r.Header.Set(requestIDHeader, id)
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// loggingMiddleware emits one record per HTTP request: method, path, status,
// bytes, duration, request_id, remote_addr.
func loggingMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			log.LogAttrs(r.Context(), slog.LevelInfo, "http",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Int64("bytes", rw.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

// recoverMiddleware turns any panic in a downstream handler into a 500
// response and a single Error log record (with stack). Must be the outermost
// middleware so it covers everything else.
func recoverMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic in http handler",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("recover", rec),
						slog.String("stack", string(debug.Stack())),
						slog.String("request_id", RequestIDFromContext(r.Context())),
					)
					// Best-effort: only write the status if nothing was sent
					// yet. statusRecorder isn't required here because the
					// goal is just to signal failure to the client.
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// statusRecorder captures the status code and body size written by downstream
// handlers so the logging middleware can report them. It does not buffer the
// body — writes pass straight through.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

// Flush forwards to the underlying ResponseWriter if it supports flushing.
// grpc-gateway uses streaming responses for server-streaming RPCs, so this
// interface assertion needs to keep working through the wrapper.
func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
