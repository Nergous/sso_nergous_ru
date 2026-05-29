-- name: CreateServiceAccount :exec
INSERT INTO service_accounts
    (id, name, description, client_secret_hash, status, etag, created_at, updated_at, last_authenticated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetServiceAccountById :one
SELECT id, name, description, client_secret_hash, status, etag,
    created_at, updated_at, last_authenticated_at
FROM service_accounts
WHERE id = ?;

-- name: UpdateServiceAccountWithEtag :execresult
-- Single write-path covers admin updates (name/description/status) AND
-- credential rotation (client_secret_hash). The aggregate carries the
-- canonical post-mutation state; the row is rewritten in full.
UPDATE service_accounts SET
    name = ?, description = ?, client_secret_hash = ?, status = ?, etag = ?,
    updated_at = ?, last_authenticated_at = ?
WHERE id = ? AND etag = ?;

-- name: UpdateServiceAccount :execresult
UPDATE service_accounts SET
    name = ?, description = ?, client_secret_hash = ?, status = ?, etag = ?,
    updated_at = ?, last_authenticated_at = ?
WHERE id = ?;

-- name: DeleteServiceAccountWithEtag :execresult
DELETE FROM service_accounts WHERE id = ? AND etag = ?;

-- name: DeleteServiceAccount :execresult
DELETE FROM service_accounts WHERE id = ?;

-- name: CountServiceAccountByID :one
-- Used by the repository to discriminate ErrServiceAccountNotFound vs
-- ErrEtagMismatch after a 0-rows-affected UPDATE/DELETE.
SELECT COUNT(*) FROM service_accounts WHERE id = ?;
