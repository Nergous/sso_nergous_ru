-- Identity directory: per-row queries. Dynamic ListUsers lives in the
-- repository's hand-written list.go (sqlc cannot template variable WHERE /
-- ORDER BY economically).

-- name: CreateUser :exec
INSERT INTO users
    (id, email, username, password_hash, display_name, avatar_url, locale, timezone,
     status, etag, created_at, updated_at, last_login_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = ?;

-- name: GetUserByEmail :one
-- Used by AuthService.Login when the caller authenticates by email.
-- Returns the full row including password_hash so the use-case can run
-- bcrypt verification without a follow-up Get.
SELECT * FROM users
WHERE email = ?;

-- name: GetUserByUsername :one
-- Used by AuthService.Login when the caller authenticates by username.
SELECT * FROM users
WHERE username = ?;

-- name: UpdateUserWithEtag :execresult
UPDATE users SET
    email = ?, username = ?, display_name = ?, password_hash = ?, avatar_url = ?,
    locale = ?, timezone = ?, status = ?, etag = ?,
    updated_at = ?, last_login_at = ?
WHERE id = ? AND etag = ?;

-- name: UpdateUser :execresult
UPDATE users SET
    email = ?, username = ?, display_name = ?, password_hash = ?, avatar_url = ?,
    locale = ?, timezone = ?, status = ?, etag = ?,
    updated_at = ?, last_login_at = ?
WHERE id = ?;

-- name: DeleteUserWithEtag :execresult
DELETE FROM users WHERE id = ? AND etag = ?;

-- name: DeleteUser :execresult
DELETE FROM users WHERE id = ?;

-- name: CountUserByID :one
-- Used by the repository to discriminate ErrUserNotFound vs
-- ErrEtagMismatch after a 0-rows-affected UPDATE/DELETE.
SELECT COUNT(*) FROM users WHERE id = ?;


-- name: UpdateUserPasswordWithEtag :execresult
UPDATE users SET
    password_hash = ?, etag = ?, updated_at = ?
WHERE id = ? AND etag = ?;

-- name: UpdateUserLastLoginAt :execresult
UPDATE users SET
    last_login_at = ?
WHERE id = ?;

-- name: IncrementFailedLogins :execresult
UPDATE users SET
    failed_login_attempts = failed_login_attempts + 1
WHERE id = ?;

-- name: LockUser :execresult
UPDATE users SET
    lockout_until = ?, failed_login_attempts = 0
WHERE id = ?;

-- name: ResetLoginFailures :execresult
UPDATE users SET
    failed_login_attempts = 0, lockout_until = NULL
WHERE id = ?;