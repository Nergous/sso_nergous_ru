package mariadb

import (
	"context"
	"fmt"
	"strings"

	domain "sso/internal/modules/serviceaccount/internal/domain"
	"sso/internal/modules/serviceaccount/internal/mariadb/dbgen"
)

const listSelectCols = `id, name, description, client_secret_hash, status, etag, created_at, updated_at, last_authenticated_at`

func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.ListResult, error) {
	if q.PageSize <= 0 {
		return domain.ListResult{}, fmt.Errorf("service_account repo: list: page_size must be > 0")
	}

	where, args := buildWhere(q)
	orderBy := orderClauseFor(q.OrderBy)

	limit := q.PageSize + 1

	var query string
	if len(where) == 0 {
		query = fmt.Sprintf(`SELECT %s FROM service_accounts ORDER BY %s LIMIT %d`,
			listSelectCols, orderBy, limit)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM service_accounts WHERE %s ORDER BY %s LIMIT %d`,
			listSelectCols, strings.Join(where, " AND "), orderBy, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("service_account repo: list: %w", err)
	}
	defer rows.Close()

	pageRows := make([]dbgen.ServiceAccount, 0, q.PageSize)
	for rows.Next() {
		var sa dbgen.ServiceAccount
		if err := rows.Scan(
			&sa.ID, &sa.Name, &sa.Description, &sa.ClientSecretHash,
			&sa.Status, &sa.Etag, &sa.CreatedAt, &sa.UpdatedAt, &sa.LastAuthenticatedAt,
		); err != nil {
			return domain.ListResult{}, fmt.Errorf("service_account repo: list: scan: %w", err)
		}
		pageRows = append(pageRows, sa)
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("service_account repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(pageRows) > q.PageSize {
		pageRows = pageRows[:q.PageSize]
		last := pageRows[len(pageRows)-1]
		nextCursor = &domain.PageCursor{
			CreatedAt:        last.CreatedAt,
			ServiceAccountID: domain.ServiceAccountID(last.ID),
			Name:             last.Name,
		}
	}

	out := make([]*domain.ServiceAccount, 0, len(pageRows))
	for _, row := range pageRows {
		out = append(out, dbgenToDomain(row))
	}
	return domain.ListResult{ServiceAccounts: out, NextCursor: nextCursor}, nil
}

func buildWhere(q domain.ListQuery) ([]string, []any) {
	var (
		where []string
		args  []any
	)

	if len(q.Statuses) > 0 {
		ph := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			ph[i] = "?"
			args = append(args, uint8(s))
		}
		where = append(where, "status IN ("+strings.Join(ph, ",")+")")
	}

	if q.Search != "" {
		pattern := "%" + escapeLike(q.Search) + "%"
		where = append(where, "(name LIKE ? OR description LIKE ?)")
		args = append(args, pattern, pattern)
	}

	if q.After != nil {
		clause, cArgs := keysetClause(q.OrderBy, *q.After)
		where = append(where, clause)
		args = append(args, cArgs...)
	}

	return where, args
}

func keysetClause(order domain.ListOrderBy, c domain.PageCursor) (string, []any) {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "(created_at, id) > (?, ?)", []any{c.CreatedAt, c.ServiceAccountID.String()}
	case domain.OrderByServiceAccountIDDesc:
		return "id < ?", []any{c.ServiceAccountID.String()}
	case domain.OrderByServiceAccountIDAsc:
		return "id > ?", []any{c.ServiceAccountID.String()}
	case domain.OrderByNameDesc:
		return "(name, id) < (?, ?)", []any{c.Name, c.ServiceAccountID.String()}
	case domain.OrderByNameAsc:
		return "(name, id) > (?, ?)", []any{c.Name, c.ServiceAccountID.String()}
	case domain.OrderByLastAuthenticatedAtDesc:
		// LastAuthenticatedAt nullable — COALESCE to epoch keeps the
		// keyset-comparison total. CreatedAt is reused as the cursor's
		// generic time field for any time-keyed ordering.
		return "(COALESCE(last_authenticated_at,'1970-01-01'), id) < (?, ?)",
			[]any{c.CreatedAt, c.ServiceAccountID.String()}
	case domain.OrderByLastAuthenticatedAtAsc:
		return "(COALESCE(last_authenticated_at,'1970-01-01'), id) > (?, ?)",
			[]any{c.CreatedAt, c.ServiceAccountID.String()}
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "(created_at, id) < (?, ?)", []any{c.CreatedAt, c.ServiceAccountID.String()}
	}
}

func orderClauseFor(order domain.ListOrderBy) string {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "created_at ASC, id ASC"
	case domain.OrderByServiceAccountIDDesc:
		return "id DESC"
	case domain.OrderByServiceAccountIDAsc:
		return "id ASC"
	case domain.OrderByNameDesc:
		return "name DESC, id DESC"
	case domain.OrderByNameAsc:
		return "name ASC, id ASC"
	case domain.OrderByLastAuthenticatedAtDesc:
		return "last_authenticated_at DESC, id DESC"
	case domain.OrderByLastAuthenticatedAtAsc:
		return "last_authenticated_at ASC, id ASC"
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "created_at DESC, id DESC"
	}
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

