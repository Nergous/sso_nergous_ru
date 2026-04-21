package errmap

import (
	"errors"

	"sso/internal/domain"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errDomain = "sso.nergous.ru"

type mapping struct {
	code   codes.Code
	reason string
	msg    string
}

var table = []struct {
	target error
	m      mapping
}{
	{domain.ErrInvalidCredentials, mapping{codes.Unauthenticated, "INVALID_CREDENTIALS", "invalid credentials"}},
	{domain.ErrInvalidToken, mapping{codes.Unauthenticated, "INVALID_TOKEN", "invalid token"}},
	{domain.ErrTokenExpired, mapping{codes.Unauthenticated, "INVALID_TOKEN", "token expired"}},
	{domain.ErrUserAlreadyExists, mapping{codes.AlreadyExists, "USER_ALREADY_EXISTS", "user already exists"}},
	{domain.ErrUserNotFound, mapping{codes.NotFound, "USER_NOT_FOUND", "user not found"}},
	{domain.ErrAppNotFound, mapping{codes.NotFound, "APP_NOT_FOUND", "app not found"}},
	{domain.ErrAppAlreadyExists, mapping{codes.AlreadyExists, "APP_ALREADY_EXISTS", "app already exists"}},
	{domain.ErrPermissionDenied, mapping{codes.PermissionDenied, "PERMISSION_DENIED", "permission denied"}},
	{domain.ErrValidationFailed, mapping{codes.InvalidArgument, "VALIDATION_FAILED", "validation failed"}},
	{domain.ErrPasswordMismatch, mapping{codes.Unauthenticated, "PASSWORD_MISMATCH", "password mismatch"}},
}

func lookup(err error) (mapping, bool) {
	for _, row := range table {
		if errors.Is(err, row.target) {
			return row.m, true
		}
	}
	return mapping{}, false
}

// ToV1 returns a legacy-style gRPC status: safe public message, no details.
// Unknown errors → codes.Internal with a generic message (never the raw text).
func ToV1(err error) error {
	if err == nil {
		return nil
	}
	if m, ok := lookup(err); ok {
		return status.Error(m.code, m.msg)
	}
	return status.Error(codes.Internal, "internal error")
}

// ToV2 attaches google.rpc.ErrorInfo so clients can switch on the reason.
func ToV2(err error) error {
	if err == nil {
		return nil
	}
	if m, ok := lookup(err); ok {
		st := status.New(m.code, m.msg)
		if withDetails, derr := st.WithDetails(&errdetails.ErrorInfo{
			Reason: m.reason,
			Domain: errDomain,
		}); derr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}
	return status.Error(codes.Internal, "internal error")
}
