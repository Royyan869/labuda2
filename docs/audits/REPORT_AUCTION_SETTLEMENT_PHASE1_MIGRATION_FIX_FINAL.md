# REPORT — AUCTION SETTLEMENT PHASE 1 MIGRATION 000062 BLOCKER FIX + RUNTIME REPROOF

> Generated: 2026-09-02
> Mode: IMPLEMENTATION + VERIFICATION — NARROW FIX ONLY
> References:
> - `docs/audits/REPORT_AUCTION_SETTLEMENT_PHASE1_DB_VERIFICATION_FINAL.md` (proven defect)
> - `docs/audits/REPORT_AUCTION_SETTLEMENT_BACKEND_PHASE1_FINAL.md` (Phase 1 implementation)
> Authority: current filesystem + live PostgreSQL runtime evidence

---

# VERDICT

**BLOCKED**

The migration blocker is fixed and **fully re-proven at runtime**: clean replay
000001–000062 passes deterministically (three clean replays + one full
up/down/up cycle), every schema assertion passes against live PostgreSQL, all
runnable DB-backed suites pass, and the governance/settlement schema behavior
(immutability trigger, EXTEND stacking, auction settlement columns) is proven
against real rows.

The verdict is **BLOCKED**, not GREEN, because the end-to-end runtime business
scenarios A–E, the relist runtime proof, the NO-EXTENSION runtime proof, and the
race/idempotency DB proofs **cannot be executed**: every harness package that
drives those flows (`internal/commerce/order/tests`, `internal/serverboot`,
`tests/`) fails to compile due to the **pre-existing F2/F3** `NewOrderFromSource`
signature drift in the dirty working tree — explicitly out of scope for this
narrow corrective pass and not caused by Phase 1. Those scenarios are covered by
unit tests only, which the task defines as insufficient for GREEN.

---

## 1. Root Cause

`backend/migrations/000062_auction_settlement_canonicalization.up.sql` contained:

```sql
CREATE INDEX idx_commerce_restrictions_active ON commerce_restrictions (user_id)
    WHERE restricted_until > NOW();
```

PostgreSQL requires index predicates to use only IMMUTABLE functions;
`now()` is STABLE, so every clean replay failed deterministically with:

```text
ERROR: functions in index predicate must be marked IMMUTABLE (SQLSTATE 42P17)
```

Proven previously on live PostgreSQL with two independent clean-replay runs
(000001–000061 succeeded; 000062 rolled back at statement 7).

---

## 2. Exact Fix

**File changed (only):** `backend/migrations/000062_auction_settlement_canonicalization.up.sql`

- **Removed** the invalid partial index statement entirely.
- The `commerce_restrictions` table keeps `user_id uuid NOT NULL REFERENCES users(id) UNIQUE`,
  which provides the access index for the table's only lookup pattern.
- Added an explanatory comment documenting why no additional index exists
  (see §3).
- **No new migration version** (`000063`) was created. The canonical sequence
  remains `000001 … 000062` with `000062` now valid PostgreSQL.
- No business semantics, Go code, tests, or other migrations were changed.

---

## 3. Index Design

Inspected every consumer before choosing the design:

| Consumer | Query pattern |
|---|---|
| `commercegov.Repository.GetRestrictionForUpdate` (repository.go:49-68) | `SELECT ... FROM commerce_restrictions WHERE user_id = $1 FOR UPDATE` |
| `commercegov.IsUserRestricted` (commercegov.go:186) | delegates to `GetRestrictionForUpdate`; evaluates `restricted_until` vs `now()` **in application code** |
| `commercegov.RecordViolationAndRestrict` | `GetRestrictionForUpdate` + `UpsertRestriction` (`ON CONFLICT (user_id)`) |
| Upsert path | `ON CONFLICT (user_id)` — served by the `UNIQUE(user_id)` index |

Findings:
- The only SQL lookup is **by `user_id`**; no query filters by
  `restricted_until` and no query filters "active-only" in SQL.
