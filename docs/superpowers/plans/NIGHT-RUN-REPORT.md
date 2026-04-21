# Night Run Report — 2026-04-21

## Результат: SUCCESS

## Стейдж: stage-1 — domain errors + protovalidate

## Ссылки

- PR: https://github.com/Nergous/sso_nergous_ru/pull/3
- Branch: `claude/stage-1-errors`
- Session transcript: https://claude.ai/code/routines (текущий run)

## Сделано

- коммитов на ветке: 9 (manifest-in-progress + 7 task commits + manifest-done)
- acceptance: build ✅ vet ✅ test ✅
- DoD: все пункты плана отмечены
  - `go build ./... && go vet ./... && go test ./...` — зелёные
  - В сервисах нет `serr.*`, нет inline `errors.New` для доменных условий — только `internal/domain/errors.go`
  - В контроллерах ни одного `err.Error()` в `status.Error` (grep: нет совпадений)
  - `lib/serr/` не существует
  - `protovalidate` interceptor подключён, chain `Timeout → Validate`
  - v1-поведение сохранено: `codes.Unauthenticated` для `ErrInvalidCredentials`, `codes.NotFound` для `Err*NotFound`, и т.д.; raw-текст из БД больше не утекает в `Internal`

### Commits

```
da20480 chore(manifest): mark stage-1 done
ae6c2ad chore: remove obsolete lib/serr package
2911d17 feat(grpc): add protovalidate unary interceptor
2b42a39 refactor(controllers): route errors through errmap.ToV1
ba1dc37 refactor(services): use domain errors exclusively
0f64f60 refactor(repo): return domain errors instead of raw gorm errors
db6d424 feat(errmap): map domain errors to v1/v2 gRPC statuses
64defb0 feat(domain): introduce sentinel errors mirroring v2 ErrorReason
7661eef chore(manifest): mark stage-1 in progress
```

## Что было необычного

- При `go get google.golang.org/genproto/googleapis/rpc/errdetails@latest` (Step 3.1) toolchain автоматически переключился на `go1.25.9` — актуальная версия genproto требует `go >= 1.25`. Директива `go` в `go.mod` обновилась с 1.24.0 → 1.25.0. Это прямое следствие `go get` из плана, других версий не трогал. Acceptance остался зелёным.
- Чтобы intermediate-коммит `refactor(services): use domain errors exclusively` собирался, пришлось одной строкой обновить `internal/controllers/auth.go` (свитч `services.ErrInvalidCredentials` → `domain.ErrInvalidCredentials`). Полная замена на `errmap.ToV1` уехала в следующий коммит Task 6, как и предусмотрено планом.
- `isDuplicate` в `internal/repositories/errors.go` опирается на `*mysql.MySQLError` (код 1062) — как и указано в плане. На SQLite (in-memory в тестах) duplicate-check через этот хелпер не срабатывает, но существующий Stage 0 тест `TestAuthService_RegisterDuplicateEmail_Fails` проверяет только `require.Error`, так что он остаётся зелёным.

## Что делать человеку утром

1. Прочитать PR #3 (описание + diff)
2. Прогнать глазами 9 коммитов ветки
3. Смержить PR merge-commit'ом (не squash — чтобы сохранить историю Tasks)
4. После merge: MANIFEST в main автоматически обновится (он в PR)
5. Следующая ночь подхватит stage-2 (data layer)
