package grpcadapter

import (
	grpcerr "sso/internal/platform/grpc/errors"
	"sso/internal/modules/role/internal/domain"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes role-domain sentinels to their gRPC status.
// ErrRoleHasAssignments uses Reason = ERROR_REASON_UNSPECIFIED — its
// proto reason code (ERROR_REASON_ROLE_HAS_ASSIGNMENTS = 84) was
// retired, so grpcerr.MapError emits a bare FailedPrecondition with no
// ErrorInfo attachment.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrRoleNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_NOT_FOUND,
		Message: "role not found",
	},
	domain.ErrRoleAlreadyExists: {
		Code:    codes.AlreadyExists,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_ALREADY_EXISTS,
		Message: "role already exists",
	},
	domain.ErrEtagMismatch: {
		Code:    codes.Aborted,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ETAG_MISMATCH,
		Message: "etag mismatch",
	},
	domain.ErrRoleDisabled: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_DISABLED,
		Message: "role is disabled",
	},
	domain.ErrRoleNotInApp: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_ROLE_NOT_IN_APP,
		Message: "role is not in app",
	},
	domain.ErrRoleHasAssignments: {
		Code:    codes.FailedPrecondition,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_UNSPECIFIED,
		Message: "role has assignments",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
