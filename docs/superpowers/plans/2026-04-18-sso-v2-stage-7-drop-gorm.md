# SSO v2 Migration — Stage 7: Multi-Backend Storage (Drop GORM)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 6 merged to main, все `BOOTSTRAP_ADMIN_EMAIL`/`DB_*` env vars уже работают.

**Goal:** Ввести `Storage` интерфейс с несколькими реализациями (MariaDB, SQLite), написать репозитории на чистом `database/sql` с per-dialect SQL, выпилить GORM целиком. SQLite pure-Go включает in-memory integration-тесты.

**Architecture:**
- `internal/storage/storage.go` — интерфейс `Storage` + `DBProvider` + factory `New(cfg)`
- `internal/storage/mariadb/` — MariaDB-реализация, миграции через goose (embedded `migrations/*.sql`)
- `internal/storage/sqlite/` — SQLite pure-Go (`modernc.org/sqlite`, **без CGO**), миграции через goose
- `internal/storage/postgres/` — scaffold, `return nil, errors.ErrUnsupported` — для будущего
- `internal/repositories/mariadb/` — репозитории под MariaDB (raw SQL, MySQL-dialect)
- `internal/repositories/sqlite/` — репозитории под SQLite (raw SQL, SQLite-dialect)
- Сервисы по-прежнему зависят от `repositories.UserRepository/AppRepository/TokenRepository` интерфейсов (введены в Stage 0) — им всё равно, какой backend
- `app.New` выбирает набор репо по `cfg.Database.Driver`
- Integration-тесты переезжают на in-memory SQLite (реальный SQL, нет Docker)

**Tech Stack:** `database/sql`, `github.com/go-sql-driver/mysql`, `modernc.org/sqlite`, `github.com/pressly/goose/v3`.

**Branch:** `stage-7-multi-backend`

---

## Principle: "Abstraction at repository layer"

Единственное ради-чего всё это делается — менять БД по конфигу без перекомпиляции интерфейсов. Вот схема:

```
┌───────────────────────┐
│ services.AuthService  │── зависит от repositories.UserRepository (интерфейс)
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐       ┌───────────────────────┐       ┌───────────────────────┐
│ repositories/mariadb  │  OR   │ repositories/sqlite   │  OR   │ repositories/postgres │
│   (MariaDB raw SQL)   │       │   (SQLite raw SQL)    │       │       (stub)          │
└───────────┬───────────┘       └───────────┬───────────┘       └───────────┬───────────┘
            │                                │                                │
            ▼                                ▼                                ▼
┌───────────────────────┐       ┌───────────────────────┐       ┌───────────────────────┐
│  storage/mariadb      │       │   storage/sqlite      │       │   storage/postgres    │
│  (*sql.DB, Migrate)   │       │  (*sql.DB, Migrate)   │       │     (stub)            │
└───────────────────────┘       └───────────────────────┘       └───────────────────────┘
```

`app.New` читает `cfg.Database.Driver`, вызывает `storage.New(cfg)`, потом по driver выбирает конструкторы репо из нужного подпакета. Сервисы видят только интерфейсы.

---

## File Structure

**Создаётся:**
- `internal/storage/storage.go` — интерфейс + фабрика
- `internal/storage/mariadb/storage.go` — `sql.DB` обёртка + `Migrate()` через goose
- `internal/storage/mariadb/migrations/00001_init.sql` до `00005_system_admin.sql` — те же миграции что планировались в Stage 2/4, теперь файлами
- `internal/storage/sqlite/storage.go` — то же для SQLite
- `internal/storage/sqlite/migrations/*.sql` — SQLite-диалект
- `internal/storage/postgres/storage.go` — stub с `errors.ErrUnsupported`
- `internal/repositories/mariadb/{user,app,token}.go`
- `internal/repositories/sqlite/{user,app,token}.go`
- `internal/storage/storage_integration_test.go` — integration-тесты через SQLite in-memory

**Модифицируется:**
- `internal/repositories/iface.go` — расширить интерфейсы (добавить `IsSystemAdmin`, `UpdatePassword` из Stage 3/4 если ещё не добавили)
- `internal/app/app.go` — factory dispatch по driver
- `internal/models/*.go` — удалить все `gorm:` теги

