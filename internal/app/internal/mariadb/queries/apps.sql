-- Apps directory: per-row queries. Dynamic ListApps lives in the
-- repository's hand-written list.go.

-- name: CreateApp :exec
INSERT INTO apps
    (id, name, slug, link, status, etag, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAppByID :one
SELECT id, name, slug, link, status, etag, created_at, updated_at
FROM apps
WHERE id = ?;

-- name: UpdateAppWithEtag :execresult
UPDATE apps SET
    name = ?, link = ?, status = ?, etag = ?, updated_at = ?
WHERE id = ? AND etag = ?;

-- name: UpdateApp :execresult
UPDATE apps SET
    name = ?, link = ?, status = ?, etag = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteAppWithEtag :execresult
DELETE FROM apps WHERE id = ? AND etag = ?;

-- name: DeleteApp :execresult
DELETE FROM apps WHERE id = ?;

-- name: CountAppByID :one
-- Used by the repository to discriminate ErrAppNotFound vs
-- ErrEtagMismatch after a 0-rows-affected UPDATE/DELETE.
SELECT COUNT(*) FROM apps WHERE id = ?;
