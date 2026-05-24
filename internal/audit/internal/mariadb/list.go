package mariadb

import (
	"context"
	"fmt"
	"strings"

	domain "sso/internal/audit/internal/domain"
	"sso/internal/audit/internal/mariadb/dbgen"
)

// listSelectCols mirrors the column list of GetAuditEventByID. Kept as
// a constant so the scan order in List stays in lockstep with
// dbgen.AuditEvent.
const listSelectCols = `id, occurred_at, event_type, actor_type, actor_id, actor_ip, ` +
	`user_agent, subject_type, subject_id, app_id, ` +
	`outcome, reason, metadata`

// defaultPageSize is the cap applied when ListQuery.PageSize == 0
// ("server default" per proto). The hard upper bound (1000) is enforced
// by the gRPC validator on the wire — the repo just clamps the default.
const defaultPageSize = 50

// List paginates the audit log. Ordering is fixed at
// `occurred_at DESC, id DESC` (proto contract — no user-selectable
// order_by); the only knob is the AuditFilters set.
//
// Hand-written rather than sqlc-generated because the WHERE clause
// varies per request and the keyset cursor encodes a tuple comparison.
func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.ListResult, error) {
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	where, args := buildWhere(q)

	// Fetch one extra row to detect "has next page".
	limit := pageSize + 1

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(listSelectCols)
	sb.WriteString(" FROM audit_events")
	if len(where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
	}
	sb.WriteString(" ORDER BY occurred_at DESC, id DESC LIMIT ")
	fmt.Fprintf(&sb, "%d", limit)

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("audit repo: list: %w", err)
	}
	defer rows.Close()

	pageRows := make([]dbgen.AuditEvent, 0, pageSize)
	for rows.Next() {
		var ev dbgen.AuditEvent
		if err := rows.Scan(
			&ev.ID, &ev.OccurredAt, &ev.EventType,
			&ev.ActorType, &ev.ActorID, &ev.ActorIp,
			&ev.UserAgent,
			&ev.SubjectType, &ev.SubjectID, &ev.AppID,
			&ev.Outcome, &ev.Reason, &ev.Metadata,
		); err != nil {
			return domain.ListResult{}, fmt.Errorf("audit repo: list: scan: %w", err)
		}
		pageRows = append(pageRows, ev)
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("audit repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(pageRows) > pageSize {
		pageRows = pageRows[:pageSize]
		last := pageRows[len(pageRows)-1]
		nextCursor = &domain.PageCursor{
			OccurredAt: last.OccurredAt,
			AuditID:    domain.AuditID(last.ID),
		}
	}

	out := make([]*domain.Audit, 0, len(pageRows))
	for _, ev := range pageRows {
		a, err := dbgenToDomain(ev)
		if err != nil {
			return domain.ListResult{}, fmt.Errorf("audit repo: list: hydrate: %w", err)
		}
		out = append(out, a)
	}

	// TotalSize: nil — counting over the audit log can be expensive over
	// long ranges and the proto field is optional. Callers that need it
	// can issue a separate COUNT(*) with the same filters.
	return domain.ListResult{Audits: out, NextCursor: nextCursor}, nil
}

// buildWhere assembles AND-joined predicates and matching args from the
// filter set. The result may be empty (no filters at all).
func buildWhere(q domain.ListQuery) ([]string, []any) {
	var (
		where []string
		args  []any
	)

	f := q.Filters

	if f.From != nil {
		where = append(where, "occurred_at >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		where = append(where, "occurred_at < ?")
		args = append(args, *f.To)
	}

	if len(f.EventTypes) > 0 {
		ph := make([]string, len(f.EventTypes))
		for i, et := range f.EventTypes {
			ph[i] = "?"
			args = append(args, et.String())
		}
		where = append(where, "event_type IN ("+strings.Join(ph, ",")+")")
	}

	if f.ActorType != domain.ActorTypeUnknown {
		where = append(where, "actor_type = ?")
		args = append(args, uint8(f.ActorType))
	}
	if f.ActorID != "" {
		where = append(where, "actor_id = ?")
		args = append(args, f.ActorID.String())
	}
	if f.ActorIP != "" {
		where = append(where, "actor_ip = ?")
		args = append(args, f.ActorIP)
	}

	if f.SubjectType != domain.SubjectTypeUnknown {
		where = append(where, "subject_type = ?")
		args = append(args, uint8(f.SubjectType))
	}
	if f.SubjectID != "" {
		where = append(where, "subject_id = ?")
		args = append(args, f.SubjectID.String())
	}

	if f.AppID != "" {
		where = append(where, "app_id = ?")
		args = append(args, f.AppID.String())
	}

	if len(f.Outcomes) > 0 {
		ph := make([]string, len(f.Outcomes))
		for i, o := range f.Outcomes {
			ph[i] = "?"
			args = append(args, uint8(o))
		}
		where = append(where, "outcome IN ("+strings.Join(ph, ",")+")")
	}

	if q.After != nil {
		// Keyset: strictly after the cursor under the (occurred_at DESC,
		// id DESC) ordering. MariaDB supports tuple comparison.
		where = append(where, "(occurred_at, id) < (?, ?)")
		args = append(args, q.After.OccurredAt, q.After.AuditID.String())
	}

	return where, args
}
