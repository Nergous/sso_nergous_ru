package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Extractor pulls the rate-limit Key out of a request for one policy. ok is
// false when no key can be derived — for example a missing peer address or an
// unexpected request type — in which case the interceptor skips that policy for
// the request rather than rejecting it.
type Extractor func(ctx context.Context, req any) (key Key, ok bool)

// MethodLimit binds one Policy to the Extractor that produces its key. A single
// gRPC method may carry several, each defending a different dimension (say a
// per-IP and a per-username limit on the same login call).
type MethodLimit struct {
	Policy    PolicyName
	Extractor Extractor
}

// Interceptor enforces rate-limit policies on gRPC unary methods. Build it with
// New and install Unary on the server. It is safe for concurrent use; its
// policy and binding tables are treated as immutable after construction.
type Interceptor struct {
	limiters map[PolicyName]*KeyedLimiter
	bindings map[string][]MethodLimit
}

// New builds an Interceptor. policies maps each PolicyName to its
// configuration, bindings maps a gRPC full method name to the limits guarding
// it, and cleanupInterval sets how often each limiter's idle-eviction sweep
// runs. New returns an error if any binding references a policy missing from
// policies, so a wiring mistake fails at startup instead of silently leaving a
// method unprotected.
func New(
	policies map[PolicyName]Policy,
	bindings map[string][]MethodLimit,
	cleanupInterval time.Duration,
) (*Interceptor, error) {
	limiters := make(map[PolicyName]*KeyedLimiter, len(policies))
	for name, p := range policies {
		limiters[name] = NewKeyedLimiter(p, cleanupInterval)
	}

	// Fail fast on wiring mistakes: a binding that references an
	// unregistered policy would otherwise be silently skipped at request
	// time, leaving the RPC unprotected (fail-open).
	for method, mls := range bindings {
		for _, ml := range mls {
			if _, ok := limiters[ml.Policy]; !ok {
				return nil, fmt.Errorf("ratelimit: method %q binds unknown policy %q", method, ml.Policy)
			}
		}
	}

	return &Interceptor{
		limiters: limiters,
		bindings: bindings,
	}, nil
}

// Start launches the background idle-eviction sweep for every limiter; each
// runs until ctx is cancelled. Start does not block.
func (i *Interceptor) Start(ctx context.Context) {
	for _, l := range i.limiters {
		go l.Start(ctx)
	}
}

// Unary returns a grpc.UnaryServerInterceptor that applies the configured
// limits. A method with no binding passes through untouched. For a bound method
// each limit is evaluated in order: a limit whose Extractor yields no key is
// skipped, and the first limit that denies aborts the request with
// codes.ResourceExhausted. Limits evaluated before the denying one have already
// counted the request against their buckets.
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

// rateLimitedError builds the ResourceExhausted status returned to a throttled
// caller, attaching an ErrorInfo (reason and domain) and a RetryInfo carrying
// the suggested back-off.
func rateLimitedError(retryAfter time.Duration) error {
	st := status.New(codes.ResourceExhausted, "rate limit exceeded")
	info := &errdetails.ErrorInfo{
		Reason: ssocommonv1.ErrorReason_ERROR_REASON_RATE_LIMITED.String(),
		Domain: grpcerr.ErrorDomain,
		Metadata: map[string]string{
			"retry_after_ms": strconv.FormatInt(retryAfter.Milliseconds(), 10),
		},
	}
	// RetryInfo is the canonical gRPC back-off detail understood by standard
	// clients and proxies; ErrorInfo carries our domain-specific reason. The
	// retry_after_ms metadata stays for existing clients reading it.
	retry := &errdetails.RetryInfo{RetryDelay: durationpb.New(retryAfter)}
	withDetails, err := st.WithDetails(info, retry)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
