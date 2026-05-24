CREATE TABLE IF NOT EXISTS recovery_code_batches (
    id           CHAR(36)    NOT NULL,
    user_id      CHAR(36)    NOT NULL,
    generated_at DATETIME(6) NOT NULL,
    revoked_at   DATETIME(6)     NULL,

    PRIMARY KEY (id),
    CONSTRAINT fk_recovery_batches_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    KEY idx_recovery_batches_user_active (user_id, revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS recovery_codes (
    batch_id   CHAR(36)      NOT NULL,
    code_hash  VARBINARY(32) NOT NULL,
    used_at    DATETIME(6)       NULL,

    PRIMARY KEY (batch_id, code_hash),
    CONSTRAINT fk_recovery_codes_batch
        FOREIGN KEY (batch_id) REFERENCES recovery_code_batches(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
