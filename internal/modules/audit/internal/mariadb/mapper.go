package mariadb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/netip"

	domain "sso/internal/modules/audit/internal/domain"
	"sso/internal/modules/audit/internal/mariadb/dbgen"
)

// dbgenToDomain reconstructs a domain.Audit from the row produced by
// sqlc. Validation of typed-ID forms happens once on Create — here we
// reconstruct without re-validating (audit is append-only and trusted).
func dbgenToDomain(r dbgen.AuditEvent) (*domain.Audit, error) {
	meta, err := metadataFromDB(r.Metadata)
	if err != nil {
		return nil, fmt.Errorf("audit mapper: metadata: %w", err)
	}

	ip, err := ipFromDB(r.ActorIp)
	if err != nil {
		return nil, fmt.Errorf("audit mapper: actor_ip: %w", err)
	}

	return domain.RestoreAudit(domain.RestoreAuditParams{
		ID:          domain.AuditID(r.ID),
		EventType:   domain.ParseEventTypeSlug(r.EventType),
		ActorType:   domain.ActorType(r.ActorType),
		ActorID:     domain.ActorID(stringFromNull(r.ActorID)),
		SubjectType: domain.SubjectType(r.SubjectType),
		SubjectID:   domain.SubjectID(stringFromNull(r.SubjectID)),
		AppID:       domain.AppID(stringFromNull(r.AppID)),
		Outcome:     domain.AuditOutcome(r.Outcome),
		Reason:      stringFromNull(r.Reason),
		IpAddress:  ip,
		UserAgent:  stringFromNull(r.UserAgent),
		Metadata:   meta,
		OccurredAt: r.OccurredAt,
	}), nil
}

// toCreateParams converts the domain aggregate into the sqlc-generated
// INSERT params. Metadata is JSON-encoded; empty maps become SQL NULL
// rather than an empty JSON object so reads round-trip to a nil map.
func toCreateParams(a *domain.Audit) (dbgen.CreateAuditEventParams, error) {
	meta, err := metadataToDB(a.Metadata())
	if err != nil {
		return dbgen.CreateAuditEventParams{}, fmt.Errorf("audit mapper: metadata: %w", err)
	}
	return dbgen.CreateAuditEventParams{
		ID:          a.ID().String(),
		OccurredAt:  a.OccurredAt(),
		EventType:   a.EventType().String(),
		ActorType:   uint8(a.ActorType()),
		ActorID:     nullableString(a.ActorID().String()),
		ActorIp:     nullableString(ipToDB(a.IPAddress())),
		UserAgent:   nullableString(a.UserAgent()),
		SubjectType: uint8(a.SubjectType()),
		SubjectID:   nullableString(a.SubjectID().String()),
		AppID:       nullableString(a.AppID().String()),
		Outcome:     uint8(a.Outcome()),
		Reason:      nullableString(a.Reason()),
		Metadata:    meta,
	}, nil
}

// ---------- nullable bridges ----------

func nullableString(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func stringFromNull(n sql.NullString) string {
	if !n.Valid {
		return ""
	}
	return n.String
}

func ipToDB(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}
	return addr.String()
}

func ipFromDB(n sql.NullString) (netip.Addr, error) {
	if !n.Valid || n.String == "" {
		return netip.Addr{}, nil
	}
	addr, err := netip.ParseAddr(n.String)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("parse: %w", err)
	}
	return addr, nil
}

func metadataToDB(m map[string]string) (json.RawMessage, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return json.Marshal(m)
}

func metadataFromDB(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]string)
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
