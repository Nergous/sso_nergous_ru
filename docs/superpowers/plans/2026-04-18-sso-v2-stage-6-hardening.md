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

## Task 4: Секреты + .env + driver config

- [ ] **Step 4.1: Зависимость godotenv (если не в транзитиве)**

```bash
go get github.com/joho/godotenv@latest
go mod tidy
```

(Уже в transitive через cleanenv, но сделаем прямой зависимостью — читаем `.env` в main).

- [ ] **Step 4.2: config — убрать default_secret из YAML, добавить Driver**

`internal/config/config.go`:

```go
type Database struct {
	Driver     string `yaml:"driver" env:"DB_DRIVER" env-default:"mariadb"`
	Host       string `yaml:"host" env:"DB_HOST" env-default:"localhost"`
	Port       int    `yaml:"port" env:"DB_PORT" env-required:"true"`
	UsernameDB string `yaml:"username-db" env:"DB_USERNAME" env-required:"true"`
	Password   string `yaml:"password" env:"DB_PASSWORD"`
	DBName     string `yaml:"dbname" env:"DB_NAME" env-default:"sso"`
}

type Config struct {
	// ... остальное ...
	DefaultSecret string `env:"DEFAULT_SECRET" env-required:"true"` // yaml tag убран
}
```

Строго говоря — `DefaultSecret` больше не читается из YAML.

- [ ] **Step 4.3: .env loader в main**

`cmd/sso/main.go` — в самом начале `main()`:

```go
import "github.com/joho/godotenv"

func main() {
	// Load .env if present (silent on absence — .env is dev convenience only).
	_ = godotenv.Load()

	cfg := config.MustLoad()
	// ... existing code ...
}
```

**Policy:** `.env` load делаем BEST-EFFORT — если файла нет, работаем дальше. В prod `.env` может отсутствовать, секреты придут через systemd env / k8s secrets / etc.

- [ ] **Step 4.4: local.yaml + .env.example**

Очистить `config/local.yaml` от секретов:

```yaml
# Secrets are loaded from environment variables (or .env file).
# See .env.example for the full list.
env: local
token_ttl: 1h
refresh_ttl: 720h
grpc:
  port: 44044
  timeout: 5s
metrics:
  port: 9090
v1_sunset_date: ""  # ISO-8601; empty = no sunset
database:
  driver: mariadb
  host: localhost
  port: 3306
  dbname: sso
```

Create `.env.example` в корне репо:

```bash
# Copy to .env for local development. Never commit .env.

# Database
DB_DRIVER=mariadb           # mariadb | sqlite | postgres (when implemented)
DB_USERNAME=sso
DB_PASSWORD=changeme

# JWT fallback secret — if app has no own Secret, this is used.
# Rotate in production.
DEFAULT_SECRET=dev-only-change-me

# Bootstrap (optional): email of the user promoted to system-admin on migration.
# BOOTSTRAP_ADMIN_EMAIL=admin@example.com
```

Добавить `.env` в `.gitignore` (проверь что его ещё нет):
```bash
echo ".env" >> .gitignore
```

- [ ] **Step 4.5: README / docs/DEV.md**

Create `docs/DEV.md`:

```markdown
# Dev setup

## Prerequisites
- Go 1.24+
- MariaDB 11+ running locally (or SQLite for tests — no setup needed)

## Configuration

Config comes from three sources, in priority order:

1. **Env vars** (highest) — including those loaded from `.env`
2. **YAML file** at `CONFIG_PATH` or `--config=...`
3. **Defaults** in struct tags

Secrets (`DB_PASSWORD`, `DEFAULT_SECRET`) are **env-only** — never commit them.

### Quick start

```bash
cp .env.example .env
# Edit .env with your values
go run ./cmd/sso --config=./config/local.yaml
```

## Env vars

See `.env.example` for the authoritative list with descriptions.
```

- [ ] **Step 4.6: Тесты**

Тесты Stage 0 не трогают `config.MustLoad` — они собирают сервисы напрямую. Проверь:

```bash
grep -rn "config.MustLoad\|config.MustLoadByPath" --include="*_test.go"
```

Expected: пусто. Если нет — тесты не затронуты.

```bash
go build ./... && go test ./...
```

- [ ] **Step 4.7: Commit**

```bash
git add go.mod go.sum internal/config/ config/ cmd/sso/main.go .env.example .gitignore docs/
git commit -m "feat(config): env-only secrets, .env support, driver selector"
```

---

## Definition of Done Stage 6

- `/metrics` endpoint отдаёт Prometheus-метрики на `METRICS_PORT` (default 9090)
- `/healthz` возвращает 200
- RPM Login/Register rate-limit работает по IP
- `DEFAULT_SECRET`, `DB_PASSWORD` — только из env
- `.env` загружается в `main` (best-effort), есть `.env.example` и `.env` в `.gitignore`
- Новое поле `Database.Driver` читается из env/yaml с default `mariadb` (подготовка к Stage 7)
- Health HTTP-сервер корректно останавливается в graceful shutdown
- Все тесты зелёные

**После Stage 6** — v2-миграция функционально завершена. Stage 7 = архитектурный рефакторинг (multi-backend storage + drop GORM).
