# REPORT — AUCTION SETTLEMENT PHASE 1 DATABASE / MIGRATION RUNTIME VERIFICATION

> Generated: 2026-09-02
> Mode: VERIFICATION ONLY (no source/migration/test modification)
> Reference: `docs/audits/REPORT_AUCTION_SETTLEMENT_BACKEND_PHASE1_FINAL.md`
> Authority: current filesystem + live PostgreSQL runtime evidence

---

# VERDICT

**FAILED**

Database was genuinely available (Docker Desktop + `labuda-postgres` healthy),
so this is not an infra block. A real, deterministic Phase 1 migration defect
was proven at runtime:

- Clean migration replay fails at **`000062` statement 7**:
  `CREATE INDEX idx_commerce_restrictions_active ON commerce_restrictions (user_id) WHERE restricted_until > NOW()`
  → `ERROR: functions in index predicate must be marked IMMUTABLE (SQLSTATE 42P17)`.

Because `000062` cannot apply, every DB-backed test that bootstraps through the
canonical migration runner fails, and the Phase 1 canonical schema
(`commerce_violations`/`commerce_restrictions`, enum without `expired_bnr`,
new auction columns, dropped `settlement_deadline`) is NOT reachable. Phase 1
cannot be closed until the migration is corrected and clean-replay is re-proven.

Two additional, pre-existing (non-Phase-1) working-tree inconsistencies block
the order/serdeverboot integration suites from compiling at all (see §11).

---

## 1. Environment

| Item | Status | Evidence |
|---|---|---|
| Docker Desktop | Running (server 29.7.2 / Docker Desktop 4.86.0) | `docker version` Server reachable |
| `labuda-postgres` (postgres:16-alpine) | Up (healthy), port 5432 | `docker ps`: `Up 30 seconds (healthy)`; `pg_isready -U labuda` → accepting connections |
| `labuda-redis` | Up (healthy), port 6379 | `docker ps` |
| `labuda` dev DB | Present | compose env `POSTGRES_DB=labuda` |
| `labuda_test` test DB | Present | `psql -tc "SELECT 1 FROM pg_database WHERE datname='labuda_test'"` → 1 |
| Test config | `.env`: `DB_HOST=localhost DB_PORT=5432 DB_USER=labuda DB_PASSWORD=labuda123 DB_NAME=labuda`; `DB_TEST_NAME` defaults to `labuda_test` (config.go:323) | isolation verified by `pkg/testdb` tests |
| Canonical migration runner | `pkg/migration.Run` via `pkg/testdb.runMigrationsWithLogger` (drop public schema → replay all `*.up.sql`) | testdb.go:425-453 |

No environment workaround or source change was needed to bring the DB up;
Docker had already been started in the prior session.

---

## 2. Migration Replay

Clean-replay mechanism used (canonical, no source change): `pkg/testdb`
bootstrap — it acquires an advisory lock, `DROP SCHEMA public CASCADE`,
recreates `public`, then runs `migration.Run` over `migrations/` (000001 →
000062). Triggered by running a DB-backed test:

COMMAND

```text
cd backend
go test ./internal/worker/ -run "TestPresenceLastSeenHandler_ReplaysMonotonicUpsert" -count=1 -v
```

RESULT

```text
--- FAIL: TestPresenceLastSeenHandler_ReplaysMonotonicUpsert (102.24s)
    presence_last_seen_handler_test.go:45: Failed to run test database migrations: run migrations:
      migration 62_auction_settlement_canonicalization statement 7:
      ERROR: functions in index predicate must be marked IMMUTABLE (SQLSTATE 42P17)
      SQL: CREATE INDEX idx_commerce_restrictions_active ON commerce_restrictions (user_id)
           WHERE restricted_until > NOW();
```

Second run (determinism check):

```text
--- FAIL: TestPresenceLastSeenHandler_ReplaysMonotonicUpsert (88.06s)
    ... migration 62_auction_settlement_canonicalization statement 7:
        ERROR: functions in index predicate must be marked IMMUTABLE (SQLSTATE 42P17)
```

Findings:

- Migrations **000001 … 000061 replay cleanly** (the failure occurs only at 000062).
- `000062` **fails deterministically** on statement 7; the enclosing migration
  transaction rolls back (no partial objects; `schema_migrations` top = 61).
- Migration version after failure: `61 | drop_shipping_quotes_auction_id`
  (verified via `schema_migrations`).
- No duplicate-object / constraint / index dependency failure beyond the one
  statement (all other 000062 statements execute successfully — see §4).

---

## 3. Schema Verification

Because 000062 never commits, the live `labuda_test` schema is the pre-000062
state (000001–000061). Actual inspected state:

