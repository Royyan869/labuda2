# Labuda Backend

Go API server for the Labuda marketplace platform. Gin framework, Firebase auth, Midtrans payments, PostgreSQL, Redis.

- Production entrypoint: [`cmd/core_server/main.go`](cmd/core_server/main.go)
- Route registration: [`cmd/core_server/routes_core.go`](cmd/core_server/routes_core.go)
- Migration governance: [`migrations/README.md`](migrations/README.md)
- Entrypoint classification: [`cmd/README.md`](cmd/README.md)

---

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.25+ | `go version` |
| PostgreSQL | 14+ | local or Docker |
| Redis | 6+ | local or Docker |
| Docker (optional) | any | `docker-compose up -d` starts Postgres + Redis |
| Firebase service account | — | JSON key file, see env setup |

---

## Setup from Zero

### 1. Start infrastructure

```bash
# From repo root
docker-compose up -d
# Starts PostgreSQL on :5432 and Redis on :6379
```

### 2. Configure environment

```bash
cd backend
cp .env.example .env
# Edit .env — set DB credentials, Firebase project, Midtrans keys
# Minimum required: DB_*, FIREBASE_PROJECT_ID, FIREBASE_SERVICE_ACCOUNT_KEY_PATH
```

Key env vars (see [`.env.example`](.env.example) for full list):

| Variable | Default | Required |
|---|---|---|
| `DB_HOST` | `localhost` | Yes |
| `DB_PORT` | `5432` | Yes |
| `DB_USER` | `postgres` | Yes |
| `DB_PASSWORD` | — | Yes |
| `DB_NAME` | `labuda_db` | Yes |
| `REDIS_HOST` | `localhost` | Yes |
| `FIREBASE_PROJECT_ID` | — | Yes |
| `FIREBASE_SERVICE_ACCOUNT_KEY_PATH` | `./configs/firebase-service-account.json` | Yes |
| `MIDTRANS_SERVER_KEY` | — | For payment flows |
| `DEV_MOCK_FIREBASE_AUTH` | `false` | Set `true` for local dev without Firebase |

### 3. Run migrations

```bash
cd backend
go run ./cmd/migrate
# Applies all pending top-level migrations in backend/migrations/NNNNNN_*.up.sql
# Tracks applied versions in schema_migrations table
```

Run this before starting `core_server`. The server does not auto-apply migrations.

> **Do not use the external `migrate` CLI.** It writes a different `schema_migrations` table schema. Mixing it with `go run ./cmd/migrate` on the same database corrupts the migration state. Always use `go run ./cmd/migrate`.

See [`migrations/README.md`](migrations/README.md) for migration authoring rules.

### 4. Seed reference data

```bash
cd backend
go run ./cmd/seed
# Seeds platform_configs, seller subscription configs, etc.
```

### 5. Create admin user

The admin panel cannot create an admin itself. For local development the seeder creates
`admin@test.local` (with `buyer@test.local` and `seller@test.local`) and grants the entire
canonical capability universe (`capability.AllCapabilityStrings()`), so the fixture admin has
derived full access (`capability.IsFullAccessAdmin`) and further capabilities can be managed
through the panel. In environments where the seeder is not run, bootstrap the first admin with
the canonical out-of-band CLI `go run ./cmd/bootstrap-admin --user-id <uuid>` (or
`--email <email>`), which promotes one existing verified active user to the canonical admin
role and grants that same entire universe.

#### Provision the Firebase identity for the dev admin (Admin dashboard login)

The seeder creates the Labuda **DB** admin identity only. To sign in to the Admin dashboard you
also need a real **Firebase Auth** account whose email matches that Labuda admin
(`admin@test.local`), because `/api/v1/auth/firebase/exchange` resolves an unknown Firebase UID by
email and links it to the existing DB row. Without it, the normal Firebase login cannot succeed.

```bash
cd backend
read -s PW   # silent: not echoed, not in shell history
printf '%s' "$PW" | go run ./cmd/dev-firebase-admin --email admin@test.local --password-stdin
```

- **Development only.** The command refuses to run unless `ENV=development` (an unset `ENV`
  defaults to `production`, so it fails closed).
- **Provisioning only.** It creates (or reports) the Firebase Auth user. It never creates a
  Labuda DB user, never grants capabilities or roles, never issues session tokens, and never
  bypasses Firebase login or the exchange. Login still goes through the normal Firebase flow.
- **Idempotent.** If the Firebase user already exists it is left untouched (`already_exists`,
  password preserved). A disabled user is reported (`exists_disabled`), never silently enabled.
- **Password handling.** The password is read from stdin only (`--password-stdin`); it is never
  accepted as an argument, logged, printed, or stored. Prerequisite: a valid
  `FIREBASE_SERVICE_ACCOUNT_KEY_PATH` and `FIREBASE_PROJECT_ID` (see the env table above).

### 6. Run the server

```bash
# Windows or any shell
cd backend && go run ./cmd/core_server

# Linux/Mac if make is installed
cd backend && make run
```

Server listens on `PORT` (default `8080`). Health: `GET /health` (not under `/api/v1/`).
Additional probes: `GET /health/ready`, `GET /health/live`.

---

## Common Commands

```bash
# Build
go build ./...

# Test all packages
go test ./...

# Test finance suite only (CI-safe, no DB)
go test ./internal/finance/... ./internal/finance/verifier ./internal/finance/worker -count=1 -timeout 60s

# Test commerce/order suite
go test ./internal/commerce/order/... -count=1 -timeout 90s

# Lint
golangci-lint run

# Format
go fmt ./...
```

---

## Directory Layout

```
backend/
├── cmd/               Entrypoints and dev tools — see cmd/README.md
│   ├── core_server/   Production HTTP server (only this runs in prod)
│   ├── migrate/       Migration runner
│   ├── seed/          Reference data seeder
│   └── corpus_driver/ CI scenario runner
├── internal/          Domain code (DDD/Clean Architecture)
│   ├── commerce/      Orders, For Sale, auctions, checkout, negotiation
│   ├── finance/       Ledger, withdrawals, refunds, billing
│   ├── governance/    Disputes, moderation, warnings
│   ├── interaction/   Chat, notifications, ratings
│   ├── social/        Content, follows, likes, comments
│   └── user/          Auth, profiles, seller, verification
├── migrations/        Canonical PostgreSQL migration chain (000001 baseline + additive hardening)
├── docs/              Generated Swagger/API docs (docs.go, swagger.*)
├── pkg/               Shared infrastructure (db, redis, firebase, config)
└── scripts/           Guard scripts and CI helpers
```

---

## Artifact / Log Policy

- **Never commit** `backend/scenario_logs/`, `backend/audit_runs/`, or `backend/logs/` — all gitignored.
- **Never commit** `.log`, `.exe`, `.ps1`, or `.patch` files in `backend/`.
- Compiled binaries go to `backend/bin/` (gitignored).

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `ping failed: dial tcp: connection refused` | Start PostgreSQL: `docker-compose up -d` |
| `Failed to read migrations directory` | Run from `backend/` dir, not repo root |
| `pq: relation "schema_migrations" does not exist` | Run `cd backend && go run ./cmd/migrate` before starting the server |
| `Firebase: could not fetch token` | Check `FIREBASE_SERVICE_ACCOUNT_KEY_PATH` and file existence |
| Port 8080 in use | Change `PORT=` in `.env` |
| Legacy golang-migrate table or dirty state | The external `migrate` CLI is unsupported — its `schema_migrations` shape is incompatible with the canonical runner. Recreate the DB (or drop the legacy table) and rerun `go run ./cmd/migrate`. |
