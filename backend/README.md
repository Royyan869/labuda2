# Labuda Backend

Go API server for the Labuda marketplace platform. Gin framework, Firebase auth, Midtrans payments, PostgreSQL, Redis.

- Production entrypoint: [`cmd/core_server/main.go`](cmd/core_server/main.go)
- Route registration: [`cmd/core_server/routes_core.go`](cmd/core_server/routes_core.go)
- Migration governance: [`../docs/operations/migration-governance.md`](../docs/operations/migration-governance.md)
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

> **Do not use `make migrate-up`.** The Makefile uses the external `migrate` CLI tool, which has a different `schema_migrations` table schema. Mixing it with `go run ./cmd/migrate` on the same database corrupts the migration state. Always use `go run ./cmd/migrate`.

See [`docs/operations/migration-governance.md`](../docs/operations/migration-governance.md) for migration authoring rules.

### 4. Seed reference data

```bash
cd backend
go run ./cmd/seed
# Seeds platform_configs, seller subscription configs, etc.
```

### 5. Create admin user

The seeder does not create admin users. See [`docs/operations/admin-bootstrap.md`](../docs/operations/admin-bootstrap.md) for how to bootstrap the first admin via SQL.

For a full owner-test seed (buyer/seller/order states), see [`docs/operations/dev-seed-guide.md`](../docs/operations/dev-seed-guide.md).

### 5. Run the server

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
│   ├── commerce/      Orders, listings, auctions, checkout, negotiation
│   ├── finance/       Ledger, withdrawals, refunds, billing
│   ├── governance/    Disputes, moderation, warnings
│   ├── interaction/   Chat, notifications, ratings
│   ├── social/        Content, follows, likes, comments
│   └── user/          Auth, profiles, seller, verification
├── migrations/        PostgreSQL migration chain (000100+)
│   └── legacy_do_not_run/
│       ├── 000_init/  Legacy split-init docs — NOT run
│       ├── archive/   Frozen table definitions — NOT run
│       └── snapshots/ Snapshot material — NOT run
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
| `migrate: Dirty database` | If you are using the Makefile / external `migrate` CLI path, run `make migrate-force version=N` where N is the dirty version. For `go run ./cmd/migrate`, recreate the DB and rerun migrations. |
