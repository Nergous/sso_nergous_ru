# SSO v2 Migration — Stage 2: Data Layer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 1 merged to main.

**Goal:** Подготовить модели и репозитории к v2 API: timestamps на `User`, fix composite key на `Admin`, индексы на FK, пагинация в List-методах. Ввести явные миграции через `goose`.

**Architecture:** AutoMigrate остаётся параллельно с goose-миграциями в dev-конфиге (для testcontainers). В prod — только goose. Пагинация — простая offset-based с курсором в виде base64(offset:anchor_id) для будущего перехода на keyset. **Интерфейс репозитория** меняется — нужна обратная совместимость со Stage 1 сервисами (v1-контроллеры пока не знают про пагинацию, вызывают с `page_size=0, page_token=""` → возвращается весь список до 1000 записей).

**Tech Stack:** `github.com/pressly/goose/v3`, GORM.

**Branch:** `stage-2-data-layer`

---

## File Structure

**Создаётся:**
- `migrations/00001_init.sql` — baseline (эквивалент текущего AutoMigrate)
- `migrations/00002_user_timestamps.sql` — `created_at/updated_at` на `users`
- `migrations/00003_admin_composite_key.sql` — пересоздание `admins` с `(user_id,app_id)` unique
- `migrations/00004_refresh_token_indexes.sql` — индексы на FK
- `internal/storage/mariadb/migrate.go` — обёртка над `goose.Up`
- `internal/pagination/cursor.go` — encode/decode opaque page_token
- `internal/pagination/cursor_test.go`

**Модифицируется:**
- `internal/models/user.go` — добавить `CreatedAt`, `UpdatedAt`
- `internal/models/admin.go` — composite unique index через tag
- `internal/models/refreshToken.go` — индексы на `user_id`, `app_id`
- `internal/models/app.go` — `IsEnabled` default `true`
- `internal/repositories/{user,app}.go` — пагинация в `GetAll*`, `GetAllUsersForApp`
- `internal/services/{user,app}.go` — пробрасывать page_size/page_token
- `internal/controllers/{user,app}.go` — v1: игнорируют пагинацию, передают `0, ""`
- `internal/app/app.go` — запустить `goose.Up` вместо/до `AutoMigrate`

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-2-data-layer
go build ./... && go test ./...
```
Expected: green. Если нет — STOP.

---

## Task 2: Pagination cursor

**Files:**
- Create: `internal/pagination/cursor.go`
- Create: `internal/pagination/cursor_test.go`

- [ ] **Step 2.1: Failing test**

```go
package pagination_test

import (
	"testing"

	"sso/internal/pagination"
)

func TestCursor_RoundTrip(t *testing.T) {
	tok := pagination.Encode(42)
	if tok == "" {
		t.Fatal("empty token")
	}
	offset, err := pagination.Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 42 {
		t.Fatalf("want 42, got %d", offset)
	}
}

func TestCursor_EmptyTokenMeansZeroOffset(t *testing.T) {
	offset, err := pagination.Decode("")
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Fatalf("want 0, got %d", offset)
	}
}

func TestCursor_GarbageTokenErrors(t *testing.T) {
	if _, err := pagination.Decode("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error")
	}
}

