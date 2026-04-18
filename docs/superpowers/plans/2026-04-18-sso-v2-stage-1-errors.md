# SSO v2 Migration — Stage 1: Domain Errors + Protovalidate

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 0 merged to main, integration tests green.

**Goal:** Ввести доменную модель ошибок, совместимую с v2 `ErrorReason` enum. Подключить `protovalidate-go` interceptor. Удалить `lib/serr`. v1-контроллеры продолжают работать через adapter.

**Architecture:** Сервисы возвращают **только** доменные sentinel-ошибки. Два errmap-адаптера: `ToV1` (старый стиль для текущих v1-контроллеров) и `ToV2` (новый стиль с `google.rpc.ErrorInfo`). Интерцептор валидации подключается на gRPC-сервер — он no-op для v1-запросов (у них нет `buf.validate` аннотаций).

**Tech Stack:** `buf.build/go/protovalidate`, `google.golang.org/genproto/googleapis/rpc/errdetails`, errors.Is/As.

**Branch:** `stage-1-errors`

---

## File Structure

**Создаётся:**
- `internal/domain/errors.go` — все доменные sentinel-ошибки
- `internal/domain/errors_test.go` — тесты на errors.Is цепочки
- `internal/transport/grpc/errmap/errmap.go` — `ToV1`, `ToV2`
- `internal/transport/grpc/errmap/errmap_test.go` — маппинг unit-тесты
- `internal/transport/grpc/interceptors/validate.go` — protovalidate interceptor

**Модифицируется:**
- `internal/services/auth.go` — заменить `serr.*` и raw errors на доменные
- `internal/services/user.go` — то же
- `internal/services/app.go` — то же
- `internal/repositories/{user,app,token}.go` — маппить `gorm.ErrRecordNotFound` в доменные `ErrXxxNotFound`
- `internal/controllers/{auth,user,app}.go` — заменить `status.Error(codes.Internal, err.Error())` на `return nil, errmap.ToV1(err)`
- `internal/app/grpc/app.go` — подключить `chain(Timeout, Validate)` interceptor
- `lib/serr/` — **удалить целиком** в финальной задаче

---

## Task 1: Создать ветку и baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-1-errors
go build ./... && go test ./...
```
Expected: exit 0 везде. Если красное — STOP, не твой ветке дело.

---

## Task 2: Доменные ошибки

**Files:**
- Create: `internal/domain/errors.go`
- Create: `internal/domain/errors_test.go`

- [ ] **Step 2.1: Failing test**

Create `internal/domain/errors_test.go`:

```go
package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"sso/internal/domain"
)

func TestErrors_AreDistinct(t *testing.T) {
	all := []error{
		domain.ErrUserNotFound,
		domain.ErrUserAlreadyExists,
		domain.ErrAppNotFound,
		domain.ErrAppAlreadyExists,
		domain.ErrInvalidCredentials,
		domain.ErrInvalidToken,
		domain.ErrTokenExpired,
		domain.ErrPasswordMismatch,
		domain.ErrPermissionDenied,
		domain.ErrValidationFailed,
	}
	for i, a := range all {
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("expected %v and %v to be distinct", a, b)
			}
		}
	}
}

func TestErrors_WrappedDetectableByErrorsIs(t *testing.T) {
	wrapped := fmt.Errorf("auth.Login: %w", domain.ErrInvalidCredentials)
	if !errors.Is(wrapped, domain.ErrInvalidCredentials) {
		t.Fatal("errors.Is must match through wrap")
	}
}
```

```bash
go test ./internal/domain/ -v
```
Expected: FAIL — пакет не существует.

- [ ] **Step 2.2: Реализация**

Create `internal/domain/errors.go`:

```go
// Package domain defines cross-cutting sentinel errors used by services.
// Controllers translate these into gRPC status responses via
// internal/transport/grpc/errmap.
//
// The set mirrors ssov2.ErrorReason so that v2 controllers can return
// google.rpc.ErrorInfo with a stable reason string.
package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrAppNotFound        = errors.New("app not found")
	ErrAppAlreadyExists   = errors.New("app already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrPasswordMismatch   = errors.New("password mismatch")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrValidationFailed   = errors.New("validation failed")
)
```

```bash
go test ./internal/domain/ -v
```
Expected: PASS.

- [ ] **Step 2.3: Commit**

```bash
git add internal/domain/
git commit -m "feat(domain): introduce sentinel errors mirroring v2 ErrorReason"
```

---

## Task 3: errmap.ToV1 и ToV2

**Files:**
- Create: `internal/transport/grpc/errmap/errmap.go`
- Create: `internal/transport/grpc/errmap/errmap_test.go`

- [ ] **Step 3.1: Добавить зависимость**

```bash
go get google.golang.org/genproto/googleapis/rpc/errdetails@latest
go mod tidy
```

- [ ] **Step 3.2: Failing test**

Create `internal/transport/grpc/errmap/errmap_test.go`:

```go
package errmap_test

