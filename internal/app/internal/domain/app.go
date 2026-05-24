package domain

import (
	"fmt"
	"sso/internal/kernel/etag"
	"sso/internal/kernel/validation"
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------------------------------------------
// AppID — RFC 4122 UUID, generated as v7 (k-sortable).
// ----------------------------------------------------------------------------

type AppID string

func NewAppID() (AppID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate app id: %w", err)
	}
	return AppID(id.String()), nil
}

func ParseAppID(s string) (AppID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "app_id", Reason: "must be a valid UUID"}
	}
	return AppID(s), nil
}

func (a AppID) String() string { return string(a) }

// ----------------------------------------------------------------------------
// App aggregate
// ----------------------------------------------------------------------------
//
// Field visibility split:
//
//   Unexported (only the aggregate itself can change them):
//     id, slug, status, etag, createdAt, updatedAt
//   * id and createdAt are immutable after construction.
//   * slug is immutable after Create per the proto contract.
//   * status is advanced only by lifecycle helpers
//     (Disable/Enable/EnterMaintenance/ExitMaintenance).
//   * etag and updatedAt are advanced exclusively by bumpVersion.
//
//   Exported (plain data; mutate freely or via ApplyPatch):
//     Name, Link

type App struct {
	id        AppID
	slug      string
	status    AppStatus
	etag      etag.Etag
	createdAt time.Time
	updatedAt time.Time

	Name string
	Link string
}

// NewAppParams carries the values supplied by the CreateApp use-case.
// Server-managed fields (etag/timestamps stamped here; status defaults to
// ACTIVE) are not part of it.
type NewAppParams struct {
	ID   AppID
	Name string
	Slug string
	Link string
	Now  time.Time
}

// NewApp constructs a fresh App. Status defaults to ACTIVE; created_at /
// updated_at stamped from Now; etag freshly minted.
func NewApp(p NewAppParams) *App {
	return &App{
		id:        p.ID,
		slug:      p.Slug,
		status:    AppStatusActive,
		etag:      etag.New(),
		createdAt: p.Now,
		updatedAt: p.Now,
		Name:      p.Name,
		Link:      p.Link,
	}
}

// RestoreAppParams carries the full row read back from the repository.
type RestoreAppParams struct {
	ID        AppID
	Name      string
	Slug      string
	Link      string
	Status    AppStatus
	Etag      etag.Etag
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RestoreApp rebuilds an App from a persisted row. No validation: the row
// is trusted (it was written by NewApp/ApplyPatch earlier).
func RestoreApp(p RestoreAppParams) *App {
	return &App{
		id:        p.ID,
		slug:      p.Slug,
		status:    p.Status,
		etag:      p.Etag,
		createdAt: p.CreatedAt,
		updatedAt: p.UpdatedAt,
		Name:      p.Name,
		Link:      p.Link,
	}
}

// Read-only accessors for the unexported fields.
func (a *App) ID() AppID            { return a.id }
func (a *App) Slug() string         { return a.slug }
func (a *App) Status() AppStatus    { return a.status }
func (a *App) Etag() etag.Etag      { return a.etag }
func (a *App) CreatedAt() time.Time { return a.createdAt }
func (a *App) UpdatedAt() time.Time { return a.updatedAt }

// ----------------------------------------------------------------------------
// AppPatch — set of changes for ApplyPatch. nil pointer = "field not in
// the update mask"; non-nil pointer = "set to this value".
//
// slug, status, app_id, etag, timestamps are intentionally absent — they
// are not patch-able through this surface (proto contract).
// ----------------------------------------------------------------------------

type AppPatch struct {
	Name *string
	Link *string
}

func (p AppPatch) IsEmpty() bool {
	return p.Name == nil && p.Link == nil
}

// ----------------------------------------------------------------------------
// Mutators — lifecycle transitions
// ----------------------------------------------------------------------------
//
// State machine (based on proto descriptions and AppStatus enum):
//
//   Disable           any non-DISABLED  → DISABLED        (idempotent)
//   Enable            DISABLED          → ACTIVE          (idempotent on ACTIVE; rejects MAINTENANCE)
//   EnterMaintenance  ACTIVE            → MAINTENANCE     (idempotent on MAINTENANCE; rejects DISABLED)
//   ExitMaintenance   MAINTENANCE       → ACTIVE          (idempotent on ACTIVE; rejects DISABLED)
//
// Rejecting the wrong starting state surfaces ErrAppDisabled or
// ErrAppInMaintenance as appropriate; the gRPC layer maps these to
// FAILED_PRECONDITION.

// Disable transitions any state → DISABLED. Idempotent on DISABLED.
func (a *App) Disable(now time.Time) {
	if a.status == AppStatusDisabled {
		return
	}
	a.status = AppStatusDisabled
	a.bumpVersion(now)
}

// Enable transitions DISABLED → ACTIVE. Idempotent on ACTIVE. Rejects
// MAINTENANCE — caller must use ExitMaintenance.
func (a *App) Enable(now time.Time) error {
	if a.status == AppStatusMaintenance {
		return ErrAppInMaintenance
	}
	if a.status == AppStatusActive {
		return nil
	}
	a.status = AppStatusActive
	a.bumpVersion(now)
	return nil
}

// EnterMaintenance transitions ACTIVE → MAINTENANCE. Idempotent on
// MAINTENANCE. Rejects DISABLED.
func (a *App) EnterMaintenance(now time.Time) error {
	if a.status == AppStatusDisabled {
		return ErrAppDisabled
	}
	if a.status == AppStatusMaintenance {
		return nil
	}
	a.status = AppStatusMaintenance
	a.bumpVersion(now)
	return nil
}

// ExitMaintenance transitions MAINTENANCE → ACTIVE. Idempotent on ACTIVE.
// Rejects DISABLED.
func (a *App) ExitMaintenance(now time.Time) error {
	if a.status == AppStatusDisabled {
		return ErrAppDisabled
	}
	if a.status == AppStatusActive {
		return nil
	}
	a.status = AppStatusActive
	a.bumpVersion(now)
	return nil
}

// ApplyPatch applies the supplied changes. Bumps etag/updated_at only when
// at least one field actually changes.
func (a *App) ApplyPatch(p AppPatch, now time.Time) {
	changed := false
	if p.Name != nil && *p.Name != a.Name {
		a.Name = *p.Name
		changed = true
	}
	if p.Link != nil && *p.Link != a.Link {
		a.Link = *p.Link
		changed = true
	}
	if changed {
		a.bumpVersion(now)
	}
}

func (a *App) bumpVersion(now time.Time) {
	a.updatedAt = now
	a.etag = etag.New()
}
