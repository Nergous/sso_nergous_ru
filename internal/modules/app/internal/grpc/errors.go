package app

import (
	"sso/internal/modules/app/internal/domain"
	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes app-domain sentinels to their gRPC status. State-
// machine refusals (ErrAppDisabled, ErrAppInMaintenance) surface as
// FailedPrecondition with their dedicated reason codes.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrAppNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_APP_NOT_FOUND,
		Message: "app not found",
	},
	domain.ErrAppAlreadyExists: {
		Code:    codes.AlreadyExists,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_APP_ALREADY_EXISTS,
		Message: "app already exists",
	},
	domain.ErrEtagMismatch: {
		Code:    codes.Aborted,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ETAG_MISMATCH,
		Message: "etag mismatch",
	},
	domain.ErrAppDisabled: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_APP_DISABLED,
		Message: "app is disabled",
	},
	domain.ErrAppInMaintenance: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_APP_IN_MAINTENANCE,
		Message: "app is in maintenance",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
