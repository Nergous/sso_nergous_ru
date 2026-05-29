-- Recovery-code batches and their codes.

-- name: CreateRecoveryCodeBatch :exec
INSERT INTO recovery_code_batches (
    id, user_id, generated_at, revoked_at
) VALUES (?, ?, ?, ?);

-- name: CreateRecoveryCode :exec
INSERT INTO recovery_codes (
    batch_id, code_hash, used_at
) VALUES (?, ?, ?);

-- name: GetActiveRecoveryBatchByUser :one
SELECT * FROM recovery_code_batches
WHERE user_id = ? AND revoked_at IS NULL
LIMIT 1;

-- name: ListRecoveryCodesByBatch :many
SELECT * FROM recovery_codes
WHERE batch_id = ?;

-- name: RevokeRecoveryBatch :execresult
UPDATE recovery_code_batches
SET revoked_at = ?
WHERE id = ? AND revoked_at IS NULL;

-- name: RevokeActiveRecoveryBatchesForUser :execresult
UPDATE recovery_code_batches
SET revoked_at = ?
WHERE user_id = ? AND revoked_at IS NULL;

-- name: ConsumeRecoveryCode :execresult
UPDATE recovery_codes
SET used_at = ?
WHERE batch_id = ? AND code_hash = ? AND used_at IS NULL;
