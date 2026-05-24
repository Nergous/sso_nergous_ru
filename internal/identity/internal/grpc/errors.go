package grpcadapter

import (
	"sso/internal/identity/internal/domain"
	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes identity-domain sentinels to their gRPC status. The
// public ErrUserAlreadyExists message stays generic ("registration
// failed") for user-enumeration mitigation. ErrUserNotDeleted reuses
// the USER_DELETED reason code — DELETED is a strict superset for
// signalling purposes on this RPC family.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrUserNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_NOT_FOUND,
		Message: "user not found",
	},
	domain.ErrUserAlreadyExists: {
		Code:    codes.AlreadyExists,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_ALREADY_EXISTS,
		Message: "registration failed",
	},
	domain.ErrEtagMismatch: {
		Code:    codes.Aborted,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ETAG_MISMATCH,
		Message: "etag mismatch",
	},
	domain.ErrUserDeleted: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_DELETED,
		Message: "user is deleted",
	},
	domain.ErrUserNotDeleted: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_DELETED,
		Message: "user is not in deleted state",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
