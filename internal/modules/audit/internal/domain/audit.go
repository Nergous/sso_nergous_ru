// Package audit defines the Audit aggregate (append-only security event)
// together with its supporting types, contracts (Repository, Emitter)
// and errors.
//
// Invariants enforced by NewAudit:
//   - id is RFC 4122 UUID v7; generated if the caller did not supply one.
//   - eventType is in the canonical EventType set (EventType.IsKnown()).
//   - actorType is not ActorTypeUnknown.
//   - actorID parses as a UUID when actorType ∈ {User, ServiceAccount};
//     actorID is empty for System / Anonymous actors.
//   - outcome is not AuditOutcomeUnknown.
//   - reason ≤ ReasonMaxLen; empty when outcome == Success.
//   - when subjectID is set: subjectType is not SubjectTypeUnknown
//     and subjectID parses as a UUID.
//   - when appID is set: it parses as a UUID.
//   - occurredAt is non-zero (defaults to time.Now().UTC()).
//   - ipAddress, when set, parses as a netip.Addr.
//   - userAgent is truncated to UserAgentMaxLen bytes.
//   - metadata has at most MetadataMaxEntries; keys ≤ MetadataKeyMaxLen,
//     values ≤ MetadataValueMaxLen.
//
// Audit instances are conceptually immutable: state is exposed through
// getters only and metadata is defensive-copied on construction and
// restoration so callers cannot mutate internal state through retained
// references.
package domain

import (
	"fmt"
	"maps"
	"net/netip"
	"time"

	"sso/internal/kernel/actor"
	"sso/internal/kernel/validation"

	"github.com/google/uuid"
)

const (
	MetadataMaxEntries  = 32
	MetadataKeyMaxLen   = 64
	MetadataValueMaxLen = 512

	UserAgentMaxLen = 512
	ReasonMaxLen    = 128
)

// ----------------------------------------------------------------------------
// AuditID — RFC 4122 UUID, generated as v7 (k-sortable).
// ----------------------------------------------------------------------------

type AuditID string

func NewAuditID() (AuditID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate audit id: %w", err)
	}
	return AuditID(id.String()), nil
}

func ParseAuditID(s string) (AuditID, error) {
	if _, err := uuid.Parse(s); err != nil {
		return "", &validation.Error{Field: "audit_id", Reason: "must be a valid UUID"}
	}
	return AuditID(s), nil
}

func (a AuditID) String() string { return string(a) }

// ----------------------------------------------------------------------------
// ActorType
// ----------------------------------------------------------------------------

// ActorType mirrors sso.audit.v1.ActorType:
//
//	USER             — human user (actor_id = user UUID).
//	SERVICE_ACCOUNT  — machine principal (actor_id = SA UUID).
//	SYSTEM           — server-internal (background job, migration);
//	                   actor_id is empty.
//	ANONYMOUS        — unauthenticated caller (failed Login,
//	                   ResetPasswordWithRecoveryCode); actor_id is empty,
//	                   actor_ip is the primary signal.
type ActorType uint8

const (
	ActorTypeUnknown   ActorType = 0
	ActorTypeUser      ActorType = 1
	ActorTypeService   ActorType = 2
	ActorTypeSystem    ActorType = 3
	ActorTypeAnonymous ActorType = 4
)

func (a ActorType) String() string {
	switch a {
	case ActorTypeUser:
		return "user"
	case ActorTypeService:
		return "service_account"
	case ActorTypeSystem:
		return "system"
	case ActorTypeAnonymous:
		return "anonymous"
	default:
		return "unknown"
	}
}

func (a ActorType) IsKnown() bool {
	switch a {
	case ActorTypeUser, ActorTypeService, ActorTypeSystem, ActorTypeAnonymous:
		return true
	default:
		return false
	}
}

// RequiresActorID reports whether an ActorID is mandatory for this actor
// kind. System and Anonymous actors have no UUID — actor_ip / event_type
// carry the identity instead.
func (a ActorType) RequiresActorID() bool {
	return a == ActorTypeUser || a == ActorTypeService
}

// ----------------------------------------------------------------------------
// AuditOutcome
//
//   - Success — the action completed.
//   - Failure — the action was attempted but did not complete due to
//     internal/operational reasons (validation error, downstream timeout,
//     wrong credentials, etc.).
//   - Denied  — the action was rejected by an authorisation policy
//     (RBAC, ownership check, rate-limit gate).
//
// Use Failure for "couldn't"; use Denied for "wasn't allowed to".
// ----------------------------------------------------------------------------

type AuditOutcome uint8

