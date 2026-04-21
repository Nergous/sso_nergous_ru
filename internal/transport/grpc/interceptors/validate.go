package interceptors

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ValidateUnaryInterceptor runs protovalidate against the request message.
// Messages without buf.validate annotations (e.g. v1 types) pass through
// as a no-op.
func ValidateUnaryInterceptor() grpc.UnaryServerInterceptor {
	validator, err := protovalidate.New()
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return handler(ctx, req)
		}
		if err := validator.Validate(msg); err != nil {
			var verr *protovalidate.ValidationError
			if errors.As(err, &verr) {
				return nil, status.Error(codes.InvalidArgument, verr.Error())
			}
			return nil, status.Error(codes.InvalidArgument, "validation failed")
		}
		return handler(ctx, req)
	}
}
