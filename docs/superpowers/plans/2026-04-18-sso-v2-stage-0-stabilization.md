# SSO v2 Migration — Stage 0: Stabilization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Починить блокирующие баги, убрать антипаттерны, завести integration-тесты — подготовить кодовую базу к безопасной миграции на v2.

**Architecture:** Работаем на ветке `stage-0-stabilization`. Каждая задача — отдельный коммит. Никаких изменений API v1 — только internal refactor + тесты. После завершения Stage 0 — `go build ./... && go test ./...` зелёные, поведение v1 RPC не изменилось.

**Tech Stack:** Go 1.24, testify, testcontainers-go, MariaDB 11, grpc-go, GORM, slog.

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
- `internal/services/auth_integration_test.go` — integration-тесты на AuthService
- `internal/testutil/db.go` — helper для подъёма testcontainers MariaDB

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

## Task 8: Integration-тесты для AuthService

**Files:**
- Create: `internal/testutil/db.go`
- Create: `internal/services/auth_integration_test.go`

**Контекст:** это единственный этап Stage 0, который требует внешних зависимостей. `testcontainers-go` поднимает MariaDB в Docker, прогоняет `Migrate`, затем тесты работают с реальной БД. **Предусловие:** на машине агента установлен и запущен Docker.

- [ ] **Step 8.1: Добавить зависимости**

```bash
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/mariadb@latest
go get github.com/stretchr/testify@latest
go mod tidy
```

- [ ] **Step 8.2: Создать testutil helper**

Create `internal/testutil/db.go`:

```go
package testutil

import (
	"context"
	"testing"
	"time"

	"sso/internal/storage/mariadb"

	"github.com/testcontainers/testcontainers-go/modules/mariadb"
)

// NewTestStorage spins up a MariaDB testcontainer, runs migrations, and
// returns a ready Storage plus a cleanup closure. Fails the test if Docker
// is unavailable or migrations fail.
func NewTestStorage(t *testing.T) (*mariadb_storage.Storage, func()) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	container, err := mariadb.Run(ctx,
		"mariadb:11",
		mariadb.WithDatabase("ssotest"),
		mariadb.WithUsername("sso"),
		mariadb.WithPassword("sso"),
	)
	if err != nil {
		t.Fatalf("failed to start mariadb container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to get dsn: %v", err)
	}

	storage, err := mariadb_storage.NewStorage(dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to open storage: %v", err)
	}

	if err := storage.Migrate(); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("failed to migrate: %v", err)
	}

	cleanup := func() {
		_ = storage.Close()
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = container.Terminate(termCtx)
	}

	return storage, cleanup
}
```

**Важно:** пакет `sso/internal/storage/mariadb` и testcontainers `mariadb`-модуль имеют одинаковое имя пакета. В импортах использован алиас `mariadb_storage` для первого — обеспечь, что имя алиаса совпадает с использованием в коде выше.

- [ ] **Step 8.3: Написать failing integration-тест для Login happy path**

Create `internal/services/auth_integration_test.go`:

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

func newAuthTestSuite(t *testing.T) (*services.AuthService, *repositories.UserRepo, *repositories.AppRepo, func()) {
	t.Helper()
	storage, cleanup := testutil.NewTestStorage(t)

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)

	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, time.Hour, userR, appR, tokenR)

	return svc, userR, appR, cleanup
}

func TestAuthService_RegisterThenLogin_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	// seed an app
	appID, err := appR.CreateApp(ctx, &models.App{
		Name:   "test-app",
		Secret: "super-secret",
		Link:   "https://example.com",
	})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "alice@example.com", "correcthorse", "https://steamcommunity.com/id/alice", "alice.png")
	require.NoError(t, err)
	require.NotZero(t, userID)

	accessToken, refreshToken, err := svc.Login(ctx, "alice@example.com", "correcthorse", appID)
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
}

func TestAuthService_Login_WrongPassword_ReturnsInvalidCredentials(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "bob@example.com", "rightpass", "https://s.com", "p.png")
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, "bob@example.com", "wrongpass", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Login_UnknownEmail_ReturnsInvalidCredentials(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()

	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, "ghost@example.com", "whatever", appID)
	require.ErrorIs(t, err, services.ErrInvalidCredentials)
}

