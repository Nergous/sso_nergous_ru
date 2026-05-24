-- SQLite port of migrations/mariadb/0001_users.up.sql.
--
-- Differences from MariaDB:
--   - SQLite has TEXT/INTEGER/REAL/BLOB type affinities; CHAR/VARCHAR/
--     DATETIME map to TEXT, TINYINT UNSIGNED to INTEGER. Lengths are
--     advisory and not enforced.
--   - No ENGINE/CHARSET/COLLATE clauses.
--   - Indexes (UNIQUE and otherwise) declared as separate CREATE INDEX
--     statements rather than inline KEY clauses.
--   - DATETIME(6) → TEXT. The Go driver (modernc.org/sqlite) serialises
--     time.Time as RFC 3339 with nanosecond precision; lex-comparable for
--     same-zone times, which keeps tuple-keyset pagination correct.

CREATE TABLE IF NOT EXISTS users (
    id            TEXT    NOT NULL PRIMARY KEY,
    email         TEXT    NOT NULL,
    username      TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    display_name  TEXT    NOT NULL,
    avatar_url    TEXT,
    locale        TEXT    NOT NULL DEFAULT '',
    timezone      TEXT    NOT NULL DEFAULT '',
    status        INTEGER NOT NULL,
    etag          TEXT    NOT NULL,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    last_login_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email    ON users (email);
CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username ON users (username);
CREATE INDEX        IF NOT EXISTS idx_users_status_created ON users (status, created_at, id);
