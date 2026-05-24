-- Append-only security event log.
--
-- No FKs to users / apps / sessions on purpose: an audit event must
-- survive cascade-delete of its actor / subject. We trade referential
-- integrity for tamper-evidence and retention safety.
--
-- Indexes mirror the filter set in sso.audit.v1.AuditFilters:
--   idx_audit_occurred       — keyset by (occurred_at, id), default scan
--   idx_audit_actor          — "events by actor X"
--   idx_audit_subject        — "events on (subject_type, subject_id)"
--   idx_audit_app            — "events scoped to app Y"
--   idx_audit_event_type     — "events of type Z"
--
-- event_type stays VARCHAR(128): the proto enum is intentionally open
-- (new event types added without a schema bump). Internal Go code uses
-- the typed audit.EventType; the column carries the slug form.
CREATE TABLE IF NOT EXISTS audit_events (
    id            CHAR(36)             NOT NULL,
    occurred_at   DATETIME(6)          NOT NULL,
    event_type    VARCHAR(128)         NOT NULL,
    actor_type    TINYINT UNSIGNED     NOT NULL,
    actor_id      CHAR(36)                 NULL,
    actor_ip      VARCHAR(45)              NULL,
    user_agent    VARCHAR(512)             NULL,
    subject_type  TINYINT UNSIGNED     NOT NULL,
    subject_id    CHAR(36)                 NULL,
    app_id        CHAR(36)                 NULL,
    outcome       TINYINT UNSIGNED     NOT NULL,
    reason        VARCHAR(128)             NULL,
    metadata      JSON                     NULL,

    PRIMARY KEY (id),
    KEY idx_audit_occurred   (occurred_at, id),
    KEY idx_audit_actor      (actor_id, occurred_at),
    KEY idx_audit_subject    (subject_type, subject_id, occurred_at),
    KEY idx_audit_app        (app_id, occurred_at),
    KEY idx_audit_event_type (event_type, occurred_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
