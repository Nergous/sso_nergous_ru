# SSO v2 Migration — Stage 6: Production Hardening

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Precondition: Stage 5 merged to main.

**Goal:** Наблюдаемость + rate limit на Login + безопасное хранение секретов.

**NOT in scope:** Разделение SigningKey и AppSecret с `kid`-versioning — это breaking change для клиентов, отдельный план после stabilization.

**Architecture:** 
- Prometheus метрики через `grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery` и `providers/prometheus`
- HTTP-endpoint на отдельном порту для `/metrics`
- In-memory rate limiter по IP и по email на Login/Register (sliding window)
- Секреты `DB password`, `default_secret` — только из env, не из YAML

**Branch:** `stage-6-hardening`

---

## File Structure

**Создаётся:**
- `internal/observability/metrics.go` — Prometheus registry + интерцептор
- `internal/observability/http.go` — /metrics + /healthz HTTP-сервер
- `internal/transport/grpc/interceptors/ratelimit.go`
- `internal/transport/grpc/interceptors/ratelimit_test.go`

**Модифицируется:**
- `internal/config/config.go` — секция `metrics: {port}`, убрать `default_secret` из YAML (только env)
- `internal/app/app.go` — запуск HTTP-сервера метрик параллельно с gRPC
- `cmd/sso/main.go` — остановка HTTP-сервера в graceful shutdown

---

## Task 1: Branch + baseline

- [ ] **Step 1.1:**
```bash
git checkout main && git pull && git checkout -b stage-6-hardening
go build ./... && go test ./...
```
Expected: green.

---

## Task 2: Prometheus metrics

- [ ] **Step 2.1: deps**

```bash
go get github.com/grpc-ecosystem/go-grpc-middleware/v2@latest
go get github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus@latest
go get github.com/prometheus/client_golang@latest
go mod tidy
```

- [ ] **Step 2.2: observability/metrics.go**

```go
// Package observability exposes Prometheus metrics and health HTTP handlers.
package observability

import (
	grpcprom "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type Metrics struct {
	registry   *prometheus.Registry
	srvMetrics *grpcprom.ServerMetrics
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	srv := grpcprom.NewServerMetrics(grpcprom.WithServerHandlingTimeHistogram())
	reg.MustRegister(srv)
	return &Metrics{registry: reg, srvMetrics: srv}
}

func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

func (m *Metrics) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return m.srvMetrics.UnaryServerInterceptor()
}
```

- [ ] **Step 2.3: observability/http.go**

```go
package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPServer struct {
	log *slog.Logger
	srv *http.Server
}

func NewHTTPServer(log *slog.Logger, port int, m *Metrics) *HTTPServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.Registry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return &HTTPServer{
		log: log,
		srv: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *HTTPServer) MustRun() {
	s.log.Info("observability http server starting", slog.String("addr", s.srv.Addr))
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
```

- [ ] **Step 2.4: Подключить в app**

`internal/config/config.go`:

```go
type MetricsConfig struct {
	Port int `yaml:"port" env:"METRICS_PORT" env-default:"9090"`
}
// в Config:
Metrics MetricsConfig `yaml:"metrics"`
```

`internal/app/app.go` — создать Metrics, HTTPServer, прокинуть интерцептор в grpc-app:

```go
metrics := observability.NewMetrics()
httpSrv := observability.NewHTTPServer(log, cfg.Metrics.Port, metrics)
```

В `grpcapp.New()` добавь параметр `metricsInt grpc.UnaryServerInterceptor` и вставь в chain **первым** (до timeout — чтобы метрики ловили всё, включая timeout-ошибки):

```go
grpc.ChainUnaryInterceptor(
	metrics.UnaryServerInterceptor(),
	interceptors.TimeoutUnaryInterceptor(5*time.Second),
	// ... остальные
),
```

`cmd/sso/main.go` — запуск параллельно и shutdown:

```go
go application.GRPCServer.MustRun()
go application.HTTPServer.MustRun()

// ... после <-stop:
application.GRPCServer.Stop()
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer shutdownCancel()
if err := application.HTTPServer.Shutdown(shutdownCtx); err != nil {
	log.Error("http shutdown", slog.Any("err", err))
}
```

Для этого вынеси `HTTPServer` в структуру `app.App`.

- [ ] **Step 2.5: сборка + тесты + commit**

```bash
go build ./... && go test ./...
git add go.mod go.sum internal/observability/ internal/app/ internal/config/ cmd/sso/
git commit -m "feat(obs): Prometheus metrics + /metrics /healthz HTTP server"
```

---

## Task 3: Rate limit на Login/Register

**Подход:** in-memory sliding window. Быстро, просто, достаточно для single-instance SSO. Если пойдёт масштабирование — позже заменим на Redis.

- [ ] **Step 3.1: interceptor с sliding window**

