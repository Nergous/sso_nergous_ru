package domain

import (
	"fmt"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// ServiceAccountID — RFC 4122 UUIDv7. Doubles as the public identifier
// passed to AuthenticateServiceAccount (see proto: client_id slot was
// removed; the UUID stands in).
// ----------------------------------------------------------------------------

type ServiceAccountID string

func NewServiceAccountID() (ServiceAccountID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate service account id: %w", err)
	}
	return ServiceAccountID(id.String()), nil
}

func ParseServiceAccountID(s string) (ServiceAccountID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{
			Field:  "service_account_id",
			Reason: "must be a valid UUID",
		}
	}
	return ServiceAccountID(s), nil
}

func (s ServiceAccountID) String() string { return string(s) }

// ----------------------------------------------------------------------------
// ServiceAccount aggregate
// ----------------------------------------------------------------------------
//
// secretHash is a server-side-only field; never serialised to the wire.
// The plaintext secret leaves the system exactly twice — at create and
// at rotation — and the use-case layer is responsible for returning it
// to the caller. The aggregate only ever holds the hash.

type ServiceAccount struct {
	id         ServiceAccountID
	etag       etag.Etag
	status     ServiceAccountStatus
	createdAt  time.Time
	updatedAt  time.Time
	secretHash []byte

	Name                string
	Description         string
	LastAuthenticatedAt time.Time
}

type NewServiceAccountParams struct {
	ID          ServiceAccountID
	Name        string
	Description string
	SecretHash  []byte
	Now         time.Time
}

func NewServiceAccount(p NewServiceAccountParams) *ServiceAccount {
	return &ServiceAccount{
		id:          p.ID,
		etag:        etag.New(),
		status:      ServiceAccountActive,
		createdAt:   p.Now,
		updatedAt:   p.Now,
		secretHash:  p.SecretHash,
		Name:        p.Name,
		Description: p.Description,
	}
}

type RestoreServiceAccountParams struct {
	ID                  ServiceAccountID
	Etag                etag.Etag
	Status              ServiceAccountStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
	SecretHash          []byte
	Name                string
	Description         string
	LastAuthenticatedAt time.Time
}

func RestoreServiceAccount(p RestoreServiceAccountParams) *ServiceAccount {
	return &ServiceAccount{
		id:                  p.ID,
		etag:                p.Etag,
		status:              p.Status,
		createdAt:           p.CreatedAt,
		updatedAt:           p.UpdatedAt,
		secretHash:          p.SecretHash,
		Name:                p.Name,
		Description:         p.Description,
		LastAuthenticatedAt: p.LastAuthenticatedAt,
	}
}

func (s *ServiceAccount) ID() ServiceAccountID         { return s.id }
func (s *ServiceAccount) Etag() etag.Etag              { return s.etag }
func (s *ServiceAccount) Status() ServiceAccountStatus { return s.status }
func (s *ServiceAccount) CreatedAt() time.Time         { return s.createdAt }
func (s *ServiceAccount) UpdatedAt() time.Time         { return s.updatedAt }
func (s *ServiceAccount) SecretHash() []byte           { return s.secretHash }

type ServiceAccountPatch struct {
	Name        *string
	Description *string
}

func (p ServiceAccountPatch) IsEmpty() bool {
	return p.Name == nil && p.Description == nil
}

// Disable transitions ACTIVE → DISABLED. Idempotent on DISABLED.
func (s *ServiceAccount) Disable(now time.Time) {
	if s.status == ServiceAccountDisabled {
		return
	}
	s.status = ServiceAccountDisabled
	s.bumpVersion(now)
}

// Enable transitions DISABLED → ACTIVE. Idempotent on ACTIVE.
func (s *ServiceAccount) Enable(now time.Time) {
	if s.status == ServiceAccountActive {
		return
	}
	s.status = ServiceAccountActive
	s.bumpVersion(now)
}

func (s *ServiceAccount) ApplyPatch(p ServiceAccountPatch, now time.Time) {
	changed := false
	if p.Name != nil && *p.Name != s.Name {
		s.Name = *p.Name
		changed = true
	}
	if p.Description != nil && *p.Description != s.Description {
		s.Description = *p.Description
		changed = true
	}
	if changed {
		s.bumpVersion(now)
	}
}

// RotateSecret swaps the stored hash and bumps etag/updated_at. The
// caller (use-case layer) is responsible for hashing the freshly
// generated plaintext secret and persisting it.
func (s *ServiceAccount) RotateSecret(newHash []byte, now time.Time) {
	s.secretHash = newHash
	s.bumpVersion(now)
}

func (s *ServiceAccount) bumpVersion(now time.Time) {
	s.updatedAt = now
	s.etag = etag.New()
}
