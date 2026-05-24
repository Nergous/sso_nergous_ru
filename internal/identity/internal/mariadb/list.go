package mariadb

import (
	"context"
	"fmt"
	"strings"

	"sso/internal/identity/internal/domain"
	"sso/internal/identity/internal/mariadb/dbgen"
	"sso/internal/kernel/dbutil"
)

// listSelectCols mirrors the column list used by the sqlc GetUserByID query.
// Kept as a constant so the scan order in List stays in lockstep with the
// dbgen.User struct field order.
const listSelectCols = `
    id, email, username, display_name, avatar_url, locale, timezone,
    status, etag, created_at, updated_at, last_login_at`

// List paginates the identity directory. Hand-written rather than sqlc-
// generated because the WHERE / ORDER BY shape varies per request (filters
// and OrderBy combine into 2^N × 6 unique queries — beyond sqlc's reach).
func (r *Repository) List(ctx context.Context, q domain.ListQuery) (domain.ListResult, error) {
	if q.PageSize <= 0 {
		return domain.ListResult{}, fmt.Errorf("identity repo: list: page_size must be > 0")
	}

	where, args := buildWhere(q)
	orderBy := orderClauseFor(q.OrderBy)

	// Fetch one extra row to detect "has next page".
	limit := q.PageSize + 1

	query := fmt.Sprintf(`SELECT %s FROM users WHERE %s ORDER BY %s LIMIT %d`,
		listSelectCols, strings.Join(where, " AND "), orderBy, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return domain.ListResult{}, fmt.Errorf("identity repo: list: %w", err)
	}
	defer rows.Close()

	users := make([]*domain.User, 0, q.PageSize)
	for rows.Next() {
		var u dbgen.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.AvatarUrl,
			&u.Locale, &u.Timezone, &u.Status, &u.Etag,
			&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
		); err != nil {
			return domain.ListResult{}, fmt.Errorf("identity repo: list: scan: %w", err)
		}
		users = append(users, dbgenToDomain(u))
	}
	if err := rows.Err(); err != nil {
		return domain.ListResult{}, fmt.Errorf("identity repo: list: rows: %w", err)
	}

	var nextCursor *domain.PageCursor
	if len(users) > q.PageSize {
		// More data exists; trim the probe row and emit a cursor pointing
		// at the last row we actually return.
		users = users[:q.PageSize]
		last := users[len(users)-1]
		nextCursor = &domain.PageCursor{
			CreatedAt: last.CreatedAt(),
			UserID:    last.ID(),
			Username:  last.Username,
		}
	}

	return domain.ListResult{Users: users, NextCursor: nextCursor}, nil
}

// buildWhere assembles the filter portion of a ListUsers SELECT. Returns the
// list of AND-joined predicates and the matching positional args. The slice
// is never empty: it always contains at least the status filter (the
// proto-default "exclude DELETED" or the explicit list).
func buildWhere(q domain.ListQuery) ([]string, []any) {
	var (
		where []string
		args  []any
	)

	// --- status -----------------------------------------------------------
	if len(q.Statuses) == 0 {
		where = append(where, "status != ?")
		args = append(args, uint8(domain.UserStatusDeleted))
	} else {
		ph := make([]string, len(q.Statuses))
		for i, s := range q.Statuses {
			ph[i] = "?"
			args = append(args, uint8(s))
		}
		where = append(where, "status IN ("+strings.Join(ph, ",")+")")
	}

	// --- search -----------------------------------------------------------
	if q.Search != "" {
		// LIKE is case-insensitive under utf8mb4_unicode_ci.
		pattern := "%" + dbutil.EscapeLike(q.Search) + "%"
		where = append(where,
			"(email LIKE ? OR username LIKE ? OR display_name LIKE ?)")
		args = append(args, pattern, pattern, pattern)
	}

	// --- explicit field filters ------------------------------------------
	if len(q.Emails) > 0 {
		where = append(where, "email IN ("+placeholders(len(q.Emails))+")")
		for _, v := range q.Emails {
			args = append(args, v)
		}
	}
	if len(q.Usernames) > 0 {
		where = append(where, "username IN ("+placeholders(len(q.Usernames))+")")
		for _, v := range q.Usernames {
			args = append(args, v)
		}
	}
	if len(q.DisplayNames) > 0 {
		where = append(where, "display_name IN ("+placeholders(len(q.DisplayNames))+")")
		for _, v := range q.DisplayNames {
			args = append(args, v)
		}
	}

	// --- keyset cursor ----------------------------------------------------
	if q.After != nil {
		clause, cArgs := keysetClause(q.OrderBy, *q.After)
		where = append(where, clause)
		args = append(args, cArgs...)
	}

	return where, args
}

// keysetClause builds the "strictly after the cursor" predicate matching the
// active OrderBy. MariaDB supports tuple comparison: (a, b) < (c, d) is
// equivalent to a<c OR (a=c AND b<d).
func keysetClause(order domain.ListOrderBy, c domain.PageCursor) (string, []any) {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "(created_at, id) > (?, ?)", []any{c.CreatedAt, c.UserID.String()}
	case domain.OrderByUserIDDesc:
		return "id < ?", []any{c.UserID.String()}
	case domain.OrderByUserIDAsc:
		return "id > ?", []any{c.UserID.String()}
	case domain.OrderByUsernameDesc:
		return "(username, id) < (?, ?)", []any{c.Username, c.UserID.String()}
	case domain.OrderByUsernameAsc:
		return "(username, id) > (?, ?)", []any{c.Username, c.UserID.String()}
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "(created_at, id) < (?, ?)", []any{c.CreatedAt, c.UserID.String()}
	}
}

// orderClauseFor maps a domain OrderBy to a SQL ORDER BY clause. id is
// always the tie-breaker (proto contract: "Tie-breaker user_id is always
// appended implicitly").
func orderClauseFor(order domain.ListOrderBy) string {
	switch order {
	case domain.OrderByCreatedAtAsc:
		return "created_at ASC, id ASC"
	case domain.OrderByUserIDDesc:
		return "id DESC"
	case domain.OrderByUserIDAsc:
		return "id ASC"
	case domain.OrderByUsernameDesc:
		return "username DESC, id DESC"
	case domain.OrderByUsernameAsc:
		return "username ASC, id ASC"
	case domain.OrderByCreatedAtDesc, domain.OrderByUnspecified:
		fallthrough
	default:
		return "created_at DESC, id DESC"
	}
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}
