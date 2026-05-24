# SSO

Single Sign-On gRPC service for nergous.ru.

## Quick start

```bash
task key:generate                # Ed25519 keypair under keys/
cp .env.example .env             # fill in DB creds + SSO_SEED_* vars
task db:create                   # create the MariaDB database
task migrate:up                  # apply schema
task seed                        # seed sso-admin app + roles + first admin user
task run                         # run the gRPC server
```

## Configuration

- `config/config.yaml` — base config (env, log, gRPC, DB, JWT, audit).
- `.env` — overrides + CLI utility credentials. Loaded automatically by every
  binary via `godotenv.Load()`. **Git-ignored**. See `.env.example`.

CLI utilities (`cmd/migrate`, `cmd/seed`, `cmd/audit-purge`) resolve DB params
in order: **flag → env var → default**.

| Flag        | Env var       | Default     |
| ----------- | ------------- | ----------- |
| `-host`     | `DB_HOST`     | `127.0.0.1` |
| `-port`     | `DB_PORT`     | `3306`      |
| `-user`     | `DB_USERNAME` | `root`      |
| `-password` | `DB_PASSWORD` | (empty)     |
| `-db`       | `DB_NAME`     | `sso`       |
| `-tls`      | `DB_TLS`      | `false`     |

## Commands

Each command is available as a `task` (recommended) or a raw `go run` invocation.

### Server

| Task         | Raw command                               |
| ------------ | ----------------------------------------- |
| `task run`   | `go run ./cmd/sso -cp config/config.yaml` |
| `task build` | `go build ./...`                          |

### Database management

| Task                                     | Raw command                                                                         |
| ---------------------------------------- | ----------------------------------------------------------------------------------- |
| `task db:create`                         | `go run ./cmd/migrate -cmd create-db`                                               |
| `task db:drop`                           | `go run ./cmd/migrate -cmd drop-db`                                                 |
| `task db:reset`                          | `db:drop` + `db:create` + `migrate:up`                                              |
| `task migrate:up`                        | `go run ./cmd/migrate -migrations ./migrations/mariadb -cmd up`                     |
| `task migrate:down`                      | `go run ./cmd/migrate -migrations ./migrations/mariadb -cmd down -steps 1`          |
| `task migrate:version`                   | `go run ./cmd/migrate -migrations ./migrations/mariadb -cmd version`                |
| `task migrate:drop`                      | `go run ./cmd/migrate -migrations ./migrations/mariadb -cmd drop`                   |
| `task migrate:force -- -force-version N` | `go run ./cmd/migrate -migrations ./migrations/mariadb -cmd force -force-version N` |

`migrate:drop` wipes all tables including `schema_migrations`; `db:drop` removes
the entire database.

### Operations

#### Seed admin

Provisions the `sso-admin` app, 6 admin roles, and one admin user with all
roles assigned. Idempotent — repeated runs print `existed` lines and change
nothing.

| Task        | Raw command         |
| ----------- | ------------------- |
| `task seed` | `go run ./cmd/seed` |

Required env vars (in `.env`):

| Variable                      | Required | Description                        |
| ----------------------------- | -------- | ---------------------------------- |
| `SSO_SEED_ADMIN_EMAIL`        | ✓        | Admin email (UNIQUE)               |
| `SSO_SEED_ADMIN_PASSWORD`     | ✓        | Plaintext password (bcrypt-hashed) |
| `SSO_SEED_ADMIN_USERNAME`     | ✓        | Admin username (UNIQUE)            |
| `SSO_SEED_ADMIN_APP_LINK`     | —        | Default `https://sso-admin.local/` |
| `SSO_SEED_ADMIN_DISPLAY_NAME` | —        | Default `Admin`                    |
| `SSO_SEED_ADMIN_BCRYPT_COST`  | —        | Default `12` (range 4..31)         |

If the user already exists, the password is **NOT** overwritten.

#### Audit purge

Deletes `audit_events` rows older than the retention window in capped
batches with sleep between rounds (live-workload-friendly).

| Task                       | Raw command                         |
| -------------------------- | ----------------------------------- |
| `task audit:purge`         | `go run ./cmd/audit-purge`          |
| `task audit:purge:dry-run` | `go run ./cmd/audit-purge -dry-run` |

Flags (pass after `--` when using task, e.g. `task audit:purge -- -retention-days 90`):

| Flag              | Default | Description                                 |
| ----------------- | ------- | ------------------------------------------- |
| `-retention-days` | `365`   | Delete rows older than (now − N days)       |
| `-before`         | —       | RFC3339 cutoff; overrides `-retention-days` |
| `-batch-size`     | `1000`  | `DELETE ... LIMIT N` per batch              |
| `-sleep`          | `100ms` | Pause between batches                       |
| `-dry-run`        | `false` | Only `SELECT COUNT(*)` — no DELETE          |

### Development

| Task                                | Raw command                                  |
| ----------------------------------- | -------------------------------------------- |
| `task sqlc` (alias `task sqlc:gen`) | `sqlc generate`                              |
| `task tidy`                         | `go mod tidy`                                |
| `task fmt`                          | `gofmt -w -s .`                              |
| `task vet`                          | `go vet ./...`                               |
| `task test`                         | `go test ./...`                              |
| `task test:e2e`                     | `go test -tags e2e ./internal/tests/e2e/...` |
| `task check`                        | vet + build + test + test:e2e                |

### Keys

| Task                | Raw command                                                                                                                                          |
| ------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- |
| `task key:generate` | `openssl genpkey -algorithm Ed25519 -out keys/ed25519_private.pem && openssl pkey -in keys/ed25519_private.pem -pubout -out keys/ed25519_public.pem` |

## Project layout

- `cmd/sso` — gRPC server
- `cmd/migrate` — schema migrations + DB create/drop
- `cmd/seed` — one-shot admin seed
- `cmd/audit-purge` — audit log retention purge
- `internal/` — domain packages (identity, app, role, access, serviceaccount, auth, audit) + platform glue
- `migrations/mariadb` — golang-migrate SQL files
- `protos/proto/sso` — proto contracts
