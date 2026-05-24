// Package httpserver runs the public HTTP listener that fronts the gRPC
// server via grpc-gateway. It owns the HTTP-side cross-cutting concerns
// (request ID, structured logging, panic recovery, CORS) and the
// process-level probes (/healthz, /readyz, /metrics). Authentication is
// intentionally NOT done here: gateway forwards the Authorization header
// to gRPC as metadata, and the existing grpcauth interceptor verifies it
// once, on the gRPC side, so there is a single source of truth.
package httpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sso/internal/platform/config"
)

// Deps holds the inputs httpserver needs from bootstrap. GRPCTarget is the
// loopback endpoint of the in-process gRPC server (e.g. "127.0.0.1:44044");
// the gateway opens one ClientConn to that target and shares it across all
// service handlers. Readiness is invoked on every /readyz probe — pass nil
// for "always ready".
type Deps struct {
	Cfg        config.HTTPConfig
	Log        *slog.Logger
	GRPCTarget string
	Readiness  ReadinessFunc
}

type Server struct {
	cfg        config.HTTPConfig
	httpServer *http.Server
	conn       *grpc.ClientConn
	log        *slog.Logger
}

// New wires the gateway, the probe handlers, and the middleware chain. The
// returned Server owns the gRPC ClientConn it opened for gateway → backend
// traffic; Stop closes it together with the HTTP listener.
//
// The ctx is used only by gateway registrars (they spawn watcher goroutines
// for streaming RPCs). Cancelling it later does not stop the HTTP listener
// — use Stop for that.
func New(ctx context.Context, deps Deps) (*Server, error) {
	if deps.Log == nil {
		return nil, errors.New("httpserver: Log is required")
	}
	if deps.GRPCTarget == "" {
		return nil, errors.New("httpserver: GRPCTarget is required")
	}

	conn, err := dialBackend(deps.GRPCTarget)
	if err != nil {
		return nil, fmt.Errorf("httpserver: dial gRPC backend: %w", err)
	}

	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(incomingHeaderMatcher),
	)
	if err := registerGatewayHandlers(ctx, mux, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}

	root := http.NewServeMux()
	root.Handle("/healthz", healthzHandler())
	root.Handle("/readyz", readyzHandler(deps.Log, deps.Readiness))
	root.Handle("/metrics", metricsStubHandler())
	root.Handle("/", mux)

	// Outermost first: recover wraps everything so a panic in any later
	// middleware or handler is captured. requestID before logging so the
	// logger sees the assigned ID. CORS before the gateway so preflight
	// is answered without dialling gRPC.
	var handler http.Handler = root
	handler = corsMiddleware(deps.Cfg.CORS.AllowedOrigins)(handler)
	handler = loggingMiddleware(deps.Log)(handler)
	handler = requestIDMiddleware(handler)
	handler = recoverMiddleware(deps.Log)(handler)

	srv := &http.Server{
		Addr:    deps.Cfg.Address(),
		Handler: handler,
		// BaseContext gives every request a context rooted in process
		// lifetime — handlers can shadow it with their own deadlines,
		// but there's always a real ctx to start from.
		BaseContext: func(_ net.Listener) context.Context { return context.Background() },
	}

	if deps.Cfg.TLS.Enabled {
		srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	return &Server{
		cfg:        deps.Cfg,
		httpServer: srv,
		conn:       conn,
		log:        deps.Log,
	}, nil
}

// Run starts the HTTP listener and blocks until the server stops. Returns nil
// on clean shutdown (http.ErrServerClosed) and the underlying error otherwise.
func (s *Server) Run() error {
	s.log.Info("httpserver: starting",
		slog.String("address", s.cfg.Address()),
		slog.Bool("tls", s.cfg.TLS.Enabled),
	)
	var err error
	if s.cfg.TLS.Enabled {
		err = s.httpServer.ListenAndServeTLS(s.cfg.TLS.CertPath, s.cfg.TLS.KeyPath)
	} else {
		err = s.httpServer.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Stop performs a graceful HTTP shutdown bounded by the caller-supplied
// context, then closes the shared gRPC ClientConn. The conn close is
// best-effort: by the time we get here the gRPC server is also being torn
// down, so a transient error on close is not actionable.
func (s *Server) Stop(ctx context.Context) error {
	shutdownErr := s.httpServer.Shutdown(ctx)
	if closeErr := s.conn.Close(); closeErr != nil {
		s.log.Warn("httpserver: close gateway conn", slog.Any("error", closeErr))
	}
	return shutdownErr
}

// dialBackend opens the in-process loopback gRPC connection used by the
// gateway. Plaintext is correct here: the traffic never leaves the loopback
// interface, and adding TLS on top would just require us to pin our own
// self-signed cert against ourselves. When the public gRPC listener gains
// TLS in TODO §5.7, this dial stays plaintext.
func dialBackend(target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// incomingHeaderMatcher extends runtime.DefaultHeaderMatcher with the
// request-ID header. Default already forwards Authorization (the gRPC auth
// interceptor reads it) and the standard grpcgateway-* set; we just need
// X-Request-Id to ride along so gRPC's unaryRequestID picks up the same ID
// instead of minting a fresh one.
func incomingHeaderMatcher(key string) (string, bool) {
	if strings.EqualFold(key, requestIDHeader) {
		// Lower-cased to match gRPC metadata convention (and how
		// unaryRequestID reads "x-request-id").
		return strings.ToLower(requestIDHeader), true
	}
	return runtime.DefaultHeaderMatcher(key)
}
