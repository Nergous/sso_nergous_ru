# SSO v2 Migration — Master Roadmap

> **For agentic workers:** This is a ROADMAP, not an executable plan. Each numbered stage has (or will have) its own plan file in this directory. Execute stages **in order** — later stages depend on earlier ones.

**Goal:** Переехать c `github.com/Nergous/sso_protos/gen/go/sso` (v1, flat) на `github.com/Nergous/sso_protos/gen/go/sso/auth/v2` без даунтайма, попутно исправив накопленные проблемы архитектуры.

**Стратегия:** Dual-serve — v1 и v2 gRPC-сервисы зарегистрированы одновременно на одном `grpc.Server`. Клиенты переключаются постепенно. v1 удаляется, когда метрики покажут 0 RPS.

**Tech Stack:** Go 1.24, gRPC, GORM + MariaDB, protovalidate-go, google.rpc errdetails, slog, bcrypt, JWT (golang-jwt/jwt/v5), cleanenv.

---

## Почему такой порядок

Миграцию (Stage 3) **нельзя начинать**, пока сервисы возвращают `err.Error()` строками — v2 требует типизированных ошибок через `google.rpc.ErrorInfo`. Поэтому сначала доменные ошибки (Stage 1). Но менять модель ошибок в сервисах без тестов — blind change, поэтому ещё раньше — тесты (Stage 0). Модели/репозитории (Stage 2) меняем до контроллеров v2, потому что v2-ответы требуют `created_at/updated_at` и пагинацию из репо.

---

## Stages

### Stage 0: Подготовка и стабилизация
**Plan file:** `2026-04-18-sso-v2-stage-0-stabilization.md` (создан)

Исправить блокирующие баги и обеспечить testability. Без этого этапа миграция слепая.

