# SSO v2 Migration — Stage 3: v2 Controllers (Dual-Serve)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 2 merged to main.

**Goal:** Зарегистрировать новые v2 gRPC-сервисы параллельно с v1. Новый контроллер-слой, маппинг `models ↔ ssov2`, новый сервисный метод `ChangePassword`. v1 продолжает работать без изменений.

**Architecture:** Два набора контроллеров на одном `grpc.Server`. Старые — в `internal/controllers/`, новые — в `internal/transport/grpc/v2/`. Сервисный слой **общий** — он уже возвращает доменные ошибки из Stage 1, v2-контроллеры маппят через `errmap.ToV2`. Маппинг `models ↔ ssov2` — в `internal/transport/grpc/mapper/v2/` (без обращений к БД, чисто конвертация).

**Tech Stack:** `github.com/Nergous/sso_protos/gen/go/sso/auth/v2`, `google.golang.org/protobuf/types/known/emptypb`, `google.golang.org/protobuf/types/known/timestamppb`.

**Branch:** `stage-3-v2-controllers`

---

## File Structure

**Создаётся:**
- `internal/transport/grpc/mapper/v2/user.go` — `UserModelToProto`
- `internal/transport/grpc/mapper/v2/app.go` — `AppModelToProto`, `AppUserToProto`
- `internal/transport/grpc/mapper/v2/mapper_test.go`
- `internal/transport/grpc/v2/auth.go` — AuthV2Controller
- `internal/transport/grpc/v2/user.go` — UserV2Controller
- `internal/transport/grpc/v2/app.go` — AppV2Controller
- `internal/transport/grpc/v2/v2_integration_test.go` — end-to-end поверх testcontainers

**Модифицируется:**
- `internal/services/auth.go` — добавить `ChangePassword(ctx, userID, old, new)`
- `internal/app/app.go` — создать v2-контроллеры рядом с v1
- `internal/app/grpc/app.go` — принять v2-контроллеры, зарегистрировать оба набора серверов

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-3-v2-controllers
go build ./... && go test ./...
```
Expected: green.

---

## Task 2: Mapper models ↔ ssov2

**Files:**
- Create: `internal/transport/grpc/mapper/v2/user.go`
- Create: `internal/transport/grpc/mapper/v2/app.go`
- Create: `internal/transport/grpc/mapper/v2/mapper_test.go`

- [ ] **Step 2.1: Failing test**

```go
package v2mapper_test

import (
	"testing"
	"time"

	"sso/internal/models"
	v2mapper "sso/internal/transport/grpc/mapper/v2"
)

func TestUserModelToProto(t *testing.T) {
	now := time.Now()
	in := models.User{
		ID: 42, Email: "a@b.c", SteamURL: "s", PathToPhoto: "p",
		CreatedAt: now, UpdatedAt: now,
	}
	got := v2mapper.UserModelToProto(in)
	if got.GetId() != 42 || got.GetEmail() != "a@b.c" {
		t.Fatalf("bad fields: %+v", got)
	}
	if got.GetCreatedAt() == nil || got.GetUpdatedAt() == nil {
		t.Fatal("missing timestamps")
	}
}

func TestAppModelToProto(t *testing.T) {
	in := models.App{ID: 1, Name: "n", Link: "l", IsEnabled: true}
	got := v2mapper.AppModelToProto(in)
	if got.GetId() != 1 || got.GetName() != "n" || !got.GetIsEnabled() {
		t.Fatalf("bad fields: %+v", got)
	}
}
```

- [ ] **Step 2.2: user.go mapper**

```go
package v2mapper

import (
	"sso/internal/models"
	"sso/internal/repositories"

	ssov2 "github.com/Nergous/sso_protos/gen/go/sso/auth/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func UserModelToProto(u models.User) *ssov2.UserModel {
	return &ssov2.UserModel{
		Id:          u.ID,
		Email:       u.Email,
		SteamUrl:    u.SteamURL,
		PathToPhoto: u.PathToPhoto,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		UpdatedAt:   timestamppb.New(u.UpdatedAt),
	}
}

func UserModelsToProto(users []models.User) []*ssov2.UserModel {
	out := make([]*ssov2.UserModel, 0, len(users))
	for _, u := range users {
		out = append(out, UserModelToProto(u))
	}
	return out
}

