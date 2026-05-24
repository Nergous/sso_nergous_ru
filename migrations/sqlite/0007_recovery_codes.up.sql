CREATE TABLE IF NOT EXISTS recovery_code_batches (
    id           TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    generated_at TEXT NOT NULL,
    revoked_at   TEXT     NULL,

    PRIMARY KEY (id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_recovery_batches_user_active ON recovery_code_batches (user_id, revoked_at);

CREATE TABLE IF NOT EXISTS recovery_codes (
    batch_id  TEXT NOT NULL,
    code_hash BLOB NOT NULL,
    used_at   TEXT     NULL,

    PRIMARY KEY (batch_id, code_hash),
    FOREIGN KEY (batch_id) REFERENCES recovery_code_batches(id) ON DELETE CASCADE
);
