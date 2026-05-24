-- Identity directory: one row per user.
--
-- id            UUIDv7 (k-sortable; cheap keyset pagination by id alone).
-- etag          UUIDv4 regenerated on every mutation; optimistic-locking token.
-- status        1=ACTIVE, 2=BLOCKED, 3=DELETED (mirrors UserStatus enum).
-- avatar_url    NULL = absent (proto3 optional). NULL is distinct from "".
-- locale/timezone  empty string = "use system default".
-- created_at, updated_at, last_login_at  microsecond precision (DATETIME(6)).

CREATE TABLE IF NOT EXISTS users (
    id            CHAR(36)         NOT NULL,
    email         VARCHAR(254)     NOT NULL,
    username      VARCHAR(128)     NOT NULL,
    password_hash VARBINARY(255)       NULL,
    display_name  VARCHAR(128)     NOT NULL,
    avatar_url    VARCHAR(2048)        NULL,
    locale        VARCHAR(35)      NOT NULL DEFAULT '',
    timezone      VARCHAR(64)      NOT NULL DEFAULT '',
    status        TINYINT UNSIGNED NOT NULL,
    etag          CHAR(36)         NOT NULL,
    created_at    DATETIME(6)      NOT NULL,
    updated_at    DATETIME(6)      NOT NULL,
    last_login_at DATETIME(6)          NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_users_email    (email),
    UNIQUE KEY uk_users_username (username),
    KEY idx_users_status_created (status, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
