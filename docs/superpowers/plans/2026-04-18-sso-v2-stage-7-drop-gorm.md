# SSO v2 Migration — Stage 7: Drop GORM

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 6 merged to main, v2-миграция завершена.

**Goal:** Заменить GORM на `database/sql` + `go-sql-driver/mysql`. Сократить зависимости, убрать reflection-слой, повысить прозрачность SQL.

**Architecture:** 
- `internal/storage/mariadb.Storage` хранит `*sql.DB` вместо `*gorm.DB`
- Репозитории пишут явный SQL через prepared statements
- Models — чистые Go struct без тегов
- Goose уже был настроен в Stage 2, он работает с `*sql.DB` — миграции не меняются
- Error handling: `errors.Is(err, sql.ErrNoRows)` вместо `gorm.ErrRecordNotFound`

**Branch:** `stage-7-drop-gorm`

---

## File Structure

**Создаётся:**
- `internal/storage/mariadb/scan.go` — helpers для `sql.Rows` → model

**Модифицируется (полная переписка):**
- `internal/storage/mariadb/mariadb.go` — `sql.Open` вместо `gorm.Open`
- `internal/storage/mariadb/migrate.go` — убрать AutoMigrate-wrapper (если остался)
- `internal/repositories/user.go` — все методы на raw SQL
- `internal/repositories/app.go` — все методы на raw SQL
- `internal/repositories/token.go` — все методы на raw SQL
- `internal/repositories/errors.go` — `sql.ErrNoRows` вместо `gorm.ErrRecordNotFound`
- `internal/models/*.go` — удалить все `gorm:` теги
- `internal/testutil/db.go` — убрать `storage.Migrate()` (AutoMigrate), использовать goose

**Удаляется:**
- `gorm.io/gorm`, `gorm.io/driver/mysql` из `go.mod`

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-7-drop-gorm
go build ./... && go test ./...
```
Expected: green. Это baseline — после стейджа тесты должны продолжать проходить.

---

## Task 2: Storage на database/sql

- [ ] **Step 2.1: mariadb.go переписан**

```go
package mariadb

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(dsn string) (*Storage, error) {
	const op = "storage.mariadb.New"

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
	return &Storage{DB: db}, nil
}

func (s *Storage) Close() error { return s.DB.Close() }
```

- [ ] **Step 2.2: migrate.go (если ещё не перепилен под sql.DB)**

После Stage 2 goose работал через `storage.DB.DB()` (gorm → sql.DB bridge). Теперь `storage.DB` **уже** `*sql.DB`:

```go
func (s *Storage) RunGooseMigrations() error {
	if email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"); email != "" {
		if _, err := s.DB.Exec("SET @bootstrap_email = ?", email); err != nil {
			return fmt.Errorf("goose bootstrap var: %w", err)
		}
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	return goose.Up(s.DB, "migrations")
}
```

Также **удалить** старую `Migrate()` (AutoMigrate-обёртку) если она ещё существует. Её использовал только `testutil` — пусть он теперь вызывает `RunGooseMigrations`.

- [ ] **Step 2.3: Сборка **проломится**** — это нормально, репо пока используют gorm. Task 3 чинит.

- [ ] **Step 2.4: Commit** — пока НЕ коммитим. Task 2 и 3 идут одним коммитом.

---

## Task 3: Repositories на raw SQL

- [ ] **Step 3.1: models без gorm-тегов**

Пример `internal/models/user.go`:

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

То же для `App`, `Admin`, `RefreshToken` — удалить все `gorm:"..."` теги.

- [ ] **Step 3.2: errors.go обновить**

```go
package repositories

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

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
```

- [ ] **Step 3.3: UserRepo на raw SQL**

```go
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/pagination"
	"sso/internal/storage/mariadb"
)

type UserRepo struct{ storage *mariadb.Storage }

func NewUserRepo(storage *mariadb.Storage) *UserRepo { return &UserRepo{storage: storage} }

const userCols = "id, email, pass_hash, steam_url, path_to_photo, is_system_admin, created_at, updated_at"

