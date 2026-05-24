package grpcadapter

import (
	domain "sso/internal/audit/internal/domain"
	usecase "sso/internal/audit/internal/service"
	grpcerr "sso/internal/platform/grpc/errors"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/grpc/codes"
)

// errorMap routes audit-domain sentinels to their gRPC status.
// ErrAuditNotFound uses Reason = ERROR_REASON_UNSPECIFIED — there is no
// audit-specific reason in errors.proto today (the proto comment only
// mandates the gRPC status), so grpcerr.MapError emits a bare
// NOT_FOUND with no ErrorInfo attachment. If a dedicated reason is
// added later (next free tag in the 200 block), update this entry —
// the single place to change.
var errorMap = map[error]grpcerr.ErrorMapping{
	domain.ErrAuditNotFound: {
		Code:    codes.NotFound,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_UNSPECIFIED,
		Message: "audit event not found",
	},
	usecase.ErrPermissionDenied: {
		Code:    codes.PermissionDenied,
		Reason:  ssocommonv1.ErrorReason_ERROR_REASON_PERMISSION_DENIED,
		Message: "caller lacks AUDIT_READ",
	},
}

// toGRPCError is the per-package thin wrapper around grpcerr.MapError.
func toGRPCError(err error) error {
	return grpcerr.MapError(err, errorMap)
}
