package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

var (
	dbHost     string
	dbPort     string
	dbUser     string
	dbPassword string
	dbName     string
	dbTLS      string

	migrationsPath string
	cmd            string
	steps          int
	charset        string
	collation      string
	forceVersion   int
)

func init() {
	_ = godotenv.Load()

	flag.StringVar(&dbHost, "host", envOr("DB_HOST", "127.0.0.1"), "database host")
	flag.StringVar(&dbPort, "port", envOr("DB_PORT", "3306"), "database port")
	flag.StringVar(&dbUser, "user", envOr("DB_USERNAME", "root"), "database user")
	flag.StringVar(&dbPassword, "password", os.Getenv("DB_PASSWORD"), "database password")
	flag.StringVar(&dbName, "db", envOr("DB_NAME", "sso"), "database name")
	flag.StringVar(&dbTLS, "tls", envOr("DB_TLS", "false"), "database TLS mode (false|true|skip-verify|preferred)")

	flag.StringVar(&migrationsPath, "migrations", "./migrations", "path to migrations folder")
	flag.StringVar(&cmd, "cmd", "up", "command: up | down | drop | force | version | create-db | drop-db | audit:purge | seed-admin")
	flag.IntVar(&steps, "steps", 0, "for up/down: limit number of migrations to apply (0 = all)")
	flag.StringVar(&charset, "charset", "utf8mb4", "for create-db: CHARACTER SET")
	flag.StringVar(&collation, "collation", "utf8mb4_unicode_ci", "for create-db: COLLATE")
	flag.IntVar(&forceVersion, "force-version", -1, "for cmd=force: target version to forcibly set")

	flag.Parse()
}

func main() {
	if cmd == "create-db" || cmd == "drop-db" {
		serverDSN := buildServerDSN(dbHost, dbPort, dbUser, dbPassword, dbTLS)
		if err := manageDatabase(cmd, serverDSN, dbName, charset, collation); err != nil {
			log.Fatalf("%s: %v", cmd, err)
		}
		fmt.Printf("%s: done\n", cmd)
		return
	}

	migrateDSN := buildMigrateDSN(dbHost, dbPort, dbUser, dbPassword, dbName, dbTLS)
	m, err := migrate.New("file://"+migrationsPath, migrateDSN)
	if err != nil {
		log.Fatalf("init migrate: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			log.Printf("close: src=%v db=%v", srcErr, dbErr)
		}
	}()

	switch cmd {
	case "up":
		err = runSteps(m.Up, m.Steps, steps)
	case "down":
		err = runSteps(m.Down, func(n int) error { return m.Steps(-n) }, steps)
	case "drop":
		err = m.Drop()
	case "force":
		if forceVersion < 0 {
			log.Fatal("force requires -force-version >= 0")
		}
		err = m.Force(forceVersion)
	case "version":
		v, dirty, vErr := m.Version()
		if errors.Is(vErr, migrate.ErrNilVersion) {
			fmt.Println("no migrations applied")
			return
		}
		if vErr != nil {
			log.Fatalf("version: %v", vErr)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown cmd: %s\n", cmd)
		os.Exit(2)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("%s: %v", cmd, err)
	}
	fmt.Printf("%s: done\n", cmd)
}

func manageDatabase(cmd, serverDSN, dbName, charset, collation string) error {
	if dbName == "" {
		return fmt.Errorf("database name is required")
	}

	db, err := sql.Open("mysql", serverDSN)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	switch cmd {
	case "create-db":
		stmt := fmt.Sprintf(
			"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s",
			dbName, charset, collation,
		)
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("create database: %w", err)
		}
	case "drop-db":
		if _, err := db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName)); err != nil {
			return fmt.Errorf("drop database: %w", err)
		}
	default:
		return fmt.Errorf("unknown cmd: %s", cmd)
	}
	return nil
}

func buildMigrateDSN(host, port, user, password, dbname, tls string) string {
	addr := net.JoinHostPort(host, port)
	return fmt.Sprintf(
		"mysql://%s:%s@tcp(%s)/%s?multiStatements=true&parseTime=true&loc=UTC&tls=%s",
		user, password, addr, dbname, tls,
	)
}

func buildServerDSN(host, port, user, password, tls string) string {
	addr := net.JoinHostPort(host, port)
	return fmt.Sprintf("%s:%s@tcp(%s)/?tls=%s", user, password, addr, tls)
}

func runSteps(all func() error, n func(int) error, steps int) error {
	if steps == 0 {
		return all()
	}
	return n(steps)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
