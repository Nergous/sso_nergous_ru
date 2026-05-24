package grpcadapter

import (
	"sso/internal/access/internal/domain"
	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes access-domain sentinels to their gRPC status.
// ErrUserNotEligible reuses USER_BLOCKED — DELETED is a strict superset
// for signalling purposes on this assignment surface.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrUserNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_NOT_FOUND,
		Message: "user not found",
	},
	domain.ErrRoleNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_NOT_FOUND,
		Message: "role not found",
	},
	domain.ErrAppNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_APP_NOT_FOUND,
		Message: "app not found",
	},
	domain.ErrRoleDisabled: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_DISABLED,
		Message: "role is disabled",
	},
	domain.ErrRoleNotInApp: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_NOT_IN_APP,
		Message: "role does not belong to app",
	},
	domain.ErrUserNotEligible: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_USER_BLOCKED,
		Message: "user is not eligible for assignments",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
