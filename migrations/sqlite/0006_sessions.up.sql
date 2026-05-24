CREATE TABLE IF NOT EXISTS sessions (
    id                          TEXT        NOT NULL,  
    user_id                     TEXT        NOT NULL,  
    refresh_token_hash          BLOB        NOT NULL,  
    user_agent                  TEXT        NULL,      
    ip_address                  TEXT        NULL,      
    issued_at                   TEXT        NOT NULL,  
    expires_at                  TEXT        NOT NULL,  
    refresh_token_expires_at    TEXT        NOT NULL,  
    last_seen_at                TEXT        NOT NULL,  
    revoked_at                  TEXT        NULL,

    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Unique constraint on refresh_token_hash
CREATE UNIQUE INDEX uk_sessions_refresh_token_hash ON sessions (refresh_token_hash);

-- Index for user_id + issued_at queries
CREATE INDEX idx_sessions_user_issued ON sessions (user_id, issued_at);

-- Additional useful indexes for session management
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_revoked_at ON sessions (revoked_at);