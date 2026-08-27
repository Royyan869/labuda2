# `backend/cmd/` — Entrypoints and Tools

## Authority Classification

Every subdirectory here is a `main` package. This file is the canonical classification record.
Do not add a new subdirectory without updating this table.

### Tier 1 — Production / CI Entrypoints

| cmd | Purpose | Notes |
|---|---|---|
| `core_server` | **Production HTTP server** | Start after `go run ./cmd/migrate`. Does not apply migrations itself. All routes, workers, DI root. |
| `migrate` | Migration runner (custom PGX runner) | Explicit manual migration command. Applies the numbered `backend/migrations/` chain. |
| `seed` | Data seeder | Populates platform_configs and reference data for local dev. Run after migrations. |
| `corpus_driver` | CI corpus scenario runner | Referenced by `serverboot/dependencies.go`; runs E2E governance scenarios. |

### Tier 2 — Canonical Dev / Staging Tools

| cmd | Purpose | Notes |
|---|---|---|
| `staging_rollout_ab` | Staging A/B rollout driver | Active — see `docs/operations/staging_activation_playbook.md`. |
| `verify_partial_refund_semantics` | Refund correctness verifier | Active — called by recent convergence work. |
| `midtrans_sandbox_validation` | Payment gateway smoke test | Run against Midtrans sandbox. |
| `recon_audit` | Financial reconciliation audit | Spot-check for ledger consistency. |

### Tier 3 — Historical Proof / Fixture Tools (no current callers)

_(none currently — PHASE 1 CLEANUP, 2026-07-10: the 38 entries formerly listed here no longer
exist as directories under `backend/cmd/`. `git log`/`git status` confirm they were already
deleted in a prior commit; this table was stale documentation for binaries that were gone.)_

New proof/fixture tools land here going forward. Classify with a `Convergence pass` and
`Delete candidate after` note per the Authoring Rules below.

### Tier 4 — Staged for Deletion

_(none currently — `verify_seller_debt` was staged here for deletion pending migration 000192;
as of PHASE 1 CLEANUP (2026-07-10) the directory no longer exists under `backend/cmd/`, so the
staged deletion is complete.)_

## Undocumented — Needs Classification

| cmd | Notes |
|---|---|
| `verify_withdrawal_admin_lifecycle_16` | Exists under `backend/cmd/` but was never added to this table. Contains only `dbcheck_cols.go.tmp` (not a valid `.go` file — does not compile as a binary). Needs an owner decision: classify properly or delete. |

## Authoring Rules

1. New production entrypoints → `Tier 1`. Require Makefile target and README update.
2. New proof/fixture tools → Name with `verify_` or `proof_` prefix. Classify in Tier 3 immediately.
3. After first owner-test cycle, all Tier 3 tools should be evaluated for deletion.
4. No cmd subdirectory may contain `.py`, `.txt`, or `.patch` files — those belong in `docs/` or `scripts/`.
