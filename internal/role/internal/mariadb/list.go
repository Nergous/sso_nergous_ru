package mariadb

import (
	"context"
	"fmt"
	"strings"

	"sso/internal/kernel/dbutil"
	"sso/internal/role/internal/domain"
	"sso/internal/role/internal/mariadb/dbgen"
)

// listSelectCols mirrors the column list of GetRoleByID. Kept as a
// constant so the scan order in List stays in lockstep with dbgen.Role.
const listSelectCols = `id, app_id, name, description, status, etag, created_at, updated_at`

// List paginates roles within a single app. Hand-written rather than
// sqlc-generated because the WHERE / ORDER BY shape varies per request.
//
// Permissions live in a separate table; after the page is fetched, a
// single batched SELECT loads all permissions for the page's role_ids
// and groups them in Go. Avoids the N+1 of querying permissions per
// role at the cost of one extra round-trip per page.
func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.ListResult, error) {
	if q.PageSize <= 0 {
		return domain.ListResult{}, fmt.Errorf("role repo: list: page_size must be > 0")
	}
	if q.AppID == "" {
		return domain.ListResult{}, fmt.Errorf("role repo: list: app_id is required")
	}

	where, args := buildWhere(q)
	orderBy := orderClauseFor(q.OrderBy)

	// Fetch one extra row to detect "has next page".
	limit := q.PageSize + 1

	query := fmt.Sprintf(`SELECT %s FROM roles WHERE %s ORDER BY %s LIMIT %d`,
		listSelectCols, strings.Join(where, " AND "), orderBy, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("role repo: list: %w", err)
	}
	defer rows.Close()

	pageRows := make([]dbgen.Role, 0, q.PageSize)
	for rows.Next() {
		var role dbgen.Role
		if err := rows.Scan(
			&role.ID, &role.AppID, &role.Name, &role.Description,
			&role.Status, &role.Etag, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return domain.ListResult{}, fmt.Errorf("role repo: list: scan: %w", err)
		}
		pageRows = append(pageRows, role)
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("role repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(pageRows) > q.PageSize {
		// More data exists; trim the probe row and emit a cursor pointing
		// at the last row we actually return.
		pageRows = pageRows[:q.PageSize]
		last := pageRows[len(pageRows)-1]
		nextCursor = &domain.PageCursor{
			CreatedAt: last.CreatedAt,
			RoleID:    domain.RoleID(last.ID),
			Name:      last.Name,
		}
	}

	permsByRoleID, err := r.loadPermissionsForPage(ctx, pageRows)
	if err != nil {
		return domain.ListResult{}, err
	}

	out := make([]*domain.Role, 0, len(pageRows))
	for _, row := range pageRows {
		out = append(out, dbgenToDomain(row, permsByRoleID[row.ID]))
	}

	return domain.ListResult{Roles: out, NextCursor: nextCursor}, nil
}

// loadPermissionsForPage issues one SELECT to fetch all permissions for
// the supplied page of roles, grouping results by role_id in Go. Roles
// with no permissions get nil from the resulting map (a normal Go zero
// value lookup), which is fine — RestoreRole accepts nil.
//
// ORDER BY role_id, permission keeps each per-role slice sorted, which
// matches the canonical order maintained by domain (NewRole / ApplyPatch
// canonicalise via slices.Sort), so direct slice equality keeps working.
func (r *Repository) loadPermissionsForPage(
	ctx context.Context, pageRows []dbgen.Role,
) (map[string][]string, error) {
	if len(pageRows) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(pageRows))
	args := make([]any, len(pageRows))
	for i, row := range pageRows {
		placeholders[i] = "?"
		args[i] = row.ID
	}
	query := `SELECT role_id, permission FROM role_permissions WHERE role_id IN (` +
		strings.Join(placeholders, ",") +
		`) ORDER BY role_id, permission`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("role repo: list: load permissions: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]string, len(pageRows))
	for rows.Next() {
		var roleID, permission string
		if err := rows.Scan(&roleID, &permission); err != nil {
			return nil, fmt.Errorf("role repo: list: scan permission: %w", err)
		}
		out[roleID] = append(out[roleID], permission)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("role repo: list: permissions rows: %w", err)
	}
	return out, nil
}

// buildWhere assembles the AND-joined predicates and matching args.
// Always non-empty: app_id filter is mandatory.
func buildWhere(q domain.ListQuery) ([]string, []any) {
	var (
		where []string
		args  []any
	)

	// --- app scope (required) --------------------------------------------
	where = append(where, "app_id = ?")
	args = append(args, q.AppID.String())

	// --- status filter ---------------------------------------------------
	// Per proto: empty list means no filter (both ACTIVE and DISABLED
	// returned). Not "exclude DISABLED by default" — different from users.
	if len(q.Statuses) > 0 {
		ph := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			ph[i] = "?"
			args = append(args, uint8(s))
		}
		where = append(where, "status IN ("+strings.Join(ph, ",")+")")
	}

	// --- search ----------------------------------------------------------
	// Across name and description per proto.
	if q.Search != "" {
		pattern := "%" + dbutil.EscapeLike(q.Search) + "%"
		where = append(where, "(name LIKE ? OR description LIKE ?)")
		args = append(args, pattern, pattern)
	}

	// --- keyset cursor ---------------------------------------------------
	if q.After != nil {
		clause, cArgs := keysetClause(q.OrderBy, *q.After)
		where = append(where, clause)
		args = append(args, cArgs...)
	}

	return where, args
}

// keysetClause builds the "strictly after the cursor" predicate matching
// the active OrderBy. MariaDB supports tuple comparison: (a, b) < (c, d)
// is equivalent to a<c OR (a=c AND b<d).
func keysetClause(order domain.ListOrderBy, c domain.PageCursor) (string, []any) {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "(created_at, id) > (?, ?)", []any{c.CreatedAt, c.RoleID.String()}
	case domain.OrderByRoleIDDesc:
		return "id < ?", []any{c.RoleID.String()}
	case domain.OrderByRoleIDAsc:
		return "id > ?", []any{c.RoleID.String()}
	case domain.OrderByNameDesc:
		return "(name, id) < (?, ?)", []any{c.Name, c.RoleID.String()}
	case domain.OrderByNameAsc:
		return "(name, id) > (?, ?)", []any{c.Name, c.RoleID.String()}
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "(created_at, id) < (?, ?)", []any{c.CreatedAt, c.RoleID.String()}
	}
}

// orderClauseFor maps a domain OrderBy to a SQL ORDER BY clause. id is
// always the tie-breaker (proto contract).
func orderClauseFor(order domain.ListOrderBy) string {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "created_at ASC, id ASC"
	case domain.OrderByRoleIDDesc:
		return "id DESC"
	case domain.OrderByRoleIDAsc:
		return "id ASC"
	case domain.OrderByNameDesc:
		return "name DESC, id DESC"
	case domain.OrderByNameAsc:
		return "name ASC, id ASC"
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "created_at DESC, id DESC"
	}
}