const (
	OutcomeUnknown AuditOutcome = 0
	OutcomeSuccess AuditOutcome = 1
	OutcomeFailure AuditOutcome = 2
	OutcomeDenied  AuditOutcome = 3
)

func (a AuditOutcome) String() string {
	switch a {
	case OutcomeSuccess:
		return "success"
	case OutcomeFailure:
		return "failure"
	case OutcomeDenied:
		return "denied"
	default:
		return "unknown"
	}
}

func (a AuditOutcome) IsKnown() bool {
	return a == OutcomeSuccess || a == OutcomeFailure || a == OutcomeDenied
}

// ----------------------------------------------------------------------------
// Audit aggregate
// ----------------------------------------------------------------------------

type Audit struct {
	id          AuditID
	eventType   EventType
	actorType   ActorType
	actorID     ActorID
	subjectType SubjectType
	subjectID   SubjectID
	appID       AppID
	outcome     AuditOutcome
	reason      string
	ipAddress   netip.Addr
	userAgent   string
	metadata    map[string]string
	occurredAt  time.Time
}

// NewAuditParams carries the raw inputs (mostly strings from transport)
// for constructing an Audit. NewAudit parses + validates and turns it
// into a typed, immutable aggregate.
type NewAuditParams struct {
	ID          AuditID // optional; auto-generated when empty
	EventType   EventType
	ActorType   ActorType
	ActorID     string // required for USER / SERVICE; empty for SYSTEM / ANONYMOUS
	SubjectType SubjectType
	SubjectID   string // optional; required when SubjectType is set
	AppID       string // optional
	Outcome     AuditOutcome
	Reason      string // optional; ErrorReason name when outcome != Success
	IpAddress   string // optional; parsed via netip.ParseAddr
	UserAgent   string // optional; truncated to UserAgentMaxLen
	Metadata    map[string]string
	Now         time.Time // optional; defaults to time.Now().UTC()
}

func NewAudit(p NewAuditParams) (*Audit, error) {
	if !p.EventType.IsKnown() {
		return nil, &validation.Error{Field: "event_type", Reason: "must be a known event_type"}
	}
	if !p.ActorType.IsKnown() {
		return nil, &validation.Error{Field: "actor_type", Reason: "must be a known actor_type"}
	}
	if !p.Outcome.IsKnown() {
		return nil, &validation.Error{Field: "outcome", Reason: "must be a known outcome"}
	}

	var actorID ActorID
	if p.ActorType.RequiresActorID() {
		var err error
		actorID, err = ParseActorID(p.ActorID)
		if err != nil {
			return nil, err
		}
	} else if p.ActorID != "" {
		return nil, &validation.Error{
			Field:  "actor_id",
			Reason: "must be empty for system / anonymous actors",
		}
	}

	var subjectID SubjectID
	if p.SubjectID != "" {
		if !p.SubjectType.IsKnown() {
			return nil, &validation.Error{Field: "subject_type", Reason: "must be set when subject_id is set"}
		}
		var err error
		subjectID, err = ParseSubjectID(p.SubjectID)
		if err != nil {
			return nil, err
		}
	}

	var appID AppID
	if p.AppID != "" {
		var err error
		appID, err = ParseAppID(p.AppID)
		if err != nil {
			return nil, err
		}
	}

	var ip netip.Addr
	if p.IpAddress != "" {
		addr, err := netip.ParseAddr(p.IpAddress)
		if err != nil {
			return nil, &validation.Error{Field: "ip_address", Reason: "must be a valid IP address"}
		}
		ip = addr
	}

	userAgent := p.UserAgent
	if len(userAgent) > UserAgentMaxLen {
		userAgent = userAgent[:UserAgentMaxLen]
	}

	reason := p.Reason
	if len(reason) > ReasonMaxLen {
		return nil, &validation.Error{
			Field:  "reason",
			Reason: fmt.Sprintf("must be at most %d bytes", ReasonMaxLen),
		}
	}
	if p.Outcome == OutcomeSuccess && reason != "" {
		return nil, &validation.Error{
			Field:  "reason",
			Reason: "must be empty when outcome is success",
		}
	}

	metadata, err := validateAndCloneMetadata(p.Metadata)
	if err != nil {
		return nil, err
	}

	id := p.ID
	if id == "" {
		nid, err := NewAuditID()
		if err != nil {
			return nil, fmt.Errorf("audit: %w", err)
		}
		id = nid
	}

	now := p.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return &Audit{
		id:          id,
		eventType:   p.EventType,
		actorType:   p.ActorType,
		actorID:     actorID,
		subjectType: p.SubjectType,
		subjectID:   subjectID,
		appID:       appID,
		outcome:     p.Outcome,
		reason:      reason,
		ipAddress:   ip,
		userAgent:   userAgent,
		metadata:    metadata,
		occurredAt:  now,
	}, nil
}

