// Package access is the bounded context for role assignments and
// authorization decisions.
//
// It deliberately does NOT import the identity / role / app domains.
// Cross-context handles are represented by typed string aliases
// (UserID, RoleID, AppID, ActorID); the use-case layer is the level
// where access cooperates with role.Repository / identity.Repository
// for precondition checks.
package domain

import (
	"time"

	"sso/internal/kernel/validation"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// Cross-context UUID handles. These are intentionally typed aliases (not
// imports) so the bounded context stays thin. Validate-on-input via
// Parse*; constructors are not provided because access never creates
// these IDs — it only references them.
// ----------------------------------------------------------------------------

type UserID string
type RoleID string
type AppID string

// ActorID is the principal that performed a write (granted_by_user_id in
// the proto). Practically a user_id or service_account_id; access does
// not need to discriminate, so it's a single opaque alias. Empty value
// means "unknown actor" (placeholder until the auth interceptor lands).
type ActorID string

func ParseUserID(s string) (UserID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "user_id", Reason: "must be a valid UUID"}
	}
	return UserID(s), nil
}

func ParseRoleID(s string) (RoleID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "role_id", Reason: "must be a valid UUID"}
	}
	return RoleID(s), nil
}

func ParseAppID(s string) (AppID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "app_id", Reason: "must be a valid UUID"}
	}
	return AppID(s), nil
}

// ParseActorID accepts the empty string (unknown actor) or a valid UUID.
func ParseActorID(s string) (ActorID, error) {
	if s == "" {
		return "", nil
	}
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "granted_by_user_id", Reason: "must be a valid UUID"}
	}
	return ActorID(s), nil
}

func (u UserID) String() string  { return string(u) }
func (r RoleID) String() string  { return string(r) }
func (a AppID) String() string   { return string(a) }
func (a ActorID) String() string { return string(a) }

// ----------------------------------------------------------------------------
// RoleAssignment — flat (user × role) tuple with audit metadata.
//
// Unlike most aggregates in this codebase the RoleAssignment is
// immutable: the proto has no UpdateRoleAssignment surface and there
// is no etag, status or patch. Only Grant/Remove operations exist. The
// fields are exposed as plain values; constructors stamp granted_at
// and the use-case layer fills in the rest.
// ----------------------------------------------------------------------------

type RoleAssignment struct {
	UserID          UserID
	RoleID          RoleID
	AppID           AppID
	GrantedByUserID ActorID
	GrantedAt       time.Time
}

type NewRoleAssignmentParams struct {
	UserID          UserID
	RoleID          RoleID
	AppID           AppID
	GrantedByUserID ActorID
	Now             time.Time
}

func NewRoleAssignment(p NewRoleAssignmentParams) *RoleAssignment {
	return &RoleAssignment{
		UserID:          p.UserID,
		RoleID:          p.RoleID,
		AppID:           p.AppID,
		GrantedByUserID: p.GrantedByUserID,
		GrantedAt:       p.Now,
	}
}
