# Migration Plans Manifest

**Как это работает:** каждый стейдж = один plan-файл = одна ветка = один merge-коммит в main. Agent читает этот manifest, берёт **первый несделанный** стейдж, исполняет его план, после успешного мержа обновляет здесь checkbox на `- [x]` и коммитит обновлённый manifest в main.

**Status legend:**
- `- [ ]` — todo, ещё не запускался
- `- [~]` — в процессе (ветка создана, не смержена) — агент к нему НЕ прикасается без ручного сигнала
- `- [x]` — done, смержено в main
- `- [!]` — blocked, смотри заметку рядом

## Stages (strict order)

- [ ] **stage-0** — [2026-04-18-sso-v2-stage-0-stabilization.md](2026-04-18-sso-v2-stage-0-stabilization.md) — fix bugs + tests
- [ ] **stage-1** — [2026-04-18-sso-v2-stage-1-errors.md](2026-04-18-sso-v2-stage-1-errors.md) — domain errors + protovalidate
- [ ] **stage-2** — [2026-04-18-sso-v2-stage-2-data-layer.md](2026-04-18-sso-v2-stage-2-data-layer.md) — timestamps, indexes, pagination, goose
- [ ] **stage-3** — [2026-04-18-sso-v2-stage-3-v2-controllers.md](2026-04-18-sso-v2-stage-3-v2-controllers.md) — v2 controllers, dual-serve
- [ ] **stage-4** — [2026-04-18-sso-v2-stage-4-authz.md](2026-04-18-sso-v2-stage-4-authz.md) — authz, system-admin
- [ ] **stage-5** — [2026-04-18-sso-v2-stage-5-deprecation.md](2026-04-18-sso-v2-stage-5-deprecation.md) — v1 deprecation infra
- [ ] **stage-6** — [2026-04-18-sso-v2-stage-6-hardening.md](2026-04-18-sso-v2-stage-6-hardening.md) — metrics, rate limit, secrets
- [ ] **stage-7** — [2026-04-18-sso-v2-stage-7-drop-gorm.md](2026-04-18-sso-v2-stage-7-drop-gorm.md) — multi-backend storage (MariaDB + SQLite) + drop GORM

## Human notes

- Stage 4 содержит policy assumptions в начале файла — прочитать до первой ночи, которая до него дойдёт.
- Stage 7 идёт последним намеренно — дешевле делать на стабилизированной кодовой базе с покрытием.
- Stage 5 НЕ выполняет реальный cutover клиентов — только готовит инфраструктуру. Фактическое удаление v1 — ручной шаг из `docs/CUTOVER.md` позже.
- Тесты не требуют Docker: Stage 0 вводит in-memory SQLite через `glebarez/sqlite` (GORM-driver, pure-Go, без CGO). Stage 7 мигрирует helper на raw `modernc.org/sqlite` — тесты сервисов не меняются. Всё гоняется в эфемерной VM scheduled агента.
