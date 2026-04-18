# SSO v2 Migration — Stage 4: Authorization Layer

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 3 merged to main.

**Goal:** Ввести слой авторизации — парсинг access-токена из metadata, извлечение identity в ctx, проверки прав внутри сервисов.

**Architecture:** `AuthInterceptor` парсит `authorization: Bearer <token>`, валидирует через `AuthService.ValidateToken`, кладёт identity в `context.Context` через типизированный ключ. Методы без auth — в whitelist. Сервисы извлекают identity из ctx и проверяют права. Используется концепция `system-admin` — boolean флаг на `User` для глобальных операций (создание/удаление приложений).

**Branch:** `stage-4-authz`

---

## Policy assumptions (прочти ДО первой ночи)

Если хоть одно расходится с твоим видением — скажи **до** запуска этого стейджа:

| RPC | Кто может вызвать |
|-----|-------------------|
| `Auth.Login/Register/Refresh/ValidateToken` | Без авторизации (whitelist) |
| `Auth.Logout` | Любой аутентифицированный (инвалидирует свой токен) |
| `User.UserInfo(id)` | Любой аутентифицированный |
| `User.GetAllUsers` | Только system-admin |
| `User.UpdateUser(id)` | Сам (`ctx.user_id == id`) или system-admin |
| `User.ChangePassword(user_id)` | Только сам |
| `User.DeleteUser(id)` | Сам или system-admin |
| `App.GetApp/GetAllApps` | Любой аутентифицированный |
| `App.CreateApp/UpdateApp/DeleteApp/ChangeStatusApp` | Только system-admin |
| `App.IsAdmin/AddAdmin/RemoveAdmin` | Только system-admin (пока — позже можно смягчить на app-admin своего приложения) |
| `App.GetAllUsersForApp(app_id)` | System-admin или admin этого app_id |

**Концепция "system-admin":** новое поле `is_system_admin bool` на `User`. Первый system-admin сеется в миграции (email из env `BOOTSTRAP_ADMIN_EMAIL`).

---

## File Structure

**Создаётся:**
- `internal/auth/identity.go` — типизированный ctx-ключ, helper для извлечения
- `internal/auth/identity_test.go`
- `internal/transport/grpc/interceptors/authz.go` — AuthInterceptor
- `internal/transport/grpc/interceptors/authz_test.go`
- `internal/storage/mariadb/migrations/00005_system_admin.sql` — поле + bootstrap
- `internal/storage/mariadb/migrations/00006_bootstrap_admin.sql` — сеет первого system-admin

**Модифицируется:**
- `internal/models/user.go` — `IsSystemAdmin bool`
- `internal/services/{user,app,auth}.go` — методы принимают `identity.Identity` или читают из ctx, проверяют права
- `internal/transport/grpc/v2/{user,app,auth}.go` — не ловят authz-ошибки специально (errmap сам переведёт `ErrPermissionDenied`)
- `internal/app/grpc/app.go` — добавить `AuthInterceptor` в chain **после** timeout, **после** validate

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-4-authz
go build ./... && go test ./...
```
Expected: green. Иначе STOP.

---

## Task 2: identity package

**Files:** `internal/auth/identity.go`, `internal/auth/identity_test.go`

- [ ] **Step 2.1: test**

```go
package identity_test

import (
	"context"
	"testing"

	"sso/internal/auth/identity"
)

