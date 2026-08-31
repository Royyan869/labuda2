# LABUDA — MODERATION IMPLEMENTATION SLICE 1: CANONICAL SCHEMA FOUNDATION

- **Tanggal:** 2026-08-30
- **Mode:** IMPLEMENTATION SLICE 1 — SCHEMA FOUNDATION ONLY
- **Authority:** `LABUDA — CANONICAL MODERATION SPECIFICATION v1`, `LABUDA — CANONICAL MODERATION DESIGN v1`, `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1`, `docs/audits/moderation/REPORT_CASE_AUDIT_1..4`
- **Baseline:** current filesystem

---

## 1. What Changed

### Files created

| File | Purpose |
|---|---|
| `backend/migrations/000055_canonical_moderation_foundation.up.sql` | Canonical foundation: enums, `reports`, `cases`, `decisions`, `enforcements`, Warning/Appeal provenance |
| `backend/migrations/000055_canonical_moderation_foundation.down.sql` | Reverse of 000055 |
| `backend/migrations/000056_drop_legacy_moderation_schema.up.sql` | Drop rejected `moderation_cases` + old enums |
| `backend/migrations/000056_drop_legacy_moderation_schema.down.sql` | Restore legacy schema (reversibility only) |
| `backend/tests/migration_000055_canonical_moderation_foundation_test.go` | Integration proof: replay + constraint proof |
| `docs/audits/moderation/REPORT_CASE_IMPLEMENTATION_SLICE_1.md` | This report |

### Files modified

| File | Change | Reason |
|---|---|---|
| `backend/internal/platform/admin/infrastructure/repository/admin_repository_impl.go` | Dashboard "Pending reports" query: `moderation_cases WHERE status='pending'` → `cases WHERE status='open'` | Consumer of dropped table; minimal adaptation so runtime does not break (line ~389-394) |
| `backend/tests/migration_000047_schema_state_proof_test.go` | Assertions for `moderation_resource_enum` replaced with `moderation_target_type_enum` (000055/000056) | Test tied to rejected vocabulary (§20) |
| `backend/tests/migration_000054_drop_dead_chat_commerce_references_test.go` | Fixed broken helper call `setupTestDB` → `testdb.SetupDB` | Pre-existing broken test (referenced nonexistent function) blocking package compile |

### What was NOT changed (deliberately)

- No Go moderation runtime (`moderation_repository_impl.go`, `moderation_service.go`, handlers, workers).
- No admin/mobile code beyond the single dashboard query above.
- No outbox worker/retry.
- No `000001`–`000054` historical migrations.

---

## 2. Canonical Tables

### `reports` (000055)

```sql
id          uuid PK DEFAULT gen_random_uuid()
reporter_id uuid NOT NULL FK → users(id) ON DELETE CASCADE
target_type moderation_target_type_enum NOT NULL   -- content|comment|for_sale|auction|user
target_id   uuid NOT NULL
reason      text NOT NULL CHECK (length(btrim(reason)) > 0)
case_id     uuid NULL FK → cases(id) ON DELETE SET NULL   -- added after cases; correlation
created_at  timestamptz NOT NULL DEFAULT now()
```

Indexes: `idx_reports_reporter (reporter_id, created_at DESC)`, `idx_reports_target (target_type, target_id, created_at DESC)`, `idx_reports_case_id (case_id) WHERE case_id IS NOT NULL`.

### `cases` (000055)

```sql
id           uuid PK DEFAULT gen_random_uuid()
subject_type moderation_target_type_enum NOT NULL
subject_id   uuid NOT NULL
status       case_status_enum NOT NULL DEFAULT 'open'   -- open|resolved
created_at   timestamptz NOT NULL DEFAULT now()
closed_at    timestamptz NULL
updated_at   timestamptz NOT NULL DEFAULT now()
```

Indexes: `uniq_active_case_per_subject (subject_type, subject_id) WHERE status='open'` (UNIQUE), `idx_cases_subject`, `idx_cases_status (status, created_at) WHERE status='open'`.

### `decisions` (000055)

```sql
id            uuid PK DEFAULT gen_random_uuid()
case_id       uuid NOT NULL FK → cases(id) ON DELETE CASCADE
decided_by    uuid NOT NULL FK → users(id)
outcome       decision_outcome_enum NOT NULL   -- no_violation|violation
decision_note text NULL
created_at    timestamptz NOT NULL DEFAULT now()
```

Index: `idx_decisions_case (case_id, created_at DESC)`.
**Immutable:** trigger `trg_decisions_immutable` rejects any `UPDATE` (append-only history).

### `enforcements` (000055)