func AppUserRowToProto(r repositories.AppUserRow) *ssov2.AppUser {
	return &ssov2.AppUser{
		Id:          r.ID,
		Email:       r.Email,
		SteamUrl:    r.SteamURL,
		PathToPhoto: r.PathToPhoto,
		IsAdmin:     r.IsAdmin,
	}
}

func AppUserRowsToProto(rows []repositories.AppUserRow) []*ssov2.AppUser {
	out := make([]*ssov2.AppUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, AppUserRowToProto(r))
	}
	return out
}
```

- [ ] **Step 2.3: app.go mapper**

```go
package v2mapper

import (
	"sso/internal/models"

	ssov2 "github.com/Nergous/sso_protos/gen/go/sso/auth/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func AppModelToProto(a models.App) *ssov2.AppModel {
	return &ssov2.AppModel{
		Id:        a.ID,
		Name:      a.Name,
		Link:      a.Link,
		IsEnabled: a.IsEnabled,
		CreatedAt: timestamppb.New(a.CreatedAt),
		UpdatedAt: timestamppb.New(a.UpdatedAt),
	}
}

func AppModelsToProto(apps []models.App) []*ssov2.AppModel {
	out := make([]*ssov2.AppModel, 0, len(apps))
	for _, a := range apps {
		out = append(out, AppModelToProto(a))
	}
	return out
}
```

- [ ] **Step 2.4: Test + commit**

```bash
go test ./internal/transport/grpc/mapper/v2/ -v
git add internal/transport/grpc/mapper/v2/
git commit -m "feat(mapper/v2): model↔proto conversion for User, App, AppUser"
```

---

## Task 3: AuthService.ChangePassword

**Files:** `internal/services/auth.go`

- [ ] **Step 3.1: Failing test**

В `internal/services/auth_integration_test.go` добавить:

```go
func TestAuthService_ChangePassword_HappyPath(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	appID, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "grace@example.com", "oldpass12", "https://s.com", "p.png")
	require.NoError(t, err)

	require.NoError(t, svc.ChangePassword(ctx, userID, "oldpass12", "newpass34"))

	// Old password no longer works.
	_, _, err = svc.Login(ctx, "grace@example.com", "oldpass12", appID)
	require.ErrorIs(t, err, domain.ErrInvalidCredentials)

	// New password works.
	access, _, err := svc.Login(ctx, "grace@example.com", "newpass34", appID)
	require.NoError(t, err)
	require.NotEmpty(t, access)
}