func TestWithAndFrom(t *testing.T) {
	ctx := identity.With(context.Background(), identity.Identity{UserID: 42, IsSystemAdmin: true})
	got, ok := identity.From(ctx)
	if !ok || got.UserID != 42 || !got.IsSystemAdmin {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestFromEmptyCtxReturnsFalse(t *testing.T) {
	if _, ok := identity.From(context.Background()); ok {
		t.Fatal("empty ctx should return ok=false")
	}
}
```

- [ ] **Step 2.2: impl**

```go
// Package identity carries the authenticated caller through ctx.
package identity

import "context"

type Identity struct {
	UserID        uint32
	IsSystemAdmin bool
}

type ctxKey struct{}

func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func From(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	return id, ok
}

// MustFrom panics if no identity — use only in code paths guarded by the
// auth interceptor (post-whitelist RPCs).
func MustFrom(ctx context.Context) Identity {
	id, ok := From(ctx)
	if !ok {
		panic("identity: no identity in ctx — missing auth interceptor?")
	}
	return id
}
```

- [ ] **Step 2.3: test + commit**
```bash
go test ./internal/auth/identity/ -v
git add internal/auth/
git commit -m "feat(auth): typed ctx carrier for authenticated identity"
```

---

## Task 3: is_system_admin миграция + модель

- [ ] **Step 3.1: миграция**

Create `internal/storage/mariadb/migrations/00005_system_admin.sql`:

```sql
-- +goose Up
ALTER TABLE users ADD COLUMN is_system_admin BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_users_system_admin ON users (is_system_admin);

-- +goose Down
DROP INDEX idx_users_system_admin ON users;
ALTER TABLE users DROP COLUMN is_system_admin;
```

- [ ] **Step 3.2: bootstrap-миграция (условная)**

Create `internal/storage/mariadb/migrations/00006_bootstrap_admin.sql`:

```sql
-- +goose Up
-- +goose StatementBegin
-- Promotes a pre-existing user (if any) matching email from env var
-- BOOTSTRAP_ADMIN_EMAIL to system-admin. Idempotent: if the user does not
-- exist yet (fresh deployments), this is a no-op and the next server start
-- should create the user via Register, then re-run or manual promote.
UPDATE users SET is_system_admin = TRUE
  WHERE email = (SELECT COALESCE(NULLIF(TRIM(@bootstrap_email), ''), '__no_bootstrap__'))
  LIMIT 1;
-- +goose StatementEnd

-- +goose Down
-- No-op: we don't auto-demote on rollback.
SELECT 1;
```

**Примечание для агента:** bootstrap реально прописывается через env var `BOOTSTRAP_ADMIN_EMAIL` в app.go перед goose.Up через `db.Exec("SET @bootstrap_email = ?", os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))`. Это добавляется в `storage/mariadb/migrate.go`:

```go
func (s *Storage) RunGooseMigrations() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL"); email != "" {
		if _, err := sqlDB.Exec("SET @bootstrap_email = ?", email); err != nil {
			return fmt.Errorf("goose bootstrap var: %w", err)
		}
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	return goose.Up(sqlDB, "migrations")
}
```

Импорт `"os"` добавить.

- [ ] **Step 3.3: модель**

`internal/models/user.go` — добавить поле:

```go
IsSystemAdmin bool `gorm:"not null;default:false;index"`
```

- [ ] **Step 3.4: mapper v2 — добавить флаг в UserModel**

v2 proto `UserModel` **не имеет** поля `is_system_admin` (см. v2/user.proto). Значит система знает о system-admin флаге только внутри, наружу через API не показывает. Это OK — право управления admins и без того ограничено system-admin'ом.

Никаких изменений в mapper.

- [ ] **Step 3.5: сборка + тесты + commit**

```bash
go build ./... && go test ./...
git add internal/storage/mariadb/migrations/ internal/storage/mariadb/migrate.go internal/models/user.go
git commit -m "feat(models): add is_system_admin flag with bootstrap migration"
```

---

## Task 4: AuthInterceptor

**Files:** `internal/transport/grpc/interceptors/authz.go`, `authz_test.go`

- [ ] **Step 4.1: Whitelist и интерцептор**

```go
package interceptors

import (
	"context"
	"strings"

	"sso/internal/auth/identity"
	"sso/internal/domain"
	"sso/internal/transport/grpc/errmap"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// whitelistedMethods bypass the auth interceptor entirely.
var whitelistedMethods = map[string]bool{
	// v1
	"/auth.Auth/Login":         true,
	"/auth.Auth/Register":      true,
	"/auth.Auth/Refresh":       true,
	"/auth.Auth/ValidateToken": true,
	// v2
	"/sso.auth.v2.Auth/Login":         true,
	"/sso.auth.v2.Auth/Register":      true,
	"/sso.auth.v2.Auth/Refresh":       true,
	"/sso.auth.v2.Auth/ValidateToken": true,
}

// TokenValidator abstracts AuthService for tests.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (userID uint32, valid bool, err error)
}

