package domain

import (
	"fmt"
	"slices"
	"time"

	"sso/internal/modules/app"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// RoleID — RFC 4122 UUID, generated as v7 (k-sortable).
// ----------------------------------------------------------------------------

type RoleID string

func NewRoleID() (RoleID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate role id: %w", err)
	}
	return RoleID(id.String()), nil
}

func ParseRoleID(s string) (RoleID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "role_id", Reason: "must be a valid UUID"}
	}
	return RoleID(s), nil
}

func (r RoleID) String() string { return string(r) }

// ----------------------------------------------------------------------------
// Role aggregate
// ----------------------------------------------------------------------------
//
// Field visibility split:
//
//   Unexported (only the aggregate itself can change them):
//     id, appID, status, etag, createdAt, updatedAt, permissions
//   * id and appID are immutable after construction.
//   * createdAt is immutable after construction.
//   * status is advanced only by Disable/Enable.
//   * etag and updatedAt are advanced exclusively by bumpVersion.
//   * permissions is canonicalised (sorted) on store, so the equality
//     check in ApplyPatch is a straight slice compare.
//
//   Exported (plain data):
//     Name, Description

type Role struct {
	id          RoleID
	appID       AppID
	status      RoleStatus
	etag        etag.Etag
	createdAt   time.Time
	updatedAt   time.Time
	permissions []string

	Name        string
	Description string
}

// NewRoleParams carries the values supplied by the CreateRole use-case.
// Server-managed fields (status defaults to ACTIVE here; etag/timestamps
// stamped by NewRole) are not part of it.
type NewRoleParams struct {
	ID          RoleID
	AppID       app.AppID
	Name        string
	Description string
	Permissions []string
	Now         time.Time
}

// NewRole constructs a fresh Role. Status defaults to ACTIVE per proto;
// etag freshly minted; created_at / updated_at stamped from Now.
// Permissions are stored in sorted (canonical) order.
func NewRole(p NewRoleParams) *Role {
	return &Role{
		id:          p.ID,
		appID:       p.AppID,
		status:      RoleStatusActive,
		etag:        etag.New(),
		createdAt:   p.Now,
		updatedAt:   p.Now,
		permissions: canonicalPerms(p.Permissions),
		Name:        p.Name,
		Description: p.Description,
	}
}

// RestoreRoleParams carries the full row read back from the repository.
type RestoreRoleParams struct {
	ID          RoleID
	AppID       app.AppID
	Name        string
	Description string
	Permissions []string
	Status      RoleStatus
	Etag        etag.Etag
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RestoreRole rebuilds a Role from a persisted row. No validation: the
// row is trusted (it was written by NewRole/ApplyPatch earlier).
// Permissions are NOT re-sorted because the SQL query that produces them
// already returns them ORDER BY permission ASC.
func RestoreRole(p RestoreRoleParams) *Role {
	return &Role{
		id:          p.ID,
		appID:       p.AppID,
		status:      p.Status,
		etag:        p.Etag,
		createdAt:   p.CreatedAt,
		updatedAt:   p.UpdatedAt,
		permissions: p.Permissions,
		Name:        p.Name,
		Description: p.Description,
	}
}

// Read-only accessors for the unexported fields.
func (r *Role) ID() RoleID            { return r.id }
func (r *Role) AppID() app.AppID      { return r.appID }
func (r *Role) Status() RoleStatus    { return r.status }
func (r *Role) Etag() etag.Etag       { return r.etag }
func (r *Role) CreatedAt() time.Time  { return r.createdAt }
func (r *Role) UpdatedAt() time.Time  { return r.updatedAt }
func (r *Role) Permissions() []string { return r.permissions }

// ----------------------------------------------------------------------------
// RolePatch — set of changes for ApplyPatch. nil pointer = "field not in
// the update mask"; non-nil pointer = "set to this value, even if the
// value is the zero value".
//
// status, role_id, app_id, etag, timestamps are intentionally absent —
// they are not patch-able through this surface (proto contract).
// ----------------------------------------------------------------------------

type RolePatch struct {
	Name        *string
	Description *string
	Permissions *[]string
}

func (p RolePatch) IsEmpty() bool {
	return p.Name == nil && p.Description == nil && p.Permissions == nil
}

// ----------------------------------------------------------------------------
// Mutators
// ----------------------------------------------------------------------------

// Disable transitions ACTIVE → DISABLED. Idempotent on DISABLED.
func (r *Role) Disable(now time.Time) {
	if r.status == RoleStatusDisabled {
		return
	}
	r.status = RoleStatusDisabled
	r.bumpVersion(now)
}

// Enable transitions DISABLED → ACTIVE. Idempotent on ACTIVE.
func (r *Role) Enable(now time.Time) {
	if r.status == RoleStatusActive {
		return
	}
	r.status = RoleStatusActive
	r.bumpVersion(now)
}

// ApplyPatch applies the supplied changes. Bumps etag/updated_at only when
// at least one field actually changes. Permissions are canonicalised
// (sorted) before comparison and storage so the equality check is direct.
func (r *Role) ApplyPatch(p RolePatch, now time.Time) {
	changed := false
	if p.Name != nil && *p.Name != r.Name {
		r.Name = *p.Name
		changed = true
	}
	if p.Description != nil && *p.Description != r.Description {
		r.Description = *p.Description
		changed = true
	}
	if p.Permissions != nil {
		newPerms := canonicalPerms(*p.Permissions)
		if !slices.Equal(newPerms, r.permissions) {
			r.permissions = newPerms
			changed = true
		}
	}
	if changed {
		r.bumpVersion(now)
	}
}

func (r *Role) bumpVersion(now time.Time) {
	r.updatedAt = now
	r.etag = etag.New()
}

// canonicalPerms returns a sorted copy of p, leaving the input slice
// untouched. nil input yields nil — distinguishable from an empty
// permission set should that distinction ever matter (currently it does
// not — RolePatch.Permissions == nil already means "not in mask").
func canonicalPerms(p []string) []string {
	if p == nil {
		return nil
	}
	out := append([]string(nil), p...)
	slices.Sort(out)
	return out
}