func TestAuthService_Refresh_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "carol@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "carol@example.com", "pw123456", appID)
	require.NoError(t, err)

	newAccess, newRefresh, err := svc.Refresh(ctx, refreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, newAccess)
	require.NotEmpty(t, newRefresh)
	require.NotEqual(t, refreshToken, newRefresh)
}

func TestAuthService_Refresh_ExpiredToken_ReturnsErrTokenExpired(t *testing.T) {
	// Use a 1ns refresh TTL so the token is born expired.
	storage, cleanup := testutil.NewTestStorage(t)
	defer cleanup()

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)
	log := testutil.NewTestLogger()
	svc := services.NewAuthService(log, storage, time.Minute, time.Nanosecond, userR, appR, tokenR)

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "dave@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "dave@example.com", "pw123456", appID)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, _, err = svc.Refresh(ctx, refreshToken)
	require.ErrorIs(t, err, services.ErrTokenExpired)
}

func TestAuthService_Logout_DeletesRefreshToken(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	_, err = svc.RegisterNewUser(ctx, "eve@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	_, refreshToken, err := svc.Login(ctx, "eve@example.com", "pw123456", appID)
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, refreshToken))

	// Using the refresh token after logout should fail.
	_, _, err = svc.Refresh(ctx, refreshToken)
	require.Error(t, err)
}

func TestAuthService_ValidateToken_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "frank@example.com", "pw123456", "https://s.com", "p.png")
	require.NoError(t, err)

	accessToken, _, err := svc.Login(ctx, "frank@example.com", "pw123456", appID)
	require.NoError(t, err)

	gotUserID, valid, err := svc.ValidateToken(ctx, accessToken)
	require.NoError(t, err)
	require.True(t, valid)
	require.Equal(t, userID, gotUserID)
}

func TestAuthService_ValidateToken_GarbageToken_Errors(t *testing.T) {
	svc, _, _, cleanup := newAuthTestSuite(t)
	defer cleanup()

	_, _, err := svc.ValidateToken(context.Background(), "not-a-jwt")
	require.Error(t, err)
}
```

- [ ] **Step 8.4: Добавить test logger helper**

Create `internal/testutil/logger.go`:

```go
package testutil

import (
	"io"
	"log/slog"
)

// NewTestLogger returns a slog.Logger that discards all output — use in tests
// so logs don't pollute `go test -v` output.
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
```

- [ ] **Step 8.5: Запустить тесты**

```bash
go test ./internal/services/ -v -run TestAuthService -count=1
```

Expected: все 8 тестов PASS. Если Docker недоступен — тесты упадут на этапе `testutil.NewTestStorage` с понятной ошибкой.

**Возможные проблемы и их решения:**
- `failed to start mariadb container: Cannot connect to the Docker daemon` — Docker не запущен. Запустить Docker Desktop / `sudo systemctl start docker`.
- Импорт `mariadb_storage` vs `mariadb` (testcontainers) конфликтует — использовать явный алиас в `testutil/db.go`:
  ```go
  import (
      ssomariadb "sso/internal/storage/mariadb"
      tcmariadb "github.com/testcontainers/testcontainers-go/modules/mariadb"
  )
  ```
  и соответственно `*ssomariadb.Storage` / `tcmariadb.Run(...)`. Если ошибка компиляции в Step 8.5 говорит про конфликт имён — применить этот вариант.

- [ ] **Step 8.6: Commit**

```bash
git add go.mod go.sum internal/testutil/ internal/services/auth_integration_test.go
git commit -m "test(auth): add integration tests for AuthService via testcontainers"
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
- [x] `go test ./...` — все тесты зелёные
- [x] Все integration-тесты `TestAuthService_*` проходят с реальной MariaDB
- [x] В коде нет `*context.Context`, `fmt.Println`, `log.Print(dsn)`
- [x] `grpc.Server` использует единый `TimeoutUnaryInterceptor`
- [x] `application.Storage.Close()` вызывается в graceful shutdown
- [x] `Refresh` с истёкшим токеном возвращает `ErrTokenExpired`

После мержа Stage 0 в main — начинать Stage 1 (domain errors + protovalidate), см. roadmap.
