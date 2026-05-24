-- SQLite port of migrations/mariadb/0002_apps.up.sql. Same shape, different
-- type affinities — see migrations/sqlite/0001_users.up.sql for the
-- general porting rules.

CREATE TABLE IF NOT EXISTS apps (
    id          TEXT    NOT NULL PRIMARY KEY,
    name        TEXT    NOT NULL,
    slug        TEXT    NOT NULL,
    link        TEXT    NOT NULL,
    status      INTEGER NOT NULL,
    etag        TEXT    NOT NULL,
    created_at  TEXT    NOT NULL,
    updated_at  TEXT    NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_apps_name ON apps (name);
CREATE UNIQUE INDEX IF NOT EXISTS uk_apps_slug ON apps (slug);
CREATE INDEX        IF NOT EXISTS idx_apps_status_created ON apps (status, created_at, id);
