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

	var handler http.Handler = root
	handler = corsMiddleware(deps.Cfg.CORS.AllowedOrigins)(handler)
	handler = loggingMiddleware(deps.Log)(handler)
	handler = requestIDMiddleware(handler)
	handler = recoverMiddleware(deps.Log)(handler)

	srv := &http.Server{
		Addr:              deps.Cfg.Address(),
		Handler:           handler,
		ReadTimeout:       deps.Cfg.ReadTimeout,
		ReadHeaderTimeout: deps.Cfg.ReadHeaderTimeout,
		WriteTimeout:      deps.Cfg.WriteTimeout,
		IdleTimeout:       deps.Cfg.IdleTimeout,
		BaseContext:       func(_ net.Listener) context.Context { return context.Background() },
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

func (s *Server) Stop(ctx context.Context) error {
	shutdownErr := s.httpServer.Shutdown(ctx)
	if closeErr := s.conn.Close(); closeErr != nil {
		s.log.Warn("httpserver: close gateway conn", slog.Any("error", closeErr))
	}
	return shutdownErr
}

func dialBackend(target string) (*grpc.ClientConn, error) {
	return grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

func incomingHeaderMatcher(key string) (string, bool) {
	if strings.EqualFold(key, requestIDHeader) {
		return strings.ToLower(requestIDHeader), true
	}
	return runtime.DefaultHeaderMatcher(key)
}