```sql
SELECT enum_range(NULL::auction_status_enum);
-- {draft,scheduled,active,waiting_settlement,expired_bnr,ended,cancelled}
--   ^^^ expired_bnr STILL PRESENT (000062 not applied)

SELECT column_name FROM information_schema.columns
WHERE table_name='auctions'
  AND column_name IN ('shipping_resolved_at','seller_action_required','seller_quote_provided','settlement_deadline');
-- settlement_deadline          (still present; new columns absent)

SELECT conname FROM pg_constraint WHERE conrelid='auctions'::regclass AND conname='auction_order_consistency';
-- auction_order_consistency    (present, legacy form)
```

Post-000062 assertions (enum without `expired_bnr`, new columns present,
`settlement_deadline` absent, `buyer_bnr_strikes` dropped,
`commerce_violations`/`commerce_restrictions` present) **could not be reached
at runtime** — the migration must be fixed and replayed before they can be
verified against a live schema.

---

## 4. Data Migration Verification

`000062` transformation semantics were exercised statement-by-statement
against the live DB inside a rollback transaction (no state change, no source
change):

- Every statement **except** the failing `idx_commerce_restrictions_active`
  executed successfully: commerce tables + trigger function/trigger, auction
  column adds, enum-dependent drop (default/CHECK/index), the
  `expired_bnr → draft` UPDATE (`UPDATE 0` — no rows on the clean baseline),
  enum type swap (`auction_status_enum_new`), `DROP TYPE auction_status_enum`,
  rename, default restore, relaxed `auction_order_consistency` CHECK, partial
  unique index re-create, `DROP COLUMN settlement_deadline`,
  `DROP TABLE buyer_bnr_strikes`.
- Round-trip proof (up-minus-bad-index → down) inside one rolled-back
  transaction also succeeded for every down statement (buyer_bnr_strikes
  recreate, settlement_deadline restore, legacy enum rebuild, constraint/index
  restore, column drops, commerce table/function drops).

Data-safety conclusions on the transformation logic:
- The data migration only touches rows with `status='expired_bnr'` and clears
  exactly the intended settlement fields (`order_id`, `settlement_deadline`,
  `current_winner_id`, `current_bid`, `shipping_resolved_at`); it does not
  touch product binding (`product_id`) or seller ownership (`seller_id`).
- On a fresh baseline there are zero `expired_bnr` rows, so no legitimate data
  is at risk in a clean replay.
- A seeded `expired_bnr` row was NOT exercised end-to-end because the migration
  cannot commit; the UPDATE statement itself is simple and verified to parse/run.

---

## 5. DB Test Results

| Suite | Result | Root cause |
|---|---|---|
| `internal/worker` DB-backed (`TestPresenceLastSeenHandler_*`) | **FAIL** (bootstrap) | migration 000062 stmt 7 42P17 |
| `internal/worker` unit + notification/outbox | PASS | — |
| `internal/commerce/auction/...` (all tests) | PASS | package has no DB-backed tests; all unit |
| `internal/commerce/governance/...` | PASS | unit |
| `internal/commerce/order/application` (unit) | PASS | — |
| `internal/commerce/order/tests` (`-tags integration`) | **compile FAIL** | pre-existing `NewOrderFromSource` signature mismatch in `order_canonical_test.go` (see §11) |
| `internal/commerce/shipping/quote/...` (`-tags integration`) | compiles clean | — (no DB-backed run executed; DB bootstrap would hit 000062) |
| `internal/serverboot` (`-tags integration`) | **compile FAIL** | pre-existing `NewOrderFromSource` mismatch in `payment_intent_verification_integration_test.go` (see §11) |

No DB-backed test in any affected package can reach its assertions while
000062 cannot apply.

---

## 6. Business Scenario Results (A–E)

**Not executable.** Every DB-backed flow (normal shipping, private quote,
seller default, buyer shipping timeout, buyer payment BNR) requires the 000062
schema (new columns + commerce tables). The migration blocker prevents the
scenarios from running against PostgreSQL. Business behavior remains unit-
proven only (entity/service/worker unit tests), which the verification prompt
explicitly does not accept as substitute runtime proof.

---

## 7. Relist Result

**Not executable against PostgreSQL** (schema not reachable). Unit-level proof
exists (`auction_settlement_canonical_test.go`: DRAFT clears bid/winner/order/
shipping/seller flags; `MinimumBid()==StartPrice` after relist), but runtime
proof with live bid rows is pending the migration fix.

---

## 8. Deadline Result (NO EXTENSION)

**Not executable against PostgreSQL** (schema not reachable). Unit-level proof
exists (`auction_settlement_deadline_test.go`: quotes at T+1h/T+23h/T+23h59m
all keep deadline T+24h). Runtime proof pending migration fix.

---

## 9. Race / Idempotency Result

**Not executable against PostgreSQL.** The settlement worker's
`end_at + 24h` query, `FOR UPDATE SKIP LOCKED`, atomic
violation+restriction+DRAFT, and duplicate-outbox behavior depend on the 000062
columns/tables. No DB-backed race evidence could be produced.

