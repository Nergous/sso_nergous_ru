# SSO v2 Migration — Stage 0: Stabilization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Починить блокирующие баги, убрать антипаттерны, завести integration-тесты сервисного слоя на in-memory SQLite — подготовить кодовую базу к безопасной миграции на v2.

**Architecture:** Работаем на ветке `stage-0-stabilization`. Каждая задача — отдельный коммит. Никаких изменений API v1 — только internal refactor + тесты. После завершения Stage 0 — `go build ./... && go test ./...` зелёные, поведение v1 RPC не изменилось. **Тесты НЕ требуют Docker** — in-memory SQLite через pure-Go драйвер (`glebarez/sqlite` поверх `modernc.org/sqlite`). Реальный SQL, никаких контейнеров.

**Tech Stack:** Go 1.24, testify, grpc-go, GORM, `github.com/glebarez/sqlite` (CGO-free SQLite для GORM), slog.

---

## Scope

**В этом Stage:**
- Fix 5 конкретных багов (Refresh expired, DSN log, graceful shutdown, context pointer, hardcoded timeout)
- Integration-тесты для AuthService (happy path + ключевые ошибки)
- Удалить неиспользуемые `fmt.Println`

**НЕ в этом Stage:**
- Доменные ошибки (Stage 1)
- Удаление `lib/serr` (Stage 1)
- Пагинация (Stage 2)
- v2-контроллеры (Stage 3)

---

## File Structure

**Создаётся:**
- `internal/transport/grpc/interceptors/timeout.go` — общий timeout-interceptor
- `internal/transport/grpc/interceptors/timeout_test.go` — unit-тест на interceptor
- `internal/repositories/iface.go` — интерфейсы UserRepository/AppRepository/TokenRepository (нужны Stage 7)
- `internal/testutil/sqlite.go` — helper создаёт GORM+SQLite in-memory Storage с AutoMigrate
- `internal/testutil/logger.go` — silent slog для тестов
- `internal/services/auth_test.go` — integration-тесты сервиса через реальные репо + SQLite

**Модифицируется (mechanical refactor `*context.Context` → `context.Context`):**
- `internal/services/{auth,user,app}.go`
- `internal/repositories/{user,app,token}.go`
- `internal/controllers/{auth,user,app}.go` (вызовы в сервисы)
- `lib/serr/serr.go` (если ломается сигнатура — не ломается, не трогаем)

**Точечно:**
- `internal/services/auth.go:254` — fix Refresh expired bug
- `internal/config/config.go:83` — убрать `log.Print(dsn)`
- `cmd/sso/main.go` — добавить `storage.Close()`
- `internal/app/app.go` — вернуть `storage` наружу (или хранить в App)
- `internal/app/grpc/app.go` — подключить timeout interceptor
- `internal/repositories/user.go:57` — убрать `fmt.Println`

---

## Task 1: Создать ветку и проверить baseline

- [ ] **Step 1.1: Создать ветку**

```bash
git checkout -b stage-0-stabilization
```

- [ ] **Step 1.2: Зафиксировать baseline компиляции**

```bash
go build ./...
```

Expected: exit 0, без вывода.

- [ ] **Step 1.3: Зафиксировать baseline тестов**

```bash
go test ./...
```

Expected: `?   <pkg>   [no test files]` для всех пакетов, exit 0.

---

## Task 2: Fix — `Refresh` продолжает выдавать токены по истёкшему refresh-токену

**Files:**
- Modify: `internal/services/auth.go:254-261`

**Контекст бага:** В `Refresh` есть проверка `if time.Now().After(rTkn.ExpiresAt)`, внутри — `DeleteRefreshToken`, но после блока код продолжается, как будто токен валиден, и выдаёт новую пару. Нужно вернуть ошибку сразу.

- [ ] **Step 2.1: Добавить sentinel-ошибку**

Modify `internal/services/auth.go` — в блок `var ( ... )` (строка 20):

```go
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInternal           = errors.New("internal error")
	ErrTokenExpired       = errors.New("refresh token expired")
)
```

- [ ] **Step 2.2: Исправить логику Refresh**

Modify `internal/services/auth.go:254-261` — заменить:

```go
	if time.Now().After(rTkn.ExpiresAt) {
		err = a.tokenR.DeleteRefreshToken(ctx, refreshToken)

		ok, err := serr.Gerr(op, "refresh token not found", "failed to delete refresh token", a.log, err)
		if !ok {
			return "", "", err
		}
	}
```

