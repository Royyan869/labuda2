# `backend/cmd/` — Entrypoints and Tools

## Authority Classification

Every subdirectory here is a `main` package. This file is the canonical classification record.
Do not add a new subdirectory without updating this table.

### Tier 1 — Production / CI Entrypoints

| cmd | Purpose | Notes |
|---|---|---|
| `core_server` | **Production HTTP server** | Start after `go run ./cmd/migrate`. Does not apply migrations itself. All routes, workers, DI root. |
| `migrate` | Migration runner (custom PGX runner) | Explicit manual migration command. Applies the numbered `backend/migrations/` chain. |
| `bootstrap-admin` | **Canonical first-admin bootstrap (production)** | Out-of-band CLI. Promotes exactly one existing verified, active user to the canonical admin role (`capability/entity.AdminRole`) and grants the entire canonical capability universe (`capability.AllCapabilityStrings()`) in the same transaction; full access is then a derived state (`capability.IsFullAccessAdmin`). There is no bootstrap preset and no fixed capability set. Usage: `go run ./cmd/bootstrap-admin --user-id <uuid>` or `--email <email>`. |
| `seed` | Data seeder (dev fixture only) | Populates local dev users (`buyer@test.local`, `seller@test.local`, `admin@test.local` with fixed UUIDs away from SystemCaller) and sample content. Bypasses business logic. **Not a production bootstrap** — use `bootstrap-admin`. Run after migrations. |
| `corpus_driver` | CI corpus scenario runner | Referenced by `serverboot/dependencies.go`; runs E2E governance scenarios. |

### Tier 2 — Canonical Dev / Staging Tools

| cmd | Purpose | Notes |
|---|---|---|
| `dev-reset-data` | Local/dev database reset | Resets the local/dev DB to a clean state while preserving the three canonical owner-test accounts (identity + minimal capability rows). Dry-run by default; `--execute` to run. |
| `dev-firebase-admin` | Dev-only Firebase identity provisioning | Creates (or reports) the real Firebase Auth user for an existing seeded/bootstrapped Labuda dev admin, so the Admin dashboard can log in through the normal Firebase flow. Refuses unless `ENV=development`; password via stdin only. Never touches the Labuda DB, roles, or capabilities. Usage: `go run ./cmd/dev-firebase-admin --email <email> --password-stdin`. |
| `worker_sql_alignment` | Worker SQL alignment proof | Runs the worker SQL alignment proof against a disposable Postgres database (`make test-worker-sql-alignment`). |
| `midtrans_sandbox_validation` | Payment gateway smoke test | Run against Midtrans sandbox. |
| `recon_audit` | Financial reconciliation audit | Spot-check for ledger consistency. |

## Authoring Rules

1. New production entrypoints → `Tier 1`. Require Makefile target and README update.
2. New proof/fixture tools → Name with `verify_` or `proof_` prefix. Classify in `Tier 2` immediately and evaluate for deletion after their proof cycle.
3. No cmd subdirectory may contain `.py`, `.txt`, or `.patch` files — those belong in `docs/` or `scripts/`.