- "Is the restriction currently active?" is evaluated in application code
  (`IsUserRestricted` compares `restricted_until` against `now`), never in an
  index predicate.
- The failed partial index had **no demonstrated query value**; the
  `UNIQUE(user_id)` constraint already serves the sole access pattern.

**Decision:** no additional index. The `UNIQUE(user_id)` index (from the table
constraint) is the correct and sufficient access path. This follows the task
guidance: prefer the simplest valid index that supports actual query patterns,
and do not add indexes without demonstrated query value. A hypothetical
`(user_id, restricted_until)` index would be unused today.

Verified against PostgreSQL (§5): `commerce_restrictions` has exactly
`commerce_restrictions_pkey (id)` and `commerce_restrictions_user_id_key (user_id)`;
**zero** index predicates use `NOW()`/`CURRENT_TIMESTAMP`/`CURRENT_DATE` or any
other non-IMMUTABLE time function.

---

## 4. Clean Migration Replay

Canonical mechanism used: `pkg/testdb` bootstrap (advisory lock → `DROP SCHEMA
public CASCADE` → `CREATE SCHEMA` → `migration.Run` over `migrations/`),
triggered by a DB-backed test.

```text
COMMAND: go test ./internal/worker/ -run "TestPresenceLastSeenHandler_ReplaysMonotonicUpsert" -count=1
```

| Run | Result | schema_migrations |
|---|---|---|
| 1 | PASS (88.11s) — "Test database migrations completed using canonical runner" | 62 / 62 |
| 2 | PASS (87.34s) | 62 / 62 |
| 3 (after down) | PASS (92.12s) | 62 / 62 |

Verified state after replay:

```text
SELECT MAX(version), COUNT(*) FROM schema_migrations;  → 62 | 62
SELECT version, name ... ORDER BY version DESC LIMIT 3;
  62 | auction_settlement_canonicalization
  61 | drop_shipping_quotes_auction_id
  60 | drop_legacy_shipping_snapshot_columns
```

No partial migration state at any point; three independent clean replays
establish deterministic success.

---

## 5. Schema Assertions

All assertions verified against live PostgreSQL after replay:

**Auction status enum** — PASS:
```text
SELECT enum_range(NULL::auction_status_enum);
→ {draft,scheduled,active,waiting_settlement,ended,cancelled}   (no expired_bnr)
```

**Auction columns** — PASS:
```text
auctions has:  seller_action_required, seller_quote_provided, shipping_resolved_at
auctions has NO settlement_deadline
```

**Governance tables** — PASS:
```text
commerce_restrictions  → exists
commerce_violations    → exists
buyer_bnr_strikes      → absent
```

**Indexes** — PASS (no non-IMMUTABLE predicates):
```text
commerce_restrictions:  commerce_restrictions_pkey (id),
                        commerce_restrictions_user_id_key (user_id)   ← UNIQUE, serves the only lookup
commerce_violations:    commerce_violations_pkey (id),
                        idx_commerce_violations_user (user_id, created_at DESC),
                        idx_commerce_violations_source (source_type, source_id)
Index predicates using NOW()/CURRENT_TIMESTAMP/CURRENT_DATE: 0 rows
```

**Trigger / constraints** — PASS:
```text
trg_commerce_violations_immutable BEFORE DELETE OR UPDATE ... EXECUTE FUNCTION prevent_commerce_violations_mutation()
auction_order_consistency CHECK ((order_id IS NULL) OR (status='ended') OR (status='waiting_settlement'))
uniq_active_auction_per_product partial UNIQUE on the NEW enum values
```

**Runtime behavior proof (real rows, live DB):**
- `INSERT` into `commerce_violations` succeeds; `UPDATE` is rejected by the
  trigger (`ERROR: commerce_violations rows are immutable`); the row survives.