на:

```go
	if time.Now().After(rTkn.ExpiresAt) {
		if err := a.tokenR.DeleteRefreshToken(ctx, refreshToken); err != nil {
			a.log.Warn("failed to delete expired refresh token", slog.String("op", op), slog.Any("err", err))
		}
		return "", "", ErrTokenExpired
	}
```

- [ ] **Step 2.3: Убедиться, что всё компилируется**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 2.4: Commit**

```bash
git add internal/services/auth.go
git commit -m "fix(auth): return error when refreshing with expired token"
```

---

## Task 3: Fix — убрать `log.Print(dsn)` с credentials

**Files:**
- Modify: `internal/config/config.go:83`

- [ ] **Step 3.1: Удалить строку**

Edit `internal/config/config.go:73-86`, заменить функцию `GetDSN`:

```go
func (cfg *Database) GetDSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.UsernameDB,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)
}
```

Также убрать неиспользуемый импорт `"log"` из шапки файла.

- [ ] **Step 3.2: Проверить сборку**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 3.3: Commit**

```bash
git add internal/config/config.go
git commit -m "fix(config): stop logging DSN with credentials"
```

---

## Task 4: Убрать `fmt.Println(rows.Error)` из UserRepo

**Files:**
- Modify: `internal/repositories/user.go:55-59`

- [ ] **Step 4.1: Удалить println**

Modify `internal/repositories/user.go`, заменить:

```go
func (r *UserRepo) CreateUser(
	ctx *context.Context,
	user *models.User,
) (uint32, error) {
	rows := r.storage.DB.WithContext(*ctx).Create(&user)
	if rows.Error != nil {
		fmt.Println(rows.Error)
		return 0, rows.Error
	}

	return user.ID, nil
}
```

на:

```go
func (r *UserRepo) CreateUser(
	ctx *context.Context,
	user *models.User,
) (uint32, error) {
	if err := r.storage.DB.WithContext(*ctx).Create(&user).Error; err != nil {
		return 0, err
	}
	return user.ID, nil
}
```

Убрать импорт `"fmt"` если он больше нигде не используется.

- [ ] **Step 4.2: Проверить сборку**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 4.3: Commit**

```bash
git add internal/repositories/user.go
git commit -m "chore(repo): remove stray fmt.Println from CreateUser"
```

---

## Task 5: Refactor — `*context.Context` → `context.Context` во всём коде

**Files (все одновременно):**
- Modify: `internal/services/auth.go` — все методы `Login`, `Logout`, `RegisterNewUser`, `Refresh`, `ValidateToken`
- Modify: `internal/services/user.go` — все методы
- Modify: `internal/services/app.go` — все методы
- Modify: `internal/repositories/user.go` — все методы
- Modify: `internal/repositories/app.go` — все методы
- Modify: `internal/repositories/token.go` — все методы
- Modify: `internal/controllers/auth.go` — все места `c.AuthS.X(&ctx, ...)` заменить на `c.AuthS.X(ctx, ...)`
- Modify: `internal/controllers/user.go` — аналогично
- Modify: `internal/controllers/app.go` — аналогично

**Контекст:** передача `*context.Context` — антипаттерн. `context.Context` уже интерфейс, копирование указателя не экономит ничего, ломает совместимость. Это чисто механический рефакторинг.

- [ ] **Step 5.1: Заменить сигнатуры в репозиториях**

В каждом методе `internal/repositories/{user,app,token}.go`:
- Заменить параметр `ctx *context.Context` на `ctx context.Context`
- Заменить `r.storage.DB.WithContext(*ctx)` на `r.storage.DB.WithContext(ctx)` (без разыменования)

Пример — `internal/repositories/user.go`, каждая функция:

```go
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var user models.User
	if err := r.storage.DB.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}
```

Проделать для всех 5 методов `user.go`, всех методов `app.go` и `token.go`. **Паттерн замены идентичный** в каждом: `*context.Context` → `context.Context`, `*ctx` → `ctx`.

- [ ] **Step 5.2: Заменить сигнатуры в сервисах**

В `internal/services/{auth,user,app}.go`:
- Параметр `ctx *context.Context` → `ctx context.Context`
- Все вызовы в репо: `a.userR.GetUserByID(ctx, ...)` (было `a.userR.GetUserByID(ctx, ...)` — тут передавался указатель, теперь просто ctx)