// RestoreAuditParams reconstructs an Audit from persistent storage.
// Fields are already typed — the repository parses strings (UUIDs, IPs)
// before calling RestoreAudit.
type RestoreAuditParams struct {
	ID          AuditID
	EventType   EventType
	ActorType   ActorType
	ActorID     ActorID
	SubjectType SubjectType
	SubjectID   SubjectID
	AppID       AppID
	Outcome     AuditOutcome
	Reason      string
	IpAddress   netip.Addr
	UserAgent   string
	Metadata    map[string]string
	OccurredAt  time.Time
}

func RestoreAudit(p RestoreAuditParams) *Audit {
	return &Audit{
		id:          p.ID,
		eventType:   p.EventType,
		actorType:   p.ActorType,
		actorID:     p.ActorID,
		subjectType: p.SubjectType,
		subjectID:   p.SubjectID,
		appID:       p.AppID,
		outcome:     p.Outcome,
		reason:      p.Reason,
		ipAddress:   p.IpAddress,
		userAgent:   p.UserAgent,
		metadata:    maps.Clone(p.Metadata),
		occurredAt:  p.OccurredAt,
	}
}

// MapActorKind bridges the kernel/actor Kind (wire-stable string) onto
// the audit ActorType enum.
func MapActorKind(k actor.Kind) ActorType {
	switch k {
	case actor.KindUser:
		return ActorTypeUser
	case actor.KindServiceAccount:
		return ActorTypeService
	default:
		return ActorTypeUnknown
	}
}

// BaseFromActor returns a NewAuditParams pre-filled with the
// actor-derived fields (EventType, ActorType, ActorID, IpAddress,
// UserAgent) shared by every authenticated mutating use-case. Callers
// set Subject* / AppID / Outcome / Reason inline before emitting.
//
// For anonymous flows (Login, Register, ResetPasswordWithRecoveryCode,
// AuthenticateServiceAccount) the use-case builds NewAuditParams
// directly — there is no actor to source from.
func BaseFromActor(a actor.Actor, evt EventType) NewAuditParams {
	return NewAuditParams{
		EventType: evt,
		ActorType: MapActorKind(a.Kind),
		ActorID:   a.ID,
		IpAddress: a.IpAddress,
		UserAgent: a.UserAgent,
	}
}

// ----------------------------------------------------------------------------
// Getters
// ----------------------------------------------------------------------------

func (a *Audit) ID() AuditID              { return a.id }
func (a *Audit) EventType() EventType     { return a.eventType }
func (a *Audit) ActorType() ActorType     { return a.actorType }
func (a *Audit) ActorID() ActorID         { return a.actorID }
func (a *Audit) SubjectType() SubjectType { return a.subjectType }
func (a *Audit) SubjectID() SubjectID     { return a.subjectID }
func (a *Audit) AppID() AppID             { return a.appID }
func (a *Audit) Outcome() AuditOutcome    { return a.outcome }
func (a *Audit) Reason() string           { return a.reason }
func (a *Audit) IPAddress() netip.Addr    { return a.ipAddress }
func (a *Audit) UserAgent() string        { return a.userAgent }
func (a *Audit) OccurredAt() time.Time    { return a.occurredAt }

// Metadata returns the internal metadata map. Callers must treat the
// returned map as read-only; mutating it is a contract violation.
func (a *Audit) Metadata() map[string]string { return a.metadata }

// ----------------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------------

func validateAndCloneMetadata(m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return nil, nil
	}
	if len(m) > MetadataMaxEntries {
		return nil, &validation.Error{
			Field:  "metadata",
			Reason: fmt.Sprintf("must have at most %d entries", MetadataMaxEntries),
		}
	}
	for k, v := range m {
		if len(k) > MetadataKeyMaxLen {
			return nil, &validation.Error{
				Field:  "metadata",
				Reason: fmt.Sprintf("key %q exceeds %d bytes", k, MetadataKeyMaxLen),
			}
		}
		if len(v) > MetadataValueMaxLen {
			return nil, &validation.Error{
				Field:  "metadata",
				Reason: fmt.Sprintf("value for %q exceeds %d bytes", k, MetadataValueMaxLen),
			}
		}
	}
	return maps.Clone(m), nil
}