- Restriction EXTEND stacking on the real table: count 1 → +7d, count 2 →
  +15d, count 3 → +30d; final `restricted_until` = 2026-10-24 (now+52d),
  `currently_active = t`, count = 3. Mirrors `Repository.UpsertRestriction` /
  `RecordViolationAndRestrict` semantics.

---

## 6. DB Test Results

| Suite | Result | Notes |
|---|---|---|
| `internal/worker/...` (full, incl. DB-backed) | **PASS** | includes migration bootstrap + order/alert/overdue DB tests |
| `internal/integration/payment/...` (`-tags integration`) | **PASS** | payment application/orchestrator/recon/repository DB tests |
| `internal/commerce/auction/...` | PASS | no DB-backed tests in-package (unit) |
| `internal/commerce/shipping/quote/application + delivery/http + entity` | PASS | unit |
| `internal/commerce/shipping/quote/infrastructure/repository` (race tests) | **BLOCKED (pre-existing)** | `chat_rooms.context_json does not exist` — baseline schema/code drift (000030 drops the column; repo/entity still reference it), unrelated to 000062 |
| `internal/commerce/order/tests` (`-tags integration`) | **compile BLOCKED (F2)** | pre-existing `NewOrderFromSource` mismatch |
| `internal/serverboot` (`-tags integration`) | **compile BLOCKED (F3)** | pre-existing `NewOrderFromSource` mismatch |
| `tests/` (`-tags integration`) | **compile BLOCKED (F2-family)** | pre-existing `NewOrderFromSource` mismatch in `order_item_product_identity_convergence_integration_test.go` |
| `internal/presence/...` | pre-existing harness flakiness | migration bootstrap + per-test `defer Close()` without cleanup races the shared schema reset; failures show `users`/`user_presence` gone mid-process — unrelated to 000062 |

---

## 7. Business Scenarios A–E

**NOT EXECUTABLE.** The canonical DB-backed harnesses for the full auction
settlement flows (claim → order → payment → ENDED/DRAFT) live in
`internal/commerce/order/tests/auction_settlement_test.go`, the `serverboot`
chat projection integration tests, and the top-level `tests/` package — all of
which fail to **compile** because of the pre-existing F2/F3
`NewOrderFromSource` signature drift (files not touched by Phase 1, and
explicitly out of scope to fix here). Per task rules these are reported
precisely and not modified.

Unit-level coverage of every scenario exists and passes
(`internal/commerce/auction/...`, `internal/commerce/governance/...`,
`internal/worker` notification tests), but that is not accepted as substitute
runtime proof for GREEN.

---

## 8. Relist Runtime Proof

**NOT EXECUTABLE end-to-end** for the same F2/F3 reason (the flows that create
an auction, run it through settlement failure to DRAFT, and rebid require the
blocked order/serdeverboot harness packages). The schema-level contract is
verified against live PostgreSQL: the DRAFT-reset fields exist and accept the
canonical state (`shipping_resolved_at`, `seller_action_required`,
`seller_quote_provided` present; entity unit test proves the reset + relist
`MinimumBid()==StartPrice`). Runtime proof with a live rebid is pending the
F2/F3 resolution.

---

## 9. NO-EXTENSION Runtime Proof

**NOT EXECUTABLE against a live DB flow** (same F2/F3 blocker). The deadline is
derived (`auction.end_at + 24h`, column `settlement_deadline` dropped — verified
absent in the live schema), and unit tests prove quotes at T+1h/T+23h/T+23h59m
all retain T+24h. Live end-to-end proof pending F2/F3 resolution.

---

## 10. Race / Idempotency

**DB-backed proofs not executable** for the settlement-worker duplicate/race
scenarios (they live in the blocked harness packages). Runnable DB-backed
idempotency evidence obtained:
- `internal/integration/payment/...` (payment success/expiry finalization
  paths) — PASS against live DB.
- Shipping-quote race tests (reactivation vs replacement, duplicate
  reactivation) exist but are blocked by the pre-existing `chat_rooms.context_json`
  drift.