Конкретно: любое `func (a *XService) Method(ctx *context.Context, ...)` → `func (a *XService) Method(ctx context.Context, ...)`. Внутри тела — нигде не должно остаться `*ctx` или `&ctx`.

- [ ] **Step 5.3: Заменить вызовы в контроллерах**

В `internal/controllers/{auth,user,app}.go` все вызовы вида `c.AuthS.Login(&ctx, email, ...)` заменить на `c.AuthS.Login(ctx, email, ...)`.

- [ ] **Step 5.4: Проверить сборку**

```bash
go build ./...
```

Expected: exit 0. Если есть ошибки типа "cannot use &ctx" — значит где-то осталось `&ctx`. Если "cannot use ctx (type context.Context) as type *context.Context" — осталась старая сигнатура. Чинить до зелёной сборки.

- [ ] **Step 5.5: Запустить go vet**

```bash
go vet ./...
```

Expected: exit 0, без предупреждений.

- [ ] **Step 5.6: Commit**

```bash
git add internal/
git commit -m "refactor: pass context.Context by value, not by pointer"
```

---

## Task 6: Timeout interceptor (вместо 18 дублирований)

**Files:**
- Create: `internal/transport/grpc/interceptors/timeout.go`
- Create: `internal/transport/grpc/interceptors/timeout_test.go`
- Modify: `internal/app/grpc/app.go` — подключить interceptor
- Modify: `internal/controllers/{auth,user,app}.go` — удалить `ctx, cancel := context.WithTimeout(...)` из каждого RPC-метода (18 мест)

- [ ] **Step 6.1: Создать директорию и написать failing test**

```bash
mkdir -p internal/transport/grpc/interceptors
```

Create `internal/transport/grpc/interceptors/timeout_test.go`:

```go
package interceptors

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
)

func TestTimeoutInterceptor_AppliesDeadline(t *testing.T) {
	interceptor := TimeoutUnaryInterceptor(50 * time.Millisecond)

	handler := func(ctx context.Context, req any) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			return nil, errors.New("expected deadline on ctx")
		}
		if until := time.Until(deadline); until <= 0 || until > 60*time.Millisecond {
			return nil, errors.New("unexpected deadline")
		}
		return "ok", nil
	}

	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/X/Y"}, handler)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp != "ok" {
		t.Fatalf("unexpected resp: %v", resp)
	}
}

func TestTimeoutInterceptor_RespectsExistingShorterDeadline(t *testing.T) {
	interceptor := TimeoutUnaryInterceptor(1 * time.Second)

	parent, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	handler := func(ctx context.Context, req any) (any, error) {
		deadline, _ := ctx.Deadline()
		if until := time.Until(deadline); until > 15*time.Millisecond {
			return nil, errors.New("deadline got extended beyond parent")
		}
		return nil, nil
	}

	if _, err := interceptor(parent, nil, &grpc.UnaryServerInfo{FullMethod: "/X/Y"}, handler); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
```

- [ ] **Step 6.2: Запустить тест — должен упасть компиляцией**

```bash
go test ./internal/transport/grpc/interceptors/
```

Expected: FAIL — `undefined: TimeoutUnaryInterceptor`.

- [ ] **Step 6.3: Написать реализацию**

Create `internal/transport/grpc/interceptors/timeout.go`:

```go
package interceptors

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// TimeoutUnaryInterceptor applies a default timeout to every unary RPC.
// If the caller already set a shorter deadline, that deadline is preserved
// because context.WithTimeout only shortens — never extends — deadlines.
func TimeoutUnaryInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(ctx, req)
	}
}
```

- [ ] **Step 6.4: Запустить тест — должен пройти**

```bash
go test ./internal/transport/grpc/interceptors/ -v
```

Expected: PASS (оба теста).

- [ ] **Step 6.5: Подключить interceptor в grpc-app**

Modify `internal/app/grpc/app.go`:

```go
package grpcapp

import (
	"fmt"
	"log/slog"
	"net"
	"time"

	"sso/internal/controllers"
	"sso/internal/transport/grpc/interceptors"

	"google.golang.org/grpc"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	port       int
}

func New(
	log *slog.Logger,
	port int,
	authController *controllers.AuthController,
	userController *controllers.UserController,
	appController *controllers.AppController,
) *App {
	gRPCServer := grpc.NewServer(
		grpc.UnaryInterceptor(interceptors.TimeoutUnaryInterceptor(5 * time.Second)),
	)

	controllers.RegisterAuth(gRPCServer, authController.AuthS)
	controllers.RegisterApp(gRPCServer, appController.AppS, appController.DefaultSecret)
	controllers.RegisterUser(gRPCServer, userController.UserS)

	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		port:       port,
	}
}
```

(остальные методы `MustRun`, `Run`, `Stop` не трогаем)

- [ ] **Step 6.6: Удалить дублирующий WithTimeout из auth.go контроллера**

В `internal/controllers/auth.go` в каждом из 5 методов (`Login`, `Logout`, `Refresh`, `Register`, `ValidateToken`) удалить:

```go
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
```

После удаления импорт `"time"` может стать неиспользуемым — убрать его.

- [ ] **Step 6.7: Удалить дублирующий WithTimeout из user.go контроллера**

В `internal/controllers/user.go` из каждого из 4 методов (`UserInfo`, `GetAllUsers`, `UpdateUser`, `DeleteUser`) удалить ту же пару строк + убрать импорт `"time"` если не нужен.

- [ ] **Step 6.8: Удалить дублирующий WithTimeout из app.go контроллера**

В `internal/controllers/app.go` из каждого из 9 методов — тот же паттерн.

- [ ] **Step 6.9: Проверить сборку и тесты**

```bash
go build ./... && go test ./...
```

Expected: exit 0.

- [ ] **Step 6.10: Commit**

```bash
git add internal/
git commit -m "refactor(grpc): extract timeout into unary interceptor"
```

---

## Task 7: Graceful shutdown для БД

**Files:**
- Modify: `internal/app/app.go` — expose storage
- Modify: `cmd/sso/main.go` — вызвать `storage.Close()` после `Stop`

- [ ] **Step 7.1: Хранить storage в App**

Modify `internal/app/app.go`:

```go
package app

import (
	"log/slog"
	"time"

	grpcapp "sso/internal/app/grpc"
	"sso/internal/controllers"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/storage/mariadb"
)

type App struct {
	GRPCServer *grpcapp.App
	Storage    *mariadb.Storage
}

func New(
	log *slog.Logger,
	grpcPort int,
	dsn string,
	tokenTTL time.Duration,
	refreshTTL time.Duration,
	defaultSecret string,
) *App {
	storage, err := mariadb.NewStorage(dsn)
	if err != nil {
		panic(err)
	}

	if err := storage.Migrate(); err != nil {
		panic(err)
	}

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	userS := services.NewUserService(log, userR)
	appS := services.NewAppService(log, appR)
	authS := services.NewAuthService(log, storage, tokenTTL, refreshTTL, userR, appR, tokenR)

	userC := controllers.NewUserController(userS)
	appC := controllers.NewAppController(appS, defaultSecret)
	authC := controllers.NewAuthController(authS)

	grpcApp := grpcapp.New(log, grpcPort, authC, userC, appC)
	return &App{
		GRPCServer: grpcApp,
		Storage:    storage,
	}
}
```

- [ ] **Step 7.2: Вызвать Close в main**

Modify `cmd/sso/main.go`, заменить блок после `<-stop`:

```go
	<-stop

	application.GRPCServer.Stop()
	if err := application.Storage.Close(); err != nil {
		log.Error("failed to close storage", slog.Any("err", err))
	}
	log.Info("app stopped")
```

- [ ] **Step 7.3: Проверить сборку**

```bash
go build ./...
```

Expected: exit 0.

- [ ] **Step 7.4: Commit**

```bash
git add internal/app/app.go cmd/sso/main.go
git commit -m "feat(shutdown): close storage on graceful shutdown"
```

---

## Task 8: Integration-тесты AuthService через in-memory SQLite

**Files:**
- Create: `internal/repositories/iface.go` — интерфейсы репо (нужны Stage 7 для multi-backend)
- Create: `internal/testutil/sqlite.go` — helper для in-memory Storage
- Create: `internal/testutil/logger.go`
- Create: `internal/services/auth_test.go`
- Modify: `internal/services/auth.go` и `user.go`, `app.go` — принимать интерфейсы вместо `*UserRepo` / `*AppRepo` / `*TokenRepo`

