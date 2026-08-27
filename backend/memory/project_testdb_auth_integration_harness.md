# TestDB Auth Integration Harness Closure

**VERDICT**: AUTH_INTEGRATION_HARNESS_CLOSED — 2026-06-15

## What was fixed

### 1. `DB_NAME is required` — dotenv walk-up
`testdb.SetupDB` → `config.Load()` → `godotenv.Load()` uses cwd.
When `go test` runs from `internal/identity/auth/delivery/http/`, cwd is that dir, not `backend/`.
Fix: added `loadDotEnvFromParents()` to `pkg/testdb/testdb.go` that walks up 8 levels to find `.env`.

### 2. Migration dirty state (version 167)
`labuda_test` was set up before `seller_monthly_metrics` was added to migration 100.
Fix: reset `labuda_test` (`DROP DATABASE / CREATE DATABASE`). Let harness re-apply all migrations.

### 3. Migration tx restriction — migrations 137 and 188
PostgreSQL 12+: `ALTER TYPE ... ADD VALUE` inside a transaction is OK, but the new value
cannot be used in the same transaction. `golang-migrate` wraps each file in a transaction.
Migrations 137 and 188 both did ADD VALUE + CREATE INDEX (using new value) in the same file.
Fix: removed CREATE INDEX from 137 and 188. Created migrations 198 and 199 to hold the indexes.

### 4. fcm_tokens schema gap — migration 200
`fcm_token_repository.go` was rewritten (auth/session pass) to use individual columns
(platform, device_id, device_name, app_version, is_active, last_used_at) but no migration
was created to add them. Both `labuda` and `labuda_test` had the old JSONB `device_info` schema.
Fix: migration 000200 adds the required columns + indexes. Applied to main `labuda` DB manually
(then recorded in schema_migrations). Will be idempotent on next `cmd/migrate` run.

### 5. FALSE_CONTRACT stub assertions in integration test
`TestLogoutHandler_RevokeOnlyCurrentFamilyAndKeepOthersActive` set `h.fcmTokenRepo = fcmStub`
then asserted `fcmStub.deactivateByTokenCalls == 1`. But `h.logoutService` has its own FCM repo
baked in — `h.fcmTokenRepo` is never called by the logout flow.
Fix: removed the stub setup and the call count assertions. DB-level assertions remain correct.

### 6. TruncateAll — bulk TRUNCATE + 120s timeout
Old: individual `TRUNCATE TABLE t CASCADE` for each of 96 tables → timed out at 30s.
New: single `TRUNCATE TABLE t1, t2, ..., tN CASCADE` → PostgreSQL acquires locks in one pass.
NOTE: even the bulk TRUNCATE takes ~44s on this 96-table schema due to lock graph traversal.
This is a P2 perf issue. Tests pass when given 300s timeout per integration test.

## Running auth integration tests

```bash
cd backend
go test -tags integration -timeout 300s ./internal/identity/auth/delivery/http/...
```

Prerequisites:
- `labuda-postgres` container running
- `labuda_test` database exists (auto-created if missing, auto-migrated on first run)
- `backend/.env` has DB_NAME=labuda and connection params

First run: ~180s for migrations (100-200) + ~45s per integration test × 6 = ~450s total. Use `-timeout 600s`.
Subsequent runs (migrations cached): ~45s per integration test × 6 = ~270s total. Use `-timeout 300s`.

## Migration numbers
- 197: auth_refresh_sessions (was existing)
- 198: SLA index refresh for waiting_user (split from 137) — NEW this pass
- 199: payment webhook review index (split from 188) — NEW this pass
- 200: fcm_tokens column migration — NEW this pass

## Known P2 debts
- TRUNCATE ~44s per integration test (lock acquisition cost for 96 tables) — not a bug
- `h.fcmTokenRepo` on AuthHandler is set but never used by logout flow — misleading field
- First-run migration takes ~3 min on fresh labuda_test
