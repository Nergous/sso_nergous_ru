package domain

import "errors"

var (
	ErrServiceAccountNotFound           = errors.New("serviceAccount: not found")
	ErrServiceAccountAlreadyExists      = errors.New("serviceAccount: already exists")
	ErrServiceAccountDisabled           = errors.New("serviceAccount: is disabled")
	ErrServiceAccountInvalidCredentials = errors.New("serviceAccount: invalid credentials")
	ErrEtagMismatch                     = errors.New("serviceAccount: etag mismatch")
)