import (
	"errors"
	"fmt"
	"testing"

	"sso/internal/domain"
	"sso/internal/transport/grpc/errmap"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToV1_MapsDomainErrors(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want codes.Code
	}{
		{"invalid creds", domain.ErrInvalidCredentials, codes.Unauthenticated},
		{"user not found", domain.ErrUserNotFound, codes.NotFound},
		{"already exists", domain.ErrUserAlreadyExists, codes.AlreadyExists},
		{"permission", domain.ErrPermissionDenied, codes.PermissionDenied},
		{"validation", domain.ErrValidationFailed, codes.InvalidArgument},
		{"unknown", errors.New("boom"), codes.Internal},
		{"nil", nil, codes.OK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := errmap.ToV1(c.in)
			if c.in == nil {
				if got != nil {
					t.Fatalf("nil → %v", got)
				}
				return
			}
			if s, _ := status.FromError(got); s.Code() != c.want {
				t.Fatalf("want %v, got %v", c.want, s.Code())
			}
		})
	}
}

func TestToV1_HidesRawMessage(t *testing.T) {
	// Domain error wrapped with DB noise — raw text must NOT leak.
	wrapped := fmt.Errorf("repo: SELECT failed: %w", domain.ErrUserNotFound)
	got := errmap.ToV1(wrapped)
	s, _ := status.FromError(got)
	if s.Message() == wrapped.Error() {
		t.Fatal("raw error leaked")
	}
}

func TestToV2_AttachesErrorInfo(t *testing.T) {
	got := errmap.ToV2(domain.ErrUserNotFound)
	s, _ := status.FromError(got)
	if s.Code() != codes.NotFound {
		t.Fatalf("code: %v", s.Code())
	}
	found := false
	for _, d := range s.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			if info.GetReason() == "USER_NOT_FOUND" && info.GetDomain() == "sso.nergous.ru" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("missing ErrorInfo{reason: USER_NOT_FOUND, domain: sso.nergous.ru}")
	}
}

func TestToV2_UnknownErrorYieldsInternalNoInfo(t *testing.T) {
	got := errmap.ToV2(errors.New("raw db error"))
	s, _ := status.FromError(got)
	if s.Code() != codes.Internal {
		t.Fatalf("want Internal, got %v", s.Code())
	}
	if s.Message() == "raw db error" {
		t.Fatal("raw error leaked to client")
	}
}
```

- [ ] **Step 3.3: Реализация**

Create `internal/transport/grpc/errmap/errmap.go`:

```go
package errmap