func TestClampPageSize(t *testing.T) {
	cases := []struct{ in, want int32 }{
		{0, 50}, {-1, 50}, {10, 10}, {1000, 1000}, {5000, 1000},
	}
	for _, c := range cases {
		if got := pagination.ClampPageSize(c.in); got != c.want {
			t.Fatalf("ClampPageSize(%d)=%d want %d", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2.2: Implementation**

```go
// Package pagination provides an opaque offset-based cursor for List RPCs.
// Callers treat Encode/Decode tokens as opaque strings per AIP-158.
package pagination

import (
	"encoding/base64"
	"errors"
	"strconv"
)

const (
	DefaultPageSize int32 = 50
	MaxPageSize     int32 = 1000
)

func ClampPageSize(n int32) int32 {
	if n <= 0 {
		return DefaultPageSize
	}
	if n > MaxPageSize {
		return MaxPageSize
	}
	return n
}

func Encode(offset int) string {
	return base64.URLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func Decode(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New("negative offset")
	}
	return n, nil
}
```

- [ ] **Step 2.3: Test + commit**
```bash
go test ./internal/pagination/ -v
git add internal/pagination/
git commit -m "feat(pagination): opaque offset-based cursor"
```

---

## Task 3: Модели — timestamps, composite key, default

- [ ] **Step 3.1: models/user.go — добавить timestamps**

```go
package models

import "time"

type User struct {
	ID          uint32    `gorm:"primaryKey"`
	Email       string    `gorm:"uniqueIndex;not null"`
	PassHash    string    `gorm:"not null"`
	SteamURL    string
	PathToPhoto string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
```

- [ ] **Step 3.2: models/admin.go — composite unique**

```go
package models

type Admin struct {
	ID     uint32 `gorm:"primaryKey"`
	UserID uint32 `gorm:"uniqueIndex:idx_admin_user_app,priority:1;index"`
	AppID  uint32 `gorm:"uniqueIndex:idx_admin_user_app,priority:2;index"`
}
```

- [ ] **Step 3.3: models/refreshToken.go — индексы на FK**

В существующих тегах `RefreshToken.UserID` и `AppID` добавить `;index` к gorm-тегу. FK-ограничения оставить как есть.

- [ ] **Step 3.4: models/app.go — IsEnabled default true**

Изменить tag на поле `IsEnabled` с `default:0` (или его эквивалент) на `default:true`.

- [ ] **Step 3.5: Сборка + тесты**
```bash
go build ./... && go test ./...
```
Тестcontainer пересоздаст схему через AutoMigrate — всё должно пройти.

- [ ] **Step 3.6: Commit**
```bash
git add internal/models/
git commit -m "feat(models): timestamps on User, composite key on Admin, FK indexes"
```

---

## Task 4: Репозитории — pagination + UPDATE через Updates

- [ ] **Step 4.1: user.go GetAllUsers с пагинацией**

Изменить сигнатуру:

```go
func (r *UserRepo) GetAllUsers(ctx context.Context, pageSize int32, pageToken string) ([]models.User, string, error) {
	offset, err := pagination.Decode(pageToken)
	if err != nil {
		return nil, "", domain.ErrValidationFailed
	}
	pageSize = pagination.ClampPageSize(pageSize)

	var users []models.User
	err = r.storage.DB.WithContext(ctx).
		Order("id").
		Offset(offset).
		Limit(int(pageSize) + 1).
		Find(&users).Error
	if err != nil {
		return nil, "", err
	}
	var nextToken string
	if int32(len(users)) > pageSize {
		users = users[:pageSize]
		nextToken = pagination.Encode(offset + int(pageSize))
	}
	return users, nextToken, nil
}
```

Импортировать `"sso/internal/pagination"`, `"sso/internal/domain"`.

- [ ] **Step 4.2: app.go GetAllApps — тот же паттерн**

- [ ] **Step 4.3: app.go GetAllUsersForApp — пагинация + правильный JOIN**

Текущая реализация JOIN только admins. Нужно: все пользователи приложения (те, у кого есть refresh_token для app_id ИЛИ кто админ этого app_id). Компромисс — для этого этапа оставить семантику "только админы" (как сейчас), но с пагинацией. Семантика "все пользователи app" — отдельная задача, не блокирует миграцию.

Добавить pageSize/pageToken параметры, возвращать `([]AppUserRow, string, error)`.

```go
type AppUserRow struct {
	ID          uint32
	Email       string
	SteamURL    string
	PathToPhoto string
	IsAdmin     bool
}

func (r *AppRepo) GetAllUsersForApp(ctx context.Context, appID uint32, pageSize int32, pageToken string) ([]AppUserRow, string, error) {
	offset, err := pagination.Decode(pageToken)
	if err != nil {
		return nil, "", domain.ErrValidationFailed
	}
	pageSize = pagination.ClampPageSize(pageSize)

	var rows []AppUserRow
	err = r.storage.DB.WithContext(ctx).
		Table("users u").
		Select("u.id, u.email, u.steam_url, u.path_to_photo, TRUE as is_admin").
		Joins("JOIN admins a ON a.user_id = u.id").
		Where("a.app_id = ?", appID).
		Order("u.id").
		Offset(offset).
		Limit(int(pageSize) + 1).
		Scan(&rows).Error
	if err != nil {
		return nil, "", err
	}
	var next string
	if int32(len(rows)) > pageSize {
		rows = rows[:pageSize]
		next = pagination.Encode(offset + int(pageSize))
	}
	return rows, next, nil
}
```

- [ ] **Step 4.4: app.go UpdateApp через Updates**

Переписать аналогично Stage 1 Task 4.2 `UpdateUser` (через `.Model(...).Where("id=?").Updates(map[...])` с RowsAffected проверкой).

- [ ] **Step 4.5: Сборка**
```bash
go build ./...
```

Компиляция провалится — вызовы `GetAllUsers` и др. в сервисах не ожидают новые аргументы. Это нормально, фиксим в Task 5.

- [ ] **Step 4.6: Не коммитим пока** — Task 4 и 5 едут одним коммитом (иначе main красный между ними).

---

## Task 5: Сервисы пробрасывают pagination

- [ ] **Step 5.1: services/user.go GetAllUsers**

```go
func (a *UserService) GetAllUsers(ctx context.Context, pageSize int32, pageToken string) ([]models.User, string, error) {
	users, next, err := a.userR.GetAllUsers(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("auth.GetUsers: %w", err)
	}
	return users, next, nil
}
```

- [ ] **Step 5.2: services/app.go GetAllApps, GetAllUsersForApp — аналогично**

- [ ] **Step 5.3: Контроллеры v1 — передают 0, ""**

В `internal/controllers/user.go GetAllUsers`:

```go
users, _, err := c.UserS.GetAllUsers(ctx, 0, "")
if err != nil { return nil, errmap.ToV1(err) }
```

(next_page_token в v1 ответе нет — игнорируем.)

То же для `GetAllApps`, `GetAllUsersForApp` в `internal/controllers/app.go`.

- [ ] **Step 5.4: Сборка + тесты**
```bash
go build ./... && go test ./...
```
Expected: PASS.

- [ ] **Step 5.5: Commit (Task 4 + 5 вместе)**

```bash
git add internal/repositories/ internal/services/ internal/controllers/
git commit -m "feat(data): paginate list operations end-to-end"
```

---

## Task 6: Goose migrations

- [ ] **Step 6.1: Зависимость**

```bash
go get github.com/pressly/goose/v3@latest
go mod tidy
```

- [ ] **Step 6.2: migrations/00001_init.sql**

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL,
    pass_hash VARCHAR(255) NOT NULL,
    steam_url VARCHAR(255),
    path_to_photo VARCHAR(2048),
    UNIQUE KEY idx_users_email (email)
);

CREATE TABLE IF NOT EXISTS apps (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    secret VARCHAR(255) NOT NULL,
    link VARCHAR(2048) NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME,
    updated_at DATETIME,
    UNIQUE KEY idx_apps_name (name)
);

CREATE TABLE IF NOT EXISTS admins (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    app_id BIGINT UNSIGNED NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    token VARCHAR(255) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    app_id BIGINT UNSIGNED NOT NULL,
    expires_at DATETIME NOT NULL,
    UNIQUE KEY idx_refresh_tokens_token (token),
    CONSTRAINT fk_refresh_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_app FOREIGN KEY (app_id) REFERENCES apps(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 6.3: migrations/00002_user_timestamps.sql**

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN created_at DATETIME, ADD COLUMN updated_at DATETIME;

-- +goose Down
ALTER TABLE users DROP COLUMN updated_at, DROP COLUMN created_at;
```

- [ ] **Step 6.4: migrations/00003_admin_composite_key.sql**

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

- [ ] **Step 6.5: migrations/00004_refresh_token_indexes.sql**

```sql
-- +goose Up
CREATE INDEX idx_refresh_user ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_app ON refresh_tokens (app_id);

-- +goose Down
DROP INDEX idx_refresh_app ON refresh_tokens;
DROP INDEX idx_refresh_user ON refresh_tokens;
```

- [ ] **Step 6.6: Обёртка в storage**

Create `internal/storage/mariadb/migrate.go`:

```go
package mariadb

import (
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunGooseMigrations applies all pending SQL migrations from the embedded FS.
func (s *Storage) RunGooseMigrations() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	return goose.Up(sqlDB, "migrations")
}
```

- [ ] **Step 6.7: Переместить миграции в embed-путь**

Migrations должны лежать под `internal/storage/mariadb/migrations/`, чтобы `embed` нашёл. Перенести:
```bash
mkdir -p internal/storage/mariadb/migrations
git mv migrations/*.sql internal/storage/mariadb/migrations/
rmdir migrations
```

- [ ] **Step 6.8: Запускать goose в app.New**

Modify `internal/app/app.go` — заменить:
```go
if err := storage.Migrate(); err != nil {
	panic(err)
}
```
на:
```go
if err := storage.RunGooseMigrations(); err != nil {
	panic(err)
}
```

(`storage.Migrate()` через AutoMigrate **оставить** определённым — тесты testutil.NewTestStorage продолжат использовать его для свежих контейнеров.)

- [ ] **Step 6.9: Сборка + тесты**
```bash
go build ./... && go test ./...
```
Expected: PASS. (testutil использует AutoMigrate — goose не прогонится в тестах, это OK.)

- [ ] **Step 6.10: Commit**

```bash
git add go.mod go.sum internal/storage/mariadb/migrate.go internal/storage/mariadb/migrations/ internal/app/app.go
git commit -m "feat(storage): run SQL migrations via goose at startup"
```

---

## Definition of Done Stage 2

- Все тесты зелёные
- `User` имеет `CreatedAt`/`UpdatedAt`
- `Admin` — composite unique + per-column индексы
- `RefreshToken` — индексы на `user_id`, `app_id`
- `App.IsEnabled` default `true`
- `GetAllUsers`, `GetAllApps`, `GetAllUsersForApp` — с пагинацией (page_size clamp, opaque page_token)
- v1 API отвечает как раньше (пагинация прозрачна)
- `goose up` накатывает 4 миграции в embed-FS
- `repositories.UpdateUser`/`UpdateApp` — single-query UPDATE с RowsAffected-проверкой

Дальше — Stage 3 (v2 controllers, dual-serve).
