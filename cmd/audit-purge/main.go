package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/joho/godotenv"
)

var (
	dbHost     string
	dbPort     string
	dbUser     string
	dbPassword string
	dbName     string
	dbTLS      string

	purgeBefore        string
	purgeRetentionDays int
	purgeDryRun        bool
	purgeBatchSize     int
	purgeSleep         time.Duration
)

func init() {
	_ = godotenv.Load()

	flag.StringVar(&dbHost, "host", envOr("DB_HOST", "127.0.0.1"), "database host")
	flag.StringVar(&dbPort, "port", envOr("DB_PORT", "3306"), "database port")
	flag.StringVar(&dbUser, "user", envOr("DB_USERNAME", "root"), "database user")
	flag.StringVar(&dbPassword, "password", os.Getenv("DB_PASSWORD"), "database password")
	flag.StringVar(&dbName, "db", envOr("DB_NAME", "sso"), "database name")
	flag.StringVar(&dbTLS, "tls", envOr("DB_TLS", "false"), "database TLS mode (false|true|skip-verify|preferred)")

	flag.StringVar(&purgeBefore, "before", "", "for audit:purge: cutoff time (RFC3339); overrides -retention-days")
	flag.IntVar(&purgeRetentionDays, "retention-days", 365, "for audit:purge: rows older than (now - N days) are removed")
	flag.BoolVar(&purgeDryRun, "dry-run", false, "for audit:purge: only report SELECT COUNT(*); no DELETE")
	flag.IntVar(&purgeBatchSize, "batch-size", 1000, "for audit:purge: DELETE LIMIT per batch")
	flag.DurationVar(&purgeSleep, "sleep", 100*time.Millisecond, "for audit:purge: pause between batches")

	flag.Parse()
}

func main() {
	cutoff, err := resolveAuditCutoff(purgeBefore, purgeRetentionDays)
	if err != nil {
		log.Fatalf("audit:purge: %v", err)
	}
	appDSN := buildAppDSN(dbHost, dbPort, dbUser, dbPassword, dbName, dbTLS)
	if err := auditPurge(appDSN, cutoff, purgeDryRun, purgeBatchSize, purgeSleep); err != nil {
		log.Fatalf("audit:purge: %v", err)
	}
}

func resolveAuditCutoff(before string, retentionDays int) (time.Time, error) {
	if before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse -before: %w", err)
		}
		return t.UTC(), nil
	}
	if retentionDays <= 0 {
		return time.Time{}, fmt.Errorf("-retention-days must be > 0 when -before is empty")
	}
	return time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour), nil
}

func buildAppDSN(host, port, user, password, dbname, tls string) string {
	addr := net.JoinHostPort(host, port)
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC&tls=%s",
		user, password, addr, dbname, tls,
	)
}

const (
	countAuditEvents  = `SELECT COUNT(*) FROM audit_events WHERE occurred_at < ?`
	deleteAuditEvents = `DELETE FROM audit_events WHERE occurred_at < ? LIMIT ?`
)

func auditPurge(dsn string, cutoff time.Time, dryRun bool, batchSize int, sleep time.Duration) error {
	if batchSize <= 0 {
		return fmt.Errorf("-batch-size must be > 0")
	}
	if sleep < 0 {
		return fmt.Errorf("-sleep must be >= 0")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	if dryRun {
		var count int64
		err := db.QueryRowContext(ctx, countAuditEvents, cutoff).Scan(&count)
		if err != nil {
			return fmt.Errorf("count: %w", err)
		}
		fmt.Printf("audit:purge dry-run: cutoff=%s rows_to_delete=%d\n",
			cutoff.Format(time.RFC3339), count)
		return nil
	}

	fmt.Printf("audit:purge: cutoff=%s batch_size=%d sleep=%s\n",
		cutoff.Format(time.RFC3339), batchSize, sleep)

	var totalDeleted int64
	for round := 1; ; round++ {
		res, err := db.ExecContext(ctx, deleteAuditEvents, cutoff, batchSize)
		if err != nil {
			return fmt.Errorf("delete batch %d: %w", round, err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("rows affected: %w", err)
		}
		totalDeleted += n
		fmt.Printf("audit:purge: batch %d deleted=%d total=%d\n", round, n, totalDeleted)

		if n == 0 {
			break
		}
		if int64(batchSize) > n {
			// Partial batch — nothing left under the cutoff.
			break
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}
	}

	fmt.Printf("audit:purge: done total_deleted=%d\n", totalDeleted)
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
