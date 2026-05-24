package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

const (
	seedAppName            = "sso-admin"
	seedAppSlug            = "sso-admin"
	seedStatusActive uint8 = 1
)

var seedRoles = []struct {
	name        string
	permissions []string
}{
	{"sso.admin.users", []string{"users:*"}},
	{"sso.admin.apps", []string{"apps:*"}},
	{"sso.admin.roles", []string{"roles:*"}},
	{"sso.admin.service_accounts", []string{"service_accounts:*"}},
	{"sso.admin.audit", []string{"audit:read"}},
	{"sso.admin.super", []string{
		"users:*", "apps:*", "roles:*",
		"service_accounts:*", "audit:*",
		"sessions:*", "access:*",
	}},
}

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

	flag.Parse()
}

func main() {
	appDSN := buildAppDSN(dbHost, dbPort, dbUser, dbPassword, dbName, dbTLS)

	if err := seedAdmin(appDSN); err != nil {
		log.Fatalf("seed-admin: %v", err)
	}
}

type SeedData struct {
	email    string
	password string
	username string

	appLink     string
	displayName string

	bcryptCost int
}

func getSeedDataFromEnv() (*SeedData, error) {
	email := envOr("SSO_SEED_ADMIN_EMAIL", "")
	password := envOr("SSO_SEED_ADMIN_PASSWORD", "")
	username := envOr("SSO_SEED_ADMIN_USERNAME", "")
	if email == "" || password == "" || username == "" {
		return nil, fmt.Errorf("SSO_SEED_ADMIN_EMAIL, SSO_SEED_ADMIN_PASSWORD, SSO_SEED_ADMIN_USERNAME are required")
	}

	appLink := envOr("SSO_SEED_ADMIN_APP_LINK", "https://sso-admin.local/")

	displayName := envOr("SSO_SEED_ADMIN_DISPLAY_NAME", "Admin")

	bcryptCost := 12
	if s := os.Getenv("SSO_SEED_ADMIN_BCRYPT_COST"); s != "" {
		c, err := strconv.Atoi(s)
		if err != nil || c < 4 || c > 31 {
			return nil, fmt.Errorf("SSO_SEED_ADMIN_BCRYPT_COST must be integer 4..31, got %q", s)
		}
		bcryptCost = c
	}
	return &SeedData{email, password, username, appLink, displayName, bcryptCost}, nil
}

