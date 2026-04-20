# Night Run Report — 2026-04-20

## Результат: SUCCESS

## Стейдж: stage-0 — Stabilization (fix bugs + tests)

## Ссылки

- PR: https://github.com/Nergous/sso_nergous_ru/pull/2
- Branch: `claude/stage-0-stabilization`
- Session transcript: https://claude.ai/code/routines (текущий run)

## Сделано

- коммитов на ветке: 10 (8 task commits + `chore(manifest): mark stage-0 in progress` + `chore(manifest): mark stage-0 done`)
- acceptance: build ✅ vet ✅ test ✅
- DoD: все пункты плана отмечены
- 9 integration-тестов AuthService + 2 unit-теста TimeoutInterceptor, все зелёные (<1s, без Docker)

### Commits

```
f3d242b chore(manifest): mark stage-0 done
2418bbf test(auth): SQLite-backed integration tests (no Docker required)
b4be023 feat(shutdown): close storage on graceful shutdown
9a36891 refactor(grpc): extract timeout into unary interceptor
c36b174 refactor: pass context.Context by value, not by pointer
a9dd367 chore(repo): remove stray fmt.Println from CreateUser
e80bc18 fix(config): stop logging DSN with credentials
e3446be fix(auth): return error when refreshing with expired token
4cf8d8e chore(manifest): mark stage-0 in progress
```

## Что было необычного

- Предыдущий ночной запуск (PR #1) был закрыт со статусом `[STOPPED]` из-за неработающего Docker daemon. После этого план был обновлён на in-memory SQLite (`github.com/glebarez/sqlite`, pure-Go, без CGO). В этот раз всё прошло гладко по плану: 9 integration-тестов AuthService отрабатывают за <1s без контейнеров.
- Интерфейсы репозиториев (`UserRepository`, `AppRepository`, `TokenRepository`) выровнены под реальные сигнатуры concrete-типов, а не под пример из плана (как и указано в плане: "Если интерфейс расходится — корректируй интерфейс под существующие методы"). Отличия:
  - `AppRepository.GetAppByID` → `*models.App` (не `models.App`)
  - `AppRepository.UpdateApp` принимает `*models.App`
  - `AppRepository.AddAdmin` принимает `*models.Admin` (не userID/appID)
  - `AppRepository.GetAllUsersForApp` → `[]models.AppUser` (не `[]models.User`)
  - `TokenRepository.CreateRefreshToken` → `(*models.RefreshToken, error)` (не `(uint32, error)`)
- `Refresh`-логика: по плану после истечения токена сразу возвращается `ErrTokenExpired`, ошибка удаления expired-токена логируется через `a.log.Warn` (не прерывает flow).

## Что делать человеку утром

1. Прочитать PR #2 (описание + diff)
2. Прогнать глазами 10 коммитов ветки
3. Смержить PR merge-commit'ом (не squash — чтобы сохранить историю Tasks)
4. После merge: MANIFEST в main автоматически обновится (он в PR)
5. Следующая ночь подхватит stage-1 (domain errors + protovalidate)
