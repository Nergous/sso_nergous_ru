package mariadb

import (
	"database/sql"

	"sso/internal/kernel/etag"
	"sso/internal/modules/role/internal/domain"
	"sso/internal/modules/role/internal/mariadb/dbgen"
)

// dbgenToDomain hydrates a domain.Role from a freshly-scanned sqlc row
// plus the permission strings fetched separately from role_permissions.
//
// Permissions are stored in their own table, so the repository fetches
// them with a follow-up query (GetRolePermissions, ORDER BY permission)
// and passes them in here. RestoreRole accepts the already-sorted slice
// without re-sorting.
func dbgenToDomain(r dbgen.Role, perms []string) *domain.Role {
	desc := ""
	if r.Description.Valid {
		desc = r.Description.String
	}
	return domain.RestoreRole(domain.RestoreRoleParams{
		ID:          domain.RoleID(r.ID),
		AppID:       domain.AppID(r.AppID),
		Name:        r.Name,
		Description: desc,
		Permissions: perms,
		Status:      domain.RoleStatus(r.Status),
		Etag:        etag.Etag(r.Etag),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	})
}

// toCreateParams flattens a domain.Role into the sqlc CreateRole arg
// shape. Permissions are not part of the row INSERT — the repository
// loops over r.Permissions() and calls InsertRolePermission per entry
// inside the same transaction.
func toCreateParams(r *domain.Role) dbgen.CreateRoleParams {
	return dbgen.CreateRoleParams{
		ID:          r.ID().String(),
		AppID:       r.AppID().String(),
		Name:        r.Name,
		Description: descriptionToDB(r.Description),
		Status:      uint8(r.Status()),
		Etag:        r.Etag().String(),
		CreatedAt:   r.CreatedAt(),
		UpdatedAt:   r.UpdatedAt(),
	}
}

// toUpdateParams — unconditional update (no etag in WHERE). Permissions
// are not part of the row UPDATE; the repository handles them by
// DeleteRolePermissions + N×InsertRolePermission inside the same tx.
func toUpdateParams(r *domain.Role) dbgen.UpdateRoleParams {
	return dbgen.UpdateRoleParams{
		Name:        r.Name,
		Description: descriptionToDB(r.Description),
		Status:      uint8(r.Status()),
		Etag:        r.Etag().String(),
		UpdatedAt:   r.UpdatedAt(),
		ID:          r.ID().String(),
	}
}

// toUpdateWithEtagParams — conditional update. Etag_2 is sqlc's
// positional name for the second occurrence of `etag = ?` (in WHERE).
func toUpdateWithEtagParams(r *domain.Role, expectedEtag etag.Etag) dbgen.UpdateRoleWithEtagParams {
	return dbgen.UpdateRoleWithEtagParams{
		Name:        r.Name,
		Description: descriptionToDB(r.Description),
		Status:      uint8(r.Status()),
		Etag:        r.Etag().String(),
		UpdatedAt:   r.UpdatedAt(),
		ID:          r.ID().String(),
		Etag_2:      expectedEtag.String(),
	}
}

// toInsertPermissionParams flattens a single (role_id, permission) pair
// into the sqlc InsertRolePermission arg shape. Callers (CreateRole and
// UpdateRole with mask containing "permissions") loop over the role's
// permission slice and call this once per entry inside a transaction.
func toInsertPermissionParams(roleID domain.RoleID, permission string) dbgen.InsertRolePermissionParams {
	return dbgen.InsertRolePermissionParams{
		RoleID:     roleID.String(),
		Permission: permission,
	}
}

// descriptionToDB maps the empty domain value to SQL NULL. The proto
// contract says "Empty if not set", so an empty string is the canonical
// "absent" value at the wire — and we mirror that to NULL on disk.
func descriptionToDB(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
