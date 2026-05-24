package domain

import (
	"errors"
)

var (
	ErrRoleNotFound       = errors.New("role: not found")
	ErrRoleAlreadyExists  = errors.New("role: already exists")
	ErrEtagMismatch       = errors.New("role: etag mismatch")
	ErrRoleDisabled       = errors.New("role: disabled")
	ErrRoleNotInApp       = errors.New("role: not in app")
	ErrRoleHasAssignments = errors.New("role: has assignments")
)
