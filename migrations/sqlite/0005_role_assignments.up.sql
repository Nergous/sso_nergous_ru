CREATE TABLE IF NOT EXISTS role_assignments (
    user_id            TEXT        NOT NULL, 
    role_id            TEXT        NOT NULL, 
    app_id             TEXT        NOT NULL, 
    granted_by_user_id TEXT        NOT NULL, 
    granted_at         TEXT        NOT NULL, 
    
    PRIMARY KEY (user_id, role_id),
    
    FOREIGN KEY (role_id) REFERENCES roles(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Per-user listing within an app, ordered by granted_at (default)
-- role_id at the tail is the keyset tie-breaker
CREATE INDEX idx_role_assignments_user_app_granted 
ON role_assignments (user_id, app_id, granted_at, role_id);

-- Reverse lookup (e.g. for role-introspection / cascade reasoning)
CREATE INDEX idx_role_assignments_role 
ON role_assignments (role_id);