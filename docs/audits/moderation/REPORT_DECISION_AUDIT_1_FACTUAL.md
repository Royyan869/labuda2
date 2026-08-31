# AUDIT 1 — DECISION DOMAIN FACTUAL AUDIT

- **Tanggal audit:** 2026-08-31
- **Mode:** READ-ONLY PRE-IMPLEMENTATION AUDIT — tidak ada implementasi
- **Satu-satunya artefak baru:** laporan ini
- **Baseline:** current filesystem (bukan git history)
- **Authority desain:** `LABUDA — CANONICAL MODERATION BUSINESS TRUTH v1.md`, `CANONICAL MODERATION DESIGN v1.md`, `CANONICAL MODERATION SPECIFICATION v1.md`
- **Input faktual:** Audit 3 (canonical design), Audit 5 (case boundary), Slice 3 implementation reports, codebase inspection
- **Evidence rule:** setiap klaim disertai `file:line`, nama migration, tabel/index/constraint
- **Scope:** Decision domain only — schema, relationship, immutability, outcome semantics, enforcement boundary, audit trail, admin consumer, legacy residue, invariants

---

## 1. Audit Scope

This audit establishes the **factual baseline** for the Decision entity before its Go runtime is implemented.

Canonical boundary:

```text
Report
  ↓
Case
  ↓
Decision
  ↓
Enforcement
```

Decision must be an independent entity/authority. This audit verifies:

1. Current DB schema for `decisions` table
2. Decision ↔ Case relationship (cardinality, lifecycle coupling)
3. Decision immutability (DB trigger, code paths)
4. Decision outcome semantics (vocabulary correctness)
5. Decision ↔ Enforcement boundary (no false-success)
6. Enforcement creation relationship
7. Audit trail for Decision mutations
8. Admin consumer code that references Decision
9. Legacy/residue map
10. Canonical invariant proof matrix
11. Business ambiguities
12. Implementation readiness
13. Exact blockers/gaps

---

## 2. Current Decision Schema

### 2.1 Table Definition (Migration 000055)

```sql
-- backend/migrations/000055_canonical_moderation_foundation.up.sql
CREATE TABLE decisions (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    case_id       uuid NOT NULL,
    decided_by    uuid NOT NULL,
    outcome       decision_outcome_enum NOT NULL,
    decision_note text,
    created_at    timestamp with time zone DEFAULT now() NOT NULL
);
```

### 2.2 Columns

| Column | Type | Nullable | Default | Notes |
|---|---|---|---|---|
| `id` | uuid | NOT NULL | gen_random_uuid() | PK |
| `case_id` | uuid | NOT NULL | — | FK → cases(id) ON DELETE CASCADE |
| `decided_by` | uuid | NOT NULL | — | FK → users(id) |
| `outcome` | decision_outcome_enum | NOT NULL | — | 'no_violation' or 'violation' |
| `decision_note` | text | NULLABLE | NULL | Optional free-text note |
| `created_at` | timestamptz | NOT NULL | now() | Immutable creation timestamp |

### 2.3 Constraints

| Constraint | Type | Definition | Evidence |
|---|---|---|---|
| `decisions_pkey` | PK | `id` | migration 000055 |
| `decisions_case_id_fkey` | FK | `case_id → cases(id) ON DELETE CASCADE` | migration 000055 |
| `decisions_decided_by_fkey` | FK | `decided_by → users(id)` | migration 000055 |
| `trg_decisions_immutable` | TRIGGER | BEFORE UPDATE → RAISE EXCEPTION | migration 000055 |

### 2.4 Indexes

| Index | Definition | Purpose |
|---|---|---|
| `idx_decisions_case` | `(case_id, created_at DESC)` | Lookup decisions by Case, newest first |

### 2.5 Outcome Enum

```sql
CREATE TYPE decision_outcome_enum AS ENUM ('no_violation', 'violation');
```

**FACT:** Only two values. Matches locked business decision for Decision outcome vocabulary.

### 2.6 What Is Absent (Intentionally)

| Absent Element | Status | Reason |
|---|---|---|
| `action` column | **NOT PRESENT** | Per locked business decisions: "Jangan membuat `action` column" |
| `DELETE` trigger | **NOT PRESENT** | No DELETE trigger exists. Only UPDATE is blocked. See §4 for analysis |
| Unique constraint on `case_id` | **NOT PRESENT** | Correct: multiple Decisions per Case is canonical (§3) |
| `status` column | **NOT PRESENT** | Decision has no mutable lifecycle — it is append-only |
| `updated_at` column | **NOT PRESENT** | Correct: immutable rows have no update timestamp |
| `policy_code` column | **NOT PRESENT** | Optional; not in current schema. BUSINESS DECISION REQUIRED for v1 |

### 2.7 Schema Verdict

> **Is the current schema sufficient for canonical Decision runtime?**

**YES — with one finding.**

The schema correctly implements:
- ✅ PK (uuid)
- ✅ FK to Case (required, CASCADE delete)
- ✅ FK to User (decided_by, moderator identity)
- ✅ Outcome enum (`no_violation`, `violation`)
- ✅ Optional decision_note
- ✅ Immutable trigger (blocks UPDATE)
- ✅ Append-only index (case_id, created_at DESC)
- ✅ No action column (per locked decisions)
- ✅ No status column (no mutable lifecycle)

