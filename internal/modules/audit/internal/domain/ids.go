package domain

import (
	"sso/internal/kernel/validation"

	"github.com/google/uuid"
)

// ActorID is the UUID of the principal that initiated the action.
// The kind (user vs service_account) is carried in ActorType.
type ActorID string

func ParseActorID(s string) (ActorID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "actor_id", Reason: "must be a valid UUID"}
	}
	return ActorID(s), nil
}

func (a ActorID) String() string { return string(a) }

// SubjectID is the UUID of the principal acted upon (optional — empty
// when the event has no subject, e.g. "list apps"). Kind is carried in
// SubjectType.
type SubjectID string

func ParseSubjectID(s string) (SubjectID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "subject_id", Reason: "must be a valid UUID"}
	}
	return SubjectID(s), nil
}

func (s SubjectID) String() string { return string(s) }

// AppID is the UUID of the app in whose context the event happened
// (optional — empty for cross-app events). Mirrors role.AppID; kept local
// to avoid coupling the audit bounded context to internal/domain/app.
type AppID string

func ParseAppID(s string) (AppID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "app_id", Reason: "must be a valid UUID"}
	}
	return AppID(s), nil
}

func (a AppID) String() string { return string(a) }