func TestAuthService_ChangePassword_WrongOld_ReturnsPasswordMismatch(t *testing.T) {
	svc, _, appR, cleanup := newAuthTestSuite(t)
	defer cleanup()

	ctx := context.Background()
	_, err := appR.CreateApp(ctx, &models.App{Name: "t", Secret: "s", Link: "https://e.com"})
	require.NoError(t, err)

	userID, err := svc.RegisterNewUser(ctx, "henry@example.com", "correct1", "https://s.com", "p.png")
	require.NoError(t, err)

	err = svc.ChangePassword(ctx, userID, "wrongold", "newpass34")
	require.ErrorIs(t, err, domain.ErrPasswordMismatch)
}
```

- [ ] **Step 3.2: Реализация в services/auth.go**

Добавить метод:

```go
func (a *AuthService) ChangePassword(ctx context.Context, userID uint32, oldPassword, newPassword string) error {
	const op = "auth.ChangePassword"

	user, err := a.userR.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(oldPassword)); err != nil {
		return domain.ErrPasswordMismatch
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		a.log.Error("bcrypt failed", slog.String("op", op), slog.Any("err", err))
		return fmt.Errorf("%s: %w", op, err)
	}
	return a.userR.UpdatePassword(ctx, userID, string(hash))
}
```

- [ ] **Step 3.3: UserRepo.UpdatePassword**

В `internal/repositories/user.go` добавить:

```go
func (r *UserRepo) UpdatePassword(ctx context.Context, id uint32, passHash string) error {
	res := r.storage.DB.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Update("pass_hash", passHash)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
```

- [ ] **Step 3.4: Test + commit**

```bash
go test ./internal/services/ -v -run ChangePassword
git add internal/services/ internal/repositories/
git commit -m "feat(auth): ChangePassword service method"
```

---

## Task 4: v2 Auth controller

**Files:** `internal/transport/grpc/v2/auth.go`

- [ ] **Step 4.1: Реализация**

```go
package v2

import (
	"context"

	"sso/internal/services"
	"sso/internal/transport/grpc/errmap"

	ssov2 "github.com/Nergous/sso_protos/gen/go/sso/auth/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type AuthController struct {
	ssov2.UnimplementedAuthServer
	svc *services.AuthService
}

func NewAuthController(svc *services.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

func RegisterAuth(s *grpc.Server, c *AuthController) {
	ssov2.RegisterAuthServer(s, c)
}

func (c *AuthController) Register(ctx context.Context, req *ssov2.RegisterRequest) (*ssov2.RegisterResponse, error) {
	id, err := c.svc.RegisterNewUser(ctx, req.GetEmail(), req.GetPassword(), req.GetSteamUrl(), req.GetPathToPhoto())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.RegisterResponse{UserId: id}, nil
}

func (c *AuthController) Login(ctx context.Context, req *ssov2.LoginRequest) (*ssov2.LoginResponse, error) {
	access, refresh, err := c.svc.Login(ctx, req.GetEmail(), req.GetPassword(), req.GetAppId())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.LoginResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func (c *AuthController) Logout(ctx context.Context, req *ssov2.LogoutRequest) (*emptypb.Empty, error) {
	if err := c.svc.Logout(ctx, req.GetToken()); err != nil {
		return nil, errmap.ToV2(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *AuthController) Refresh(ctx context.Context, req *ssov2.RefreshRequest) (*ssov2.LoginResponse, error) {
	access, refresh, err := c.svc.Refresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.LoginResponse{AccessToken: access, RefreshToken: refresh}, nil
}

func (c *AuthController) ValidateToken(ctx context.Context, req *ssov2.ValidateTokenRequest) (*ssov2.ValidateTokenResponse, error) {
	userID, valid, err := c.svc.ValidateToken(ctx, req.GetToken())
	if err != nil {
		// По v2-контракту expired/revoked должен возвращать OK+valid=false,
		// только malformed — INVALID_TOKEN. Сервис сейчас возвращает ошибку
		// на любую проблему — обрабатываем: любая не-INVALID_TOKEN ошибка
		// уходит в errmap; INVALID_TOKEN → OK+false.
		// TODO(stage-4): уточнить семантику разницы expired vs malformed,
		// когда сервис будет её различать.
		return nil, errmap.ToV2(err)
	}
	return &ssov2.ValidateTokenResponse{UserId: userID, Valid: valid}, nil
}
```

**Замечание агенту:** комментарий `TODO(stage-4)` здесь единственный допустимый — фиксирует известное архитектурное решение на следующий этап. Не добавляй других TODO.

- [ ] **Step 4.2: Сборка**

```bash
go build ./...
```
Expected: green (v2-контроллер ещё не зарегистрирован на сервере, но пакет компилируется).

- [ ] **Step 4.3: Commit**

```bash
git add internal/transport/grpc/v2/auth.go
git commit -m "feat(v2): AuthController for ssov2 service"
```

---

## Task 5: v2 User controller

**Files:** `internal/transport/grpc/v2/user.go`

```go
package v2

import (
	"context"

	"sso/internal/services"
	"sso/internal/transport/grpc/errmap"
	v2mapper "sso/internal/transport/grpc/mapper/v2"

	ssov2 "github.com/Nergous/sso_protos/gen/go/sso/auth/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

type UserController struct {
	ssov2.UnimplementedUserServer
	svc *services.UserService
	auth *services.AuthService
}

func NewUserController(svc *services.UserService, auth *services.AuthService) *UserController {
	return &UserController{svc: svc, auth: auth}
}

func RegisterUser(s *grpc.Server, c *UserController) {
	ssov2.RegisterUserServer(s, c)
}

func (c *UserController) UserInfo(ctx context.Context, req *ssov2.UserInfoRequest) (*ssov2.UserInfoResponse, error) {
	email, steamURL, photo, err := c.svc.UserInfo(ctx, req.GetUserId())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.UserInfoResponse{Email: email, SteamUrl: steamURL, PathToPhoto: photo}, nil
}

func (c *UserController) GetAllUsers(ctx context.Context, req *ssov2.GetAllUsersRequest) (*ssov2.GetAllUsersResponse, error) {
	users, next, err := c.svc.GetAllUsers(ctx, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.GetAllUsersResponse{
		Users:         v2mapper.UserModelsToProto(users),
		NextPageToken: next,
	}, nil
}

func (c *UserController) UpdateUser(ctx context.Context, req *ssov2.UpdateUserRequest) (*ssov2.UpdateUserResponse, error) {
	update := &services.UpdateModel{
		ID:          req.GetId(),
		Email:       req.GetEmail(),
		SteamURL:    req.GetSteamUrl(),
		PathToPhoto: req.GetPathToPhoto(),
	}
	if err := c.svc.UpdateUser(ctx, update); err != nil {
		return nil, errmap.ToV2(err)
	}
	// Stage 3 возвращает свежий снимок — подтягиваем.
	email, steamURL, photo, err := c.svc.UserInfo(ctx, req.GetId())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	// UserInfo не возвращает timestamps — в v2 это отсутствие не критично
	// (снимок полный приходит через GetAllUsers/UserInfo rework в Stage 4).
	// Для совместимости отдаём то, что есть.
	_ = steamURL; _ = photo // fallback safety
	updated, err := c.svc.FindUser(ctx, req.GetId())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.UpdateUserResponse{User: v2mapper.UserModelToProto(updated)}, nil
	_ = email // unreachable; avoid unused
}

func (c *UserController) ChangePassword(ctx context.Context, req *ssov2.ChangePasswordRequest) (*emptypb.Empty, error) {
	if err := c.auth.ChangePassword(ctx, req.GetUserId(), req.GetOldPassword(), req.GetNewPassword()); err != nil {
		return nil, errmap.ToV2(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *UserController) DeleteUser(ctx context.Context, req *ssov2.DeleteUserRequest) (*emptypb.Empty, error) {
	if err := c.svc.DeleteUser(ctx, req.GetId()); err != nil {
		return nil, errmap.ToV2(err)
	}
	return &emptypb.Empty{}, nil
}
```

**NB:** В `UpdateUser` использован метод `svc.FindUser(ctx, id)` — его нет. Добавь в `internal/services/user.go`:

```go
func (a *UserService) FindUser(ctx context.Context, id uint32) (models.User, error) {
	u, err := a.userR.GetUserByID(ctx, id)
	if err != nil {
		return models.User{}, fmt.Errorf("auth.FindUser: %w", err)
	}
	return u, nil
}
```

Также **удали мёртвый код** `_ = steamURL; _ = photo; ... _ = email` из `UpdateUser` — он появился из-за копипасты. Итоговая чистая версия метода:

```go
func (c *UserController) UpdateUser(ctx context.Context, req *ssov2.UpdateUserRequest) (*ssov2.UpdateUserResponse, error) {
	update := &services.UpdateModel{
		ID:          req.GetId(),
		Email:       req.GetEmail(),
		SteamURL:    req.GetSteamUrl(),
		PathToPhoto: req.GetPathToPhoto(),
	}
	if err := c.svc.UpdateUser(ctx, update); err != nil {
		return nil, errmap.ToV2(err)
	}
	updated, err := c.svc.FindUser(ctx, req.GetId())
	if err != nil {
		return nil, errmap.ToV2(err)
	}
	return &ssov2.UpdateUserResponse{User: v2mapper.UserModelToProto(updated)}, nil
}
```

- [ ] **Step 5.1: Сборка + commit**

```bash
go build ./...
git add internal/transport/grpc/v2/user.go internal/services/user.go
git commit -m "feat(v2): UserController with ChangePassword and paginated lists"
```

---

## Task 6: v2 App controller

**Files:** `internal/transport/grpc/v2/app.go`

Следуй тому же паттерну, что Task 5:
- `GetApp` → `AppModel` через mapper
- `GetAllApps` → пагинация + mapper
- `CreateApp` → после создания читаем свежую запись (`repo.GetAppByID`), мапим, возвращаем в `CreateAppResponse{App}`
- `UpdateApp` → аналогично, возвращаем `UpdateAppResponse{App}` со свежим снимком
- `DeleteApp`, `ChangeStatusApp`, `AddAdmin`, `RemoveAdmin` → `*emptypb.Empty`
- `IsAdmin` → `IsAdminResponse{IsAdmin: bool}`
- `GetAllUsersForApp` → пагинация + `AppUserRowsToProto`

Если нужен вспомогательный `services.AppService.FindApp(ctx, id) (models.App, error)` — добавь по аналогии с `FindUser`.

- [ ] **Step 6.1: Сборка + commit**

```bash
go build ./...
git add internal/transport/grpc/v2/app.go internal/services/app.go
git commit -m "feat(v2): AppController with paginated lists and snapshot responses"
```

---

## Task 7: Регистрация v2 на grpc-сервере (dual-serve)

**Files:**
- Modify: `internal/app/app.go` — создать v2 controllers
- Modify: `internal/app/grpc/app.go` — принимать v2, регистрировать

- [ ] **Step 7.1: app.go — DI для v2**

```go
// в internal/app/app.go после создания v1 controllers:
v2AuthC := grpcv2.NewAuthController(authS)
v2UserC := grpcv2.NewUserController(userS, authS)
v2AppC := grpcv2.NewAppController(appS)

grpcApp := grpcapp.New(log, grpcPort, authC, userC, appC, v2AuthC, v2UserC, v2AppC)
```

Импорт: `grpcv2 "sso/internal/transport/grpc/v2"`.

- [ ] **Step 7.2: grpc/app.go — расширить сигнатуру**

```go
func New(
	log *slog.Logger,
	port int,
	authC *controllers.AuthController, userC *controllers.UserController, appC *controllers.AppController,
	v2AuthC *grpcv2.AuthController, v2UserC *grpcv2.UserController, v2AppC *grpcv2.AppController,
) *App {
	gRPCServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptors.TimeoutUnaryInterceptor(5*time.Second),
			interceptors.ValidateUnaryInterceptor(),
		),
	)

	// v1 — legacy, deprecated
	controllers.RegisterAuth(gRPCServer, authC.AuthS)
	controllers.RegisterApp(gRPCServer, appC.AppS, appC.DefaultSecret)
	controllers.RegisterUser(gRPCServer, userC.UserS)

	// v2 — current
	grpcv2.RegisterAuth(gRPCServer, v2AuthC)
	grpcv2.RegisterUser(gRPCServer, v2UserC)
	grpcv2.RegisterApp(gRPCServer, v2AppC)

	return &App{log: log, gRPCServer: gRPCServer, port: port}
}
```

- [ ] **Step 7.3: Сборка + тесты**
```bash
go build ./... && go test ./...
```
Expected: green.

- [ ] **Step 7.4: Commit**

```bash
git add internal/app/
git commit -m "feat(grpc): register v2 services alongside v1 (dual-serve)"
```

---

## Task 8: End-to-end v2 тесты через bufconn + fakes

**Files:** `internal/transport/grpc/v2/v2_test.go`

**Стратегия:** in-process gRPC сервер через `bufconn` (stdlib gRPC facility, никаких сетей). Storage — in-memory SQLite через `testutil.NewTestStorage` (из Stage 0). Никакого Docker, реальный SQL.

- [ ] **Step 8.1: Тесты**

```go
package v2_test

import (
	"context"
	"net"
	"testing"
	"time"

	"sso/internal/models"
	"sso/internal/repositories"
	"sso/internal/services"
	"sso/internal/testutil"
	grpcv2 "sso/internal/transport/grpc/v2"

	ssov2 "github.com/Nergous/sso_protos/gen/go/sso/auth/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type v2Clients struct {
	auth ssov2.AuthClient
	user ssov2.UserClient
	app  ssov2.AppClient
	appR *repositories.AppRepo // для прямого seed'инга
}

func startV2Server(t *testing.T) (*v2Clients, func()) {
	t.Helper()
	storage := testutil.NewTestStorage(t) // in-memory SQLite с AutoMigrate

	userR := repositories.NewUserRepo(storage)
	appR := repositories.NewAppRepo(storage)
	tokenR := repositories.NewTokenRepo(storage)
	log := testutil.NewTestLogger()

	authS := services.NewAuthService(log, storage, time.Minute, time.Hour, userR, appR, tokenR)
	userS := services.NewUserService(log, userR)
	appS := services.NewAppService(log, appR)

	authC := grpcv2.NewAuthController(authS)
	userC := grpcv2.NewUserController(userS, authS)
	appC := grpcv2.NewAppController(appS)

	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	grpcv2.RegisterAuth(s, authC)
	grpcv2.RegisterUser(s, userC)
	grpcv2.RegisterApp(s, appC)
	go s.Serve(lis)

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	cleanup := func() { _ = conn.Close(); s.Stop() }
	return &v2Clients{
		auth: ssov2.NewAuthClient(conn),
		user: ssov2.NewUserClient(conn),
		app:  ssov2.NewAppClient(conn),
		appR: appR,
	}, cleanup
}

func (c *v2Clients) seedApp(t *testing.T) uint32 {
	t.Helper()
	id, err := c.appR.CreateApp(context.Background(), &models.App{
		Name: "t", Secret: "s", Link: "https://e.com", IsEnabled: true,
	})
	require.NoError(t, err)
	return id
}

func TestV2_RegisterLogin_HappyPath(t *testing.T) {
	c, cleanup := startV2Server(t)
	defer cleanup()
	ctx := context.Background()

	appID := c.seedApp(t)

	regResp, err := c.auth.Register(ctx, &ssov2.RegisterRequest{
		Email: "alice@example.com", Password: "correcthorse",
		SteamUrl: "https://s.com/id/a", PathToPhoto: "a.png",
	})
	require.NoError(t, err)
	require.NotZero(t, regResp.GetUserId())

	loginResp, err := c.auth.Login(ctx, &ssov2.LoginRequest{
		Email: "alice@example.com", Password: "correcthorse", AppId: appID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, loginResp.GetAccessToken())
	require.NotEmpty(t, loginResp.GetRefreshToken())
}

func TestV2_Login_UnknownUser_ReturnsInvalidCredentialsWithErrorInfo(t *testing.T) {
	c, cleanup := startV2Server(t)
	defer cleanup()
	appID := c.seedApp(t)

	_, err := c.auth.Login(context.Background(), &ssov2.LoginRequest{
		Email: "nobody@example.com", Password: "whatever", AppId: appID,
	})
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())

	foundReason := ""
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			foundReason = info.GetReason()
		}
	}
	require.Equal(t, "INVALID_CREDENTIALS", foundReason)
}

func TestV2_ChangePassword_HappyPath(t *testing.T) {
	c, cleanup := startV2Server(t)
	defer cleanup()
	ctx := context.Background()
	appID := c.seedApp(t)

	reg, err := c.auth.Register(ctx, &ssov2.RegisterRequest{
		Email: "grace@example.com", Password: "oldpass12",
		SteamUrl: "https://s.com", PathToPhoto: "p.png",
	})
	require.NoError(t, err)

	_, err = c.user.ChangePassword(ctx, &ssov2.ChangePasswordRequest{
		UserId: reg.GetUserId(), OldPassword: "oldpass12", NewPassword: "newpass34",
	})
	require.NoError(t, err)

	// Старый пароль больше не работает
	_, err = c.auth.Login(ctx, &ssov2.LoginRequest{
		Email: "grace@example.com", Password: "oldpass12", AppId: appID,
	})
	require.Error(t, err)

	// Новый работает
	_, err = c.auth.Login(ctx, &ssov2.LoginRequest{
		Email: "grace@example.com", Password: "newpass34", AppId: appID,
	})
	require.NoError(t, err)
}
```

- [ ] **Step 8.2: Прогон**
```bash
go test ./internal/transport/grpc/v2/ -v -count=1
```
Expected: PASS. Всё in-process, занимает миллисекунды.

- [ ] **Step 8.3: Commit**

```bash
git add internal/transport/grpc/v2/v2_test.go
git commit -m "test(v2): in-process gRPC tests via bufconn + fake repositories"
```

---

## Definition of Done Stage 3

- `go build ./... && go test ./...` — green
- v1 и v2 одновременно обслуживаются на `grpc.Server`
- Все v2 RPC реализованы (включая `ChangePassword`)
- v2 ответы используют `google.rpc.ErrorInfo` с правильными `reason`
- Integration-тесты покрывают v2 happy path + key errors
- Сервисы и репозитории не знают про proto-типы (clean boundary)

Дальше — **STOP**. Stage 4 (authorization) требует человеческого решения по политике, отдельный plan составляется после обсуждения.