**FINDING: No DELETE trigger.** The `trg_decisions_immutable` only blocks UPDATE. A DELETE would succeed at the DB level. However:
- No application code performs DELETE on decisions
- No repository/handler has a delete method for decisions
- ON DELETE CASCADE from cases handles cascade cleanup
- Adding a DELETE trigger is RECOMMENDED but not a BLOCKER

> **FINDING (non-blocking):** Consider adding `prevent_decisions_delete()` trigger for defense-in-depth.

---

## 3. Decision ↔ Case Relationship

### 3.1 Cardinality (Schema Evidence)

**FACT:** No unique constraint on `decisions.case_id`. This means:

```text
Case 1 → N Decision
```

Multiple Decision rows can reference the same Case. This is **CORRECT** per canonical design (Audit 3 §5, Design §5).

**Evidence:** `idx_decisions_case` is `(case_id, created_at DESC)` — a non-unique index. Migration 000055.

### 3.2 One Final Decision vs Multiple Decision History

**FACT:** The schema allows multiple Decision rows per Case. The canonical design (Audit 3 §5.6) explicitly states:

> "YA — append-only; appeal menghasilkan Decision #2 untuk Case yang sama"

**BUSINESS DECISION REQUIRED:** Does v1 support appeal producing a second Decision (reversed/upheld)?

- **If YES:** Multiple Decisions per Case is correct; latest is determined by `created_at DESC`
- **If NO (v1):** Could add a partial unique index `UNIQUE (case_id) WHERE outcome != 'reversed'` — but this is NOT in current schema and should NOT be added speculatively

**Default assumption:** Multiple Decisions per Case is supported (canonical design says so). This is **LOCKED** by canonical design.

### 3.3 Decision Requires a Case

**FACT:** `case_id uuid NOT NULL` + FK `decisions_case_id_fkey`. A Decision **cannot exist** without a Case.

```sql
-- Proof: FK constraint prevents orphaned Decision
ALTER TABLE decisions ADD CONSTRAINT decisions_case_id_fkey
    FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE CASCADE;
```

**INVARIANT I1: Decision belongs to a Case — PROVEN by DB schema.**

### 3.4 Decision Cannot Reference Report Directly

**FACT:** The `decisions` table has no `report_id` column. Decision references Case, not Report.

This is **CORRECT** per canonical design:
```text
Report → Case → Decision
```

### 3.5 Decision Lifecycle and Case Resolution

**FACT:** `CaseService.ResolveCase` (application/case_service.go:77) marks a Case as resolved when a Decision is made. This is called within the Decision creation flow.

**Evidence:**
```go
// case_service.go:77
// This is called when a Decision is made against the Case.
func (s *CaseService) ResolveCase(ctx context.Context, caseID uuid.UUID) error {
```

**FACT:** `CanonicalCase.Resolve()` (entity/canonical_case.go:98) transitions `open → resolved` and sets `ClosedAt`.

**Relationship:**
```text
Decision created (INSERT)
    ↓
Case resolved (UPDATE status → resolved)
```

**BUSINESS DECISION REQUIRED:** Should `Case resolved` happen:
- **A:** Atomically in the same transaction as Decision INSERT? (RECOMMENDED)
- **B:** As a separate step after Decision creation?

**Current code:** `CaseService.ResolveCase` exists as a separate method, suggesting option B. The Decision creation transaction has NOT been implemented yet, so this is undefined.

### 3.6 Decision Cannot Be Created Before Case Resolved

**FACT (schema):** Decision references Case by FK. Case status does NOT prevent Decision creation (no CHECK constraint on case status in decisions table).

**FACT (canonical):** Decision creation is what resolves the Case. So:
```text
Case open → Decision created → Case resolved
```

Decision is created WHILE Case is open, and the act of Decision creation resolves the Case.

**INVARIANT: Decision can be created only when Case is open — NOT ENFORCED by DB, must be enforced by service.**

### 3.7 Case Status After Decision

**FACT:** Case status values: `open`, `resolved` (case_status_enum).

**FACT:** When a Decision is made, Case transitions to `resolved`. This is canonical (Audit 3 §4).

**No `decided` or `under_review` status exists.** The schema uses the simple two-state model: `open → resolved`.

---

## 4. Decision Immutability

### 4.1 DB Trigger (Active Authority)

```sql
-- migration 000055_canonical_moderation_foundation.up.sql
CREATE OR REPLACE FUNCTION prevent_decisions_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'decisions rows are immutable (append-only governance history)';
END;
$$;

CREATE TRIGGER trg_decisions_immutable
    BEFORE UPDATE ON decisions
    FOR EACH ROW
    EXECUTE FUNCTION prevent_decisions_update();
```

**Classification: ACTIVE AUTHORITY** — This is the final immutability guard.

### 4.2 UPDATE Path Analysis

| Path | Exists? | Classification |
|---|---|---|
| DB trigger blocking UPDATE | ✅ `trg_decisions_immutable` | **ACTIVE AUTHORITY** |
| Repository method UPDATE decisions | ❌ No code found | N/A |
| Service method UPDATE decisions | ❌ No code found | N/A |
| Handler endpoint UPDATE decisions | ❌ No route exists | N/A |
| Admin API UPDATE decisions | ❌ Routes removed in Slice 2 | N/A |

**INVARIANT I2: Decision is immutable — PROVEN by DB trigger + absence of application mutation paths.**

### 4.3 DELETE Path Analysis