**Удаляется:**
- `internal/repositories/user.go`, `app.go`, `token.go`, `errors.go` (старые GORM-based)
- `internal/storage/mariadb/mariadb.go` (старая GORM-обёртка)
- `gorm.io/gorm`, `gorm.io/driver/mysql`, `jinzhu/*` из `go.mod`

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-7-multi-backend
go build ./... && go test ./...
```
Expected: green. Baseline перед большим рефакторингом.

---

## Task 2: Storage интерфейс + factory

**Files:**
- Create: `internal/storage/storage.go`

- [ ] **Step 2.1: Интерфейс + factory**

```go
// Package storage defines the persistence abstraction. Concrete backends
// live under storage/<driver>/ — one package per RDBMS. The factory in this
// file dispatches on config.Database.Driver.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sso/internal/config"
	"sso/internal/storage/mariadb"
	"sso/internal/storage/postgres"
	"sso/internal/storage/sqlite"
)

// Storage is the minimal lifecycle contract every backend must satisfy.
type Storage interface {
	// Close releases the underlying connection pool.
	Close() error
	// Migrate applies all pending schema migrations. Idempotent.
	Migrate() error
	// MigrateRefresh drops all app tables and re-runs Migrate. Test-only;
	// production code must not call it.
	MigrateRefresh(ctx context.Context) error
}

// DBProvider exposes the underlying *sql.DB. Repositories use this to
// issue raw queries; the rest of the app depends only on Storage.
type DBProvider interface {
	Storage
	GetDB() *sql.DB
}

// ErrUnsupportedDriver is returned by the factory when cfg.Driver is unknown
// or the requested backend is scaffolded but not implemented.
var ErrUnsupportedDriver = errors.New("unsupported storage driver")

// New dispatches on cfg.Driver to build a concrete backend.
func New(cfg config.Database) (DBProvider, error) {
	switch cfg.Driver {
	case "mariadb", "mysql":
		return mariadb.New(cfg)
	case "sqlite", "sqlite3":
		return sqlite.New(cfg)
	case "postgres", "postgresql":
		return postgres.New(cfg)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDriver, cfg.Driver)
	}
}
```

- [ ] **Step 2.2: Сборка (не пройдёт — пакеты ещё не существуют)**

Это ожидаемо. Task 3-5 создают недостающие пакеты, после чего всё собирается.

---

## Task 3: MariaDB backend

**Files:**
- Create: `internal/storage/mariadb/storage.go`
- Create: `internal/storage/mariadb/migrations/00001_init.sql`
- Create: `internal/storage/mariadb/migrations/00002_admin_composite.sql`
- Create: `internal/storage/mariadb/migrations/00003_refresh_token_indexes.sql`
- Create: `internal/storage/mariadb/migrations/00004_system_admin.sql`

- [ ] **Step 3.1: Зависимости**

```bash
go get github.com/pressly/goose/v3@latest
go mod tidy
```

(`go-sql-driver/mysql` уже в транзитиве, но после этого станет прямой — goose её сам не вытянет.)

```bash
go get github.com/go-sql-driver/mysql@latest
go mod tidy
```

- [ ] **Step 3.2: storage.go**

```go
// Package mariadb is the MariaDB/MySQL backend.
package mariadb

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"time"

	"sso/internal/config"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Storage struct {
	db *sql.DB
}

func New(cfg config.Database) (*Storage, error) {
	const op = "storage.mariadb.New"
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		cfg.UsernameDB, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}
	return &Storage{db: db}, nil
}

func (s *Storage) GetDB() *sql.DB { return s.db }

func (s *Storage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Storage) Migrate() error {
	const op = "storage.mariadb.Migrate"
	if email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"); email != "" {
		if _, err := s.db.Exec("SET @bootstrap_email = ?", email); err != nil {
			return fmt.Errorf("%s: bootstrap var: %w", op, err)
		}
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("%s: dialect: %w", op, err)
	}
	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("%s: up: %w", op, err)
	}
	return nil
}

