CREATE TABLE IF NOT EXISTS sessions (
    id                          CHAR(36)      NOT NULL,
    user_id                     CHAR(36)      NOT NULL,
    refresh_token_hash          VARBINARY(32) NOT NULL,
    user_agent                  VARCHAR(512)      NULL,
    ip_address                  VARCHAR(45)       NULL,
    issued_at                   DATETIME(6)   NOT NULL,
    expires_at                  DATETIME(6)   NOT NULL,
    refresh_token_expires_at    DATETIME(6)   NOT NULL,
    last_seen_at                DATETIME(6)   NOT NULL,
    revoked_at                  DATETIME(6)       NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_sessions_refresh_token_hash (refresh_token_hash),
    CONSTRAINT fk_sessions_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    KEY idx_sessions_user_issued (user_id, issued_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