| Path | Exists? | Classification |
|---|---|---|
| DB trigger blocking DELETE | ❌ **NO trigger** | **GAP** (non-blocking) |
| Repository method DELETE decisions | ❌ No code found | N/A |
| Service method DELETE decisions | ❌ No code found | N/A |
| Handler endpoint DELETE decisions | ❌ No route exists | N/A |
| CASCADE from cases DELETE | ✅ `ON DELETE CASCADE` | **LEGITIMATE** (cascade cleanup) |

**Assessment:** No application path performs DELETE on decisions. The CASCADE from cases DELETE is the only delete path, which is legitimate governance cleanup (if Case is deleted, its Decisions go with it — but Cases are never deleted per canonical design).

**FINDING (non-blocking):** No `prevent_decisions_delete()` trigger exists. Recommend adding for defense-in-depth, but NOT a blocker.

### 4.4 Immutable Trigger Test Evidence

**FACT:** Migration test (`migration_000055_canonical_moderation_foundation_test.go`) verifies:
- `decisions` table exists
- Trigger exists and prevents UPDATE
- Outcome enum has correct values

### 4.5 No Mutation Vectors in Code

**FACT:** Codebase search for `UPDATE decisions`, `DELETE FROM decisions`, `UPDATE.*decisions` in Go files found **zero matches** within the governance module.

**FACT:** No Go entity file `entity/decision.go` exists. No `application/decision_service.go`. No `infrastructure/repository/decision_repository.go`. Therefore, no application code can mutate Decisions.

**VERDICT: Decision immutability is PROVEN.**

---

## 5. Outcome Semantics

### 5.1 Canonical Vocabulary

```sql
CREATE TYPE decision_outcome_enum AS ENUM ('no_violation', 'violation');
```

**FACT:** The schema implements exactly the locked business decision vocabulary:
- `no_violation` — content complies, no enforcement needed
- `violation` — policy violated, enforcement warranted

**INVARIANT I3: outcome ∈ {no_violation, violation} — PROVEN by DB enum constraint.**

### 5.2 Legacy Vocabulary Search

| Vocabulary | Location | Classification |
|---|---|---|
| `approve/reject/enforce` (Decision type) | `entity/governance_case.go:53-59` | **LEGACY / ACTIVE CONFLICT** — `Decision` type in GovernanceCase uses `approve/reject/enforce`, which is the old super-entity vocabulary |
| `pending/approved/rejected/enforced` (GovernanceCaseStatus) | `entity/governance_case.go:46-50` | **LEGACY / ACTIVE CONFLICT** — mixes Case lifecycle + Decision outcome + Enforcement |
| `enforced` (appeal_handler.go:591) | `delivery/http/appeal_handler.go:591` | **LEGACY** — `mapStatusToDecision` returns "enforced" for enforced status |
| `enforced` (admin types) | `apps/admin/src/types/moderation.ts:3` | **LEGACY** — `ModerationCaseStatus = 'pending' | 'approved' | 'rejected' | 'enforced'` |
| `enforce` (admin CaseAction) | `apps/admin/src/types/moderation.ts:5` | **LEGACY** — `CaseAction = 'approve' | 'reject' | 'enforce'` |
| `no_violation` | Schema enum | **CANONICAL** |
| `violation` | Schema enum | **CANONICAL** |
| `removed` (legacy enum) | Dropped in migration 000056 | **DELETED** — `moderation_status_enum.removed` is gone |
| `action_type` (DomainAction) | `entity/domain_action.go`, `domain_action_repository_impl.go` | **PARKED/ZOMBIE** — no migration, no application code |
| `sanction` | Not found in governance module | **NOT PRESENT** |
| `strike` | `bnr_strike_handler.go` (finance domain) | **UNRELATED** — buyer Non-Responsive strikes are a finance concept, not moderation |

### 5.3 Conflict Assessment

**ACTIVE CONFLICT:** The legacy `Decision` type string in `governance_case.go:53-59` defines `approve/reject/enforce` as Decision values. This is NOT the canonical `no_violation/violation`. This type is used by:
- `entity/governance_case.go` (GovernanceCase entity methods)
- `entity/enforce_notes_test.go` (tests)
- `application/appeal_service.go` (references GovernanceCase status)

**These are LEGACY and will be replaced when Appeal domain is rebuilt (Slice 9).** They do NOT conflict with the canonical `decision_outcome_enum` because they operate on different tables (`moderation_cases` is dropped).

**VERDICT:** Canonical vocabulary is clean in the schema. Legacy vocabulary exists only in GovernanceCase code paths that operate on dropped tables. No active authority conflict.

---

## 6. Decision ↔ Enforcement Boundary

### 6.1 Current State: No Enforcement Creation Path

**FACT:** The `enforcements` table exists (migration 000055) with correct schema:
```sql
CREATE TABLE enforcements (
    id             uuid DEFAULT gen_random_uuid() NOT NULL,
    decision_id    uuid NOT NULL,  -- FK → decisions(id) ON DELETE CASCADE
    target_type    moderation_target_type_enum NOT NULL,
    target_id      uuid NOT NULL,
    status         enforcement_status_enum DEFAULT 'pending' NOT NULL,
    ...
    CONSTRAINT enforcements_decision_target_unique UNIQUE (decision_id, target_type, target_id)
);
```

**FACT:** **No Go code creates enforcement records.** The Decision → Enforcement creation path does not exist yet.

**FACT:** The current enforcement execution happens via outbox events:
```text
GovernanceCase.Enforce() (legacy)
    ↓
Outbox event emitted (moderation.*.removed)
    ↓
ModerationEventHandler processes event
    ↓
Target domain mutation (content/comment/for_sale/auction/user)
```