func (s *Storage) MigrateRefresh(ctx context.Context) error {
	const op = "storage.mariadb.MigrateRefresh"
	tables := []string{"refresh_tokens", "admins", "users", "apps", "goose_db_version"}
	for _, t := range tables {
		if _, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+t); err != nil {
			return fmt.Errorf("%s: drop %s: %w", op, t, err)
		}
	}
	return s.Migrate()
}
```

- [ ] **Step 3.3: Миграции (SQL-файлы)**

`migrations/00001_init.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL,
    pass_hash VARCHAR(255) NOT NULL,
    steam_url VARCHAR(2048),
    path_to_photo VARCHAR(2048),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY idx_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE apps (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    link VARCHAR(2048) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY idx_apps_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE admins (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id INT UNSIGNED NOT NULL,
    app_id INT UNSIGNED NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE refresh_tokens (
    id INT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    token VARCHAR(255) NOT NULL,
    user_id INT UNSIGNED NOT NULL,
    app_id INT UNSIGNED NOT NULL,
    expires_at DATETIME NOT NULL,
    UNIQUE KEY idx_refresh_tokens_token (token),
    CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_app FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS users;
```

`migrations/00002_admin_composite.sql`:

```sql
-- +goose Up
CREATE UNIQUE INDEX idx_admin_user_app ON admins (user_id, app_id);
CREATE INDEX idx_admin_user ON admins (user_id);
CREATE INDEX idx_admin_app ON admins (app_id);

-- +goose Down
DROP INDEX idx_admin_app ON admins;
DROP INDEX idx_admin_user ON admins;
DROP INDEX idx_admin_user_app ON admins;
```

`migrations/00003_refresh_token_indexes.sql`:

```sql
-- +goose Up
CREATE INDEX idx_refresh_user ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_app ON refresh_tokens (app_id);

-- +goose Down
DROP INDEX idx_refresh_app ON refresh_tokens;
DROP INDEX idx_refresh_user ON refresh_tokens;
```

`migrations/00004_system_admin.sql`:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN is_system_admin BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_users_system_admin ON users (is_system_admin);
-- +goose StatementBegin
UPDATE users SET is_system_admin = TRUE
  WHERE email = COALESCE(NULLIF(TRIM(@bootstrap_email), ''), '__no_bootstrap__');
-- +goose StatementEnd

-- +goose Down
DROP INDEX idx_users_system_admin ON users;
ALTER TABLE users DROP COLUMN is_system_admin;
```

- [ ] **Step 3.4: Commit**

```bash
git add go.mod go.sum internal/storage/mariadb/
git commit -m "feat(storage): MariaDB backend with goose migrations"
```

---

## Task 4: SQLite backend (pure-Go)

**Files:**
- Create: `internal/storage/sqlite/storage.go`
- Create: `internal/storage/sqlite/migrations/*.sql` (4 файла, SQLite dialect)

- [ ] **Step 4.1: Зависимости**

```bash
go get modernc.org/sqlite@latest
go mod tidy
```

`modernc.org/sqlite` — **pure Go**, без CGO. Работает везде где работает Go, включая эфемерные VM scheduled агентов.

- [ ] **Step 4.2: storage.go**

```go
// Package sqlite is a pure-Go SQLite backend (via modernc.org/sqlite).
// Primary use: in-memory integration tests. Can also run with a file-based
// DSN for lightweight deployments.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"sso/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Storage struct {
	db *sql.DB
}

// New builds a SQLite Storage. DSN resolution:
//   - DBName = ":memory:" — in-memory (recommended for tests)
//   - DBName = "/path/to/file.db" — on-disk file
//   - DBName = "file::memory:?cache=shared" — shared in-memory (same DSN returns the same DB across connections)
func New(cfg config.Database) (*Storage, error) {
	const op = "storage.sqlite.New"
	dsn := cfg.DBName
	if dsn == "" {
		dsn = ":memory:"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	// SQLite has one writer — don't over-pool.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: enable FKs: %w", op, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%s: ping: %w", op, err)
	}
	return &Storage{db: db}, nil
}

func (s *Storage) GetDB() *sql.DB { return s.db }

func (s *Storage) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Storage) Migrate() error {
	const op = "storage.sqlite.Migrate"
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("%s: dialect: %w", op, err)
	}
	if err := goose.Up(s.db, "migrations"); err != nil {
		return fmt.Errorf("%s: up: %w", op, err)
	}
	return nil
}

func (s *Storage) MigrateRefresh(ctx context.Context) error {
	const op = "storage.sqlite.MigrateRefresh"
	tables := []string{"refresh_tokens", "admins", "users", "apps", "goose_db_version"}
	for _, t := range tables {
		if _, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS "+t); err != nil {
			return fmt.Errorf("%s: drop %s: %w", op, t, err)
		}
	}
	return s.Migrate()
}
```

- [ ] **Step 4.3: Миграции (SQLite dialect)**

Ключевые отличия от MariaDB:
- `INTEGER PRIMARY KEY AUTOINCREMENT` вместо `INT UNSIGNED AUTO_INCREMENT`
- Нет `ENGINE=InnoDB`, `DEFAULT CHARSET`
- FK на уровне колонки или отдельной строки
- `DATETIME DEFAULT CURRENT_TIMESTAMP` — но `ON UPDATE` SQLite не поддерживает; updated_at придётся обновлять явно в UPDATE запросах
- `BOOLEAN` — на самом деле INTEGER 0/1

`migrations/00001_init.sql`:

```sql
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL UNIQUE,
    pass_hash TEXT NOT NULL,
    steam_url TEXT,
    path_to_photo TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE apps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    secret TEXT NOT NULL,
    link TEXT NOT NULL,
    is_enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE admins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    app_id INTEGER NOT NULL
);

CREATE TABLE refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS users;
```

`migrations/00002_admin_composite.sql`:

```sql
-- +goose Up
CREATE UNIQUE INDEX idx_admin_user_app ON admins (user_id, app_id);
CREATE INDEX idx_admin_user ON admins (user_id);
CREATE INDEX idx_admin_app ON admins (app_id);

-- +goose Down
DROP INDEX idx_admin_app;
DROP INDEX idx_admin_user;
DROP INDEX idx_admin_user_app;
```

`migrations/00003_refresh_token_indexes.sql`:

```sql
-- +goose Up
CREATE INDEX idx_refresh_user ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_app ON refresh_tokens (app_id);

-- +goose Down
DROP INDEX idx_refresh_app;
DROP INDEX idx_refresh_user;
```

`migrations/00004_system_admin.sql`:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN is_system_admin INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_users_system_admin ON users (is_system_admin);

-- +goose Down
DROP INDEX idx_users_system_admin;
-- SQLite до 3.35 не умеет DROP COLUMN; в 3.35+ умеет. modernc — свежий.
ALTER TABLE users DROP COLUMN is_system_admin;
```

**Важно:** SQLite bootstrap через `@var` не работает (переменных такого типа нет). Вариант — игнорировать bootstrap для SQLite (он и так редко прод-use). Если нужен — делать через `env.sh` скрипт перед стартом.

- [ ] **Step 4.4: Commit**

```bash
git add go.mod go.sum internal/storage/sqlite/
git commit -m "feat(storage): pure-Go SQLite backend via modernc.org/sqlite"
```

---

## Task 5: Postgres scaffold

**Files:** `internal/storage/postgres/storage.go`

- [ ] **Step 5.1: Stub**

```go
// Package postgres is a placeholder for future PostgreSQL support. Calling
// New returns ErrNotImplemented; the factory in internal/storage will wrap it.
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"sso/internal/config"
)

var ErrNotImplemented = errors.New("postgres backend not implemented yet")

type Storage struct{}

func New(_ config.Database) (*Storage, error) { return nil, ErrNotImplemented }

func (*Storage) GetDB() *sql.DB                     { return nil }
func (*Storage) Close() error                       { return nil }
func (*Storage) Migrate() error                     { return ErrNotImplemented }
func (*Storage) MigrateRefresh(_ context.Context) error { return ErrNotImplemented }
```

- [ ] **Step 5.2: Commit**

```bash
git add internal/storage/postgres/
git commit -m "feat(storage): postgres scaffold (not implemented)"
```

---

## Task 6: Models без GORM-тегов

**Files:** `internal/models/*.go`

- [ ] **Step 6.1: Почистить**

Пройтись по всем файлам и убрать все `gorm:"..."` теги, оставить чистые Go struct. Пример `user.go`:

```go
package models

import "time"

type User struct {
	ID            uint32
	Email         string
	PassHash      string
	SteamURL      string
	PathToPhoto   string
	IsSystemAdmin bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
```

То же для `App`, `Admin`, `RefreshToken`.

- [ ] **Step 6.2: Commit**

```bash
git add internal/models/
git commit -m "refactor(models): remove gorm tags"
```

---

## Task 7: Репозитории для MariaDB (raw SQL)

**Files:**
- Create: `internal/repositories/mariadb/user.go`
- Create: `internal/repositories/mariadb/app.go`
- Create: `internal/repositories/mariadb/token.go`
- Create: `internal/repositories/mariadb/errors.go`

**Принцип:** **один файл — один репо**, вся работа через `*sql.DB` полученный через `storage.DBProvider.GetDB()`. Маппинг `sql.ErrNoRows` / MySQL 1062 → доменные ошибки.

- [ ] **Step 7.1: errors.go**

```go
package mariadb

import (
	"database/sql"
	"errors"

	"github.com/go-sql-driver/mysql"
)

const mysqlErrDup uint16 = 1062

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDup
}

func isNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
```

- [ ] **Step 7.2: user.go**

Полная версия — see Stage 7 старая версия (до этой переделки) у меня уже была с raw SQL под MariaDB. Ключевые методы: `GetUserByEmail`, `GetUserByID`, `GetAllUsers(ctx, pageSize, pageToken)`, `CreateUser`, `UpdateUser`, `UpdatePassword`, `DeleteUser`, `IsSystemAdmin`.

Паттерн:
```go
package mariadb

import (
	"context"
	"database/sql"
	"fmt"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/pagination"
	"sso/internal/storage"
)

type UserRepo struct{ db *sql.DB }

func NewUserRepo(s storage.DBProvider) *UserRepo { return &UserRepo{db: s.GetDB()} }

const userCols = "id, email, pass_hash, COALESCE(steam_url, ''), COALESCE(path_to_photo, ''), is_system_admin, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.PassHash, &u.SteamURL, &u.PathToPhoto, &u.IsSystemAdmin, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE email = ?", email)
	u, err := scanUser(row)
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	return u, err
}

// ... остальные методы по аналогии (см. Stage 7 предыдущая версия или делай прямолинейно)
```

**Полный список методов с сигнатурами — от интерфейса `repositories.UserRepository` (Stage 0 + расширения в Stage 2/4).**

- [ ] **Step 7.3: app.go и token.go — по аналогии**

Паттерн такой же. Для `AppRepo.IsAdmin`:
```go
func (r *AppRepo) IsAdmin(ctx context.Context, userID, appID uint32) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, "SELECT 1 FROM admins WHERE user_id = ? AND app_id = ?", userID, appID).Scan(&one)
	if isNotFound(err) { return false, nil }
	if err != nil { return false, fmt.Errorf("app.IsAdmin: %w", err) }
	return true, nil
}
```

- [ ] **Step 7.4: Проверка реализации интерфейса**

В конце каждого файла:
```go
// user.go:
var _ repositories.UserRepository = (*UserRepo)(nil)
// app.go:
var _ repositories.AppRepository = (*AppRepo)(nil)
// token.go:
var _ repositories.TokenRepository = (*TokenRepo)(nil)
```

Если компилятор ругнётся — сигнатуры расходятся, чинить.

- [ ] **Step 7.5: Commit**

```bash
git add internal/repositories/mariadb/
git commit -m "feat(repo/mariadb): raw SQL repositories for MariaDB"
```

---

## Task 8: Репозитории для SQLite

**Files:**
- Create: `internal/repositories/sqlite/user.go`, `app.go`, `token.go`, `errors.go`

**Отличия от MariaDB:**
- `isDuplicate` — детектится через text (`errors.Is(err, sqlite3.ErrConstraintUnique)` — нужно проверить, `modernc.org/sqlite` имеет свои error types) или проверкой substring "UNIQUE constraint failed"
- `updated_at` — SQLite не делает ON UPDATE, поэтому `UPDATE users SET ..., updated_at = CURRENT_TIMESTAMP WHERE ...` (явно)
- `is_system_admin` — INTEGER 0/1, Scan в `bool` работает через modernc драйвер, но на всякий случай используй `sql.Scan` в `int` + конверт

- [ ] **Step 8.1: errors.go**

```go
package sqlite

import (
	"database/sql"
	"errors"
	"strings"
)

func isNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// isDuplicate detects UNIQUE constraint violations. modernc/sqlite returns
// errors whose text contains "UNIQUE constraint failed". Not elegant, but
// stable across driver versions.
func isDuplicate(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

- [ ] **Step 8.2: user.go, app.go, token.go — mirror MariaDB**

Только два отличия: SQLite syntax для UPDATE (явный updated_at) и `isDuplicate` из errors.go.

Для `UpdateUser`:
```go
res, err := r.db.ExecContext(ctx,
    `UPDATE users SET email = ?, pass_hash = COALESCE(NULLIF(?, ''), pass_hash),
     steam_url = ?, path_to_photo = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
    user.Email, user.PassHash, user.SteamURL, user.PathToPhoto, user.ID,
)
```

- [ ] **Step 8.3: Compile check**

```go
var _ repositories.UserRepository = (*UserRepo)(nil)
// ...
```

- [ ] **Step 8.4: Commit**

```bash
git add internal/repositories/sqlite/
git commit -m "feat(repo/sqlite): raw SQL repositories for SQLite"
```

---

## Task 9: DI factory в app.New

**Files:** `internal/app/app.go`

- [ ] **Step 9.1: Диспатчер**

```go
import (
	"fmt"

	"sso/internal/config"
	"sso/internal/repositories"
	repomariadb "sso/internal/repositories/mariadb"
	reposqlite "sso/internal/repositories/sqlite"
	"sso/internal/storage"
)

func buildRepos(s storage.DBProvider, driver string) (
	repositories.UserRepository,
	repositories.AppRepository,
	repositories.TokenRepository,
	error,
) {
	switch driver {
	case "mariadb", "mysql":
		return repomariadb.NewUserRepo(s), repomariadb.NewAppRepo(s), repomariadb.NewTokenRepo(s), nil
	case "sqlite", "sqlite3":
		return reposqlite.NewUserRepo(s), reposqlite.NewAppRepo(s), reposqlite.NewTokenRepo(s), nil
	default:
		return nil, nil, nil, fmt.Errorf("no repository set for driver %q", driver)
	}
}
```

Использование в `app.New`:

```go
store, err := storage.New(cfg.Database)
if err != nil { panic(err) }
if err := store.Migrate(); err != nil { panic(err) }

userR, appR, tokenR, err := buildRepos(store, cfg.Database.Driver)
if err != nil { panic(err) }

// остальная DI как была
```

Старое `storage.Migrate` → теперь это метод интерфейса. Старое `mariadb.NewStorage(dsn)` → `storage.New(cfg.Database)`.

- [ ] **Step 9.2: Commit**

```bash
git add internal/app/app.go
git commit -m "refactor(app): driver-aware storage and repository factory"
```

---

## Task 10: Удаление GORM

- [ ] **Step 10.1: Удалить старые файлы**

```bash
rm internal/repositories/user.go
rm internal/repositories/app.go
rm internal/repositories/token.go
rm internal/repositories/errors.go
rm internal/storage/mariadb/mariadb.go  # если ещё существует рядом с новым storage.go
rm -rf internal/storage/mariadb/migrate.go  # если была goose-обёртка из Stage 2
```

(Интерфейсы в `internal/repositories/iface.go` **оставить** — они нужны сервисам.)

- [ ] **Step 10.2: Проверка, что GORM нигде не используется**

```bash
grep -rn "gorm" --include="*.go"
```
Expected: ничего.

```bash
grep -rn "gorm" go.mod
```
Expected: `gorm.io/*` должны уйти после tidy.

- [ ] **Step 10.3: tidy**

```bash
go mod tidy
```

Проверь `go.mod` — `gorm.io/gorm`, `gorm.io/driver/mysql`, `jinzhu/inflection`, `jinzhu/now` исчезли.

- [ ] **Step 10.4: Commit**

```bash
git add go.mod go.sum internal/
git commit -m "chore: drop GORM, old single-backend repositories removed"
```

---

## Task 11: Переезд тестового helper'а на raw SQLite (без GORM)

**Files:** `internal/testutil/sqlite.go` — переписать

**Контекст:** В Stage 0 `testutil.NewTestStorage` создавал GORM+SQLite (`glebarez/sqlite`). Теперь GORM выпилен — helper должен создавать новый `*sql.DB` через `modernc.org/sqlite` и возвращать `storage.DBProvider` для совместимости со свежими per-driver репозиториями.

- [ ] **Step 11.1: Переписать helper под raw SQLite**

Replace `internal/testutil/sqlite.go`:

```go
package testutil

import (
	"testing"

	"sso/internal/config"
	"sso/internal/storage"
	"sso/internal/storage/sqlite"
)

// NewTestStorage returns an in-memory SQLite Storage with migrations applied.
// After Stage 7 rewrite: uses raw database/sql via modernc.org/sqlite instead
// of GORM+glebarez. Test bodies calling testutil.NewTestStorage(t) don't change.
func NewTestStorage(t *testing.T) storage.DBProvider {
	t.Helper()
	s, err := sqlite.New(config.Database{Driver: "sqlite", DBName: ":memory:"})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := s.Migrate(); err != nil {
		_ = s.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
```

- [ ] **Step 11.2: Адаптировать вызовы в existing тестах**

Stage-0 тесты (`services/auth_test.go`) использовали `repositories.NewUserRepo(storage)` где `storage` был GORM-обёрткой. Теперь `storage` — `storage.DBProvider`, репо переехали в `repositories/sqlite/`. Правка:

```go
// было (Stage 0):
userR := repositories.NewUserRepo(storage)
// стало:
userR := reposqlite.NewUserRepo(storage)
```

С соответствующими импортами: `reposqlite "sso/internal/repositories/sqlite"`.

Аналогично в Stage 3 `v2_test.go` и Stage 4 тестах — замени старые импорты.

- [ ] **Step 11.3: Убрать glebarez/sqlite из go.mod**

```bash
grep -rn "glebarez" --include="*.go"
```
Expected: пусто (все использования переехали на raw sqlite).

```bash
go mod tidy
```
`github.com/glebarez/sqlite` должен уйти.

- [ ] **Step 11.4: Финальный прогон**

```bash
go test ./...
```

Expected: все тесты сервисов (из Stage 0), все v2-контроллеры (из Stage 3), authz (из Stage 4) — зелёные поверх нового SQLite backend'а.

- [ ] **Step 11.5: Commit**

```bash
git add internal/testutil/sqlite.go internal/services/ internal/transport/grpc/v2/ go.mod go.sum
git commit -m "test: migrate test helper from gorm-sqlite to raw-sqlite backend"
```

---

## Definition of Done Stage 7

- `storage.Storage` + `storage.DBProvider` интерфейсы определены
- `storage.New(cfg)` диспатчит по `cfg.Database.Driver` (mariadb/sqlite/postgres-stub)
- MariaDB и SQLite backends полностью работают
- Postgres — scaffold с `ErrNotImplemented`
- Репозитории разнесены в `internal/repositories/{mariadb,sqlite}/`
- Сервисы видят только интерфейсы — не знают о конкретной БД
- GORM нигде в коде и в `go.mod` (`grep -r gorm` = пусто)
- Модели — чистые Go struct без тегов
- Integration-тесты на in-memory SQLite покрывают Login/Register/Validate flow
- `go build ./... && go vet ./... && go test ./...` — green
- Без Docker, без внешних сервисов

**После Stage 7 весь roadmap закрыт.** Когда понадобится Postgres — реализуешь по образцу SQLite в отдельной ветке.
