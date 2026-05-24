-- Sessions directory

-- name: CreateSession :exec
INSERT INTO sessions (
    id, user_id, refresh_token_hash,
    user_agent, ip_address,
    issued_at, expires_at, refresh_token_expires_at,
    last_seen_at, revoked_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetSessionById :one
SELECT * FROM sessions WHERE id = ?;

-- name: GetSessionByRefreshHash :one
SELECT * FROM sessions WHERE refresh_token_hash = ?;

-- name: UpdateSession :execresult
UPDATE sessions SET
    user_id = ?, refresh_token_hash = ?,
    user_agent = ?, ip_address = ?,
    issued_at = ?, expires_at = ?, refresh_token_expires_at = ?,
    last_seen_at = ?, revoked_at = ?
WHERE id = ?;

-- name: RotateSessionRefresh :execresult
UPDATE sessions SET
    refresh_token_hash = ?, refresh_token_expires_at = ?, last_seen_at = ?
WHERE id = ? AND refresh_token_hash = ?;

-- name: ListSessionsByUser :many
SELECT * FROM sessions
WHERE user_id = ?
ORDER BY issued_at DESC, id DESC;

-- name: RevokeAllSessionsForUser :execresult
UPDATE sessions SET revoked_at = ?
WHERE user_id = ? AND revoked_at IS NULL;