This is the **LEGACY** enforcement path. It does NOT create enforcement records in the `enforcements` table.

### 6.2 The False-Success Pattern (Legacy)

**FACT (Business Truth §10):** The critical invariant is:
```text
Decision FINAL + Enforcement PENDING = valid state
Decision FINAL + Enforcement SUCCEEDED = valid state
Decision FINAL + Enforcement FAILED = valid state
Decision FINAL + (assume success) = INVALID
```

**FACT (Audit 3 §19):** The current system has:
```text
GovernanceCase.status = 'enforced'
  = Decision made + outbox event emitted
  ≠ target mutation succeeded
```

This is the **false-success** pattern: the Case status claims enforcement succeeded when it only means enforcement was requested.

**FACT:** The Admin UI displays `enforced` as a final badge (`apps/admin/src/types/moderation.ts:243-247`):
```typescript
export const moderationCaseStatusLabels: Record<ModerationCaseStatus, string> = {
  pending: 'Pending',
  approved: 'Approved',
  rejected: 'Rejected',
  enforced: 'Enforced',  // ← FALSE SUCCESS
}
```

**INVARIANT I4: Decision is not Enforcement — PROVEN by canonical schema design.** But the legacy code/paths do NOT yet respect this invariant. The `enforcements` table enforces the boundary at DB level, but no code writes to it.

### 6.3 What Must Be True (Canonical Boundary)

```text
Decision created (INSERT decisions)
    ↓
Enforcement created (INSERT enforcements, status=pending) — ATOMIC with Decision
    ↓
Outbox event emitted — ATOMIC with Decision + Enforcement
    ↓
Worker processes outbox event
    ↓
Target domain mutation executed
    ↓
Worker writes back: UPDATE enforcements SET status=succeeded/failed
```

**GAP:** The atomic Decision + Enforcement + Outbox creation does not exist yet. The enforcement write-back does not exist yet.

### 6.4 ModerationEventHandler as Execution Engine

**FACT:** `ModerationEventHandler` (`worker/moderation_event_handler.go`) is the CURRENT enforcement executor. It:
- Receives outbox events (moderation.*.removed/restored)
- Routes to target domain methods
- Returns error (retry) or nil (success)

**FACT:** The handler does NOT write back to `enforcements` table. It does NOT reference enforcement IDs. The handler operates on outbox events, not enforcement records.

**CLASSIFICATION: ACTIVE CONSUMER (legacy pattern)** — This handler will need to be adapted to write back enforcement status when the canonical Enforcement runtime is implemented.

### 6.5 Payload Mismatch

**FACT:** Current outbox event payload (`moderation_event_handler.go:50-54`):
```go
type moderationRemovedPayload struct {
    CaseID       string  `json:"case_id"`
    ResourceType string  `json:"resource_type"`
    ResourceID   string  `json:"resource_id"`
    DecisionNote *string `json:"decision_note,omitempty"`
}
```

**Canonical payload should include:**
```text
enforcement_id  — idempotency anchor
decision_id     — traceability
case_id         — traceability (optional)
subject_type    — target type
subject_id      — target ID
action_type     — consequence type
```

**GAP:** Current payload lacks `enforcement_id` and `decision_id`. Must be replaced when Enforcement runtime is implemented.

---

## 7. Enforcement Creation Relationship

### 7.1 Schema Design

```text
Decision (1) → (N) Enforcement
```

One Decision can produce multiple Enforcements (e.g., a violation decision for a user who owns both content and a for_sale listing might enforce on both).

**FACT:** `UNIQUE (decision_id, target_type, target_id)` prevents duplicate Enforcement for the same (Decision, target).

### 7.2 Business Rules (Not Yet Implemented)

| Question | Canonical Answer | Status |
|---|---|---|
| Every `violation` Decision creates Enforcement? | **YES** (per canonical design §7) | **NOT IMPLEMENTED** |
| `no_violation` Decision creates no Enforcement? | **YES** | **NOT IMPLEMENTED** |
| One Decision → multiple Enforcements? | **YES** (multiple targets) | **NOT IMPLEMENTED** |
| Enforcement created independently of Decision? | **NO** — always requires Decision | **ENFORCED by FK** |
| Enforcement atomic with Decision? | **YES** (same transaction) | **NOT IMPLEMENTED** |
| Who creates Enforcement? | Decision service (within Decision creation TX) | **NOT IMPLEMENTED** |
| Outbox involved? | **YES** — outbox event created atomically with Enforcement | **NOT IMPLEMENTED** |

**VERDICT:** Schema supports the canonical Enforcement creation model. Implementation is entirely missing.

### 7.3 Business Decisions Still Required

1. **Does `no_violation` create an Enforcement record (with status=skipped/no_action)?** Or simply no Enforcement row?
   - Default: no Enforcement row for `no_violation`
2. **Atomic boundary:** Should Decision + Enforcement + Outbox be in one TX?
   - Default: YES (per canonical design §7)
3. **Who creates Enforcement?** Decision service or Enforcement service?
   - Default: Decision service creates Enforcement atomically

---

## 8. Audit Trail

### 8.1 Current Audit Infrastructure

| System | Type | Reliability | Used by Moderation? |
|---|---|---|---|
| `admin_audit_logs` (LogSafe) | Best-effort | **TIDAK reliable** — failure does not rollback business TX | **YES** — warning_handler, appeal_handler use LogSafe |
| `audit_events` (AuditEventRepository) | Reliable append-only | **Reliable** — in-transaction, write succeeds or TX fails | **NO** — moderation does not use audit_events |

