package domain

import (
	"context"
	"time"
)

// PageCursor is a typed keyset cursor. The application layer is
// responsible for opaque-encoding it (base64 of JSON) before handing
// back to clients; the repository deals only with the typed form.
//
// AuditID is the tie-breaker — `(OccurredAt, AuditID)` strictly
// advances in `OccurredAt DESC, AuditID DESC` order (the only order
// supported per sso.audit.v1.AuditService contract).
type PageCursor struct {
	OccurredAt time.Time
	AuditID    AuditID
}

// AuditFilters mirrors sso.audit.v1.AuditFilters. Empty slices and
// nil/zero scalar fields mean "no filter on this dimension". Repeated
// fields match OR within the field; different fields are AND-combined.
//
// ActorIP is a string (free-form) rather than netip.Addr because the
// proto field is `string` (up to 45 bytes) — clients may pass either
// IPv4 or IPv6, normalised or not.
type AuditFilters struct {
	From        *time.Time
	To          *time.Time
	EventTypes  []EventType
	ActorType   ActorType // ActorTypeUnknown → no filter
	ActorID     ActorID   // empty → no filter
	ActorIP     string    // empty → no filter
	SubjectType SubjectType // SubjectTypeUnknown → no filter
	SubjectID   SubjectID
	AppID       AppID
	Outcomes    []AuditOutcome
}

// ListQuery is the input to Repository.List.
//
// PageSize == 0 means "server default"; the repository chooses a sane
// upper bound (see implementations). Caller may pre-validate.
type ListQuery struct {
	PageSize int
	After    *PageCursor
	Filters  AuditFilters
}

// ListResult is the output of Repository.List. NextCursor is nil on the
// last page; TotalSize is nil when the repository chose not to compute
// it (consistent with the optional total_size in the proto response —
// expensive to compute on an append-only log over long ranges).
type ListResult struct {
	Audits     []*Audit
	NextCursor *PageCursor
	TotalSize  *int
}

// Repository abstracts persistence of audit events. The aggregate is
// append-only — there is no Update / Delete contract.
//
// Error contract:
//
//	Create  → repository-internal errors only (audit is append-only)
//	GetByID → ErrAuditNotFound
//	List    → no domain errors; empty Audits slice on no match
type Repository interface {
	Create(ctx context.Context, a *Audit) error
	GetByID(ctx context.Context, id AuditID) (*Audit, error)
	List(ctx context.Context, q ListQuery) (ListResult, error)
}