```sql
id              uuid PK DEFAULT gen_random_uuid()
decision_id     uuid NOT NULL FK → decisions(id) ON DELETE CASCADE
target_type     moderation_target_type_enum NOT NULL
target_id       uuid NOT NULL
status          enforcement_status_enum NOT NULL DEFAULT 'pending'  -- pending|processing|succeeded|failed
attempt_count   integer NOT NULL DEFAULT 0 CHECK (>= 0)
requested_at    timestamptz NOT NULL DEFAULT now()
started_at      timestamptz NULL
finished_at     timestamptz NULL
last_error      text NULL
next_attempt_at timestamptz NULL
created_at      timestamptz NOT NULL DEFAULT now()
updated_at      timestamptz NOT NULL DEFAULT now()
```

Indexes: `idx_enforcements_decision`, `idx_enforcements_status (status, next_attempt_at) WHERE status IN (pending,processing)`, `idx_enforcements_target`.
Unique: `enforcements_decision_target_unique (decision_id, target_type, target_id)` — idempotency anchor (one consequence per decision+target).

### `user_warnings` (modified 000055)

Added: `decision_id uuid NOT NULL` (CHECK `user_warnings_decision_id_required`), FK `→ decisions(id)`, UNIQUE `user_warnings_decision_unique (decision_id, user_id)`. Existing columns unchanged.

### `appeals` (modified 000055)

- **Dropped** `report_id` (legacy column that actually stored CaseID).
- **Added** `decision_id uuid NOT NULL` (CHECK `appeals_decision_id_required`), FK `→ decisions(id)`.
- Added CHECK `appeals_status_check (status IN ('pending','approved','rejected'))`.
- Index `idx_appeals_decision_id`.

---

## 3. Constraints

| Constraint | Type | Enforces |
|---|---|---|
| `uniq_active_case_per_subject` | partial UNIQUE INDEX | **One active Case per subject** (DB-level, not application) |
| `enforcements_decision_target_unique` | UNIQUE | One enforcement per (decision, target) — idempotency |
| `user_warnings_decision_id_required` | CHECK NOT NULL | Warning must have Decision provenance |
| `user_warnings_decision_unique` | UNIQUE | No duplicate warning per (decision, user) |
| `appeals_decision_id_required` | CHECK NOT NULL | Appeal must have Decision provenance |
| `appeals_status_check` | CHECK | Appeal status vocabulary |
| `reports_reason_not_blank` | CHECK | Non-empty reason |
| `enforcements_attempt_count_nonneg` | CHECK | Non-negative attempts |
| `trg_decisions_immutable` | TRIGGER | Decision append-only (UPDATE rejected) |
| enum types | ENUM | Target types (no `chat_message`), case status (open/resolved only), decision outcome (no_violation/violation), enforcement status (pending/processing/succeeded/failed) |

---

## 4. Indexes

| Index | Table | Keys | Partial |
|---|---|---|---|
| `uniq_active_case_per_subject` | cases | (subject_type, subject_id) | `WHERE status='open'` |
| `idx_cases_subject` | cases | (subject_type, subject_id, created_at DESC) | — |
| `idx_cases_status` | cases | (status, created_at) | `WHERE status='open'` |
| `idx_reports_reporter` | reports | (reporter_id, created_at DESC) | — |
| `idx_reports_target` | reports | (target_type, target_id, created_at DESC) | — |
| `idx_reports_case_id` | reports | (case_id) | `WHERE case_id IS NOT NULL` |
| `idx_decisions_case` | decisions | (case_id, created_at DESC) | — |
| `idx_enforcements_decision` | enforcements | (decision_id) | — |
| `idx_enforcements_status` | enforcements | (status, next_attempt_at) | `WHERE status IN (pending,processing)` |
| `idx_enforcements_target` | enforcements | (target_type, target_id) | — |
| `idx_appeals_decision_id` | appeals | (decision_id) | — |

---

## 5. Foreign Keys

| FK | From | To |
|---|---|---|
| `reports_reporter_id_fkey` | reports.reporter_id | users.id (CASCADE) |
| `reports_case_id_fkey` | reports.case_id | cases.id (SET NULL) |
| `decisions_case_id_fkey` | decisions.case_id | cases.id (CASCADE) |
| `decisions_decided_by_fkey` | decisions.decided_by | users.id |
| `enforcements_decision_id_fkey` | enforcements.decision_id | decisions.id (CASCADE) |
| `user_warnings_decision_id_fkey` | user_warnings.decision_id | decisions.id |
| `appeals_decision_id_fkey` | appeals.decision_id | decisions.id |

