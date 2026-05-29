-- Audit events queries.
--
-- The aggregate is append-only — no UPDATE / DELETE here. List is
-- hand-written (variable WHERE + keyset) in internal/persistence/mariadb/audit/list.go.

-- name: CreateAuditEvent :exec
INSERT INTO audit_events (
    id, occurred_at, event_type,
    actor_type, actor_id, actor_ip, user_agent,
    subject_type, subject_id, app_id,
    outcome, reason, metadata
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAuditEventByID :one
SELECT * FROM audit_events WHERE id = ?;
