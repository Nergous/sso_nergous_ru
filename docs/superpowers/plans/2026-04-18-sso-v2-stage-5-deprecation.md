# SSO v2 Migration — Stage 5: v1 Deprecation Infrastructure

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 4 merged to main.

**Goal:** Пометить v1 как deprecated в коде и наблюдаемости. Дать инструменты для безопасного cutover'а: логи с пометкой v1/v2, метрики по версиям, sunset-дата в конфиге.

**NOT in scope:** удаление v1 — это делается руками после того, как мониторинг покажет 0 RPS на v1. Этот стейдж только готовит почву.

**Architecture:** Отдельный UnaryInterceptor `DeprecationLoggerInterceptor` логирует все обращения к v1-методам с уровнем WARN. Конфиг получает поле `v1_sunset_date` — после этой даты v1 RPC возвращает `FAILED_PRECONDITION` с текстом "v1 API has been sunset, use v2".

**Branch:** `stage-5-deprecation`

---

## File Structure

**Создаётся:**
- `internal/transport/grpc/interceptors/deprecation.go`
- `internal/transport/grpc/interceptors/deprecation_test.go`
- `docs/CUTOVER.md` — инструкция для человека, как перевести клиентов и удалить v1

**Модифицируется:**
- `internal/config/config.go` — добавить `V1SunsetDate time.Time`
- `config/local.yaml` — пример с sunset_date в далёком будущем
- `internal/app/grpc/app.go` — подключить deprecation interceptor перед auth

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-5-deprecation
go build ./... && go test ./...
```
Expected: green.

---

## Task 2: Deprecation interceptor

**Files:** `internal/transport/grpc/interceptors/deprecation.go`, `_test.go`

- [ ] **Step 2.1: test**

```go
package interceptors_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"sso/internal/transport/grpc/interceptors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDeprecation_V1LogsWarn(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	interceptor := interceptors.DeprecationUnaryInterceptor(log, time.Time{})

	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/auth.Auth/Login"}, handler)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "deprecated") || !strings.Contains(buf.String(), "/auth.Auth/Login") {
		t.Fatalf("expected deprecation warn log, got: %s", buf.String())
	}
}

func TestDeprecation_V2DoesNotLog(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	interceptor := interceptors.DeprecationUnaryInterceptor(log, time.Time{})

	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/sso.auth.v2.Auth/Login"}, handler)
	if buf.Len() != 0 {
		t.Fatalf("v2 should not log: %s", buf.String())
	}
}

func TestDeprecation_AfterSunsetReturnsFailedPrecondition(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	interceptor := interceptors.DeprecationUnaryInterceptor(log, time.Now().Add(-time.Hour))

	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/auth.Auth/Login"}, handler)
	if err == nil {
		t.Fatal("expected error after sunset")
	}
	if st, _ := status.FromError(err); st.Code() != codes.FailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", st.Code())
	}
}

func TestDeprecation_V2UnaffectedBySunset(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	interceptor := interceptors.DeprecationUnaryInterceptor(log, time.Now().Add(-time.Hour))

	handler := func(ctx context.Context, req any) (any, error) { return "ok", nil }
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/sso.auth.v2.Auth/Login"}, handler)
	if err != nil {
		t.Fatalf("v2 should not be sunset: %v", err)
	}
}
```

- [ ] **Step 2.2: impl**

```go
package interceptors

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DeprecationUnaryInterceptor logs a warning on every v1 call and, after
// sunsetAt, rejects v1 calls with FAILED_PRECONDITION. v2 calls pass through.
// A zero sunsetAt disables the hard cutoff (only logging remains).
func DeprecationUnaryInterceptor(log *slog.Logger, sunsetAt time.Time) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !isV1(info.FullMethod) {
			return handler(ctx, req)
		}
		if !sunsetAt.IsZero() && time.Now().After(sunsetAt) {
			return nil, status.Error(codes.FailedPrecondition, "v1 API has been sunset, use v2")
		}
		log.Warn("deprecated v1 API call", slog.String("method", info.FullMethod))
		return handler(ctx, req)
	}
}