// SystemAdminLookup reports whether the given user is a system admin.
type SystemAdminLookup interface {
	IsSystemAdmin(ctx context.Context, userID uint32) (bool, error)
}

func AuthUnaryInterceptor(v TokenValidator, admins SystemAdminLookup) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if whitelistedMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, errmap.ToV2(domain.ErrInvalidToken)
		}
		authHeaders := md.Get("authorization")
		if len(authHeaders) == 0 {
			return nil, errmap.ToV2(domain.ErrInvalidToken)
		}
		token := strings.TrimPrefix(authHeaders[0], "Bearer ")
		if token == authHeaders[0] { // no prefix found
			return nil, errmap.ToV2(domain.ErrInvalidToken)
		}

		userID, valid, err := v.ValidateToken(ctx, token)
		if err != nil || !valid {
			return nil, errmap.ToV2(domain.ErrInvalidToken)
		}

		isSysAdmin, err := admins.IsSystemAdmin(ctx, userID)
		if err != nil {
			return nil, errmap.ToV2(err)
		}

		ctx = identity.With(ctx, identity.Identity{UserID: userID, IsSystemAdmin: isSysAdmin})
		return handler(ctx, req)
	}
}
```

- [ ] **Step 4.2: UserRepo.IsSystemAdmin**

В `internal/repositories/user.go`:

```go
func (r *UserRepo) IsSystemAdmin(ctx context.Context, id uint32) (bool, error) {
	var u models.User
	err := r.storage.DB.WithContext(ctx).Select("is_system_admin").Where("id = ?", id).First(&u).Error
	if isNotFound(err) {
		return false, domain.ErrUserNotFound
	}
	if err != nil {
		return false, err
	}
	return u.IsSystemAdmin, nil
}
```

- [ ] **Step 4.3: тест интерцептора**

Создай тест с моками `TokenValidator` и `SystemAdminLookup`. Ключевые кейсы:
- whitelisted method → handler вызван, без парсинга headers
- no metadata → Unauthenticated
- no authorization header → Unauthenticated
- no Bearer prefix → Unauthenticated
- token valid=false → Unauthenticated
- token valid → handler вызван, в ctx есть identity с правильным UserID и IsSystemAdmin

- [ ] **Step 4.4: Подключить в grpc-app**

В `internal/app/grpc/app.go` chain становится:

```go
grpc.ChainUnaryInterceptor(
	interceptors.TimeoutUnaryInterceptor(5*time.Second),
	interceptors.ValidateUnaryInterceptor(),
	interceptors.AuthUnaryInterceptor(authC.AuthS, userRepo), // new
),
```

Сигнатура `New()` принимает соответствующие зависимости (можно прокинуть `authS *services.AuthService` и `userR *repositories.UserRepo` напрямую, они реализуют интерфейсы).

- [ ] **Step 4.5: Обновить интеграционные тесты v1 и v2**

Теперь все не-whitelisted RPC требуют Bearer-токен. Тесты из Stage 3 (`v2_integration_test.go`) нужно расширить: после Login брать access_token и класть в metadata на последующих вызовах. Helper:

```go
func ctxWithToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}
```

Stage-0 auth_integration_test работает через **сервис**, не gRPC — интерцептор его не затрагивает. Эти тесты не меняются.

- [ ] **Step 4.6: Commit**
```bash
go build ./... && go test ./...
git add internal/
git commit -m "feat(grpc): auth interceptor with whitelist and system-admin lookup"
```

---

## Task 5: Authz внутри сервисов

**Принцип:** проверки прав делаются в сервисах (не контроллерах), т.к. сервисы — единственное место, где логика известна полностью. Сервис получает `identity.Identity` параметром (а не из ctx) — явная зависимость легче тестируется.

- [ ] **Step 5.1: UserService — обновить сигнатуры**

Пример `DeleteUser`:

```go
func (a *UserService) DeleteUser(ctx context.Context, caller identity.Identity, userID uint32) error {
	if !caller.IsSystemAdmin && caller.UserID != userID {
		return domain.ErrPermissionDenied
	}
	if err := a.userR.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("auth.DeleteUser: %w", err)
	}
	return nil
}
```

По аналогии:
- `UpdateUser(ctx, caller, model)` — self или sysadmin (model.ID == caller.UserID)
- `GetAllUsers(ctx, caller, pageSize, pageToken)` — только sysadmin → `ErrPermissionDenied`
- `UserInfo(ctx, caller, id)` — любой аутентифицированный, проверка просто `caller.UserID != 0` (интерцептор гарантирует, но защищаемся)

`AuthService.ChangePassword(ctx, caller, userID, old, new)` — только self.

`AppService`:
- `CreateApp/UpdateApp/DeleteApp/ChangeStatusApp/AddAdmin/RemoveAdmin` — sysadmin-only
- `GetApp/GetAllApps` — любой auth
- `IsAdmin(ctx, userID, appID)` — любой auth (не раскрывает больше, чем сам токен)
- `GetAllUsersForApp(ctx, caller, appID, ...)` — sysadmin ИЛИ `isAdminOfApp(caller.UserID, appID)`. Для этого в `AppRepo` уже есть `IsAdmin` — переиспользуем.

- [ ] **Step 5.2: Контроллеры v2 извлекают identity из ctx, прокидывают в сервис**

Пример `internal/transport/grpc/v2/user.go DeleteUser`:

```go
func (c *UserController) DeleteUser(ctx context.Context, req *ssov2.DeleteUserRequest) (*emptypb.Empty, error) {
	caller, _ := identity.From(ctx) // интерцептор гарантирует, что есть
	if err := c.svc.DeleteUser(ctx, caller, req.GetId()); err != nil {
		return nil, errmap.ToV2(err)
	}
	return &emptypb.Empty{}, nil
}
```

Применить ко всем v2-методам кроме whitelisted.

- [ ] **Step 5.3: v1-контроллеры — DANGER**

v1-контроллеры тоже теперь идут через AuthInterceptor (кроме whitelist). Но v1-сервисные сигнатуры не ожидают `identity.Identity`.

**Решение:** v1-контроллеры извлекают identity из ctx и **так же** прокидывают в обновлённые сервисные методы. Т.е. сигнатуры сервисов становятся идентичными для v1 и v2. Альтернатива — дублировать сервисы — хуже.

Обнови вызовы в `internal/controllers/{user,app,auth}.go` аналогично v2-контроллерам.

- [ ] **Step 5.4: Тесты**

Обнови Stage-0/Stage-3 integration-тесты: передавай valid Bearer-token через metadata, заведи тестового sysadmin через прямое `UPDATE users SET is_system_admin = TRUE WHERE id = ?` в БД тестового контейнера (helper в `testutil`).

Добавь тесты на ErrPermissionDenied:
- `UpdateUser` чужого id обычным юзером → PERMISSION_DENIED
- `GetAllUsers` обычным юзером → PERMISSION_DENIED
- `CreateApp` обычным юзером → PERMISSION_DENIED
- Sysadmin выполняет всё выше → OK

- [ ] **Step 5.5: Commit**
```bash
go build ./... && go test ./...
git add internal/
git commit -m "feat(authz): enforce per-RPC permissions in services"
```

---

## Definition of Done Stage 4

- Все тесты зелёные
- Без Bearer-токена не-whitelisted RPC возвращают UNAUTHENTICATED + `INVALID_TOKEN` reason
- Попытка обычного юзера удалить чужого → PERMISSION_DENIED + `PERMISSION_DENIED` reason
- Sysadmin может всё
- `BOOTSTRAP_ADMIN_EMAIL` env var промотирует первого пользователя в sysadmin при старте
- v1 и v2 используют один и тот же authz-слой в сервисах
