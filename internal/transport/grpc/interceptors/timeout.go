package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// TimeoutUnaryInterceptor applies a default timeout to every unary RPC.
// If the caller already set a shorter deadline, that deadline is preserved
// because context.WithTimeout only shortens — never extends — deadlines.
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
