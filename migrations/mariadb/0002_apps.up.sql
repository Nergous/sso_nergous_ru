-- Apps directory: one row per registered SSO application.
--
-- id      UUIDv7 (k-sortable; cheap keyset pagination by id alone).
-- slug    URL-safe stable identifier; immutable after creation; unique.
-- etag    UUIDv4 regenerated on every mutation; optimistic-locking token.
-- status  1=ACTIVE, 2=DISABLED, 3=MAINTENANCE (mirrors AppStatus enum).
-- created_at, updated_at  microsecond precision (DATETIME(6)).

CREATE TABLE IF NOT EXISTS apps (
    id           CHAR(36)         NOT NULL,
    name         VARCHAR(128)     NOT NULL,
    slug         VARCHAR(64)      NOT NULL,
    link         VARCHAR(2048)    NOT NULL,
    status       TINYINT UNSIGNED NOT NULL,
    etag         CHAR(36)         NOT NULL,
    created_at   DATETIME(6)      NOT NULL,
    updated_at   DATETIME(6)      NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_apps_name (name),
    UNIQUE KEY uk_apps_slug (slug),
    KEY idx_apps_status_created (status, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
