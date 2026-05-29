package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"sso/internal/platform/config"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type Server struct {
	cfg        config.GRPCConfig
	grpcServer *grpc.Server
	log        *slog.Logger
}

// Registrar attaches a gRPC service implementation to the underlying
// *grpc.Server. Bootstrap passes one Registrar per business handler (e.g.
// IdentityHandler.Register). Health and reflection are still registered
// internally so the wiring stays a single line.
type Registrar func(*grpc.Server)

// New builds the gRPC server with the standard interceptor chain. Both
// `unaryAuth` and `unaryRateLimit` are optional (nil-tolerant for tests
// / bootstraps that haven't wired them yet). Order matters:
//
//	requestID → logging → recovery → auth → ratelimit → validation
//
// Auth precedes ratelimit so policies can key on the authenticated
// subject. Ratelimit precedes validation so we don't burn CPU on
// protobuf validation for a request we're about to reject anyway.
func New(
	cfg config.GRPCConfig, log *slog.Logger,
	unaryAuth grpc.UnaryServerInterceptor,
	unaryRateLimit grpc.UnaryServerInterceptor,
	registrars ...Registrar,
) (*Server, error) {
	unary := []grpc.UnaryServerInterceptor{
		unaryRequestID(),
		unaryLogging(log),
		unaryRecovery(log),
	}
	if unaryAuth != nil {
		unary = append(unary, unaryAuth)
	}
	if unaryRateLimit != nil {
		unary = append(unary, unaryRateLimit)
	}
	unary = append(unary, unaryValidation())

	opts := []grpc.ServerOption{
		grpc.ConnectionTimeout(cfg.ConnectionTimeout),
		grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.MaxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    cfg.Keepalive.Time,
			Timeout: cfg.Keepalive.Timeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             cfg.Keepalive.MinTime,
			PermitWithoutStream: cfg.Keepalive.PermitWithoutStream,
		}),
		grpc.ChainUnaryInterceptor(unary...),
	}

	if cfg.TLS.Enabled {
		creds, err := credentials.NewServerTLSFromFile(cfg.TLS.CertPath, cfg.TLS.KeyPath)
		if err != nil {
			return nil, fmt.Errorf("load tls keypair: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	s := grpc.NewServer(opts...)

	if cfg.HealthCheck.Enabled {
		healthpb.RegisterHealthServer(s, health.NewServer())
	}
	if cfg.Reflection.Enabled {
		reflection.Register(s)
	}

	for _, register := range registrars {
		register(s)
	}

	return &Server{
		cfg:        cfg,
		grpcServer: s,
		log:        log,
	}, nil
}

func (s *Server) Run() error {
	l, err := net.Listen("tcp", s.cfg.Address())
	if err != nil {
		return err
	}
	return s.grpcServer.Serve(l)
}

func (s *Server) Stop() {
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	t := time.NewTimer(s.cfg.ShutdownTimeout)
	defer t.Stop()

	select {
	case <-done:
	case <-t.C:
		s.log.Warn("graceful shutdown timed out, force exiting")
		s.grpcServer.Stop()
	}
}

func unaryRecovery(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(ctx, "panic in unary handler",
					slog.String("method", info.FullMethod),
					slog.Any("recover", r),
					slog.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