**Стратегия:** in-memory SQLite через `github.com/glebarez/sqlite` (pure-Go SQLite-драйвер для GORM, без CGO). Тесты работают с реальными репозиториями поверх реальной БД — ловят баги всей цепочки от сервиса до SQL. Никакого Docker, никаких контейнеров. Каждый тест получает свежий `:memory:` инстанс через `GORM.Open(sqlite.Open(":memory:"))`.

В Stage 7 этот helper переедет с `glebarez/sqlite` (GORM) на `modernc.org/sqlite` (raw database/sql), но **тесты сервисов не изменятся** — они используют только interface абстракции.

- [ ] **Step 8.1: Определить интерфейсы репо**

Create `internal/repositories/iface.go`:

```go
package repositories

import (
	"context"

	"sso/internal/models"
)

// UserRepository — абстракция над хранением пользователей.
// Конкретная реализация — *UserRepo (GORM). Тесты подставляют fakes.
type UserRepository interface {
	GetUserByEmail(ctx context.Context, email string) (models.User, error)
	GetUserByID(ctx context.Context, id uint32) (models.User, error)
	GetAllUsers(ctx context.Context) ([]models.User, error)
	CreateUser(ctx context.Context, user *models.User) (uint32, error)
	UpdateUser(ctx context.Context, user models.User) error
	DeleteUser(ctx context.Context, id uint32) error
}

type AppRepository interface {
	GetAppByID(ctx context.Context, id uint32) (models.App, error)
	GetAllApps(ctx context.Context) ([]models.App, error)
	CreateApp(ctx context.Context, app *models.App) (uint32, error)
	UpdateApp(ctx context.Context, app models.App) error
	DeleteApp(ctx context.Context, id uint32) error
	ChangeStatusApp(ctx context.Context, id uint32) error
	IsAdmin(ctx context.Context, userID, appID uint32) (bool, error)
	AddAdmin(ctx context.Context, userID, appID uint32) error
	RemoveAdmin(ctx context.Context, userID, appID uint32) error
	GetAllUsersForApp(ctx context.Context, appID uint32) ([]models.User, error)
}

type TokenRepository interface {
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) (uint32, error)
	GetRefreshToken(ctx context.Context, token string) (models.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteRefreshTokenByIDs(ctx context.Context, userID, appID uint32) error
	GetUserByRefreshToken(ctx context.Context, token string) (models.User, error)
}
```

**Compile-time проверки:** в конец соответствующих файлов репозиториев добавить:
```go
// internal/repositories/user.go:
var _ UserRepository = (*UserRepo)(nil)
// internal/repositories/app.go:
var _ AppRepository = (*AppRepo)(nil)
// internal/repositories/token.go:
var _ TokenRepository = (*TokenRepo)(nil)
```

Если интерфейс расходится с реальными сигнатурами — компилятор скажет сразу. Корректируй интерфейс под существующие методы.

- [ ] **Step 8.2: Сервисы принимают интерфейсы**

Modify `internal/services/auth.go` — изменить поля и конструктор:

```go
type AuthService struct {
	log        *slog.Logger
	storage    *mariadb.Storage
	tokenTTL   time.Duration
	refreshTTL time.Duration
	userR      repositories.UserRepository
	appR       repositories.AppRepository
	tokenR     repositories.TokenRepository
}

func NewAuthService(
	log *slog.Logger,
	storage *mariadb.Storage,
	tokenTTL, refreshTTL time.Duration,
	userR repositories.UserRepository,
	appR repositories.AppRepository,
	tokenR repositories.TokenRepository,
) *AuthService {
	return &AuthService{log, storage, tokenTTL, refreshTTL, userR, appR, tokenR}
}
```

Аналогично для `UserService` (принимает `UserRepository`) и `AppService` (принимает `AppRepository`).

Callers (`internal/app/app.go`) компилятор не заметит — конкретный `*UserRepo` удовлетворяет интерфейсу.

- [ ] **Step 8.3: SQLite testutil helper**

Create `internal/testutil/sqlite.go`:

