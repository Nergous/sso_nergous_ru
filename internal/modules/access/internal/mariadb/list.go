package mariadb

import (
	"context"
	"fmt"
	"sso/internal/modules/access/internal/domain"
	"strings"
)

// ListUserRoles is hand-written: variable ORDER BY (granted_at vs role_id)
// and an optional keyset cursor make sqlc's static SELECT awkward. The
// query returns only role_id + granted_at; the use-case loads the full
// Role aggregates from role.Repository afterwards.
func (r *Repository) ListUserRoles(ctx context.Context, q domain.ListUserRolesQuery) (domain.ListUserRolesResult, error) {
	if q.PageSize <= 0 {
		return domain.ListUserRolesResult{}, fmt.Errorf("access repo: list: page_size must be > 0")
	}
	if q.UserID == "" || q.AppID == "" {
		return domain.ListUserRolesResult{}, fmt.Errorf("access repo: list: user_id and app_id are required")
	}

	where := []string{"user_id = ?", "app_id = ?"}
	args := []any{q.UserID.String(), q.AppID.String()}

	if q.After != nil {
		clause, cArgs := keysetClause(q.OrderBy, *q.After)
		where = append(where, clause)
		args = append(args, cArgs...)
	}

	limit := q.PageSize + 1
	query := fmt.Sprintf(
		`SELECT role_id, granted_at FROM role_assignments WHERE %s ORDER BY %s LIMIT %d`,
		strings.Join(where, " AND "), orderClauseFor(q.OrderBy), limit,
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ListUserRolesResult{}, fmt.Errorf("access repo: list: %w", err)
	}
	defer rows.Close()

	out := make([]domain.ListUserRolesRow, 0, q.PageSize)
	for rows.Next() {
		var (
			roleID string
			row    domain.ListUserRolesRow
		)
		if err := rows.Scan(&roleID, &row.GrantedAt); err != nil {
			return domain.ListUserRolesResult{}, fmt.Errorf("access repo: list: scan: %w", err)
		}
		row.RoleID = domain.RoleID(roleID)
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return domain.ListUserRolesResult{}, fmt.Errorf("access repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(out) > q.PageSize {
		out = out[:q.PageSize]
		last := out[len(out)-1]
		nextCursor = &domain.PageCursor{GrantedAt: last.GrantedAt, RoleID: last.RoleID}
	}
	return domain.ListUserRolesResult{Rows: out, NextCursor: nextCursor}, nil
}

func keysetClause(order domain.ListOrderBy, c domain.PageCursor) (string, []any) {
	switch order {
	case domain.OrderByGrantedAtAsc:
		return "(granted_at, role_id) > (?, ?)", []any{c.GrantedAt, c.RoleID.String()}
	case domain.OrderByRoleIDDesc:
		return "role_id < ?", []any{c.RoleID.String()}
	case domain.OrderByRoleIDAsc:
		return "role_id > ?", []any{c.RoleID.String()}
	case domain.OrderByGrantedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "(granted_at, role_id) < (?, ?)", []any{c.GrantedAt, c.RoleID.String()}
	}
}

func orderClauseFor(order domain.ListOrderBy) string {
	switch order {
	case domain.OrderByGrantedAtAsc:
		return "granted_at ASC, role_id ASC"
	case domain.OrderByRoleIDDesc:
		return "role_id DESC"
	case domain.OrderByRoleIDAsc:
		return "role_id ASC"
	case domain.OrderByGrantedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "granted_at DESC, role_id DESC"
	}
}
