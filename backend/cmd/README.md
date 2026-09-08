# `backend/cmd/` — Entrypoints and Tools

## Authority Classification

Every subdirectory here is a `main` package. This file is the canonical classification record.
Do not add a new subdirectory without updating this table.

### Tier 1 — Production / CI Entrypoints

| cmd | Purpose | Notes |
|---|---|---|
| `core_server` | **Production HTTP server** | Start after `go run ./cmd/migrate`. Does not apply migrations itself. All routes, workers, DI root. |
| `migrate` | Migration runner (custom PGX runner) | Explicit manual migration command. Applies the numbered `backend/migrations/` chain. |
| `seed` | Data seeder | Populates platform_configs, reference data, and local dev users (`buyer@test.local`, `seller@test.local`, `admin@test.local` with fixed UUIDs). Run after migrations. |
| `corpus_driver` | CI corpus scenario runner | Referenced by `serverboot/dependencies.go`; runs E2E governance scenarios. |

### Tier 2 — Canonical Dev / Staging Tools

| cmd | Purpose | Notes |
|---|---|---|
| `dev-reset-data` | Local/dev database reset | Resets the local/dev DB to a clean state while preserving the three canonical owner-test accounts (identity + minimal capability rows). Dry-run by default; `--execute` to run. |
| `worker_sql_alignment` | Worker SQL alignment proof | Runs the worker SQL alignment proof against a disposable Postgres database (`make test-worker-sql-alignment`). |
| `midtrans_sandbox_validation` | Payment gateway smoke test | Run against Midtrans sandbox. |
| `recon_audit` | Financial reconciliation audit | Spot-check for ledger consistency. |

## Authoring Rules

1. New production entrypoints → `Tier 1`. Require Makefile target and README update.
2. New proof/fixture tools → Name with `verify_` or `proof_` prefix. Classify in `Tier 2` immediately and evaluate for deletion after their proof cycle.
3. No cmd subdirectory may contain `.py`, `.txt`, or `.patch` files — those belong in `docs/` or `scripts/`.