- Fix: `Refresh` продолжает работу с истёкшим токеном ([services/auth.go:254](../../internal/services/auth.go#L254))
- Fix: DSN утекает в лог ([config/config.go:83](../../internal/config/config.go#L83))
- Добавить `storage.Close()` в graceful shutdown
- Вынести `context.WithTimeout` в `UnaryServerInterceptor` (убрать 18 дублирований)
- Заменить `*context.Context` на `context.Context` по всему коду
- Testcontainers-based integration-тесты для Auth happy path + ключевые ошибки

**Definition of done:** `go test ./...` проходит, покрыт Auth-сервис, нет `*context.Context`, DSN не светится в логах.

---

### Stage 1: Domain errors + protovalidate
**Plan file:** `2026-04-18-sso-v2-stage-1-errors.md` (TBD — написать после Stage 0)

- Создать `internal/domain/errors.go` с sentinel-ошибками, соответствующими `ErrorReason` enum из v2 protos
- Удалить `lib/serr` — заменить на стандартный `errors.Is/As` + `fmt.Errorf("%w", ...)`
- Переписать все сервисы так, чтобы возвращали **только** доменные ошибки
- Создать `internal/transport/grpc/errmap/`:
  - `errmap.ToV1(err) error` — старый стиль (codes.Internal + текст) для существующих v1-контроллеров
  - `errmap.ToV2(err) error` — новый стиль с `google.rpc.ErrorInfo{Reason, Domain="sso.nergous.ru"}` + `BadRequest` details
- Подключить `protovalidate-go` как `UnaryServerInterceptor` (валидирует только v2-типы с `buf.validate`)
- Удалить ручную валидацию `validateLogin`/`validateRegister` (она дублируется с `protovalidate` после Stage 3, но пока работает для v1)

**Definition of done:** все сервисы используют доменные ошибки, v1-контроллеры продолжают работать через `errmap.ToV1`, тесты Stage 0 зелёные.

---

### Stage 2: Модели, репозитории, миграции
**Plan file:** `2026-04-18-sso-v2-stage-2-data-layer.md` (TBD)

- Добавить `CreatedAt/UpdatedAt` в `User` (`App` уже имеет)
- Починить `Admin`: composite `uniqueIndex:idx_admin_user_app` на `(user_id, app_id)`, обычные индексы на каждое поле отдельно
- Добавить индексы на FK в `RefreshToken` (`user_id`, `app_id`)
- Исправить `App.IsEnabled` default → `true` (или сделать параметром `CreateApp`)
- Переписать `UpdateUser`/`UpdateApp` на `gorm.Updates(map[string]any{...})` с ненулевыми полями, обернуть в транзакцию
- Добавить пагинацию в `GetAllUsers`, `GetAllApps`, `GetAllUsersForApp`:
  - Opaque cursor `base64(last_id:last_created_at)` или простая offset-based с документированным trade-off
  - Clamp `page_size` ∈ [1, 1000], default 50
- Ввести `goose` для миграций, convert AutoMigrate → явные DDL-миграции, снять AutoMigrate в prod-конфиге

**Definition of done:** все репозитории возвращают доменные ошибки из Stage 1, пагинация работает, миграции накатываются через `goose up`.

---

### Stage 3: v2-контроллеры (dual-serve)
**Plan file:** `2026-04-18-sso-v2-stage-3-v2-controllers.md` (TBD)

- Создать `internal/transport/grpc/v2/{auth,user,app}.go` — новые контроллеры
- Создать `internal/transport/grpc/mapper/v2/` — чистый маппинг `models ↔ ssov2`
- Реализовать **новые** v2 RPC:
  - `Auth.Logout` возвращает `emptypb.Empty`
  - `User.ChangePassword(user_id, old_password, new_password)` — новый сервисный метод с bcrypt-проверкой старого
  - `User.UpdateUser` возвращает `UpdateUserResponse{user: UserModel}`
  - `App.UpdateApp`, `App.DeleteApp`, `App.ChangeStatusApp`, `App.AddAdmin`, `App.RemoveAdmin` — правильные возвращаемые типы (`AppModel`/`Empty`)
  - Пагинация в `GetAllUsers`/`GetAllApps`/`GetAllUsersForApp`
- Регистрация: `ssov2.RegisterAuthServer(s, v2AuthCtrl); ssov1.RegisterAuthServer(s, v1AuthCtrl)` — оба на одном `grpc.Server`
- Все ошибки через `errmap.ToV2`

**Definition of done:** v2-сервисы отвечают на все RPC, integration-тесты покрывают v2 happy path, v1 продолжает работать параллельно.

---

### Stage 4: Authorization layer
**Plan file:** `2026-04-18-sso-v2-stage-4-authz.md` (TBD)

**Требует отдельной дискуссии** до написания плана — нужно зафиксировать политику: кто может удалять юзеров, кто создаёт приложения, как отличить "system admin" от "app admin".

- `AuthInterceptor`: парсит `authorization: Bearer <token>`, валидирует, кладёт `user_id/app_id/is_admin` в context
- Whitelist методов без авторизации: `Login`, `Register`, `Refresh`, `ValidateToken`
- Authz-проверки внутри v2-сервисов: `DeleteUser` — self-only или system-admin, `UpdateUser` — self или app-admin, etc.

---

### Stage 5: Переезд клиентов
**Plan file:** `2026-04-18-sso-v2-stage-5-cutover.md` (TBD)

- Changelog + deprecation notice на v1
- Мониторинг RPS на v1 vs v2
- Когда v1 RPS = 0 на протяжении N дней — удалить v1-контроллеры, v1-импорты, `errmap.ToV1`

---

### Stage 6: Production hardening
**Plan file:** `2026-04-18-sso-v2-stage-6-hardening.md` (TBD)

- Prometheus metrics (`grpc_prometheus`)
- OpenTelemetry tracing (OTLP interceptor)
- Rate limiting на `Login` (по IP + по email)
- Секреты из env/secret-manager, удалить `default_secret` из YAML
- Рассмотреть разделение `SigningKey` и `AppSecret` с `kid` в JWT (breaking change → v3 или backward-compat поле)

---

## Правила для каждого Stage

1. **TDD:** ни одна задача не мержится без теста, написанного ДО кода.
2. **Коммиты частые:** каждый Step коммитится отдельно (стиль Conventional Commits: `feat:`, `fix:`, `refactor:`, `test:`, `chore:`).
3. **Dual-serve всегда:** пока мы в Stage 1-4, v1 обязан работать. Любой PR, который ломает v1, отклоняется.
4. **Никаких `git push --force` в main** и никаких `--no-verify`.
