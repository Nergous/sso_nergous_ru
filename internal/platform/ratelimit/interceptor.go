package ratelimit

import (
	"context"
	"strconv"
	"time"

	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Extractor func(ctx context.Context, req any) (key Key, ok bool)

type MethodLimit struct {
	Policy    PolicyName
	Extractor Extractor
}

type Interceptor struct {
	limiters map[PolicyName]*KeyedLimiter
	bindings map[string][]MethodLimit
}

func New(
	policies map[PolicyName]Policy,
	bindings map[string][]MethodLimit,
	cleanupInterval time.Duration,
) *Interceptor {
	limiters := make(map[PolicyName]*KeyedLimiter, len(policies))
	for name, p := range policies {
		limiters[name] = NewKeyedLimiter(p, cleanupInterval)
	}
	return &Interceptor{
		limiters: limiters,
		bindings: bindings,
	}
}

func (i *Interceptor) Start(ctx context.Context) {
	for _, l := range i.limiters {
		go l.Start(ctx)
	}
}

func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		limits, ok := i.bindings[info.FullMethod]
		if !ok {
			return handler(ctx, req)
		}

		for _, ml := range limits {
			key, ok := ml.Extractor(ctx, req)
			if !ok {
				continue
			}
			limiter, ok := i.limiters[ml.Policy]
			if !ok {
				continue
			}

			if allowed, retryAfter := limiter.Allow(key); !allowed {
				return nil, rateLimitedError(retryAfter)
			}
		}
		return handler(ctx, req)
	}
}

func rateLimitedError(retryAfter time.Duration) error {
	st := status.New(codes.ResourceExhausted, "rate limit exceeded")
	info := &errdetails.ErrorInfo{
		Reason: ssocommonv1.ErrorReason_ERROR_REASON_RATE_LIMITED.String(),
		Domain: grpcerr.ErrorDomain,
		Metadata: map[string]string{
			"retry_after_ms": strconv.FormatInt(retryAfter.Milliseconds(), 10),
		},
	}
	withDetails, err := st.WithDetails(info)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