### 8.2 Decision Audit Requirements (Canonical)

Per Business Truth §28, the following Decision events should be audited:
- Decision created (who, when, what outcome, which Case)
- Decision immutable evidence (snapshot at creation time)

### 8.3 Current Gap

**FACT:** No Decision creation path exists, so no Decision audit exists.

**FACT:** The `audit_events` table and `AuditEventRepository` (`governance/audit/repository/audit_event_repository.go`) provide a reliable append-only audit mechanism. It supports:
- `event_type` (string)
- `entity_type` + `entity_id` (polymorphic reference)
- `actor_type` + `actor_id` (who performed the action)
- `payload_json` (arbitrary evidence)
- `created_at` (immutable timestamp)

**FACT:** The `AdminAuditLogger.LogSafe` (`audit/admin_audit_logger.go:86-88`) writes to `admin_audit_logs` and ignores errors — this is best-effort.

### 8.4 Recommendation

Decision creation should emit audit to `audit_events` (reliable, in-transaction), NOT to `admin_audit_logs` (best-effort). The `audit_events` infrastructure already exists and is used by other domains.

**GAP:** Decision audit trail is not implemented. Must use `audit_events` (not LogSafe).

---

## 9. Admin Consumer Map

### 9.1 Current Admin Routes (Backend)

| Route | Handler | Status | Classification |
|---|---|---|---|
| `POST /admin/moderation/cases` (CreateCase) | REMOVED | **REMOVED in Slice 2** | LEGACY (code removed) |
| `GET /admin/moderation/cases` (ListCases) | REMOVED | **REMOVED in Slice 2** | LEGACY (code removed) |
| `GET /admin/moderation/cases/:id` (GetCase) | REMOVED | **REMOVED in Slice 2** | LEGACY (code removed) |
| `POST /admin/moderation/cases/:id/action` (ReviewCase) | REMOVED | **REMOVED in Slice 2** | LEGACY (code removed) |
| `GET /admin/appeals` | `AppealHandler.AdminListAppeals` | **ACTIVE** (legacy) | Uses GovernanceCase |
| `PUT /admin/appeals/:id/review` | `AppealHandler.AdminReviewAppeal` | **ACTIVE** (legacy) | Uses GovernanceCase |
| `GET /admin/warnings` | `WarningHandler.AdminListWarnings` | **ACTIVE** (legacy standalone) | No Decision provenance |
| `POST /admin/warnings` | `WarningHandler.AdminIssueWarning` | **ACTIVE** (legacy standalone) | No Decision provenance |
| `DELETE /admin/warnings/:id/revoke` | `WarningHandler.AdminRevokeWarning` | **ACTIVE** (legacy standalone) | — |

**FACT:** `routes_core.go:762-775` contains comment:
```
// ===== MODERATION ADMIN ROUTES — REMOVED (SLICE 2) =====
// The legacy admin Case review endpoints (ListCases/GetCase/
// GetCaseEvidence/ApplyAction) were backed by the rejected
// GovernanceCase runtime reading the dropped moderation_cases table.
// They are removed with that runtime. The canonical Case/Decision/
// Enforcement admin workflow is rebuilt in a later slice.
```

### 9.2 Admin Frontend (apps/admin/src)

| Component | File | Status | Classification |
|---|---|---|---|
| `ModerationCasesPage` | `pages/ModerationCasesPage.tsx` | **ACTIVE** (dead end — backend routes removed) | **ZOMBIE** |
| `CaseDetailModal` | `components/moderation/CaseDetailModal.tsx` | **ACTIVE** (dead end) | **ZOMBIE** |
| `AppealsPage` | `pages/AppealsPage.tsx` | **ACTIVE** (legacy) | Uses legacy types |
| `WarningsPage` | `pages/WarningsPage.tsx` | **ACTIVE** (legacy standalone) | No Decision provenance |
| `IssueWarningModal` | `components/moderation/IssueWarningModal.tsx` | **ACTIVE** (legacy standalone) | No Decision provenance |
| `AppealDetailModal` | `components/moderation/AppealDetailModal.tsx` | **ACTIVE** (legacy) | Uses GovernanceCase types |

### 9.3 Admin TypeScript Types (Legacy)

**FACT:** `apps/admin/src/types/moderation.ts` contains legacy types:
```typescript
export type ModerationCaseStatus = 'pending' | 'approved' | 'rejected' | 'enforced'
export type ResourceType = 'content' | 'comment' | 'user' | 'chat_message' | 'fixed_price_sale' | 'auction'
export type CaseAction = 'approve' | 'reject' | 'enforce'
```

**Classification: LEGACY** — These types reference the rejected super-entity vocabulary. They must be replaced with canonical types when the Decision/Case/Enforcement admin workflow is rebuilt.

### 9.4 What Admin Needs for Decision (Not Yet Implemented)

Per canonical design (Audit 3 §18):

| Admin Action | Route (design) | Status |
|---|---|---|
| List Cases | `GET /admin/cases` | **NOT IMPLEMENTED** |
| Inspect Case | `GET /admin/cases/:id` | **NOT IMPLEMENTED** |
| Create Decision | `POST /admin/cases/:id/decisions` | **NOT IMPLEMENTED** |
| View Decision History | `GET /admin/cases/:id/decisions` | **NOT IMPLEMENTED** |
| View Enforcement Status | `GET /admin/enforcements/:id` | **NOT IMPLEMENTED** |
| Retry Enforcement | `POST /admin/enforcements/:id/retry` | **NOT IMPLEMENTED** |

