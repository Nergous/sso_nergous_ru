// Package validation defines the canonical "field-level rejection"
// primitive raised by domain value-object parsers and consumed by the
// gRPC layer when assembling google.rpc.BadRequest details.
//
// Sits below both domain and gRPC layers so neither needs to depend on
// the other to talk about validation.
package validation

// Error is a single field-level rejection. Field is the JSON-style path
// (e.g. "email", "user.username", "filters.statuses"); Reason is a
// human-readable description suitable for the BadRequest detail.
type Error struct {
	Field  string
	Reason string
}

func (e Error) Error() string {
	if e.Field == "" {
		return "validation failed: " + e.Reason
	}
	return e.Field + ": " + e.Reason
}