Polymorphic `target_type + target_id` (reports, cases, enforcements) **cannot** be FK'd to five tables — application-layer validation is required (same pattern as legacy `ResourceExists`). This is documented in Audit 4 §H.4.

---

## 6. Migration Order

```text
000001 ... 000054   (existing chain, untouched)
000055_canonical_moderation_foundation   → create canonical tables + enums + alter warnings/appeals
000056_drop_legacy_moderation_schema    → DROP TABLE moderation_cases; DROP TYPE moderation_status_enum, moderation_resource_enum
```

Ordering rationale:
- 000055 must precede 000056 (new foundation exists before old is dropped — no dangling period).
- Both are transactional (no `ADD VALUE`), so the pgx runner executes each in one transaction.
- Down chain: 000056 restores legacy schema; 000055 restores pre-foundation state (re-adds `appeals.report_id`).

**Verified:** clean replay from empty DB through 000056 (`go run ./cmd/migrate` on fresh `labuda_slice1_replay`) → `schema_migrations` max version 56, canonical tables exist, `moderation_cases` absent.

---

## 7. Removed Legacy Schema

| Removed | Migration | Evidence |
|---|---|---|
| `moderation_cases` table (GovernanceCase super-entity) | 000056 | `pg_tables` no longer lists it |
| `moderation_status_enum` (`pending/approved/rejected/removed/enforced`) | 000056 | `pg_type` no longer lists it |
| `moderation_resource_enum` (incl. `chat_message`) | 000056 | `pg_type` no longer lists it |
| `appeals.report_id` (stored CaseID) | 000055 | column dropped; replaced by `decision_id` |
| `user_warnings` standalone capability (no Decision) | 000055 | `decision_id NOT NULL` makes standalone impossible |

No compatibility view/alias/adapter was created. The rejected architecture has no active schema presence.

---

## 8. Tests Executed

| Test | Command | Result |
|---|---|---|
| `TestCanonicalModerationFoundation` (new) | `go test -tags integration -run TestCanonicalModerationFoundation -v ./tests/ -count=1 -timeout 300s` | **PASS** |
| `TestMigration000047_SchemaStateProof` (modified) | `go test -tags integration -run TestMigration000047_SchemaStateProof -v ./tests/ -count=1 -timeout 300s` | **PASS** |
| `TestMigration000054_DropsDeadChatCommerceReferences` (fixed) | `go test -tags integration -run TestMigration000054_DropsDeadChatCommerceReferences -v ./tests/ -count=1 -timeout 300s` | **PASS** |
| Build all packages | `go build ./...` | **PASS** |
| Migration runner unit | `go test ./cmd/migrate/... -count=1 -timeout 60s` | **PASS** |
| Worker unit | `go test ./internal/worker/... -count=1 -timeout 120s` | **PASS** |
| Admin unit (application+http) | `go test ./internal/platform/admin/application ./internal/platform/admin/delivery/http` | **PASS** |
| Governance moderation unit (entity/application) | `go test ./internal/governance/moderation/entity ./internal/governance/moderation/application` | **PASS** |

### Constraint proof (from test, real PostgreSQL)

| Proof | Expected | Actual |
|---|---|---|
| Two active cases for same subject | rejected | `23505 uniq_active_case_per_subject` ✓ |
| Resolved case for same subject | allowed | ✓ |
| Invalid target type (`chat_message`) in reports/cases | rejected | enum violation ✓ |
| Decision with non-existent case_id | rejected | FK violation ✓ |
| Decision UPDATE | rejected | `decisions rows are immutable` trigger ✓ |
| Invalid decision outcome (`reversed`) | rejected | enum violation ✓ |
| Enforcement with non-existent decision_id | rejected | FK violation ✓ |
| Duplicate enforcement (decision+target) | rejected | `enforcements_decision_target_unique` ✓ |
| Invalid enforcement status (`dead_letter`) | rejected | enum violation ✓ |
| Warning without decision_id | rejected | `user_warnings_decision_id_required` ✓ |
| Duplicate warning (decision+user) | rejected | `user_warnings_decision_unique` ✓ |
| Appeal without decision_id | rejected | `appeals_decision_id_required` ✓ |

---

## 9. Actual Database Proof

Verified against real PostgreSQL (`labuda_test` and fresh replay DB) via `psql`:

```text
schema_migrations max version: 56 (clean replay from zero)

Tables present:  reports, cases, decisions, enforcements, user_warnings, appeals
Tables absent:   moderation_cases

Enums present:  moderation_target_type_enum, case_status_enum,
                decision_outcome_enum, enforcement_status_enum
Enums absent:   moderation_status_enum, moderation_resource_enum

Index present:  uniq_active_case_per_subject
Constraints:    appeals_decision_id_required, enforcements_decision_target_unique,
                user_warnings_decision_id_required, user_warnings_decision_unique
```

