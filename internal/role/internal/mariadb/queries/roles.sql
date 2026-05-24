-- Roles directory: per-row queries plus permission-management helpers.
-- Dynamic ListRoles lives in the repository's hand-written list.go (sqlc
-- cannot template variable WHERE / ORDER BY economically).
--
-- Permissions live in a separate role_permissions table (one row per
-- (role_id, permission)). The role aggregate and its permission set are
-- two distinct write surfaces here; the repository wraps them in a single
-- transaction when atomicity matters — CreateRole (insert row + insert
-- N permissions) and UpdateRole with mask containing "permissions" (wipe
-- old set + insert new set).
--
-- Lifecycle (DisableRole / EnableRole) goes through UpdateRole at the
-- service layer: the domain mutator advances status + etag + updated_at,
-- and the repo persists with UpdateRole (unconditional path), keeping the
-- optimistic-locking invariant intact. There are intentionally no
-- dedicated Disable/Enable SQL statements — they would silently bypass
-- the etag bump and leave clients with a stale concurrency token.

-- name: CreateRole :exec
INSERT INTO roles
    (id, app_id, name, description, status, etag, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetRoleByID :one
SELECT id, app_id, name, description, status, etag, created_at, updated_at
FROM roles
WHERE id = ?;

-- name: UpdateRoleWithEtag :execresult
UPDATE roles SET
    name = ?, description = ?, status = ?, etag = ?, updated_at = ?
WHERE id = ? AND etag = ?;

-- name: UpdateRole :execresult
UPDATE roles SET
    name = ?, description = ?, status = ?, etag = ?, updated_at = ?
WHERE id = ?;

-- name: DeleteRoleWithEtag :execresult
DELETE FROM roles WHERE id = ? AND etag = ?;

-- name: DeleteRole :execresult
DELETE FROM roles WHERE id = ?;

-- name: CountRoleByID :one
-- Used by the repository to discriminate ErrRoleNotFound vs
-- ErrEtagMismatch after a 0-rows-affected UPDATE/DELETE.
SELECT COUNT(*) FROM roles WHERE id = ?;

-- ============================================================================
-- Permissions (role_permissions join table)
-- ============================================================================

-- name: GetRolePermissions :many
-- Returns a role's permissions in a deterministic order so callers can
-- build a stable string slice without an extra in-memory sort.
SELECT permission FROM role_permissions WHERE role_id = ? ORDER BY permission;

-- name: InsertRolePermission :exec
-- Inserts a single (role_id, permission) row. Callers (CreateRole and
-- UpdateRole with mask containing "permissions") loop over the role's
-- permission slice inside a transaction.
INSERT INTO role_permissions (role_id, permission) VALUES (?, ?);

-- name: DeleteRolePermissions :exec
-- Wipes all permissions for a role. Used by UpdateRole.permissions: the
-- repository wipes then re-inserts the new set inside a single tx.
DELETE FROM role_permissions WHERE role_id = ?;