func scanUser(row interface{ Scan(...any) error }) (models.User, error) {
	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.PassHash, &u.SteamURL, &u.PathToPhoto, &u.IsSystemAdmin, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	row := r.storage.DB.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE email = ?", email)
	u, err := scanUser(row)
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetUserByID(ctx context.Context, id uint32) (models.User, error) {
	row := r.storage.DB.QueryRowContext(ctx, "SELECT "+userCols+" FROM users WHERE id = ?", id)
	u, err := scanUser(row)
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetAllUsers(ctx context.Context, pageSize int32, pageToken string) ([]models.User, string, error) {
	offset, err := pagination.Decode(pageToken)
	if err != nil {
		return nil, "", domain.ErrValidationFailed
	}
	pageSize = pagination.ClampPageSize(pageSize)

	rows, err := r.storage.DB.QueryContext(ctx,
		"SELECT "+userCols+" FROM users ORDER BY id LIMIT ? OFFSET ?",
		int(pageSize)+1, offset,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, "", err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var next string
	if int32(len(users)) > pageSize {
		users = users[:pageSize]
		next = pagination.Encode(offset + int(pageSize))
	}
	return users, next, nil
}

func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) (uint32, error) {
	res, err := r.storage.DB.ExecContext(ctx,
		`INSERT INTO users (email, pass_hash, steam_url, path_to_photo, is_system_admin, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		user.Email, user.PassHash, user.SteamURL, user.PathToPhoto, user.IsSystemAdmin,
	)
	if isDuplicate(err) {
		return 0, domain.ErrUserAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(id), nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, user models.User) error {
	res, err := r.storage.DB.ExecContext(ctx,
		`UPDATE users SET email = ?, pass_hash = COALESCE(NULLIF(?, ''), pass_hash),
		 steam_url = ?, path_to_photo = ?, updated_at = NOW() WHERE id = ?`,
		user.Email, user.PassHash, user.SteamURL, user.PathToPhoto, user.ID,
	)
	if err != nil {
		if isDuplicate(err) {
			return domain.ErrUserAlreadyExists
		}
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id uint32, passHash string) error {
	res, err := r.storage.DB.ExecContext(ctx,
		"UPDATE users SET pass_hash = ?, updated_at = NOW() WHERE id = ?",
		passHash, id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id uint32) error {
	_, err := r.storage.DB.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

func (r *UserRepo) IsSystemAdmin(ctx context.Context, id uint32) (bool, error) {
	var flag bool
	err := r.storage.DB.QueryRowContext(ctx, "SELECT is_system_admin FROM users WHERE id = ?", id).Scan(&flag)
	if isNotFound(err) {
		return false, domain.ErrUserNotFound
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("user.IsSystemAdmin: %w", err)
	}
	return flag, nil
}
```

- [ ] **Step 3.4: AppRepo — по аналогии**

Применить тот же паттерн:
- `GetAppByID`, `GetAllApps` — SELECT + scan
- `CreateApp` — INSERT, LastInsertId, duplicate detection
- `UpdateApp` — UPDATE + RowsAffected
- `DeleteApp` — DELETE
- `ChangeStatusApp` — `UPDATE apps SET is_enabled = NOT is_enabled WHERE id = ?`
- `IsAdmin(userID, appID)` — `SELECT 1 FROM admins WHERE user_id = ? AND app_id = ?` — `(true, nil)` если Scan OK, `(false, nil)` если `sql.ErrNoRows`
- `AddAdmin` — INSERT IGNORE, `RemoveAdmin` — DELETE
- `GetAllUsersForApp` — JOIN с пагинацией:

```sql
SELECT u.id, u.email, u.steam_url, u.path_to_photo, TRUE as is_admin
FROM users u JOIN admins a ON a.user_id = u.id
WHERE a.app_id = ? ORDER BY u.id LIMIT ? OFFSET ?
```

- [ ] **Step 3.5: TokenRepo — по аналогии**

- [ ] **Step 3.6: Сборка + тесты**

```bash
go build ./...
```

Если сервисы ещё передают `*ctx` или что-то унаследованное — чини. Должен быть чистый билд.

```bash
go test ./...
```

Integration-тесты должны пройти. Если `testutil.NewTestStorage` где-то ещё вызывает `storage.Migrate()` (старый AutoMigrate) — замени на `storage.RunGooseMigrations()`.

- [ ] **Step 3.7: Commit**

```bash
git add internal/
git commit -m "refactor(storage): replace GORM with database/sql across repos"
```

---

## Task 4: Удалить GORM из зависимостей

- [ ] **Step 4.1: Убрать из кода все упоминания**

```bash
grep -rn "gorm" --include="*.go"
```

Expected: пусто (или только комментарии, которые тоже надо убрать).

- [ ] **Step 4.2: go mod tidy**

```bash
go mod tidy
```

Посмотри `go.mod` — должны исчезнуть `gorm.io/gorm`, `gorm.io/driver/mysql`, `jinzhu/inflection`, `jinzhu/now`. `go-sql-driver/mysql` становится прямой зависимостью.

- [ ] **Step 4.3: Финальный прогон**

```bash
go build ./... && go vet ./... && go test ./...
```

Expected: зелёное.

- [ ] **Step 4.4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: remove gorm dependencies"
```

---

## Definition of Done Stage 7

- `go.mod` не содержит `gorm.io/*`, `jinzhu/*`
- Все тесты проходят с реальной MariaDB
- Репозитории используют явный SQL, видны все запросы
- Models — чистые Go struct без тегов
- Бенчмарк (если захочешь): `go test -bench=. ./internal/repositories/` покажет snappier запросы без reflection

**После Stage 7 весь roadmap закрыт.**
