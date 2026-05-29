package mariadb

import (
	"context"
	"fmt"
	"strings"

	"sso/internal/modules/app/internal/domain"
	"sso/internal/modules/app/internal/mariadb/dbgen"
	"sso/internal/kernel/dbutil"
)

const listSelectCols = `id, name, slug, link, status, etag, created_at, updated_at`

func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.ListResult, error) {
	if q.PageSize <= 0 {
		return domain.ListResult{}, fmt.Errorf("app repo: list: page_size must be > 0")
	}

	where, args := buildWhere(q)
	orderBy := orderClauseFor(q.OrderBy)
	limit := q.PageSize + 1

	var query string
	if len(where) == 0 {
		query = fmt.Sprintf(`SELECT %s FROM apps ORDER BY %s LIMIT %d`,
			listSelectCols, orderBy, limit)
	} else {
		query = fmt.Sprintf(`SELECT %s FROM apps WHERE %s ORDER BY %s LIMIT %d`,
			listSelectCols, strings.Join(where, " AND "), orderBy, limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("app repo: list: %w", err)
	}
	defer rows.Close()

	apps := make([]*domain.App, 0, q.PageSize)
	for rows.Next() {
		var a dbgen.App
		if err := rows.Scan(
			&a.ID, &a.Name, &a.Slug, &a.Link, &a.Status, &a.Etag,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return domain.ListResult{}, fmt.Errorf("app repo: list: scan: %w", err)
		}
		apps = append(apps, dbgenToDomain(a))
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("app repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(apps) > q.PageSize {
		apps = apps[:q.PageSize]
		last := apps[len(apps)-1]
		nextCursor = &domain.PageCursor{
			CreatedAt: last.CreatedAt(),
			AppID:     last.ID(),
			Name:      last.Name,
		}
	}

	return domain.ListResult{Apps: apps, NextCursor: nextCursor}, nil
}

// buildWhere assembles the filter portion of a ListApps SELECT. Unlike
// the identity adapter, an empty filter set yields an empty WHERE — apps
// have no implicit "exclude DELETED" default (no DELETED state exists).
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
		// Search across name, link, slug per the proto contract.
		pattern := "%" + dbutil.EscapeLike(q.Search) + "%"
		where = append(where,
			"(name LIKE ? OR link LIKE ? OR slug LIKE ?)")
		args = append(args, pattern, pattern, pattern)
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
		return "(created_at, id) > (?, ?)", []any{c.CreatedAt, c.AppID.String()}
	case domain.OrderByAppIDDesc:
		return "id < ?", []any{c.AppID.String()}
	case domain.OrderByAppIDAsc:
		return "id > ?", []any{c.AppID.String()}
	case domain.OrderByNameDesc:
		return "(name, id) < (?, ?)", []any{c.Name, c.AppID.String()}
	case domain.OrderByNameAsc:
		return "(name, id) > (?, ?)", []any{c.Name, c.AppID.String()}
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "(created_at, id) < (?, ?)", []any{c.CreatedAt, c.AppID.String()}
	}
}

func orderClauseFor(order domain.ListOrderBy) string {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "created_at ASC, id ASC"
	case domain.OrderByAppIDDesc:
		return "id DESC"
	case domain.OrderByAppIDAsc:
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

