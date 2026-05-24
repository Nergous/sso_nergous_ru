// Package etag provides the canonical optimistic-locking token shared
// across all bounded contexts.
//
// An Etag is a UUIDv4 string regenerated on every aggregate mutation.
// Treated as opaque on the wire: clients echo it back unchanged in
// conditional Update / Delete requests.
//
// Empty string is the in-memory wildcard sentinel (means "unconditional"
// at the repository layer); it is never written to storage.
package etag

import (
	"github.com/google/uuid"

	"sso/internal/kernel/validation"
)

// Etag is the cross-domain optimistic-locking token. Domain packages
// re-expose it via `type Etag = etag.Etag` so callsites can keep reading
// `identity.Etag`, `app.Etag`, etc. while the underlying type is shared.
type Etag string

// New mints a fresh etag. Used by aggregates on construction (NewUser /
// NewApp / ...) and on every mutation (bumpVersion).
func New() Etag {
	return Etag(uuid.NewString())
}

// Parse validates that s is a UUID and returns the typed wrapper.
// Surfaces a validation.Error tagged "etag" on bad input so the gRPC
// layer can render a BadRequest detail without further translation.
func Parse(s string) (Etag, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "etag", Reason: "must be a valid UUID"}
	}
	return Etag(s), nil
}

func (e Etag) String() string { return string(e) }
