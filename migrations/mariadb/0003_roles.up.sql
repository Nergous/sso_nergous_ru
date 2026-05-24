CREATE TABLE IF NOT EXISTS roles (
    id          CHAR(36)            NOT NULL,
    app_id      CHAR(36)            NOT NULL,
    name        VARCHAR(128)        NOT NULL,
    description VARCHAR(1024)       NULL DEFAULT '',
    status      TINYINT UNSIGNED    NOT NULL,
    etag        CHAR(36)            NOT NULL,
    created_at  DATETIME(6)         NOT NULL,
    updated_at  DATETIME(6)         NOT NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_roles_app_id_name (app_id, name),
    KEY idx_roles_status_created (status, created_at, id),
    CONSTRAINT fk_roles_app FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id    CHAR(36)    NOT NULL,
    permission VARCHAR(64) NOT NULL,

    PRIMARY KEY (role_id, permission),
    KEY idx_role_permissions_perm (permission, role_id),
    CONSTRAINT fk_role_permissions_role
        FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