```go
package interceptors

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type keyFn func(ctx context.Context, req any, fullMethod string) (string, bool)

type window struct {
	mu     sync.Mutex
	events map[string][]time.Time
}

func newWindow() *window { return &window{events: map[string][]time.Time{}} }

func (w *window) allow(key string, max int, per time.Duration, now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	cutoff := now.Add(-per)
	evs := w.events[key]
	kept := evs[:0]
	for _, t := range evs {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= max {
		w.events[key] = kept
		return false
	}
	w.events[key] = append(kept, now)
	return true
}

type RateLimitConfig struct {
	Methods map[string]struct {
		Max int
		Per time.Duration
	}
	Key keyFn
}

func RateLimitUnaryInterceptor(cfg RateLimitConfig) grpc.UnaryServerInterceptor {
	w := newWindow()
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		rule, ok := cfg.Methods[info.FullMethod]
		if !ok {
			return handler(ctx, req)
		}
		key, ok := cfg.Key(ctx, req, info.FullMethod)
		if !ok || key == "" {
			return handler(ctx, req)
		}
		if !w.allow(key+"@"+info.FullMethod, rule.Max, rule.Per, time.Now()) {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// IPKey returns the peer IP as the rate limit key.
func IPKey(ctx context.Context, _ any, _ string) (string, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", false
	}
	return p.Addr.String(), true
}
```

- [ ] **Step 3.2: test (минимум — allow/deny happy path)**

```go
func TestRateLimit_AllowsUpToMaxThenDenies(t *testing.T) {
	w := newWindow() // unexport — тест в том же пакете: _internal_test
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !w.allow("k", 3, time.Second, now) {
			t.Fatalf("should allow attempt %d", i)
		}
	}
	if w.allow("k", 3, time.Second, now) {
		t.Fatal("should deny after max")
	}
	// After window expires:
	if !w.allow("k", 3, time.Second, now.Add(2*time.Second)) {
		t.Fatal("should allow after window")
	}
}
```

Файл `ratelimit_internal_test.go` в том же пакете `interceptors`.

- [ ] **Step 3.3: Подключить в chain**

В `internal/app/grpc/app.go`:

```go
rl := interceptors.RateLimitUnaryInterceptor(interceptors.RateLimitConfig{
	Methods: map[string]struct{ Max int; Per time.Duration }{
		"/sso.auth.v2.Auth/Login":    {Max: 10, Per: time.Minute},
		"/sso.auth.v2.Auth/Register": {Max: 5, Per: time.Minute},
		"/auth.Auth/Login":           {Max: 10, Per: time.Minute},
		"/auth.Auth/Register":        {Max: 5, Per: time.Minute},
	},
	Key: interceptors.IPKey,
})

grpc.ChainUnaryInterceptor(
	metrics.UnaryServerInterceptor(),
	interceptors.TimeoutUnaryInterceptor(5*time.Second),
	rl, // before deprecation — не тратим ресурсы на отсеянные запросы
	interceptors.DeprecationUnaryInterceptor(...),
	interceptors.ValidateUnaryInterceptor(),
	interceptors.AuthUnaryInterceptor(...),
),
```

- [ ] **Step 3.4: сборка + тесты + commit**

```bash
go build ./... && go test ./...
git add internal/transport/grpc/interceptors/
git commit -m "feat(grpc): IP-based rate limit on Login/Register"
```

---

## Task 4: Секреты — только через env

- [ ] **Step 4.1: config — убрать default_secret из YAML**

`internal/config/config.go` — у поля `DefaultSecret`:

```go
DefaultSecret string `env:"DEFAULT_SECRET" env-required:"true"`
```

Убрать `yaml:"default_secret"`. Теперь это **только env**.

- [ ] **Step 4.2: local.yaml — удалить строку**

В `config/local.yaml` удалить `default_secret: ...`. Добавить комментарий в начале файла:

```yaml
# DEFAULT_SECRET is loaded from environment variable only.
# For local dev, export it: DEFAULT_SECRET=dev-only-secret go run ./cmd/sso --config=./config/local.yaml
```

То же для DB password — оставить только env-source (уже так, `env:"PASSWORD"`).

- [ ] **Step 4.3: обновить README/DEV docs**

В `README.md` (если есть) или создать `docs/DEV.md`:

```markdown
# Dev setup

Required env vars:
- `DEFAULT_SECRET` — fallback JWT signing key for apps without own secret
- `PASSWORD` — MariaDB password
- `BOOTSTRAP_ADMIN_EMAIL` (optional) — email of user to promote to system-admin on migration
- `CONFIG_PATH` (or --config flag) — path to YAML config

Example:
```bash
DEFAULT_SECRET=dev-secret PASSWORD=dev-password \
  go run ./cmd/sso --config=./config/local.yaml
```
```

- [ ] **Step 4.4: сборка + тесты + commit**

`go test` упадёт на `MustLoadByPath` если переменная `DEFAULT_SECRET` не выставлена в окружении теста. Решение: testutil выставляет fake env-переменные перед вызовом config в тестах, либо тесты не дергают `config.Load` (они и не должны — они создают сервисы напрямую).

Проверь: `grep -rn "config.MustLoad" --include="*_test.go"` — если пусто, тесты не затронуты.

```bash
go build ./... && go test ./...
git add internal/config/ config/ docs/
git commit -m "feat(config): require secrets via env vars only"
```

---

## Definition of Done Stage 6

- `/metrics` endpoint отдаёт Prometheus-метрики на `METRICS_PORT` (default 9090)
- `/healthz` возвращает 200
- RPM Login/Register rate-limit работает по IP
- `DEFAULT_SECRET` читается только из env
- Health HTTP-сервер корректно останавливается в graceful shutdown
- Все тесты зелёные

**После Stage 6** — v2-миграция технически завершена. Stage 7 (drop GORM) — уже не про миграцию, а про снижение зависимостей.