- Worker DB tests (order-overdue reminders idempotency, alert dedup) — PASS.

---

## 11. Up/Down Migration

Full up/down/up cycle executed against live PostgreSQL:

1. **Up**: clean replay to 62 (state verified: commerce tables present,
   `settlement_deadline` absent, canonical enum).
2. **Down**: executed `000062_auction_settlement_canonicalization.down.sql`
   statements with `ON_ERROR_STOP=1` — all succeeded. Verified reversal:
   enum returns `{...,expired_bnr,...}`, commerce tables dropped,
   `buyer_bnr_strikes` + `settlement_deadline` restored, new auction columns
   gone.
3. **Up again**: fresh clean replay to 62 — PASS; final state verified
   (version 62, canonical enum, commerce tables present).

The down path executes cleanly against the real 000062-applied schema, and the
cycle is repeatable.

---

## 12. Pre-existing Failures

Re-checked after the migration fix — **all still present, all unrelated to
Phase 1 / 000062** (per task, not modified):

| ID | Failure | Status after fix | Phase-1 regression? |
|---|---|---|---|
| F2 | `NewOrderFromSource` signature mismatch: `order/tests/order_canonical_test.go`, `order/application/order_completion_restore_source_integration_test.go` (`svc.restoreListingStock` undefined), `tests/order_item_product_identity_convergence_integration_test.go` | still blocks `order/tests` + `tests/` integration compile | NO |
| F3 | `NewOrderFromSource` mismatch: `serverboot/payment_intent_verification_integration_test.go` | still blocks `serverboot` integration compile | NO |
| F4 | `tests/` `TestMigrationAuthorityDocsAndRuntimeStayAligned` references missing `docs/operations/*.md` | still present (runtime, not DB) | NO |
| (newly surfaced) | `chat_rooms.context_json`/`context_set_by` referenced by chat repo/entity (unmodified from HEAD) but dropped by migration 000030 → blocks `shipping/quote/infrastructure/repository` race tests | present | NO |
| (newly surfaced) | `internal/presence` DB tests drop/race the shared schema via per-test pool Close without testdb cleanup | present | NO |

None of these involve files changed by the 000062 fix or the Phase 1
implementation.

---

## 13. Remaining Blockers

1. **F2/F3 (and F2-family) `NewOrderFromSource` signature drift** in the dirty
   working tree — blocks `internal/commerce/order/tests`,
   `internal/serverboot`, and `tests/` integration packages from compiling.
   Until resolved, the A–E / relist / NO-EXTENSION / race DB scenarios cannot
   execute, so GREEN cannot be claimed. (Outside this narrow pass.)
2. **`chat_rooms.context_json` baseline drift** — blocks the shipping-quote
   repository race tests (pre-existing, outside this pass).
3. **Presence-package DB test harness race** (pre-existing, unrelated).

No remaining blocker is caused by migration 000062 or by this fix.

---

## 14. Final Assessment

The narrow objective — make `000062` valid PostgreSQL and prove it migrates —
is **achieved and runtime-proven**:

- 000062 no longer contains any non-IMMUTABLE index predicate; the only index
  on `commerce_restrictions` is the `UNIQUE(user_id)` backing the table's sole
  access pattern.
- Clean replay 000001–000062 passes deterministically (three runs).
- Every schema assertion (enum, columns, tables, indexes, trigger, constraints)
  passes against live PostgreSQL.
- Governance runtime behavior (immutability + EXTEND stacking) proven on real
  rows.
- Full up/down/up migration cycle proven.

Phase 1 overall remains **BLOCKED** (not GREEN) solely because the end-to-end
DB-backed business scenarios cannot be executed while the pre-existing F2/F3
compile drift is unresolved in the working tree. Once F2/F3 are fixed (a
separate, explicitly-scoped cleanup), the A–E / relist / NO-EXTENSION / race
scenarios should be re-run against the now-valid migrated schema to reach GREEN.
