package service

import (
	"context"
	"time"

	domain "sso/internal/modules/audit/internal/domain"
	"sso/internal/modules/audit/auditx"
	"sso/internal/kernel/cursor"
	"sso/internal/kernel/validation"
)

// ListAuditEventsInput is the use-case input for sso.audit.v1.AuditService.
// ListAuditEvents. The gRPC adapter is responsible for proto-to-this
// translation; this struct uses already-typed domain enums so the
// validation surface is uniform.
type ListAuditEventsInput struct {
	PageSize  int32
	PageToken string
	Filters   AuditFiltersInput
}

// AuditFiltersInput mirrors sso.audit.v1.AuditFilters but in
// domain-typed form. EventTypeSlugs are the wire-string event_type
// values — they are mapped to typed audit.EventType inside the service
// (unknown slug → INVALID_ARGUMENT).
type AuditFiltersInput struct {
	From           *time.Time
	To             *time.Time
	EventTypeSlugs []string
	ActorType      domain.ActorType // ActorTypeUnknown → no filter
	ActorID        string
	ActorIP        string
	SubjectType    domain.SubjectType // SubjectTypeUnknown → no filter
	SubjectID      string
	AppID          string
	Outcomes       []domain.AuditOutcome
}

type ListAuditEventsOutput struct {
	Audits        []*domain.Audit
	NextPageToken string
	TotalSize     *int
}

func (s *Service) ListAuditEvents(ctx context.Context, in ListAuditEventsInput) (ListAuditEventsOutput, error) {
	if err := s.requireAuditRead(ctx); err != nil {
		return ListAuditEventsOutput{}, err
	}

	pageSize, err := auditx.ClampPageSize(in.PageSize)
	if err != nil {
		return ListAuditEventsOutput{}, err
	}

	after, err := decodeCursor(in.PageToken)
	if err != nil {
		return ListAuditEventsOutput{}, err
	}

	filters, err := buildFilters(in.Filters)
	if err != nil {
		return ListAuditEventsOutput{}, err
	}

	res, err := s.repo.List(ctx, domain.ListQuery{
		PageSize: pageSize,
		After:    after,
		Filters:  filters,
	})
	if err != nil {
		return ListAuditEventsOutput{}, err
	}

	nextToken, err := encodeCursor(res.NextCursor)
	if err != nil {
		return ListAuditEventsOutput{}, err
	}
	return ListAuditEventsOutput{
		Audits:        res.Audits,
		NextPageToken: nextToken,
		TotalSize:     res.TotalSize,
	}, nil
}

// buildFilters maps the use-case DTO to a domain filter set. Validation
// is field-scoped: unknown event_type slug, invalid UUID, out-of-range
// enum — each surfaces as a validation.Error tagged with the offending
// `filters.<name>` path so the gRPC BadRequest detail is precise.
func buildFilters(in AuditFiltersInput) (domain.AuditFilters, error) {
	out := domain.AuditFilters{
		From:        in.From,
		To:          in.To,
		ActorType:   in.ActorType,
		ActorIP:     in.ActorIP,
		SubjectType: in.SubjectType,
	}

	if in.ActorType != domain.ActorTypeUnknown && !in.ActorType.IsKnown() {
		return domain.AuditFilters{}, &validation.Error{Field: "filters.actor_type", Reason: "unknown actor_type"}
	}
	if in.SubjectType != domain.SubjectTypeUnknown && !in.SubjectType.IsKnown() {
		return domain.AuditFilters{}, &validation.Error{Field: "filters.subject_type", Reason: "unknown subject_type"}
	}

	if len(in.EventTypeSlugs) > 0 {
		types := make([]domain.EventType, 0, len(in.EventTypeSlugs))
		for _, slug := range in.EventTypeSlugs {
			et := domain.ParseEventTypeSlug(slug)
			if et == domain.EventTypeUnknown {
				return domain.AuditFilters{}, &validation.Error{
					Field:  "filters.event_types",
					Reason: "unknown event_type: " + slug,
				}
			}
			types = append(types, et)
		}
		out.EventTypes = types
	}

	if in.ActorID != "" {
		aid, err := domain.ParseActorID(in.ActorID)
		if err != nil {
			return domain.AuditFilters{}, &validation.Error{Field: "filters.actor_id", Reason: "must be a valid UUID"}
		}
		out.ActorID = aid
	}
	if in.SubjectID != "" {
		sid, err := domain.ParseSubjectID(in.SubjectID)
		if err != nil {
			return domain.AuditFilters{}, &validation.Error{Field: "filters.subject_id", Reason: "must be a valid UUID"}
		}
		out.SubjectID = sid
	}
	if in.AppID != "" {
		aid, err := domain.ParseAppID(in.AppID)
		if err != nil {
			return domain.AuditFilters{}, &validation.Error{Field: "filters.app_id", Reason: "must be a valid UUID"}
		}
		out.AppID = aid
	}

	for i, o := range in.Outcomes {
		if !o.IsKnown() {
			return domain.AuditFilters{}, &validation.Error{
				Field:  "filters.outcomes",
				Reason: "unknown outcome at index " + itoa(i),
			}
		}
	}
	out.Outcomes = in.Outcomes

	return out, nil
}

// itoa is a tiny non-allocating int → decimal helper used only for
// error messages (small numbers, no zero-padding needed).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ----------------------------------------------------------------------------
// Page-cursor codec — JSON over base64url, same shape as identity / role.
// ----------------------------------------------------------------------------

type pageToken struct {
	OccurredAt time.Time `json:"t,omitempty"`
	AuditID    string    `json:"i,omitempty"`
}

func encodeCursor(c *domain.PageCursor) (string, error) {
	if c == nil {
		return "", nil
	}
	return cursor.Encode(&pageToken{
		OccurredAt: c.OccurredAt,
		AuditID:    c.AuditID.String(),
	})
}

func decodeCursor(s string) (*domain.PageCursor, error) {
	t, err := cursor.Decode[pageToken](s)
	if err != nil {
		return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
	}
	if t == nil {
		return nil, nil
	}
	pc := &domain.PageCursor{OccurredAt: t.OccurredAt}
	if t.AuditID != "" {
		id, err := domain.ParseAuditID(t.AuditID)
		if err != nil {
			return nil, &validation.Error{Field: "page_token", Reason: "malformed token"}
		}
		pc.AuditID = id
	}
	return pc, nil
}