---

## 10. Legacy/Residue Map

### 10.1 GovernanceCase Entity

| Aspect | Evidence | Classification |
|---|---|---|
| `entity/governance_case.go` | Still exists, imported by appeal_service.go, appeal_handler.go | **LEGITIMATE FUTURE DEPENDENCY** (appeal, Slice 9) |
| `Decision` type (approve/reject/enforce) | `governance_case.go:53-59` | **LEGACY** — vocabulary conflict with canonical `no_violation/violation` |
| `GovernanceCaseStatus` (pending/approved/rejected/enforced) | `governance_case.go:46-50` | **LEGACY** — mixes Case+Decision+Enforcement |
| `NewGovernanceCase()` | Called by appeal_service_test.go | **LEGITIMATE** (test for legacy appeal domain) |
| `ShouldEmitEnforcementEvents()` | `governance_case.go:161-163` | **DEAD/ZOMBIE** — reads moderation_cases which is dropped |
| `Enforce()` method | `governance_case.go:140-152` | **DEAD/ZOMBIE** — operates on GovernanceCase which is legacy |
| `ErrEnforceRequiresNote` | `governance_case.go:84-91` | **DEAD/ZOMBIE** — legacy enforce path |

### 10.2 DomainAction Entity

| Aspect | Evidence | Classification |
|---|---|---|
| `entity/domain_action.go` | Full entity with idempotency, execution groups, rollback | **PARKED/ZOMBIE** |
| `entity/moderation_action.go` | ActionType enum for DomainAction | **PARKED/ZOMBIE** |
| `infrastructure/repository/domain_action_repository.go` | Interface defined | **PARKED/ZOMBIE** |
| `infrastructure/repository/domain_action_repository_impl.go` | Implementation exists | **PARKED/ZOMBIE** |
| `worker/domain_action_worker.go` | Worker exists but never instantiated | **PARKED/ZOMBIE** |
| `outbox_event_registry.go:198-203` | "DomainActionWorker is PARKED: never instantiated" | **PARKED/ZOMBIE** |

### 10.3 ModerationRepository (Legacy)

| Aspect | Evidence | Classification |
|---|---|---|
| `repository/moderation_repository.go` | Interface with `GetByID` only | **DEAD/ZOMBIE** (runtime-dead, reads dropped table) |
| `repository/moderation_repository_impl.go` | Queries `moderation_cases` table | **DEAD/ZOMBIE** (table dropped in 000056) |
| Wiring in `dependencies.go:2371` | Created for appeal domain compilation | **LEGITIMATE FUTURE DEPENDENCY** (appeal, Slice 9) |

### 10.4 AppealReversalService

| Aspect | Evidence | Classification |
|---|---|---|
| `application/appeal_reversal_service.go:1` | "PARKED — AppealReversalService is not instantiated" | **PARKED/ZOMBIE** |
| Uses DomainActionRepository | `domainActionRepo` field | **PARKED/ZOMBIE** (depends on parked entity) |
| Uses `GetByGovernanceCaseID` | `appeal_reversal_service.go:95,311` | **PARKED/ZOMBIE** |

### 10.5 ResourceType Entity

| Aspect | Evidence | Classification |
|---|---|---|
| `entity/moderation_resource_type.go` | Includes `chat_message` | **LEGACY** (canonical scope excludes chat_message) |
| `ResourceType` enum values | `content, comment, for_sale, auction, user, chat_message` | **LEGACY** — `chat_message` must be removed |

### 10.6 Warning Entity

| Aspect | Evidence | Classification |
|---|---|---|
| `entity/warning.go` | Standalone warning without DecisionID field | **LEGACY** (canonical: Decision → Warning) |
| `WarningLevel` (info/warning/severe) | `warning.go:26-34` | **LEGACY** — not part of canonical Decision vocabulary |
| `POST /admin/warnings` (standalone creation) | `routes_core.go:920-922` | **LEGACY** — must be removed when Warning slice rebuilt |

### 10.7 Admin UI Types

| Aspect | Evidence | Classification |
|---|---|---|
| `ModerationCaseStatus = 'pending' | 'approved' | 'rejected' | 'enforced'` | `types/moderation.ts:3` | **LEGACY** — super-entity vocabulary |
| `ResourceType` includes `chat_message`, `fixed_price_sale` | `types/moderation.ts:4` | **LEGACY** — must use `moderation_target_type_enum` |
| `CaseAction = 'approve' | 'reject' | 'enforce'` | `types/moderation.ts:5` | **LEGACY** — must use `no_violation/violation` |
| `moderationCaseStatusLabels.enforced = 'Enforced'` | `types/moderation.ts:246` | **LEGACY** — false-success badge |

### 10.8 Residue Summary

| Category | Items |
|---|---|
| **DEAD/ZOMBIE** (runtime-dead, kept for compilation) | GovernanceCase entity, ModerationRepository, DomainAction, DomainActionWorker, AppealReversalService, ShouldEmitEnforcementEvents, ErrEnforceRequiresNote |
| **LEGACY** (active but must be replaced) | Decision type (approve/reject/enforce), GovernanceCaseStatus, Warning standalone, admin UI types, `chat_message` resource type |
| **LEGITIMATE FUTURE DEPENDENCY** (Slice 9) | GovernanceCase entity (appeal), ModerationRepository (appeal compilation), appeal_repository_impl (appeal) |
| **PARKED** (not wired, no migration) | DomainAction entity/repository/worker, AppealReversalService |
| **UNRELATED** | bnr_strike_handler (finance strikes), DecisionContract (mobile payment) |

