// Package dbutil holds MariaDB-flavoured database/sql helpers shared
// across all per-module adapters (identity, app, role, ...).
//
// Anything that depends on the MySQL driver / wire protocol lives here;
// engine-agnostic helpers (none today) would go up one level under
// internal/platform/.
package dbutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sso/internal/kernel/etag"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// mysqlErrDup is the MySQL error number for "Duplicate entry '...' for
// key '...'" — emitted by INSERTs and UPDATEs that violate a UNIQUE
// index.
const mysqlErrDup = 1062

// IsDuplicateEntry reports whether err is a UNIQUE-constraint violation
// from the MySQL driver. Used by every Create / Update path to translate
// driver errors into the per-module ErrXxxAlreadyExists sentinel.
func IsDuplicateEntry(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDup
}

// InTx runs fn inside a write transaction. Commits on success, rolls back
// on any error (including panics — the deferred Rollback is a no-op
// after Commit).
//
// Callers wrap the *sql.Tx with their own dbgen.Queries instance:
//
//	return dbutil.InTx(ctx, db, func(tx *sql.Tx) error {
//	    q := dbgen.New(tx)
//	    // ...
//	})
func InTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Discriminate is called by per-module adapters when a conditional
// UPDATE/DELETE matched 0 rows: a follow-up COUNT(*) decides whether the
// row is missing or the etag drifted. Callers pass their domain-specific
// sentinels (errNotFound / errEtagMismatch) and a closure that runs the
// table-specific COUNT through their own *dbgen.Queries.
//
// expectedEtag == "" means the call was an unconditional write — 0 rows
// then unambiguously means missing, and count is not invoked.
//
// The two operations form a TOCTOU window outside transactions; the worst
// case is reporting NotFound when the truer answer is EtagMismatch (or
// vice versa). Neither violates the proto contract.
func Discriminate(
	ctx context.Context,
	expectedEtag etag.Etag,
	count func(context.Context) (int64, error),
	errNotFound, errEtagMismatch error,
) error {
	if expectedEtag == "" {
		// No etag check was attempted, so 0 rows = the row did not exist.
		return errNotFound
	}
	c, err := count(ctx)
	if err != nil {
		return fmt.Errorf("dbutil: discriminate: %w", err)
	}
	if c == 0 {
		return errNotFound
	}
	return errEtagMismatch
}

// EscapeLike escapes the LIKE-pattern metacharacters (`\`, `%`, `_`) in s
// so the value can be safely embedded between leading/trailing wildcards
// in a `LIKE ?` clause. Callers wrap the result with their own `%` to get
// substring semantics: `"%" + dbutil.EscapeLike(q.Search) + "%"`.
func EscapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
