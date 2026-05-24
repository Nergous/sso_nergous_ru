// Package mariadb is the platform-level adapter that turns a DatabaseConfig
// into a configured *sql.DB. It is consumed by bootstrap and ignores any
// domain concerns — repositories receive the *sql.DB it produces.
package mariadb

import (
	"context"
	"database/sql"
	"fmt"

	"sso/internal/platform/config"

	_ "github.com/go-sql-driver/mysql"
)

// Open dials MariaDB using cfg, applies pool settings, and pings the server
// (capped by cfg.Options.Timeout) to fail fast on a misconfigured deployment.
// The driver is the value from cfg.Driver (typically "mysql"); the DSN is
// rendered by cfg.DSN.
func Open(ctx context.Context, cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open(cfg.Driver, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("mariadb: open: %w", err)
	}

	db.SetMaxOpenConns(cfg.Pool.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Pool.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Pool.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.Pool.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.Options.Timeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mariadb: ping: %w", err)
	}
	return db, nil
}