---

## 11. Canonical Invariant Proof Matrix

| Invariant | Statement | Evidence | Status |
|---|---|---|---|
| **I1** | Decision belongs to a Case | `case_id uuid NOT NULL` + FK `decisions_case_id_fkey → cases(id)` | **PROVEN** |
| **I2** | Decision is immutable | `trg_decisions_immutable` BEFORE UPDATE trigger + no app mutation paths | **PROVEN** |
| **I3** | outcome ∈ {no_violation, violation} | `decision_outcome_enum` DB enum constraint | **PROVEN** |
| **I4** | Decision is not Enforcement | Separate tables: `decisions` ≠ `enforcements`; FK `enforcements.decision_id → decisions(id)` | **PROVEN** (schema) |
| **I5** | Enforcement result does not mutate Decision truth | Decision has no mutable fields; Enforcement is a separate entity | **PROVEN** (schema) |
| **I6** | No legacy GovernanceCase authority | `moderation_cases` dropped in migration 000056; GovernanceCase entity runtime-dead | **PROVEN** (DB level) — legacy Go code remains but is runtime-dead |

### Additional Invariants (Not in Lock List)

| Invariant | Statement | Status |
|---|---|---|
| **I7** | Decision always has a moderator (decided_by) | **PROVEN** — `decided_by uuid NOT NULL` + FK → users(id) |
| **I8** | Multiple Decisions per Case allowed | **PROVEN** — no unique constraint on case_id |
| **I9** | Decision cannot exist without Case | **PROVEN** — FK constraint, NOT NULL |
| **I10** | Case resolved atomically with Decision | **NOT PROVEN** — `CaseService.ResolveCase` is a separate method; atomicity not yet implemented |
| **I11** | Decision creation creates Enforcement atomically | **NOT PROVEN** — no enforcement creation path exists |

---

## 12. Business Ambiguities

### 12.1 Affects Decision Implementation Directly

| # | Ambiguity | Impact | Recommendation |
|---|---|---|---|
| **BA-1** | **One final Decision vs Decision history** — Can appeal produce Decision #2 for same Case? | Schema allows multiple; canonical design says YES; v1 may restrict | Default: YES (multiple). If v1 restricts, add partial unique index later |
| **BA-2** | **Case auto-resolved** — Does creating Decision automatically resolve Case? | `CaseService.ResolveCase` exists as separate method | Default: YES, atomically in same TX as Decision creation |
| **BA-3** | **violation always creates Enforcement?** — Or can admin create violation Decision without enforcement? | Enforcement table expects at least one row per violation | Default: YES, violation always creates at least one Enforcement |
| **BA-4** | **Decision note required?** — Must admin always provide a note? (Legacy `ErrEnforceRequiresNote` required it for `enforce`) | `decision_note text NULLABLE` — currently optional in schema | Default: recommended but not mandatory in v1. Schema allows NULL |
| **BA-5** | **Decision reason taxonomy** — Is `no_violation/violation` sufficient, or does v1 need more granular outcomes? | Per locked decisions: only `no_violation/violation` | LOCKED. No change needed |
| **BA-6** | **Reopening semantics** — If Case is resolved, can a new Decision be added? (new Report → new Case) | Design §7: "Terminal Case tidak pernah dibuka kembali" | LOCKED. New Report after resolved Case → new Case |
| **BA-7** | **Decision reviewer identity** — Should decided_by be admin UUID only, or system/worker? | Schema: `decided_by → users(id)` — requires valid user | LOCKED. Only human admins make Decisions |
| **BA-8** | **Evidence snapshot** — Should Decision capture target state at creation time? | No snapshot column in schema | BUSINESS DECISION REQUIRED for v2. Not blocking v1 |

### 12.2 Not Blocking Decision Implementation

| # | Ambiguity | Note |
|---|---|---|
| Report sold/ended object | Out of Decision scope (Report domain) |
| Auction ber-bid moderation stop | Enforcement domain concern, not Decision |
| Warning repeat/cap policy | Warning domain concern |
| Appeal eligibility scope | Appeal domain concern |

---

## 13. Implementation Readiness

### 13.1 Decision Runtime Components

| Component | DB Schema | Go Entity | Service | Repository | Handler | Tests |
|---|---|---|---|---|---|---|
| Decision | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

**STATUS: DB ONLY — no Go code exists.**

### 13.2 Dependency Readiness

| Dependency | Status | Notes |
|---|---|---|
| `cases` table (migration 000055) | ✅ READY | FK target exists |
| `users` table | ✅ READY | FK target for decided_by |
| `enforcements` table (migration 000055) | ✅ READY | Enforcement creation can reference decisions |
| Case runtime (Slice 3) | ✅ IMPLEMENTED | CaseService.ResolveCase available |
| Report runtime (Slice 2) | ✅ IMPLEMENTED | Reports link to Cases |
| `audit_events` infrastructure | ✅ READY | Can be used for Decision audit |
| Outbox infrastructure | ✅ READY (retry broken — P1) | Must be fixed before Enforcement delivery |

### 13.3 What Decision Slice Must Build

