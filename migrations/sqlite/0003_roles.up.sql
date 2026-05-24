CREATE TABLE IF NOT EXISTS roles (
    id          TEXT        NOT NULL PRIMARY KEY,
    app_id      TEXT        NOT NULL,
    name        TEXT        NOT NULL,
    description TEXT        NULL DEFAULT '',
    status      INTEGER     NOT NULL,
    etag        TEXT        NOT NULL,
    created_at  TEXT        NOT NULL,
    updated_at  TEXT        NOT NULL,
    
    FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_roles_app_id_name ON roles (app_id, name);

CREATE INDEX IF NOT EXISTS idx_roles_status_created ON roles (status, created_at, id);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id     TEXT    NOT NULL,
    permission  TEXT    NOT NULL,
    
    PRIMARY KEY (role_id, permission),
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_perm ON role_permissions (permission, role_id);