package grpcapp

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"sso/internal/controllers"
	"sso/internal/transport/grpc/interceptors"

	"google.golang.org/grpc"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	port       int
}

func New(
	log *slog.Logger,
	port int,
	authController *controllers.AuthController,
	userController *controllers.UserController,
	appController *controllers.AppController,
) *App {
	gRPCServer := grpc.NewServer(
		grpc.UnaryInterceptor(interceptors.TimeoutUnaryInterceptor(5 * time.Second)),
	)

	controllers.RegisterAuth(gRPCServer, authController.AuthS)
	controllers.RegisterApp(gRPCServer, appController.AppS, appController.DefaultSecret)
	controllers.RegisterUser(gRPCServer, userController.UserS)

	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		port:       port,
	}
}

func (a *App) MustRun() {
	if err := a.Run(); err != nil {
		panic(err)
	}
}

func (a *App) Run() error {
	const op = "grpcapp.Run"

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	a.log.Info("grpc server is running", slog.Int("port", a.port))

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *App) Stop() {
	const op = "grpcapp.Stop"

	a.log.With(slog.String("operation", op)).Info("stopping grpc server")

	a.gRPCServer.GracefulStop()
}
