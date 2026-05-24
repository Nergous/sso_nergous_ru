package grpcserver

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// errorDomain is attached to every google.rpc.ErrorInfo emitted by the
// validation interceptor. Documented in sso/common/v1/errors.proto.
const errorDomain = "sso.nergous.ru"

// unaryValidation runs protovalidate against every incoming proto request.
// Failures surface as INVALID_ARGUMENT carrying:
//   - google.rpc.ErrorInfo with reason=ERROR_REASON_VALIDATION_FAILED
//   - google.rpc.BadRequest enumerating field-level violations
//
// Compilation of message-specific validators happens lazily on first sight
// of each request type and is then cached, so steady-state cost is minimal.
//
// Non-proto requests (none in our surface today) pass through untouched.
func unaryValidation() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return handler(ctx, req)
		}
		if err := protovalidate.Validate(msg); err != nil {
			return nil, validationStatus(err).Err()
		}
		return handler(ctx, req)
	}
}

// validationStatus assembles the gRPC status returned for a validation
// failure. Returns the bare INVALID_ARGUMENT status if attaching details
// fails (it should not under normal circumstances) — surfacing the right
// code matters more than carrying the details.
func validationStatus(err error) *status.Status {
	st := status.New(codes.InvalidArgument, "validation failed")
	info := &errdetails.ErrorInfo{
		Reason: ssocommonv1.ErrorReason_ERROR_REASON_VALIDATION_FAILED.String(),
		Domain: errorDomain,
	}

	var verr *protovalidate.ValidationError
	if !errors.As(err, &verr) || len(verr.Violations) == 0 {
		// Unexpected: not a structured validation error, or empty. Attach
		// just the ErrorInfo so clients can still discriminate by reason.
		if withInfo, dErr := st.WithDetails(info); dErr == nil {
			return withInfo
		}
		return st
	}

	br := &errdetails.BadRequest{
		FieldViolations: make([]*errdetails.BadRequest_FieldViolation, 0, len(verr.Violations)),
	}
	for _, v := range verr.Violations {
		if v == nil || v.Proto == nil {
			continue
		}
		br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
			Field:       protovalidate.FieldPathString(v.Proto.GetField()),
			Description: v.Proto.GetMessage(),
		})
	}

	if withDetails, dErr := st.WithDetails(info, br); dErr == nil {
		return withDetails
	}
	return st
}
