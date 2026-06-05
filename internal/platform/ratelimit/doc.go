// Package ratelimit provides in-memory, per-key request rate limiting for
// gRPC unary methods.
//
// The package is deliberately proto-free: it never imports generated message
// types and works on opaque (context.Context, req any) values. Callers supply
// Extractor functions that know how to pull a rate-limit Key out of a specific
// request — a peer IP, a username, a service-account ID, and so on. That keeps
// knowledge of proto types at the wiring layer and leaves this package
// reusable.
//
// # Model
//
// A [Policy] names a token-bucket configuration (rate, burst, idle eviction).
// A [KeyedLimiter] applies one Policy across many keys, holding an independent
// token bucket per [Key] and evicting buckets that stay idle longer than the
// policy's IdleEvict. An [Interceptor] ties policies to gRPC methods: each
// method may be guarded by one or more [MethodLimit] entries, every entry
// pairing a Policy with the Extractor that produces its key.
//
// # Usage
//
// Build the interceptor from a set of policies and a per-method binding table,
// start the background eviction goroutines, and install the returned
// grpc.UnaryServerInterceptor:
//
//	itc, err := ratelimit.New(policies, bindings, cleanupInterval)
//	if err != nil {
//		return err
//	}
//	itc.Start(ctx)
//	server := grpc.NewServer(grpc.UnaryInterceptor(itc.Unary()))
//
// A request that exceeds a limit is rejected with codes.ResourceExhausted and
// a google.rpc.RetryInfo detail carrying the suggested back-off.
//
// # Concurrency
//
// The exported types are safe for concurrent use once constructed. The policy
// and binding maps handed to New are treated as immutable afterwards; do not
// mutate them once the interceptor is built.
package ratelimit
