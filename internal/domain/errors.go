// Package domain defines cross-cutting sentinel errors used by services.
// Controllers translate these into gRPC status responses via
// internal/transport/grpc/errmap.
//
// The set mirrors ssov2.ErrorReason so that v2 controllers can return
// google.rpc.ErrorInfo with a stable reason string.
package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrAppNotFound        = errors.New("app not found")
	ErrAppAlreadyExists   = errors.New("app already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrPasswordMismatch   = errors.New("password mismatch")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrValidationFailed   = errors.New("validation failed")
)
