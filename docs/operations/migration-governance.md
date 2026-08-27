# Migration Governance

This document is the binding doctrine for database migrations in `backend/migrations/`.
Read it before authoring, renaming, applying, or reverting any migration.

---

## 1. Migration Runner Authority

The runtime server does not auto-run migrations.

The explicit migration apply command is:

```bash
cd backend
go run ./cmd/migrate
```

The active schema authority is the canonical baseline in `backend/migrations/000001_canonical_schema.up.sql`
plus any later additive migrations. The first additive hardening pass is
`backend/migrations/000003_identity_email_uniqueness.up.sql`, which enforces the
normalized email identity invariant.
`backend/pkg/database/migrate.go` is a helper library for tooling; it is not wired into `core_server`
in the current runtime.

`backend/cmd/migrate/main.go` and the Makefile `migrate-*` targets are separate migration toolchains
with different `schema_migrations` tables. Do not mix them on the same database without resetting
`schema_migrations`.

---

## 2. Numbering Rules

- Migration files are named `NNNNNN_description.{up,down}.sql` where `NNNNNN` is a zero-padded 6-digit integer.
- The canonical baseline is `000001_canonical_schema`. New migrations start at `000002` and increment sequentially.
- Always use the next sequential number.
- Never reuse a version number that has already been committed to `main`, even if it was later superseded.
- Both `.up.sql` and `.down.sql` files are required for every migration.

---

## 3. Slot-Reuse Prohibition

Do not edit an already-merged migration in place.
Corrections are additive: write a new migration that undoes or extends the previous one.
The only exception is a migration that has never been applied to any database and has not yet been merged.

---

## 4. Destructive Migration Protocol

Migrations that DROP tables, DROP columns, or DELETE data require:

1. A corresponding application code change that removes all references to the dropped surface.
2. A `.down.sql` that can reconstruct the dropped surface for local rollback only.
3. A comment at the top of the `.up.sql` naming the ADR or convergence pass that approved the drop.

---

## 5. Supersede Protocol

When a migration is logically superseded by later work, keep both migrations.
Do not edit the original file. Document the supersession in the commit message of the DROP migration.

---

## 6. `schema_migrations` Truth Model

The golang-migrate helper uses this schema when invoked by tooling:

```sql
CREATE TABLE schema_migrations (
    version bigint NOT NULL,
    dirty   boolean NOT NULL,
    CONSTRAINT schema_migrations_pkey PRIMARY KEY (version)
);
```

A migration is considered applied when its version row exists with `dirty = false`.
A row with `dirty = true` means the migration is mid-flight or failed - the database needs manual intervention.

---

## 7. Replace Protocol (Rename a Migration)

If a migration was authored with the wrong name but the correct version number has not yet been committed to `main`:

1. Rename the file(s) on disk.
2. Update `schema_migrations` on any database where it was already applied.
3. Commit atomically (rename + any code references in one commit).

If the migration has been committed to `main`, it is immutable. Do not rename it. Create a new migration instead.

---

## 8. Pre-Production Cleanup Notes

This project has not had a production deployment.

- `git rm --cached` of runtime artifacts (`scenario_logs/`, `audit_runs/`) is safe and encouraged.
- Schema refactors that would be impossible post-launch can use multiple sequential migrations rather than a single mega-migration.
- `000001_canonical_schema.up.sql` is the single authoritative baseline. Old incremental files (v100–v214) have been squashed into it and removed from the repo. Git history preserves the chain.
- Once any production data exists, the baseline rewrite path is forbidden. Future schema changes must be expressed as additive migrations only unless an explicit governance exception is approved.

---

## 9. CI Enforcement

CI currently runs `go build ./...` and finance package tests. Migrations are not applied in CI.
Until a CI migration-smoke step is added, every migration author must manually verify:

1. `cd backend && go run ./cmd/migrate` runs cleanly on a fresh local DB.
2. `core_server` is started only after migrations are applied.

---

## 10. Archive Convention

Old incremental migration files (v100–v214) have been removed from the repo. They were squashed
into `000001_canonical_schema.up.sql` on 2026-07-03. Git history (`763bff97` and prior) is the
archive for forensic lookup. Do not restore old files under `backend/migrations/`.

---

## 11. Deferred Migration Ledger

| # | Migration | Status | Notes |
|---|---|---|---|
| 136 | `000136_drop_escrows_funding_source` | Applied 2026-05-11 | P2 convergence set |
| 138 | `000138_dispute_freeze` | Applied 2026-05-11 | P2 convergence set |
| 139 | `000139_foundation_convergence_drop_legacy_flags` | Applied 2026-05-11 | P2 convergence set - drops `email_verified`, `profile_completed`, `username_is_custom` from `users`; makes `user_profiles.username` nullable |
| 192 | `000192_drop_seller_debt` | Applied | All Go code removed (`seller_debt.go`, `seller_debt_repository.go`, `debt_recovery_test.go`, `verify_seller_debt/`, `SellerDebtsPage.tsx`). Applied as part of sequential migration replay to current version. |

The `136 / 138 / 139` convergence set was applied on 2026-05-11. The `192` entry was applied when the migration chain reached v212. Future deferred migrations get ledger entries in this table as they appear.
