CREATE TABLE IF NOT EXISTS role_assignments (
    user_id            CHAR(36)         NOT NULL,
    role_id            CHAR(36)         NOT NULL,
    app_id             CHAR(36)         NOT NULL,
    granted_by_user_id CHAR(36)         NOT NULL,
    granted_at         DATETIME(6)      NOT NULL,

    PRIMARY KEY (user_id, role_id),

    -- Per-user listing within an app, ordered by granted_at (default).
    -- role_id at the tail is the keyset tie-breaker.
    KEY idx_role_assignments_user_app_granted (user_id, app_id, granted_at, role_id),

    -- Reverse lookup (e.g. for role-introspection / cascade reasoning).
    KEY idx_role_assignments_role (role_id),

    -- Cascade on PermanentlyDeleteRole: assignments referencing the role
    -- vanish atomically with the role row. Matches the proto contract
    -- (sso.roles.v1.PermanentlyDeleteRole: "Cascade is implicit").
    CONSTRAINT fk_role_assignments_role
        FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,

    -- Cascade on PermanentlyDeleteUser: when a user is hard-deleted from
    -- identity, their assignments go with them. SoftDelete (BLOCKED /
    -- DELETED status) leaves rows in place — assignments are preserved
    -- across re-enable, only filtered out at CheckPermission time.
    CONSTRAINT fk_role_assignments_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