---

## 10. Down Migration

Down-migration SQL was executed as part of the rolled-back round-trip (§4) and
succeeded statement-by-statement. However, a true apply-through-000062 →
down-000062 rollback **cannot be executed** while the up migration fails at
statement 7. Rollback safety is therefore **not fully proven** — only the
down SQL's isolated executability is demonstrated.

---

## 11. Failures

### F1 — Phase 1 migration blocker (Phase 1 regression: YES)

```text
COMMAND          go test ./internal/worker/ -run "TestPresenceLastSeenHandler_ReplaysMonotonicUpsert" -count=1
FAILURE          migration 62_auction_settlement_canonicalization statement 7:
                 ERROR: functions in index predicate must be marked IMMUTABLE (SQLSTATE 42P17)
ROOT CAUSE       backend/migrations/000062_auction_settlement_canonicalization.up.sql
                 CREATE INDEX idx_commerce_restrictions_active ... WHERE restricted_until > NOW()
                 — PostgreSQL forbids STABLE functions (now()) in index predicates.
                 No other migration in the repo uses NOW() in an index predicate.
AFFECTED         migration 000062; all DB-backed tests; Phase 1 canonical schema
PHASE-1 REGRESSION? YES (introduced by this phase)
EVIDENCE         2 independent clean-replay runs, identical statement-7 failure;
                 rollback clean (schema_migrations max = 61; no partial objects);
                 all other 000062 statements verified executable in isolation.
```

### F2 — Pre-existing compile blocker: order integration suite (Phase 1 regression: NO)

```text
COMMAND          go vet -tags integration ./internal/commerce/order/...
FAILURE          order_canonical_test.go:653/708: too many arguments in call to
                 orderentity.NewOrderFromSource (26 args given; want 25)
ROOT CAUSE       Pre-existing dirty working tree (before Phase 1): order.go's
                 NewOrderFromSource was changed (ShippingOptionID→ShippingSetupID,
                 dropped shippingExpeditionName/shippingEstimatedDays params) while
                 order_canonical_test.go was not updated. Phase 1 did not touch
                 NewOrderFromSource or order_canonical_test.go (git diff confirms).
AFFECTED         internal/commerce/order/tests (auction_settlement_test.go cannot run)
PHASE-1 REGRESSION? NO
EVIDENCE         git diff order.go shows the rename predates this session; no Phase-1
                 keyword in that diff.
```

### F3 — Pre-existing compile blocker: serverboot integration (Phase 1 regression: NO)

```text
COMMAND          go vet -tags integration ./internal/serverboot/
FAILURE          payment_intent_verification_integration_test.go:592: not enough
                 arguments in call to orderentity.NewOrderFromSource
ROOT CAUSE       Same pre-existing NewOrderFromSource signature mismatch
                 (payment_intent_verification_integration_test.go not updated when
                 order.go changed pre-session).
AFFECTED         internal/serverboot (chat auction projection + payment integration)
PHASE-1 REGRESSION? NO
EVIDENCE         git diff --stat shows both files modified pre-session; Phase 1 made
                 no edits to NewOrderFromSource or this test.
```

### F4 — Pre-existing doc-path failure in `tests` package (Phase 1 regression: NO)

```text
COMMAND          go test ./tests/ -run TestMigrationAuthorityDocsAndRuntimeStayAligned
FAILURE          read ../../docs/operations/migration-governance.md: file not found
ROOT CAUSE       Referenced docs/operations/*.md do not exist in current filesystem.
AFFECTED         backend/tests migration-authority guard (not DB/migration runtime)
PHASE-1 REGRESSION? NO
```

No failure was relabeled "pre-existing" without proof: F2/F3/F4 were confirmed
via `git diff` to originate in the pre-session working tree and to contain no
Phase-1-touched code.

---

## 12. Final Assessment

Phase 1 **cannot be closed**.

A genuine, deterministic migration defect (000062 partial index using
`NOW()`) blocks clean replay and therefore blocks the canonical schema, all
DB-backed auction settlement tests, and every runtime business scenario
(A–E, relist, deadline, race). This is a Phase 1 regression introduced by the
Phase 1 migration and must be fixed in an implementation pass (out of scope for
this verification-only task), then re-verified:

1. Fix `idx_commerce_restrictions_active` (e.g. drop the partial index or use
   an IMMUTABLE predicate — an implementation decision).
2. Re-run clean replay 000001–000062 on a fresh `labuda_test` and confirm the
   schema assertions in §3.
3. Re-run the DB-backed suites (order/tests + serverboot also require the
   pre-existing F2/F3 compile fixes before they can execute).
4. Re-attempt business scenarios A–E, relist, NO-EXTENSION, and race checks.

Environment is fully available (Postgres healthy), so the correct verdict is
**FAILED** — not BLOCKED_BY_INFRA and not GREEN.
