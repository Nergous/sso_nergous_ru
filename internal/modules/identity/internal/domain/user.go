// Package domain holds the User aggregate together with its identifier
// types, status enum, error sentinels, and the Repository interface that
// the persistence adapters in sibling packages implement.
//
// Imports are kept to kernel-level primitives only (etag, validation,
// uuid); the package has no knowledge of transport, persistence or
// audit-bus implementation details.
package domain

import (
	"fmt"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// UserID — RFC 4122 UUID, generated as v7 so the byte order matches creation
// time and id-only keyset pagination stays cheap.
// ----------------------------------------------------------------------------

type UserID string

func NewUserID() (UserID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate user id: %w", err)
	}
	return UserID(id.String()), nil
}

func ParseUserID(s string) (UserID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "user_id", Reason: "must be a valid UUID"}
	}
	return UserID(s), nil
}

func (u UserID) String() string { return string(u) }

// ----------------------------------------------------------------------------
// User aggregate
// ----------------------------------------------------------------------------
//
// Field visibility split:
//
//   Unexported (only the aggregate itself can change them):
//     id, status, etag, createdAt, updatedAt
//   These are either immutable after construction (id, createdAt) or
//   advanced exclusively by behavioural helpers (Disable/Enable/SoftDelete
//   set status; bumpVersion advances etag and updatedAt). Direct
//   assignment would silently break the optimistic-concurrency contract.
//
//   Exported (plain data; mutate freely or via ApplyPatch):
//     Email, Username, DisplayName, AvatarURL, Locale, Timezone, LastLoginAt
//   Convention: when changing these outside ApplyPatch, the caller is
//   expected to also call bumpVersion via a helper. In practice, the
//   service layer always goes through ApplyPatch.

type User struct {
	id                  UserID
	status              UserStatus
	etag                etag.Etag
	createdAt           time.Time
	updatedAt           time.Time
	passwordHash        []byte // nil/empty = no password set (admin-created user awaiting reset)
	failedLoginAttempts int

	Email        string
	Username     string
	DisplayName  string
	AvatarURL    string // empty = absent
	Locale       string
	Timezone     string
	LastLoginAt  time.Time // zero = never logged in
	LockoutUntil time.Time // null = no lockout
}

// NewUserParams carries the values supplied by the CreateUser use-case.
// Server-managed fields (id is generated upstream; etag/timestamps stamped
// here) are not part of it.
//
// PasswordHash is optional: AuthService.Register supplies a bcrypt hash;
// IdentityService.CreateUser leaves it nil (admin-created accounts must
// go through ResetPasswordWithRecoveryCode or a similar flow before
// they can Login).
type NewUserParams struct {
	ID           UserID
	Email        string
	Username     string
	DisplayName  string
	AvatarURL    string
	Locale       string
	Timezone     string
	PasswordHash []byte
	Now          time.Time
}

// NewUser constructs a fresh User. Status defaults to ACTIVE; created_at /
// updated_at stamped from Now; etag freshly minted.
func NewUser(p NewUserParams) *User {
	return &User{
		id:           p.ID,
		status:       UserStatusActive,
		etag:         etag.New(),
		createdAt:    p.Now,
		updatedAt:    p.Now,
		passwordHash: p.PasswordHash,
		Email:        p.Email,
		Username:     p.Username,
		DisplayName:  p.DisplayName,
		AvatarURL:    p.AvatarURL,
		Locale:       p.Locale,
		Timezone:     p.Timezone,
	}
}

// RestoreUserParams carries the full row read back from the repository.
type RestoreUserParams struct {
	ID                  UserID
	Email               string
	Username            string
	DisplayName         string
	AvatarURL           string
	Locale              string
	Timezone            string
	PasswordHash        []byte
	Status              UserStatus
	Etag                etag.Etag
	CreatedAt           time.Time
	UpdatedAt           time.Time
	LastLoginAt         time.Time
	FailedLoginAttempts int
	LockoutUntil        time.Time
}

// RestoreUser rebuilds a User from a persisted row. No validation: the row
// is trusted (it was written by NewUser/ApplyPatch earlier).
func RestoreUser(p RestoreUserParams) *User {
	return &User{
		id:                  p.ID,
		status:              p.Status,
		etag:                p.Etag,
		createdAt:           p.CreatedAt,
		updatedAt:           p.UpdatedAt,
		passwordHash:        p.PasswordHash,
		Email:               p.Email,
		Username:            p.Username,
		DisplayName:         p.DisplayName,
		AvatarURL:           p.AvatarURL,
		Locale:              p.Locale,
		Timezone:            p.Timezone,
		LastLoginAt:         p.LastLoginAt,
		failedLoginAttempts: p.FailedLoginAttempts,
		LockoutUntil:        p.LockoutUntil,
	}
}

// Read-only accessors for the unexported invariant-bearing fields.
func (u *User) ID() UserID               { return u.id }
func (u *User) Status() UserStatus       { return u.status }
func (u *User) Etag() etag.Etag          { return u.etag }
func (u *User) CreatedAt() time.Time     { return u.createdAt }
func (u *User) UpdatedAt() time.Time     { return u.updatedAt }
func (u *User) FailedLoginAttempts() int { return u.failedLoginAttempts }