func seedAdmin(dsn string) error {
	data, err := getSeedDataFromEnv()
	if err != nil {
		return err
	}
	email := data.email
	password := data.password
	username := data.username
	appLink := data.appLink
	displayName := data.displayName
	bcryptCost := data.bcryptCost

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
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

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	appID, err := seedFindOrCreateApp(ctx, tx, appLink)
	if err != nil {
		return fmt.Errorf("app: %w", err)
	}

	roleIDs := make([]string, len(seedRoles))
	for i, r := range seedRoles {
		id, err := seedFindOrCreateRole(ctx, tx, appID, r.name, r.permissions)
		if err != nil {
			return fmt.Errorf("role %s: %w", r.name, err)
		}
		roleIDs[i] = id
	}

	userID, err := seedFindOrCreateUser(ctx, tx, email, username, displayName, passwordHash)
	if err != nil {
		return fmt.Errorf("user: %w", err)
	}

	now := time.Now().UTC()
	for i, roleID := range roleIDs {
		created, err := seedEnsureAssignment(ctx, tx, userID, roleID, appID, now)
		if err != nil {
			return fmt.Errorf("assignment %s: %w", seedRoles[i].name, err)
		}
		action := "existed"
		if created {
			action = "created"
		}
		fmt.Printf("assignment %s -> %s: %s\n", email, seedRoles[i].name, action)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

const (
	findApp = `SELECT id FROM apps WHERE slug = ?`

	insertIntoApps = `INSERT INTO apps (id, name, slug, link, status, etag, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
)

func seedFindOrCreateApp(ctx context.Context, tx *sql.Tx, link string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, findApp, seedAppSlug).Scan(&id)

	if err == nil {
		fmt.Printf("app %s: existed (id=%s)\n", seedAppSlug, id)
		return id, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("select: %w", err)
	}

	id, etag, now := seedNewIDv7(), seedNewEtag(), time.Now().UTC()

	if _, err := tx.ExecContext(ctx,
		insertIntoApps,
		id, seedAppName, seedAppSlug, link, seedStatusActive, etag, now, now,
	); err != nil {
		return "", fmt.Errorf("insert: %w", err)
	}

	fmt.Printf("app %s: created (id=%s)\n", seedAppSlug, id)
	return id, nil
}

const (
	findRole = `SELECT id FROM roles WHERE app_id = ? AND name = ?`

	insertIntoRoles = `INSERT INTO roles (id, app_id, name, description, status, etag, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	insertIntoRolePermissions = `INSERT INTO role_permissions (role_id, permission) VALUES (?, ?)`
)

func seedFindOrCreateRole(ctx context.Context, tx *sql.Tx, appID, name string, permissions []string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, findRole, appID, name).Scan(&id)

	if err == nil {
		fmt.Printf("role %s: existed (id=%s)\n", name, id)
		return id, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("select: %w", err)
	}

	id, etag, now := seedNewIDv7(), seedNewEtag(), time.Now().UTC()

	if _, err := tx.ExecContext(ctx, insertIntoRoles,
		id, appID, name, sql.NullString{}, seedStatusActive, etag, now, now,
	); err != nil {
		return "", fmt.Errorf("insert role: %w", err)
	}

	for _, perm := range permissions {
		if _, err := tx.ExecContext(ctx,
			insertIntoRolePermissions,
			id, perm,
		); err != nil {
			return "", fmt.Errorf("insert permission %s: %w", perm, err)
		}
	}

	fmt.Printf("role %s: created (id=%s, permissions=[%s])\n", name, id, strings.Join(permissions, ", "))
	return id, nil
}

const (
	findUser = `SELECT id FROM users WHERE email = ?`

	insertIntoUsers = `INSERT INTO users (id, email, username, password_hash, display_name, avatar_url, locale, timezone, status, etag, created_at, updated_at, last_login_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
)

func seedFindOrCreateUser(ctx context.Context, tx *sql.Tx, email, username, displayName string, passwordHash []byte) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, findUser, email).Scan(&id)

	if err == nil {
		fmt.Printf("user %s: existed (id=%s, password unchanged)\n", email, id)
		return id, nil
	}

	if err != sql.ErrNoRows {
		return "", fmt.Errorf("select: %w", err)
	}

	id, etag, now := seedNewIDv7(), seedNewEtag(), time.Now().UTC()

	if _, err := tx.ExecContext(ctx,
		insertIntoUsers,
		id, email, username,
		sql.NullString{String: string(passwordHash), Valid: true},
		displayName,
		sql.NullString{},
		"", "",
		seedStatusActive,
		etag, now, now,
		sql.NullTime{},
	); err != nil {
		return "", fmt.Errorf("insert user: %w", err)
	}

	fmt.Printf("user %s: created (id=%s)\n", email, id)
	return id, nil
}

const (
	insertIntoRoleAssignments = `INSERT IGNORE INTO role_assignments (user_id, role_id, app_id, granted_by_user_id, granted_at) VALUES (?, ?, ?, ?, ?)`
)

func seedEnsureAssignment(ctx context.Context, tx *sql.Tx, userID, roleID, appID string, now time.Time) (created bool, err error) {
	res, err := tx.ExecContext(ctx,
		insertIntoRoleAssignments,
		userID, roleID, appID, userID, now,
	)

	if err != nil {
		return false, fmt.Errorf("insert: %w", err)
	}

	n, err := res.RowsAffected()

	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return n > 0, nil
}

func seedNewIDv7() string {
	id, err := uuid.NewV7()

	if err != nil {
		panic(fmt.Sprintf("uuid.NewV7: %v", err))
	}

	return id.String()
}

func seedNewEtag() string {
	return uuid.NewString()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func buildAppDSN(host, port, user, password, dbname, tls string) string {
	addr := net.JoinHostPort(host, port)
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC&tls=%s",
		user, password, addr, dbname, tls,
	)
}
