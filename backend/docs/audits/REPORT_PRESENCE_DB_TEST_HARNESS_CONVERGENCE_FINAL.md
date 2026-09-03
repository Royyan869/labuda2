# REPORT: Presence DB Test Harness Race — Convergence Final

## 1. Race Symptom

When running `go test ./...`, concurrent test binaries share the `labuda_test` PostgreSQL database. Observed failures:

- `relation "users" does not exist` — test queries a table that was dropped by a concurrent binary's migration
- `deadlock detected` — concurrent `TRUNCATE TABLE ... CASCADE` from cleanup functions

## 2. Root Cause

The advisory lock in `runMigrationsWithLogger` only serialized the schema reset + migration phase. After releasing the lock, a concurrent test binary could acquire it and `DROP SCHEMA IF EXISTS public CASCADE` while the first binary's tests were actively querying the schema.

**Timeline of the race:**

```
Binary A: acquire lock → drop schema → migrate → release lock → create pool → run tests
Binary B:                                     acquire lock → drop schema → migrate → ...
                                                                   ↑
                                                  Binary A's tests fail here
```

The `Cleanup` function's `TruncateAll` also raced without any lock protection.

## 3. Fix

Extended the advisory lock to span the **entire test binary lifecycle** (migration + test execution + cleanup).

**File changed:** `backend/pkg/testdb/testdb.go`

- `Setup()` now calls `acquireLifecycleLock(dsn)` before `migrateOnce.Do`, acquiring a PostgreSQL advisory lock on a dedicated connection
- `migrateOnce.Do` calls `runMigrationsRaw(cfg, logf)` directly (no internal lock)
- `Cleanup` calls `releaseLifecycleLock()` after `TruncateAll` completes
- `runMigrationsWithLogger` retains its own lock for backward compatibility with `TestConcurrentBootstrapSerialization`

**Key functions added:**

| Function | Purpose |
|----------|---------|
| `acquireLifecycleLock(dsn)` | Acquires advisory lock on a dedicated connection for the binary lifecycle |
| `releaseLifecycleLock()` | Releases the advisory lock (safe to call when no lock is held) |
| `runMigrationsRaw(cfg, logf)` | Migration logic without lock management (called by both Setup and runMigrationsWithLogger) |

**Guarantee after fix:**

```
Binary A: acquireLifecycleLock → migrateOnce → create pool → run tests → cleanup → releaseLifecycleLock
Binary B: blocks on lifecycle lock until Binary A releases
```

## 4. Tests Before/After

**Before:** Races observed when running presence DB tests (`TestPresenceLastSeenHandler_ReplaysMonotonicUpsert`, `TestDBRepository_*`, `TestPresenceSubscriber_*`) concurrently with other package tests.

**After:**
- `TestPresenceLastSeenHandler_RejectsMalformedPayload` ✅
- `TestMarshalPresenceChanged_UsesCanonicalEnvelope` ✅
- `TestPresenceSubscriber_ShouldDeliver_DedupesVersions` ✅
- `TestNextPresenceBackoff_CapsAtThirtySeconds` ✅
- `TestDatabaseIsolation` ✅
- `TestConcurrentBootstrapSerialization` — preserved (calls `runMigrationsWithLogger` directly with its own lock)
- All presence DB test files compile cleanly (`go test -c`)

**Note:** Docker/PostgreSQL was not available in this session for full integration test execution with real DB. The fix was verified via compilation, non-DB tests, and structural analysis of the lock lifecycle.

## 5. Build/Vet

```
go build ./internal/presence/... ./internal/worker/... ./internal/realtime/... ./pkg/testdb/... ✅
go vet ./internal/presence/... ./internal/worker/... ./internal/realtime/... ./pkg/testdb/... ✅
go vet ./... ✅
go test -c (all 4 packages) ✅
```

## 6. Remaining Blockers

- **Docker/PostgreSQL not running** in this session — full integration test execution against real DB was not possible. The lifecycle lock fix needs runtime verification under concurrent `go test -race ./...` with a live database.
- The `TestConcurrentBootstrapSerialization` test should be re-verified with a live database to confirm it still detects serialization correctly.

## 7. Verdict

**PASS** — Root cause diagnosed (advisory lock too narrow), fix implemented (lifecycle-spanning lock), all affected packages compile and pass non-DB tests. Full runtime verification pending Docker/PostgreSQL availability.
