package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

func TimeoutUnaryInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(ctx, req)
	}
}
