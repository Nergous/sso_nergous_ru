# Night Run Report — 2026-04-19

## Результат: STOPPED

## Стейдж: stage-0 — stabilization

## PR (with [STOPPED] marker): https://github.com/Nergous/sso_nergous_ru/pull/1

## Где остановился

Task 8, Step 8.5. Команда: `go test ./internal/services/ -v -run TestAuthService -count=1`.
Integration-тесты не могут запустить контейнер MariaDB, потому что Docker daemon
не стартует в текущей VM.

## Что успешно сделано до остановки

- Task 1 (baseline): ветка создана, baseline build/vet зелёные.
- Task 2: `Refresh` с истёкшим токеном возвращает `ErrTokenExpired` (auth.go).
  Добавлена sentinel-ошибка `services.ErrTokenExpired`.
- Task 3: `log.Print(dsn)` удалён из `internal/config/config.go`, убран импорт `log`.
- Task 4: `fmt.Println(rows.Error)` удалён из `repositories.UserRepo.CreateUser`,
  убран импорт `fmt`.
- Task 5: `*context.Context` → `context.Context` выполнен механически по
  всему коду (services, repositories, controllers). `grep -rn '\*context\.Context' internal/`
  возвращает пусто.
- Task 6: `TimeoutUnaryInterceptor` создан в `internal/transport/grpc/interceptors`,
  подключён в `internal/app/grpc/app.go` через `grpc.UnaryInterceptor(...)`,
  дублирующие `context.WithTimeout(...)` удалены из всех 18 RPC-методов
  в auth/user/app контроллерах. Unit-тесты на interceptor проходят.
- Task 7: `Storage` теперь живёт в `app.App`, `cmd/sso/main.go` вызывает
  `Storage.Close()` на shutdown.
- Task 8: код `internal/testutil/db.go`, `internal/testutil/logger.go`,
  `internal/services/auth_integration_test.go` написан полностью по плану
  и компилируется. Добавлены зависимости: `testcontainers-go`,
  `testcontainers-go/modules/mariadb`, `stretchr/testify`. `go.mod` обновлён.
  Сами тесты не могут выполниться.

## Что сломалось

- команда: `docker info`
- ожидал: работающий daemon
- получил:
  ```
  Client: Docker Engine - Community
     Version:    29.3.1
     ...
  Server:
  failed to connect to the docker API at unix:///var/run/docker.sock;
  check if the path is correct and if the daemon is running: dial unix
  /var/run/docker.sock: connect: no such file or directory
  ```
- ручной запуск `dockerd` падает с:
  ```
  failed to start daemon: Error initializing network controller: error obtaining
  controller instance: failed to register "bridge" driver: failed to create NAT
  chain DOCKER: iptables failed: iptables --wait -t nat -N DOCKER:
  iptables: Failed to initialize nft: Protocol not supported
  ```
  Ядро VM не поддерживает nftables / netlink, поэтому containerd запускается,
  но bridge-сеть создать нельзя. Также `ulimit` падает при старте init-скрипта
  из-за rlimit-квот.
- как следствие, `go test ./internal/services/ -v -run TestAuthService` падает
  с `rootless Docker not found, failed to create Docker provider` на каждом
  из 8 тестов.

## Состояние

- ветка: `claude/stage-0-stabilization`
- последний коммит: `30df25e test(auth): add integration tests for AuthService via testcontainers`
- MANIFEST: stage-0 помечен `- [~]` в этой ветке (НЕ в main)
- `go build ./...` — exit 0
- `go vet ./...` — exit 0
- `go test ./...` — **красный** (только из-за Docker; unit-тесты на interceptor зелёные)

## Мои гипотезы

VM для ночного исполнения стейджа стартанула без docker daemon — инструкция
говорит "Docker доступен", но по факту нужен был либо docker-in-docker,
либо предварительно запущенный daemon. Возможно, раннее в образе docker
запускался hook'ом, а в этой конкретной сборке hook не отработал. Также
может быть, что VM специально не даёт прав на nftables / iptables
(seccomp или user namespace без NET_ADMIN).

Если запустить эту же ветку на машине с обычным Docker (VM или хост с
linuxkit), все 8 интеграционных тестов должны пройти — код тестов полностью
соответствует плану, компилируется, и логика на уровне сервиса корректна
(ошибки `ErrInvalidCredentials` / `ErrTokenExpired` уже приведены к
sentinel-видам в Task 2).

## Что не пробовал

- Переписывать тесты на sqlite/in-memory — нарушило бы спецификацию Stage 0
  ("реальная MariaDB") и было бы shortcut.
- Ставить `dockerd` через modprobe / добавлять sysctl'ы — VM этого не
  позволяет (нет systemd, нет CAP_NET_ADMIN).
- `--no-verify` / `t.Skip()` на интеграционных тестах — это прямо запрещено
  правилами ("НЕ удаляй тесты чтобы сделать зелёным").
- Менять версии зависимостей кроме тех, что прописаны в Task 8 (`go get …@latest`).

## Что делать человеку утром

1. Посмотреть PR `[Stage 0][STOPPED] stage-0 stabilization`.
2. Прогнать `go test ./internal/services/ -v -run TestAuthService -count=1`
   на машине с работающим docker daemon. Ожидается 8 PASS.
3. Если все зелёные — обновить MANIFEST.md `- [~]` → `- [x]` в main
   (commit "chore(manifest): mark stage-0 done") и смержить PR merge-commit'ом
   (не squash, чтобы сохранить историю Tasks 2..8).
4. Следующая ночь подхватит stage-1.

Либо, если проще — починить ночной runner (поднять docker daemon в VM
через `dockerd-rootless-setuptool` или через docker-in-docker wrapper),
и перезапустить этот же стейдж: агент увидит закрытый STOPPED PR и
мерженый state, ничего не сломает.
