package grpcserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const requestIDHeader = "x-request-id"

type ctxKey struct{}

var requestIDKey ctxKey

// RequestIDFromContext returns the per-RPC request ID injected by
// unaryRequestID, or "" if none is attached. Handlers can use it to tag their
// own log records.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// unaryRequestID assigns a request ID to every RPC: it reuses x-request-id
// from the client metadata when present (e.g. when an upstream service forwards
// its own ID), otherwise it mints a fresh 128-bit hex value. The ID is stored
// in the context and echoed back to the client in the response header.
func unaryRequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		id := incomingRequestID(ctx)
		if id == "" {
			id = newRequestID()
		}
		ctx = context.WithValue(ctx, requestIDKey, id)
		_ = grpc.SetHeader(ctx, metadata.Pairs(requestIDHeader, id))
		return handler(ctx, req)
	}
}

func incomingRequestID(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(requestIDHeader)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// unaryLogging emits one record per RPC: method, gRPC code, duration, peer,
// request_id, and error text on failure. Severity follows levelFor: expected
// client-side errors stay at Info; only server-side faults are Error.
func unaryLogging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := status.Code(err)

		attrs := []slog.Attr{
			slog.String("method", info.FullMethod),
			slog.String("code", code.String()),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", RequestIDFromContext(ctx)),
		}
		if p, ok := peer.FromContext(ctx); ok {
			attrs = append(attrs, slog.String("peer", p.Addr.String()))
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		log.LogAttrs(ctx, levelFor(code), "rpc", attrs...)

		return resp, err
	}
}

// levelFor maps gRPC status codes to slog levels. Codes that represent normal
// client behavior (validation failures, missing records, auth refusals) stay
// at Info — they are expected and should not pollute Error dashboards. Codes
// that indicate a server-side problem (Internal, Unavailable, DataLoss, ...)
// become Error.
func levelFor(code codes.Code) slog.Level {
	switch code {
	case codes.OK,
		codes.Canceled,
		codes.NotFound,
		codes.AlreadyExists,
		codes.InvalidArgument,
		codes.OutOfRange,
		codes.FailedPrecondition,
		codes.Unauthenticated,
		codes.PermissionDenied,
		codes.ResourceExhausted:
		return slog.LevelInfo
	default:
		return slog.LevelError
	}
}