```go
// Package testutil provides test-only helpers. Zero external dependencies —
// tests must run in any environment (no Docker, no network).
package testutil

import (
	"testing"

	"sso/internal/models"
	"sso/internal/storage/mariadb"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewTestStorage returns a fresh in-memory SQLite Storage with all tables
// migrated via GORM AutoMigrate. Uses pure-Go glebarez/sqlite — no CGO, no
// Docker. Each call yields an isolated DB.
//
// After Stage 7 (multi-backend refactor) this helper will be replaced with
// sqlite.New(":memory:") from internal/storage/sqlite. Test bodies don't
// need to change — they depend only on repository interfaces.
func NewTestStorage(t *testing.T) *mariadb.Storage {
	t.Helper()

	// SQLite :memory: DB — each Open gives a fresh one.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}, &models.App{}, &models.RefreshToken{}, &models.Admin{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	// Auto-close DB when test finishes.
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return &mariadb.Storage{DB: db}
}
```

**Важно:** `mariadb.Storage{DB: ...}` с публичным полем `DB` — проверь текущую структуру [internal/storage/mariadb/mariadb.go](internal/storage/mariadb/mariadb.go#L13). Если поле `DB` уже публичное (по коду — да, `DB *gorm.DB`) — используй так. Если приватное — сделай публичным, это разовая правка ради тестов.

- [ ] **Step 8.4: testutil logger**

Create `internal/testutil/logger.go`:

```go
package testutil

import (
	"io"
	"log/slog"
)

func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
```

- [ ] **Step 8.5: Зависимости**

```bash
go get github.com/stretchr/testify@latest
go get github.com/glebarez/sqlite@latest
go mod tidy
```

- [ ] **Step 8.6: Integration-тесты AuthService**

Create `internal/services/auth_test.go`:

```go
package services_test

import (
	"context"
	"testing"
	"time"

	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/testutil"

	"github.com/stretchr/testify/require"
)

type testSuite struct {
	svc    *services.AuthService
	userR  *repositories.UserRepo
	appR   *repositories.AppRepo
	tokenR *repositories.TokenRepo
}

func newSuite(t *testing.T, refreshTTL time.Duration) *testSuite {
	t.Helper()
	storage := testutil.NewTestStorage(t) // in-memory SQLite с AutoMigrate

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, refreshTTL, userR, appR, tokenR)
	return &testSuite{svc, userR, appR, tokenR}
}

func (s *testSuite) seedApp(t *testing.T) uint32 {
	t.Helper()
	id, err := s.appR.CreateApp(context.Background(), &models.App{
		Name: "test-app", Secret: "super-secret", Link: "https://example.com", IsEnabled: true,
	})
	require.NoError(t, err)
	return id
}

func TestAuthService_RegisterThenLogin_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)

	uid, err := s.svc.RegisterNewUser(ctx, "alice@example.com", "correcthorse", "https://s.com/id/a", "a.png")
	require.NoError(t, err)
	require.NotZero(t, uid)

	access, refresh, err := s.svc.Login(ctx, "alice@example.com", "correcthorse", appID)
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)
}

func TestAuthService_Login_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "bob@example.com", "rightpass", "https://s.com", "p.png")
	require.NoError(t, err)

	_, _, err = s.svc.Login(ctx, "bob@example.com", "wrongpass", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Login_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	s := newSuite(t, time.Hour)
	appID := s.seedApp(t)
	_, _, err := s.svc.Login(context.Background(), "ghost@example.com", "whatever", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Refresh_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "carol@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "carol@example.com", "pw123456", appID)
	require.NoError(t, err)

	access2, refresh2, err := s.svc.Refresh(ctx, refresh)
	require.NoError(t, err)
	require.NotEmpty(t, access2)
	require.NotEqual(t, refresh, refresh2)
}

func TestAuthService_Refresh_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	s := newSuite(t, time.Nanosecond) // born expired
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "dave@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "dave@example.com", "pw123456", appID)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, _, err = s.svc.Refresh(ctx, refresh)
	require.ErrorIs(t, err, services.ErrTokenExpired)
}

func TestAuthService_Logout_DeletesRefreshToken(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	_, err := s.svc.RegisterNewUser(ctx, "eve@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	_, refresh, err := s.svc.Login(ctx, "eve@example.com", "pw123456", appID)
	require.NoError(t, err)

	require.NoError(t, s.svc.Logout(ctx, refresh))

	_, _, err = s.svc.Refresh(ctx, refresh)
	require.Error(t, err)
}

func TestAuthService_ValidateToken_HappyPath(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()
	appID := s.seedApp(t)
	uid, err := s.svc.RegisterNewUser(ctx, "frank@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)
	access, _, err := s.svc.Login(ctx, "frank@example.com", "pw123456", appID)
	require.NoError(t, err)

	got, valid, err := s.svc.ValidateToken(ctx, access)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, uid, got)
}

func TestAuthService_ValidateToken_GarbageToken_Errors(t *testing.T) {
	s := newSuite(t, time.Hour)
	_, _, err := s.svc.ValidateToken(context.Background(), "not-a-jwt")
	require.Error(t, err)
}

func TestAuthService_RegisterDuplicateEmail_Fails(t *testing.T) {
	s := newSuite(t, time.Hour)
	ctx := context.Background()

	_, err := s.svc.RegisterNewUser(ctx, "dup@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	// Повторная регистрация того же email — SQLite должен вернуть duplicate через GORM
	_, err = s.svc.RegisterNewUser(ctx, "dup@example.com", "otherpw1", "https://s.com", "p2.png")
	require.Error(t, err, "duplicate email should fail")
}
```

- [ ] **Step 8.7: Прогон**

```bash
go test ./internal/services/ -v -run TestAuthService -count=1
```

Expected: все 9 тестов PASS в десятках миллисекунд. Реальный SQL, никакого Docker.

**Возможные проблемы:**
- **SQLite не имеет `UNSIGNED` типов.** GORM маппит `uint32` на SQLite `INTEGER` — работает. Если на stage 0 где-то жёстко используется `BIGINT UNSIGNED` — тест упадёт на AutoMigrate. Решение: пусть GORM сам разберётся, не указывай типы руками в моделях.
- **Duplicate detection через GORM** возвращает в ошибке текст `"UNIQUE constraint failed"` на SQLite и `"Error 1062"` на MySQL. Stage 0 пока не маппит их — тест `TestAuthService_RegisterDuplicateEmail_Fails` использует `require.Error` (любая ошибка ок), не `require.ErrorIs`. После Stage 1 ошибка будет типизированной.
- **FK constraints в SQLite выключены по умолчанию.** `glebarez/sqlite` включает их автоматически через `_pragma=foreign_keys(1)` в DSN. Если не работает — добавь `&_pragma=foreign_keys(1)` к DSN в `testutil/sqlite.go`.

- [ ] **Step 8.8: Commit**

```bash
git add go.mod go.sum internal/repositories/iface.go internal/repositories/user.go internal/repositories/app.go internal/repositories/token.go internal/testutil/ internal/services/
git commit -m "test(auth): SQLite-backed integration tests (no Docker required)"
```

---

## Task 9: Финальная проверка Stage 0

- [ ] **Step 9.1: Полный прогон**

```bash
go build ./...
go vet ./...
go test ./...
```

Expected: все три команды — exit 0.

- [ ] **Step 9.2: Проверить ручным grep, что `*context.Context` нигде не осталось**

```bash
grep -rn "\*context\.Context" internal/ lib/ cmd/
```

Expected: **ничего не найдено** (или только в тестовых файлах, если там намеренно).

- [ ] **Step 9.3: Проверить, что `log.Print(dsn)` не вернулся**

```bash
grep -rn "log.Print(dsn)" internal/ cmd/
```

Expected: ничего.

- [ ] **Step 9.4: Merge или PR**

```bash
git log --oneline main..HEAD
```

Expected: ~7-9 коммитов с префиксами `fix:`, `refactor:`, `feat:`, `test:`, `chore:`.

Затем либо merge в main локально, либо `gh pr create` (если используется PR-flow).

---

## Definition of Done

- [x] `go build ./...` — 0 ошибок
- [x] `go vet ./...` — 0 предупреждений
- [x] `go test ./...` — все тесты зелёные (без Docker)
- [x] Все unit-тесты `TestAuthService_*` проходят в миллисекундах через fakes
- [x] Репозитории реализуют `UserRepository`/`AppRepository`/`TokenRepository` интерфейсы (compile-time проверено)
- [x] В коде нет `*context.Context`, `fmt.Println`, `log.Print(dsn)`
- [x] `grpc.Server` использует единый `TimeoutUnaryInterceptor`
- [x] `application.Storage.Close()` вызывается в graceful shutdown
- [x] `Refresh` с истёкшим токеном возвращает `ErrTokenExpired`

После мержа Stage 0 в main — начинать Stage 1 (domain errors + protovalidate), см. roadmap.
