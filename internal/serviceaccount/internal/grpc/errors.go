package grpcadapter

import (
	grpcerr "sso/internal/platform/grpc/errors"
	domain "sso/internal/serviceaccount/internal/domain"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes service-account-domain sentinels to their gRPC status.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrServiceAccountNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_SERVICE_ACCOUNT_NOT_FOUND,
		Message: "service account not found",
	},
	domain.ErrServiceAccountAlreadyExists: {
		Code:    codes.AlreadyExists,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_SERVICE_ACCOUNT_ALREADY_EXISTS,
		Message: "service account already exists",
	},
	domain.ErrServiceAccountDisabled: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_SERVICE_ACCOUNT_DISABLED,
		Message: "service account is disabled",
	},
	domain.ErrEtagMismatch: {
		Code:    codes.Aborted,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ETAG_MISMATCH,
		Message: "etag mismatch",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
