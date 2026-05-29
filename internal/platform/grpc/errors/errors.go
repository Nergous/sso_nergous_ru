// Package grpcerr is the shared kernel for translating domain errors
// into gRPC statuses. It owns the wire-level assembly (ErrorInfo /
// BadRequest details); the per-sentinel mapping (which domain error →
// which code/reason) lives in each service's
// infrastructure/grpc/<domain>/errors.go.
package grpcerr

import (
	"errors"

	"sso/internal/kernel/validation"

	ssocommonv1 "github.com/Nergous/sso_protos/gen/go/sso/common/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrorDomain is attached to every google.rpc.ErrorInfo emitted by the
// SSO surface. Documented in sso/common/v1/errors.proto. Exported so
// adjacent packages (e.g. ratelimit) that build their own status objects
// without going through StatusWithReason can reuse the same domain value.
const ErrorDomain = "sso.nergous.ru"

// ValidationError is a deprecated alias retained while in-flight
// migrations finish. New code should use validation.Error directly.
//
// Deprecated: use sso/internal/kernel/validation.Error.
type ValidationError = validation.Error

// StatusWithReason assembles a gRPC status carrying an ErrorInfo with
// the given reason. If detail-attachment fails (it should not under
// normal circumstances), the bare status is returned — surfacing the
// right code matters more than carrying the details.
func StatusWithReason(code codes.Code, reason ssocommonv1.ErrorReason, msg string) error {
	st := status.New(code, msg)
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: reason.String(),
		Domain: ErrorDomain,
	})
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}

// ErrorMapping is one entry in a per-module error translation table.
// Reason == ERROR_REASON_UNSPECIFIED is the sentinel for "emit a bare
// status.Error with no ErrorInfo attachment" — used when the proto
// contract has no matching reason code (role.ErrRoleHasAssignments,
// audit.ErrAuditNotFound).
type ErrorMapping struct {
	Code    codes.Code
	Reason  ssocommonv1.ErrorReason
	Message string
}

// MapError translates a domain or use-case error into a gRPC status
// through a per-module mapping table. Lookup order:
//
//  1. err == nil → nil.
//  2. *validation.Error → StatusWithValidation (field-level details).
//  3. First errors.Is match in m → StatusWithReason (or bare
//     status.Error when Reason is UNSPECIFIED).
//  4. Fallback → codes.Internal with a generic message (never leak
//     internal detail).
//
// Map iteration order is undefined; entries must be mutually exclusive
// under errors.Is.
func MapError(err error, m map[error]ErrorMapping) error {
	if err == nil {
		return nil
	}
	var verr *validation.Error
	if errors.As(err, &verr) {
		return StatusWithValidation(verr)
	}
	for sentinel, em := range m {
		if errors.Is(err, sentinel) {
			if em.Reason == ssocommonv1.ErrorReason_ERROR_REASON_UNSPECIFIED {
				return status.Error(em.Code, em.Message)
			}
			return StatusWithReason(em.Code, em.Reason, em.Message)
		}
	}
	return status.Error(codes.Internal, "internal error")
}

// StatusWithValidation builds an INVALID_ARGUMENT response carrying both
// an ErrorInfo (reason = VALIDATION_FAILED) and a BadRequest detail with
// the offending field.
func StatusWithValidation(verr *validation.Error) error {
	st := status.New(codes.InvalidArgument, "validation failed")
	withDetails, err := st.WithDetails(
		&errdetails.ErrorInfo{
			Reason: ssocommonv1.ErrorReason_ERROR_REASON_VALIDATION_FAILED.String(),
			Domain: ErrorDomain,
		},
		&errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{{
				Field:       verr.Field,
				Description: verr.Reason,
			}},
		},
	)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
