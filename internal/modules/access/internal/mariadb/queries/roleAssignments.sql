-- name: CreateRoleAssignment :exec
-- Idempotent insert: callers wrap with INSERT IGNORE-style discrimination
-- via dbutil.IsDuplicateEntry — the use-case wants "already existed" to
-- be observable so it can populate `newly_created=false` in BulkGrantRoles.
INSERT INTO role_assignments
    (user_id, role_id, app_id, granted_by_user_id, granted_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetRoleAssignment :one
SELECT user_id, role_id, app_id, granted_by_user_id, granted_at
FROM role_assignments
WHERE user_id = ? AND role_id = ?;

-- name: DeleteRoleAssignment :execresult
DELETE FROM role_assignments WHERE user_id = ? AND role_id = ?;

-- name: CountRoleAssignmentsByUserApp :one
SELECT COUNT(*) FROM role_assignments WHERE user_id = ? AND app_id = ?;

-- name: ListActivePermissionsByUserApp :many
-- Returns all permission strings carried by ACTIVE roles assigned to the
-- user in the target app. Drives CheckPermission and BatchCheckPermission;
-- DISABLED roles are filtered server-side at the JOIN. Each (role_id,
-- permission) row is returned once — duplicates across roles ARE possible
-- and the caller (use-case layer) does the wildcard expansion + dedup
-- against the requested permission set.
SELECT r.id AS role_id, rp.permission
FROM role_assignments ra
JOIN roles r           ON r.id = ra.role_id
JOIN role_permissions rp ON rp.role_id = r.id
WHERE ra.user_id = ?
  AND ra.app_id  = ?
  AND r.status   = ?;
