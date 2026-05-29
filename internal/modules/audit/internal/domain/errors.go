package domain

import "errors"

var (
	ErrAuditNotFound      = errors.New("audit: not found")
	ErrInvalidEventType   = errors.New("audit: invalid event_type")
	ErrInvalidActorType   = errors.New("audit: invalid actor_type")
	ErrInvalidSubjectType = errors.New("audit: invalid subject_type")
	ErrInvalidOutcome     = errors.New("audit: invalid outcome")
)