`moderation_target_type_enum` values: content, comment, for_sale, auction, user — **no** chat_message, **no** fixed_price_sale.

---

## 10. Remaining Runtime Scope

Not implemented in this slice (deliberately, per instruction §18):

1. **Go repositories/services/handlers for canonical entities** — `reports`, `cases`, `decisions`, `enforcements` have no Go runtime yet. Slice 2+.
2. **Legacy Go moderation runtime** (`moderation_repository_impl.go`, `moderation_service.go`, `warning_service.go`, `appeal_service.go`, `moderation_event_handler.go`) — still references dropped `moderation_cases` and now-failing `user_warnings`/`appeals` inserts. **These will error at runtime and MUST be replaced in Slice 2** (this is the intended teardown).
3. **Outbox retry fix** (Audit 2 P1) — explicit next-scope item, not touched.
4. **Enforcement worker write-back** — depends on canonical repository layer.
5. **Warning issuance via Decision**, **Appeal → Decision flow**, **target executors** (content/comment/for_sale/auction/user).
6. **Admin/mobile** adaptation to canonical API.

---

## 11. Residue Still Outside This Slice

| Item | Status |
|---|---|
| `internal/governance/moderation/**` Go code referencing `moderation_cases` | Zombie until Slice 2 rewrite (rejected architecture, must be deleted/replaced) |
| `moderation_evidence_integration_test.go` (INSERT into `moderation_cases`) | Obsolete (GovernanceCase); will fail if run with `-tags integration`; reported for Slice 2 cleanup per §20 |
| `moderation_repository_list_by_reporter_test.go`, `moderation_handler_my_cases_test.go`, and related unit tests | **Pre-existing build breakage** (interface/mock mismatch, unrelated to this slice — confirmed via `git status`: untouched) |
| `admin_repository_metrics_integration_test.go` | Slow/hanging `TruncateAll` on Windows/Docker fsync — pre-existing environment issue, not schema-caused |
| `dev-reset-data` domain table list still contains `moderation_cases` | Harmless (tool tolerates missing tables; reports "classified but absent") |
| `fixed_price_sale` vocabulary in admin TS types + mobile mappers | Application residue; out of schema scope (Audit 4 §O.1) |

---

## 12. Risks / Unknowns

1. **Runtime moderation is now schema-broken by design.** Any running server that serves moderation endpoints will fail on those paths until Slice 2 replaces the Go layer. This is the intended "kill once" teardown, but it means Slice 1 and Slice 2 must be delivered before moderation is usable again.
2. **`user_warnings`/`appeals` existing repository INSERTs** (without `decision_id`) now violate NOT NULL. Same teardown consequence.
3. **`reports.case_id` is nullable** — correlation policy (auto-attach on intake vs. explicit) is a Slice 2 runtime decision.
4. **Polymorphic target integrity** relies on application validation (documented).
5. **Case status set** is `open`/`resolved` per canonical spec; whether `under_review` is needed remains a technical detail for Slice 2.
6. **Test environment fsync latency** on this Windows/Docker host can make full-suite integration runs slow; the canonical foundation test passed but full `-tags integration` suite is not CI-blocking.

---

## 13. Final Status

```text
PASS
```

PASS conditions met:

- **Migration clean replay:** `go run ./cmd/migrate` from empty DB → version 56, all canonical tables present, `moderation_cases` gone. ✓
- **Canonical tables exist:** `reports`, `cases`, `decisions`, `enforcements` verified in real PostgreSQL. ✓
- **Obsolete moderation schema removed:** `moderation_cases`, `moderation_status_enum`, `moderation_resource_enum` absent. ✓
- **Constraints/indexes proven:** partial unique active-case-per-subject, FK provenance, immutable decision trigger, unique enforcement/warning constraints — all proven with real DB violations. ✓
- **Report/Case/Decision/Enforcement separated:** four distinct tables, no shared super-entity. ✓
- **Warning has Decision provenance:** `decision_id NOT NULL` + FK + unique. ✓
- **Appeal has Decision provenance:** `decision_id NOT NULL` + FK, `report_id` dropped. ✓
- **No compatibility layer:** no views/aliases/adapters for the old schema. ✓
- **Tests/proof actually run:** listed in §8 against real PostgreSQL. ✓

**BLOCKED items:** none for this slice's acceptance criteria. The runtime moderation teardown (Slice 2) is required before moderation is functional again — that is the designed consequence, not a defect of this slice.