1. **Entity:** `entity/decision.go` — Decision struct, constructor, value objects
2. **Repository:** `infrastructure/repository/decision_repository.go` + impl — Create, GetByID, ListByCase
3. **Service:** `application/decision_service.go` — CreateDecision (atomic with Case resolution + Enforcement creation + Outbox event + Audit)
4. **Handler:** `delivery/http/decision_handler.go` — POST /admin/cases/:id/decisions, GET /admin/cases/:id/decisions
5. **Routes:** Add to routes_core.go under admin routes
6. **Dependencies wiring:** Add to serverboot/dependencies.go
7. **Tests:** Unit tests for entity/service, integration tests for repository

### 13.4 Transaction Boundary (Design)

```text
TX_A (Decision creation — one atomic transaction):
  1. INSERT INTO decisions (case_id, decided_by, outcome, decision_note, created_at)
  2. UPDATE cases SET status='resolved', closed_at=now() WHERE id=case_id AND status='open'
  3. IF outcome='violation':
     a. INSERT INTO enforcements (decision_id, target_type, target_id, status='pending', ...)
     b. INSERT INTO outbox (event_type='moderation.enforcement.requested', payload{enforcement_id, ...})
  4. INSERT INTO audit_events (event_type='decision.created', entity_type='decision', entity_id=decision.id, ...)
COMMIT
```

---

## 14. Exact Blockers/Gaps

### BLOCKER (Must resolve before Decision runtime)

**None.** The schema is ready. The Case runtime (Slice 3) is ready. No DB-level blocker prevents Decision implementation.

### FINDINGS (Non-blocking, should be addressed)

| # | Finding | Severity | Action |
|---|---|---|---|
| F1 | No DELETE trigger on decisions table | LOW | Add `prevent_decisions_delete()` for defense-in-depth |
| F2 | Case auto-resolve atomicity not yet implemented | MEDIUM | Decision service must atomically resolve Case |
| F3 | Enforcement creation atomic with Decision not implemented | HIGH | Must be built in Decision slice or Enforcement slice |
| F4 | Outbox retry broken (P1 from Audit 2) | HIGH | Must be fixed before Enforcement delivery works |
| F5 | ModerationEventHandler has no enforcement write-back | HIGH | Must be adapted to write back enforcement status |
| F6 | Current outbox payload lacks enforcement_id/decision_id | MEDIUM | Payload must be redesigned for canonical Enforcement |
| F7 | Admin UI uses legacy types (pending/approved/rejected/enforced) | MEDIUM | Replace with canonical types when admin workflow rebuilt |
| F8 | Warning standalone path still exists | MEDIUM | Remove when Warning slice rebuilt |
| F9 | `chat_message` in ResourceType enum | LOW | Remove in cleanup slice |
| F10 | No governance audit path for Decision mutations | MEDIUM | Use `audit_events` (not LogSafe) for Decision audit |

---

## 15. Final Verdict

### **PASS WITH FINDINGS**

**PASS** because:
1. ✅ Decision schema is correct and complete for canonical Decision entity
2. ✅ Immutability is enforced by DB trigger (`trg_decisions_immutable`)
3. ✅ Outcome vocabulary matches locked business decisions (`no_violation/violation`)
4. ✅ FK relationships are correct (Decision → Case, Decision → User)
5. ✅ Multiple Decisions per Case supported (canonical design)
6. ✅ Decision cannot exist without Case (FK NOT NULL)
7. ✅ No legacy GovernanceCase authority at DB level (`moderation_cases` dropped)
8. ✅ Canonical invariants I1–I6 are PROVEN
9. ✅ Case runtime (Slice 3) is ready as dependency
10. ✅ `enforcements` table schema supports canonical Enforcement model
11. ✅ `audit_events` infrastructure available for reliable Decision audit

**FINDINGS** (non-blocking for Decision schema, must be addressed during implementation):
1. **F1:** No DELETE trigger (defense-in-depth, recommended)
2. **F3:** Enforcement creation not atomic with Decision yet (must build)
3. **F4:** Outbox retry broken (P1, must fix before enforcement delivery)
4. **F5:** ModerationEventHandler lacks enforcement write-back
5. **F6:** Outbox payload redesign needed (enforcement_id, decision_id)
6. **F7:** Admin UI legacy types need replacement
7. **F10:** Decision audit must use `audit_events`, not LogSafe

**No BLOCKER prevents starting Decision runtime implementation (Slice 4).**

---

```text
AUDIT STATUS: PASS WITH FINDINGS

SCHEMA: decisions table (migration 000055) is correct and sufficient
IMMUTABILITY: PROVEN — trg_decisions_immutable + no app mutation paths
OUTCOME VOCABULARY: PROVEN — no_violation/violation per locked decisions
CASE RELATIONSHIP: PROVEN — 1:N (one Case, many Decisions), FK enforced
ENFORCEMENT BOUNDARY: SCHEMA PROVEN, RUNTIME NOT IMPLEMENTED
INVIOLABLES I1-I6: ALL PROVEN
LEGACY RESIDUE: DOCUMENTED, classified, no active authority conflict
IMPLEMENTATION READINESS: DB foundation ready, Go code absent
BLOCKER: NONE
FINDINGS: 10 non-blocking items documented

RECOMMENDATION: Decision runtime (Slice 4) may proceed.
The primary implementation work is building the Go layer
(entity, repository, service, handler) on top of the existing
schema foundation. Enforcement atomicity and outbox fix must
be addressed within or alongside the Decision slice.
```

---

*Audit selesai. Tidak ada implementasi yang dilakukan.*