func isV1(fullMethod string) bool {
	// v1 methods live under /auth.Auth, /auth.User, /auth.App
	return strings.HasPrefix(fullMethod, "/auth.")
}
```

- [ ] **Step 2.3: test + commit**

```bash
go test ./internal/transport/grpc/interceptors/ -v
git add internal/transport/grpc/interceptors/deprecation.go internal/transport/grpc/interceptors/deprecation_test.go
git commit -m "feat(grpc): deprecation interceptor with sunset date for v1"
```

---

## Task 3: Конфиг + подключение

- [ ] **Step 3.1: config**

В `internal/config/config.go` добавить поле в struct `Config`:

```go
V1SunsetDate time.Time `yaml:"v1_sunset_date"`
```

(без `env-required` — опциональное; zero value = sunset не наступил).

В `config/local.yaml` добавить комментарий-пример:

```yaml
# v1_sunset_date: 2027-01-01T00:00:00Z
```

- [ ] **Step 3.2: интерцептор в chain**

`internal/app/grpc/app.go` — chain становится:

```go
grpc.ChainUnaryInterceptor(
	interceptors.TimeoutUnaryInterceptor(5*time.Second),
	interceptors.DeprecationUnaryInterceptor(log, cfg.V1SunsetDate),
	interceptors.ValidateUnaryInterceptor(),
	interceptors.AuthUnaryInterceptor(authS, userR),
),
```

(Deprecation идёт между timeout и validate: решение о rejection не требует валидации payload.)

Это значит `New()` должен принимать `log` и `cfg.V1SunsetDate` — прокинь соответствующие параметры от `app.New`.

- [ ] **Step 3.3: сборка + тесты + commit**

```bash
go build ./... && go test ./...
git add internal/
git commit -m "feat(config): wire v1 deprecation interceptor with sunset date"
```

---

## Task 4: CUTOVER.md

**Files:** `docs/CUTOVER.md`

- [ ] **Step 4.1: Write**

```markdown
# v1 → v2 Cutover Guide

## Критерии готовности к удалению v1

1. Все клиенты используют v2. Проверка:
   - Логи warning `deprecated v1 API call` — 0 за последние 7 дней
   - Метрика `grpc_server_started_total{grpc_service=~"auth\\..*"}` — 0 rps
2. Метрики ошибок v2 стабильны (< baseline × 1.1) минимум 2 недели
3. Выделено окно для релиза (идеально — не в пятницу)

## Процедура sunset

### Шаг 1 (за 4 недели до sunset): объявить дату

- [ ] Установить `v1_sunset_date` в конфиге prod (YAML или env `V1_SUNSET_DATE=YYYY-MM-DDTHH:MM:SSZ`)
- [ ] Клиенты ещё работают, но каждый v1-вызов логируется WARN
- [ ] Разослать уведомления ответственным командам

### Шаг 2 (sunset-день)

- [ ] Интерцептор начинает возвращать `FAILED_PRECONDITION` на v1
- [ ] Мониторить входящий трафик — если кто-то всё ещё бьёт v1, быстро вернуть дату вперёд

### Шаг 3 (через неделю после sunset): удалить код

- [ ] Создать ветку `remove-v1`
- [ ] Удалить `internal/controllers/{auth,user,app}.go` (v1-контроллеры)
- [ ] Удалить `controllers.RegisterAuth/App/User` вызовы в `internal/app/grpc/app.go`
- [ ] Удалить зависимость `github.com/Nergous/sso_protos/gen/go/sso` (v1 flat) из импортов — останется только `/gen/go/sso/auth/v2`
- [ ] Удалить `errmap.ToV1` — останется только `ToV2`
- [ ] Удалить `DeprecationUnaryInterceptor` и `V1SunsetDate` из конфига
- [ ] Удалить префикс `/auth.` из whitelist AuthInterceptor (там останется только `/sso.auth.v2.`)
- [ ] `go mod tidy`
- [ ] Запустить полный test suite
- [ ] PR на main
```

- [ ] **Step 4.2: Commit**

```bash
git add docs/CUTOVER.md
git commit -m "docs: add v1 cutover procedure"
```

---

## Definition of Done Stage 5

- Build/test зелёные
- v1-вызовы логируются WARN с методом
- v2-вызовы идут тихо
- Sunset-дата из конфига применяется: после неё v1 → `FAILED_PRECONDITION`, v2 не затронут
- Документ `docs/CUTOVER.md` описывает процедуру