import (
	"errors"

	"sso/internal/domain"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const errDomain = "sso.nergous.ru"

type mapping struct {
	code   codes.Code
	reason string
	msg    string
}

var table = []struct {
	target error
	m      mapping
}{
	{domain.ErrInvalidCredentials, mapping{codes.Unauthenticated, "INVALID_CREDENTIALS", "invalid credentials"}},
	{domain.ErrInvalidToken, mapping{codes.Unauthenticated, "INVALID_TOKEN", "invalid token"}},
	{domain.ErrTokenExpired, mapping{codes.Unauthenticated, "INVALID_TOKEN", "token expired"}},
	{domain.ErrUserAlreadyExists, mapping{codes.AlreadyExists, "USER_ALREADY_EXISTS", "user already exists"}},
	{domain.ErrUserNotFound, mapping{codes.NotFound, "USER_NOT_FOUND", "user not found"}},
	{domain.ErrAppNotFound, mapping{codes.NotFound, "APP_NOT_FOUND", "app not found"}},
	{domain.ErrAppAlreadyExists, mapping{codes.AlreadyExists, "APP_ALREADY_EXISTS", "app already exists"}},
	{domain.ErrPermissionDenied, mapping{codes.PermissionDenied, "PERMISSION_DENIED", "permission denied"}},
	{domain.ErrValidationFailed, mapping{codes.InvalidArgument, "VALIDATION_FAILED", "validation failed"}},
	{domain.ErrPasswordMismatch, mapping{codes.Unauthenticated, "PASSWORD_MISMATCH", "password mismatch"}},
}

func lookup(err error) (mapping, bool) {
	for _, row := range table {
		if errors.Is(err, row.target) {
			return row.m, true
		}
	}
	return mapping{}, false
}

// ToV1 returns a legacy-style gRPC status: safe public message, no details.
// Unknown errors → codes.Internal with a generic message (never the raw text).
func ToV1(err error) error {
	if err == nil {
		return nil
	}
	if m, ok := lookup(err); ok {
		return status.Error(m.code, m.msg)
	}
	return status.Error(codes.Internal, "internal error")
}

// ToV2 attaches google.rpc.ErrorInfo so clients can switch on the reason.
func ToV2(err error) error {
	if err == nil {
		return nil
	}
	if m, ok := lookup(err); ok {
		st := status.New(m.code, m.msg)
		if withDetails, derr := st.WithDetails(&errdetails.ErrorInfo{
			Reason: m.reason,
			Domain: errDomain,
		}); derr == nil {
			return withDetails.Err()
		}
		return st.Err()
	}
	return status.Error(codes.Internal, "internal error")
}
```

```bash
go test ./internal/transport/grpc/errmap/ -v
```
Expected: PASS (all).

- [ ] **Step 3.4: Commit**

```bash
git add internal/transport/grpc/errmap/ go.mod go.sum
git commit -m "feat(errmap): map domain errors to v1/v2 gRPC statuses"
```

---

## Task 4: Репозитории возвращают доменные ошибки

**Files:** `internal/repositories/{user,app,token}.go`

Принцип: `errors.Is(err, gorm.ErrRecordNotFound)` → `domain.ErrXxxNotFound`. MySQL duplicate-key (1062) → `domain.ErrXxxAlreadyExists`.

- [ ] **Step 4.1: Helper для duplicate detection**

Create `internal/repositories/errors.go`:

```go
package repositories

import (
	"errors"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const mysqlErrDup uint16 = 1062

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == mysqlErrDup
}

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
```

- [ ] **Step 4.2: user.go**

Modify `internal/repositories/user.go`:

```go
package repositories

import (
	"context"

	"sso/internal/domain"
	"sso/internal/models"
	"sso/internal/storage/mariadb"
)

type UserRepo struct{ storage *mariadb.Storage }

func NewUserRepo(storage *mariadb.Storage) *UserRepo { return &UserRepo{storage: storage} }

func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	err := r.storage.DB.WithContext(ctx).Where("email = ?", email).First(&u).Error
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetUserByID(ctx context.Context, id uint32) (models.User, error) {
	var u models.User
	err := r.storage.DB.WithContext(ctx).Where("id = ?", id).First(&u).Error
	if isNotFound(err) {
		return models.User{}, domain.ErrUserNotFound
	}
	return u, err
}

func (r *UserRepo) GetAllUsers(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.storage.DB.WithContext(ctx).Find(&users).Error
	return users, err
}

func (r *UserRepo) CreateUser(ctx context.Context, user *models.User) (uint32, error) {
	err := r.storage.DB.WithContext(ctx).Create(user).Error
	if isDuplicate(err) {
		return 0, domain.ErrUserAlreadyExists
	}
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (r *UserRepo) UpdateUser(ctx context.Context, user models.User) error {
	res := r.storage.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"email":         user.Email,
		"pass_hash":     user.PassHash,
		"steam_url":     user.SteamURL,
		"path_to_photo": user.PathToPhoto,
	})
	if res.Error != nil {
		if isDuplicate(res.Error) {
			return domain.ErrUserAlreadyExists
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) DeleteUser(ctx context.Context, id uint32) error {
	return r.storage.DB.WithContext(ctx).Delete(&models.User{}, id).Error
}
```

- [ ] **Step 4.3: app.go — аналогично**

В `internal/repositories/app.go` применить тот же паттерн:
- `GetAppByID` — `isNotFound → domain.ErrAppNotFound`
- `CreateApp` — `isDuplicate → domain.ErrAppAlreadyExists`
- `UpdateApp` — через `.Updates(map[...])`, `RowsAffected == 0 → ErrAppNotFound`, duplicate → `ErrAppAlreadyExists`
- `IsAdmin` — возвращает `(false, nil)` когда `isNotFound` (это не ошибка, просто факт "не админ")
- Сохранить остальные методы как есть, но убрать zero-check на `uint32(0)` — теперь это дело сервиса

- [ ] **Step 4.4: token.go — GetRefreshToken при not found**

`GetRefreshToken`: `isNotFound → domain.ErrInvalidToken`. Остальные методы — как есть.

- [ ] **Step 4.5: Сборка + тесты**

```bash
go build ./... && go test ./...
```
Expected: зелёное. Тесты из Stage 0 **должны продолжать проходить** (сервисы теперь получают доменные ошибки вместо raw gorm — но в Stage 0 проверка `ErrorIs(err, ErrInvalidCredentials)` переживёт, потому что в Step 5 ниже сервисы будут маппить в те же sentinel'ы).

Если что-то красное на этом шаге — нормально: часть тестов Stage 0 ждёт старого поведения сервисов. Продолжай — в Task 5 починим.

- [ ] **Step 4.6: Commit**

```bash
git add internal/repositories/
git commit -m "refactor(repo): return domain errors instead of raw gorm errors"
```

---

## Task 5: Сервисы используют только доменные ошибки

**Files:**
- Modify: `internal/services/auth.go`
- Modify: `internal/services/user.go`
- Modify: `internal/services/app.go`

Принципы:
1. Удалить все вызовы `serr.Gerr/Ferr/LogFerr` — заменить на стандартные `if err != nil { log & return wrappedErr }`
2. Оборачивать в `fmt.Errorf("op: %w", err)` чтобы цепочка `errors.Is` работала
3. Sentinel'ы `ErrInvalidCredentials`, `ErrTokenExpired` из `services` переехали в `domain` — удалить их из `services/auth.go`, использовать `domain.Err*`
4. Bcrypt mismatch → `domain.ErrInvalidCredentials`
5. JWT parse error → `domain.ErrInvalidToken`

- [ ] **Step 5.1: auth.go Login**

Заменить `Login` (целиком):

```go
func (a *AuthService) Login(
	ctx context.Context,
	email, password string,
	appID uint32,
) (accessToken, refreshToken string, err error) {
	const op = "auth.Login"
	log := a.log.With(slog.String("op", op))

	user, err := a.userR.GetUserByEmail(ctx, email)
	if errors.Is(err, domain.ErrUserNotFound) {
		return "", "", domain.ErrInvalidCredentials
	}
	if err != nil {
		log.Error("failed to get user", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password)); err != nil {
		return "", "", domain.ErrInvalidCredentials
	}

	app, err := a.appR.GetAppByID(ctx, appID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	isAdmin, err := a.appR.IsAdmin(ctx, user.ID, appID)
	if err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	accessToken, err = jwt_sso.NewAccessToken(user.ID, user.Email, isAdmin, app.ID, app.Secret, a.tokenTTL)
	if err != nil {
		log.Error("failed to sign access token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	refreshToken, err = jwt_sso.NewRefreshToken()
	if err != nil {
		log.Error("failed to generate refresh token", slog.Any("err", err))
		return "", "", fmt.Errorf("%s: %w", op, err)
	}

	if err := a.tokenR.DeleteRefreshTokenByIDs(ctx, user.ID, appID); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	if _, err := a.tokenR.CreateRefreshToken(ctx, &models.RefreshToken{
		Token: refreshToken, UserID: user.ID, AppID: appID,
		ExpiresAt: time.Now().Add(a.refreshTTL),
	}); err != nil {
		return "", "", fmt.Errorf("%s: %w", op, err)
	}
	return accessToken, refreshToken, nil
}
```

- [ ] **Step 5.2: auth.go Logout, RegisterNewUser, Refresh, ValidateToken**

По тому же паттерну. Ключевые моменты:
- `RegisterNewUser`: на `domain.ErrUserAlreadyExists` — прокидывать как есть (wrap через `%w`), не превращать в Internal
- `Refresh`: если `GetRefreshToken` вернул `domain.ErrInvalidToken` — прокидывать как есть. `ErrTokenExpired` — как в Stage 0 (уже сделано).
- `ValidateToken`: любая JWT-ошибка → `domain.ErrInvalidToken`. Потерянный/disabled app → `domain.ErrInvalidToken` (клиенту не раскрываем причину).
- Удалить блок `var ( ErrInvalidCredentials, ErrInternal, ErrTokenExpired )` из `services/auth.go` — они теперь в `domain`.

- [ ] **Step 5.3: user.go, app.go**

Аналогично — прокидывать `domain.Err*` с оборачиванием. Удалить импорт `serr`.

`user.go UpdateUser`: использовать новый `repositories.UpdateUser` — теперь он сам маппит `RowsAffected==0 → ErrUserNotFound`.

- [ ] **Step 5.4: Починить Stage-0 тесты**

В `internal/services/auth_integration_test.go` заменить:
- `services.ErrInvalidCredentials` → `domain.ErrInvalidCredentials`
- `services.ErrTokenExpired` → `domain.ErrTokenExpired`

Добавить импорт `"sso/internal/domain"`.

- [ ] **Step 5.5: Сборка + тесты**

```bash
go build ./... && go test ./...
```
Expected: PASS.

- [ ] **Step 5.6: Commit**

```bash
git add internal/services/
git commit -m "refactor(services): use domain errors exclusively"
```

---

## Task 6: Контроллеры через errmap.ToV1

**Files:** `internal/controllers/{auth,user,app}.go`

Заменить все `status.Error(codes.Internal, err.Error())` и специальные case'ы на `return nil, errmap.ToV1(err)`.

- [ ] **Step 6.1: auth.go**

Пример для `Login`:

```go
func (c *AuthController) Login(ctx context.Context, req *ssov1.LoginRequest) (*ssov1.LoginResponse, error) {
	email, password, appID := req.GetEmail(), req.GetPassword(), req.GetAppId()
	if err := validateLogin(email, password, appID); err != nil {
		return nil, err
	}
	accessToken, refreshToken, err := c.AuthS.Login(ctx, email, password, appID)
	if err != nil {
		return nil, errmap.ToV1(err)
	}
	return &ssov1.LoginResponse{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
```

Убрать `if errors.Is(err, services.ErrInvalidCredentials) { ... }` — теперь это делает errmap. `validateLogin` оставить (пока нет protovalidate для v1).

- [ ] **Step 6.2: user.go, app.go — тот же паттерн**

Везде `return nil, errmap.ToV1(err)` вместо `codes.Internal + err.Error()`.

- [ ] **Step 6.3: Сборка + тесты**

```bash
go build ./... && go test ./...
```
Expected: PASS.

- [ ] **Step 6.4: Commit**

```bash
git add internal/controllers/
git commit -m "refactor(controllers): route errors through errmap.ToV1"
```

---

## Task 7: Protovalidate interceptor

**Files:**
- Create: `internal/transport/grpc/interceptors/validate.go`
- Modify: `internal/app/grpc/app.go` — chain Timeout + Validate

- [ ] **Step 7.1: Зависимость**

```bash
go get buf.build/go/protovalidate@latest
go mod tidy
```

- [ ] **Step 7.2: Interceptor**

Create `internal/transport/grpc/interceptors/validate.go`:

```go
package interceptors

import (
	"context"
	"errors"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ValidateUnaryInterceptor runs protovalidate against the request message.
// Messages without buf.validate annotations (e.g. v1 types) pass through
// as a no-op.
func ValidateUnaryInterceptor() grpc.UnaryServerInterceptor {
	validator, err := protovalidate.New()
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return handler(ctx, req)
		}
		if err := validator.Validate(msg); err != nil {
			var verr *protovalidate.ValidationError
			if errors.As(err, &verr) {
				return nil, status.Error(codes.InvalidArgument, verr.Error())
			}
			return nil, status.Error(codes.InvalidArgument, "validation failed")
		}
		return handler(ctx, req)
	}
}
```

- [ ] **Step 7.3: Chain в grpc-app**

Modify `internal/app/grpc/app.go` — заменить регистрацию сервера:

```go
gRPCServer := grpc.NewServer(
	grpc.ChainUnaryInterceptor(
		interceptors.TimeoutUnaryInterceptor(5*time.Second),
		interceptors.ValidateUnaryInterceptor(),
	),
)
```

- [ ] **Step 7.4: Сборка + тесты**

```bash
go build ./... && go test ./...
```
Expected: PASS. v1-типы без buf.validate аннотаций — проходят no-op.

- [ ] **Step 7.5: Commit**

```bash
git add internal/transport/grpc/interceptors/ internal/app/grpc/ go.mod go.sum
git commit -m "feat(grpc): add protovalidate unary interceptor"
```

---

## Task 8: Удалить lib/serr

- [ ] **Step 8.1: Убедиться, что никто не импортирует**

```bash
grep -rn "sso/lib/serr" --include="*.go"
```
Expected: ничего.

- [ ] **Step 8.2: Удалить**

```bash
rm -rf lib/serr
```

- [ ] **Step 8.3: Сборка + тесты**

```bash
go build ./... && go test ./...
```
Expected: PASS.

- [ ] **Step 8.4: Commit**

```bash
git add -A
git commit -m "chore: remove obsolete lib/serr package"
```

---

## Definition of Done Stage 1

- `go build ./... && go vet ./... && go test ./...` — все зелёные
- В сервисах нет `serr.*`, нет `errors.New` для доменных условий (только в `domain/errors.go`)
- В контроллерах ни одного `err.Error()` в `status.Error`
- `lib/serr/` не существует
- `protovalidate` interceptor подключён, цепочка `Timeout → Validate`
- v1-поведение не изменилось снаружи: ошибки маппятся в те же `codes` что и раньше (Internal для неизвестных → но с безопасным текстом)

Дальше — Stage 2 (data layer).