// PasswordHash returns the stored bcrypt hash. nil/empty means the user
// has no password set yet (admin-created account). Callers that perform
// the credential check live in the auth use-case — domain stays free of
// crypto imports.
func (u *User) PasswordHash() []byte { return u.passwordHash }

// HasPassword reports whether the user has a password on file. Useful
// at Login: a user without a password can never authenticate via the
// password grant and must be funnelled to ResetPasswordWithRecoveryCode.
func (u *User) HasPassword() bool { return len(u.passwordHash) > 0 }

// ----------------------------------------------------------------------------
// UserPatch — set of changes for ApplyPatch. nil pointer = "field not in
// the update mask"; non-nil pointer = "set to this value, even if the value
// is the zero value (e.g. clearing AvatarURL)".
// ----------------------------------------------------------------------------

type UserPatch struct {
	Email       *string
	Username    *string
	DisplayName *string
	AvatarURL   *string
	Locale      *string
	Timezone    *string
}

func (p UserPatch) IsEmpty() bool {
	return p.Email == nil && p.Username == nil && p.DisplayName == nil &&
		p.AvatarURL == nil && p.Locale == nil && p.Timezone == nil
}

// ----------------------------------------------------------------------------
// Mutators
// ----------------------------------------------------------------------------

// Disable transitions ACTIVE → BLOCKED. Idempotent on BLOCKED. Rejected on
// DELETED with ErrUserDeleted.
func (u *User) Disable(now time.Time) error {
	if u.status == UserStatusDeleted {
		return ErrUserDeleted
	}
	if u.status == UserStatusBlocked {
		return nil
	}
	u.status = UserStatusBlocked
	u.bumpVersion(now)
	return nil
}

// Enable transitions BLOCKED → ACTIVE. Idempotent on ACTIVE. Rejected on
// DELETED with ErrUserDeleted.
func (u *User) Enable(now time.Time) error {
	if u.status == UserStatusDeleted {
		return ErrUserDeleted
	}
	if u.status == UserStatusActive {
		return nil
	}
	u.status = UserStatusActive
	u.bumpVersion(now)
	return nil
}

// SoftDelete transitions any state → DELETED. Idempotent.
func (u *User) SoftDelete(now time.Time) {
	if u.status == UserStatusDeleted {
		return
	}
	u.status = UserStatusDeleted
	u.bumpVersion(now)
}

// ApplyPatch applies the supplied changes. Rejects DELETED users (lifecycle
// rule); bumps etag/updated_at only when at least one field actually changes.
func (u *User) ApplyPatch(p UserPatch, now time.Time) error {
	if u.status == UserStatusDeleted {
		return ErrUserDeleted
	}
	changed := false
	if p.Email != nil && *p.Email != u.Email {
		u.Email = *p.Email
		changed = true
	}
	if p.Username != nil && *p.Username != u.Username {
		u.Username = *p.Username
		changed = true
	}
	if p.DisplayName != nil && *p.DisplayName != u.DisplayName {
		u.DisplayName = *p.DisplayName
		changed = true
	}
	if p.AvatarURL != nil && *p.AvatarURL != u.AvatarURL {
		u.AvatarURL = *p.AvatarURL
		changed = true
	}
	if p.Locale != nil && *p.Locale != u.Locale {
		u.Locale = *p.Locale
		changed = true
	}
	if p.Timezone != nil && *p.Timezone != u.Timezone {
		u.Timezone = *p.Timezone
		changed = true
	}
	if changed {
		u.bumpVersion(now)
	}
	return nil
}

// SetPassword swaps the stored bcrypt hash. The hashing itself happens
// in the auth use-case (domain has no crypto dependency); this method
// simply records the result and bumps version.
//
// Rejects DELETED users (same lifecycle rule as ApplyPatch) — a deleted
// account can't be reactivated by setting a password. Empty hash is
// rejected here too: clearing the password is a separate operation
// (ClearPassword) so callers don't accidentally erase credentials by
// passing nil.
func (u *User) SetPassword(hash []byte, now time.Time) error {
	if u.status == UserStatusDeleted {
		return ErrUserDeleted
	}
	if len(hash) == 0 {
		return ErrInvalidPasswordHash
	}
	u.passwordHash = hash
	u.bumpVersion(now)
	return nil
}

// ClearPassword removes the stored hash. The user is left in a state
// where Login fails with the generic INVALID_CREDENTIALS — same as a
// freshly admin-created account. Used by admin-driven flows that want
// to force a password reset.
//
// Idempotent on already-clear users (no etag bump, no error).
func (u *User) ClearPassword(now time.Time) error {
	if u.status == UserStatusDeleted {
		return ErrUserDeleted
	}
	if !u.HasPassword() {
		return nil
	}
	u.passwordHash = nil
	u.bumpVersion(now)
	return nil
}

func (u *User) bumpVersion(now time.Time) {
	u.updatedAt = now
	u.etag = etag.New()
}
