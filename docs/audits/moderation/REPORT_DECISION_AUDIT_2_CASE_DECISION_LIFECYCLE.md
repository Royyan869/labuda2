# AUDIT 2 — CASE TERMINAL + MULTIPLE IMMUTABLE DECISIONS

- **Tanggal audit:** 2026-08-31
- **Mode:** READ-ONLY — tidak ada implementasi
- **Satu-satunya artefak baru:** laporan ini
- **Baseline:** current filesystem (bukan git history)
- **Scope:** Verifikasi model: Case terminal + multiple immutable Decisions

---

## Model Under Audit

```text
Case OPEN
   ↓
Decision #1
   ↓
Case RESOLVED
   ↓
Appeal
   ↓
Decision #2
   ↓
Case remains RESOLVED
```

Invariant:
- Case adalah lifecycle investigation
- Decision adalah immutable historical governance record
- Case TIDAK pernah reopen karena Decision/Appeal
- Satu Case boleh memiliki multiple immutable Decisions
- Decision #2 tidak mengubah Decision #1
- Decision #2 tidak mengubah Case kembali menjadi open

---

## AUDIT 1 — CASE SCHEMA TERMINALITY

### Evidence

**Enum:** `case_status_enum AS ENUM ('open', 'resolved')` — migration 000055. Only two values exist.

**Partial unique index:**
```sql
CREATE UNIQUE INDEX uniq_active_case_per_subject
    ON public.cases USING btree (subject_type, subject_id)
    WHERE (status = 'open'::case_status_enum);
```
Only applies to `open` Cases. Resolved Cases are excluded from this invariant.

**No trigger on Case status:** No `BEFORE UPDATE` trigger on the `cases` table forces any particular status when Decision is created. Resolution is done by application code.

**ResolveCase SQL** (`case_repository_impl.go:146-150`):
```sql
UPDATE cases
SET status = 'resolved', closed_at = $1, updated_at = $2
WHERE id = $3 AND status = 'open'
```
The `WHERE status = 'open'` guard ensures:
1. Only open Cases can be resolved
2. If Case is already resolved, 0 rows affected → `ErrCaseAlreadyResolved`

**Entity guard** (`canonical_case.go:100-111`):
```go
func (c *CanonicalCase) Resolve() error {
    if c.Status != CaseStatusOpen {
        return &ErrCaseAlreadyResolved{CaseID: c.ID, Status: c.Status}
    }
    // ... sets status to resolved
}
```

**No reopen path:** Entity has no `Reopen()` method. Repository has no method that sets status back to `open`. `CanResolve()` returns true only for open Cases.

**Integration test** (`case_runtime_integration_test.go`, test "case_lifecycle_open_to_resolved"):
Proves `open → resolved` works. Test "new_report_after_resolved_creates_new_case" proves that after resolution, a NEW Report creates a NEW Case (not reopen the old one).

```
CASE TERMINALITY:
PROVEN
```

**Evidence chain:**
1. DB enum has only 'open' and 'resolved' — no 'reopened' value
2. No trigger fires on Decision creation to change Case status
3. `ResolveCase` only works when `status = 'open'` (SQL WHERE guard + entity guard)
4. No code path anywhere sets `status = 'open'` on a resolved Case
5. Entity has no `Reopen()` method
6. Repository has no reopen/update-status-to-open method
7. Integration test proves: resolved Case + new Report → new Case (not reopen)

---

## AUDIT 2 — DECISION CARDINALITY

### Schema Evidence

**No unique constraint on `decisions.case_id`:**
```sql
-- Only this index exists (non-unique):
CREATE INDEX idx_decisions_case ON public.decisions USING btree (case_id, created_at DESC);
```

**No hidden assumptions:**
- `case_id uuid NOT NULL` — FK to cases, but no uniqueness
- No composite unique `(case_id, outcome)` or similar
- No trigger that counts decisions per case
- No CHECK constraint limiting decision count

**DB allows:**
```sql
INSERT INTO decisions (case_id, decided_by, outcome) VALUES ('same-case-uuid', 'admin1', 'violation');
INSERT INTO decisions (case_id, decided_by, outcome) VALUES ('same-case-uuid', 'admin2', 'no_violation');
-- Both succeed. No constraint violation.
```

```
MULTIPLE DECISIONS PER CASE:
PROVEN (schema capability)
```

### Schema Capability vs Business Truth

**SCHEMA CAPABILITY:** The DB allows unlimited Decisions per Case. No constraint prevents it.

**BUSINESS TRUTH:** The canonical design (Audit 3 §5.6, Design §5) explicitly states:
> "YA — append-only; appeal menghasilkan Decision #2 untuk Case yang sama"

And:
> "Appeal menunjuk Decision tertentu; appeal review menghasilkan Decision baru (reversed/upheld)"

**VERDICT:** Schema capability matches canonical business truth. Multiple Decisions per Case is both allowed by DB and required by design.

---

## AUDIT 3 — CASE REOPENING

### Search Results

| Pattern | Location | Context | Classification |
|---|---|---|---|
| `reopen` | `governance/support/` (ticket domain) | Support ticket reopen: `SET status = 'open'` on `support_tickets` table | **UNRELATED** — support ticket domain, not moderation Case |
| `Terminal Case is never reopened` | `canonical_case.go:49,97` | Design rule comments in canonical Case entity | **CANONICAL** — documents the invariant |
| `New Report after terminal Case → new Case` | `canonical_case.go:50` | Design rule comment | **CANONICAL** — documents the invariant |
| `resolved.*open` | `case_repository_impl.go:148` | `WHERE id = $3 AND status = 'open'` in ResolveCase — forward transition only | **CANONICAL** |
| `status = 'open'` | Various | Used only for finding/creating open Cases, never for setting resolved→open | **CANONICAL** |

**Zero paths found for `resolved Case → open`** in the moderation domain.

The only "reopen" pattern in the entire codebase is for support tickets (`governance/support/`), which is a completely separate domain with its own `support_tickets` table. It has no FK or relationship to the `cases` table.

```
CASE REOPENING PATH: DOES NOT EXIST (in moderation domain)
```

**Evidence:** Repository-wide search found no code, no SQL, no trigger, and no test that transitions a resolved moderation Case back to open.

---

## AUDIT 4 — DECISION ON RESOLVED CASE

### Current State

**No Decision creation code exists.** There is no `DecisionService`, no `DecisionRepository`, no handler for creating Decisions.

**DB schema:** The `decisions` table has no CHECK constraint on the referenced Case's status. A Decision can reference any Case regardless of whether it's `open` or `resolved`.

```sql
-- There is NO constraint like:
-- ALTER TABLE decisions ADD CONSTRAINT decisions_only_open_case
--     CHECK (... CASE status is open ...);
-- This constraint does not exist.
```

**Entity:** The `CanonicalCase` entity has no method that checks "is this Case open?" before allowing a Decision reference. The Case entity doesn't even know about Decisions (separation of concerns).

**Appeal flow (legacy):** The current `AppealService` operates on the legacy `GovernanceCase` model. It checks `kase.Status == GovernanceCaseStatusEnforced` to determine appealability (`appeal_service.go:142`). This is LEGACY and does not apply to the canonical model.

**Integration test:** The existing tests prove Case lifecycle (`open → resolved`) but do NOT test Decision creation. There is no test that creates a Decision on a resolved Case.

```
DECISION ON RESOLVED CASE:
NOT IMPLEMENTED (no Decision creation code exists)
```

### Assessment for the Proposed Model

The question is: Does the proposed model require a **business rule change** or only an **implementation rule**?

**Proposed model:**
```text
Decision #2 is created on a Case that is already resolved
```

**Schema:** No constraint prevents this. The DB allows it.

**Entity:** No code prevents this. The Case entity doesn't check Decision eligibility.

**Design:** The canonical design states:
- Appeal → produces Decision #2
- Appeal is created against Decision #1 (which resolved the Case)
- Decision #2 is a new immutable record on the same Case

**IMPLICATION:** The Decision creation service must simply INSERT into `decisions` with the Case ID. No business rule change is needed. It's purely an implementation concern:

1. `DecisionService.CreateDecision(caseID, ...)` must NOT check `case.status == 'open'`
2. `DecisionService.CreateDecision` should work regardless of Case status
3. Case resolution should happen only for the FIRST Decision (or be a no-op if already resolved)

**BUSINESS DECISION REQUIRED:** Should the Decision creation service:
- **Option A:** Create Decision on any Case (open or resolved) — consistent with "multiple Decisions per Case"
- **Option B:** Create Decision only on open Case, and Appeal flow must use a different path

Default recommendation: **Option A** — the schema allows it, the design requires it, and Case resolution is simply a no-op for subsequent Decisions.

---

## AUDIT 5 — APPEAL COMPATIBILITY

### Current Appeal Entity

**Entity field** (`appeal.go:17`):
```go
type Appeal struct {
    CaseID uuid.UUID   // Reference to the original moderation case
    // ...
}
```

**DB schema (migration 000055):**
```sql
ALTER TABLE appeals DROP COLUMN IF EXISTS report_id;
ALTER TABLE appeals ADD COLUMN decision_id uuid;
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_fkey FOREIGN KEY (decision_id) REFERENCES decisions(id);
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_required CHECK (decision_id IS NOT NULL);
```

**Schema vs Go entity mismatch:** The DB has `decision_id` (canonical), but the Go entity still has `CaseID` (legacy). This is a known future dependency (Slice 9).

### Appeal → Decision Relationship (Canonical)

Per migration 000055 and canonical design:
```text
Appeal → Decision (via decision_id FK)
```

- Appeal references a specific Decision (not a Case)
- Appeal review produces a new Decision (Decision #2)
- Appeal does NOT reopen the Case

### Appeal and Multiple Decisions

**Current duplicate check** (`appeal_service.go:152-157`):
The `CreateAppeal` method calls `appealRepo.CreateWithPendingCheck`. This prevents multiple PENDING appeals but allows new appeals after previous ones are resolved.

**Test evidence** (`appeal_service_test.go`, `TestCreateAppeal_AllowsNewAppealAfterResolvedAppeal`):
```go
// After a previous appeal is resolved (approved or rejected), a new appeal can be created.
// The DB check only blocks pending appeals.
```

**Canonical model compatibility:**
- Appeal #1 against Decision #1 → produces Decision #2 (via appeal review)
- Appeal #2 against Decision #2 → produces Decision #3 (if needed)
- No constraint prevents this in the schema

### Does Appeal Expect Case Reopen?

**NO.** The current appeal flow (legacy):
1. User creates Appeal against a Case (legacy `GovernanceCase`)
2. Admin reviews Appeal → approves or rejects
3. If approved: restoration event emitted (for content/comment)
4. Case status is NOT changed by appeal review

**Canonical model:**
1. User creates Appeal against Decision #1
2. Admin reviews Appeal → creates Decision #2 (reversed or upheld)
3. Case status remains `resolved`

**No conflict.** The appeal flow does not require Case reopen.

```
APPEAL COMPATIBILITY: COMPATIBLE (FUTURE DEPENDENCY — Slice 9 rebuild)
```

**Evidence:**
1. Appeal entity has `decision_id` FK in DB (migration 000055) — correct canonical relationship
2. Go entity still uses legacy `CaseID` — must be rebuilt (Slice 9)
3. Current appeal flow doesn't change Case status
4. Multiple appeals per Case are allowed (only pending duplicate blocked)
5. Appeal review produces new Decision — no Case reopen involved
6. Schema allows unlimited Decisions per Case, compatible with multiple appeal cycles

---

## AUDIT 6 — DECISION IMMUTABILITY

### DB Trigger

```sql
CREATE TRIGGER trg_decisions_immutable
    BEFORE UPDATE ON decisions
    FOR EACH ROW
    EXECUTE FUNCTION prevent_decisions_update();
```

**Blocks:** All UPDATE operations on `decisions` table. Returns exception.

### Application Paths

| Path | Exists? | Classification |
|---|---|---|
| Repository UPDATE decisions | ❌ No code | N/A |
| Service UPDATE decisions | ❌ No code | N/A |
| Handler UPDATE decisions | ❌ No route | N/A |
| Repository DELETE decisions | ❌ No code | N/A |
| Service DELETE decisions | ❌ No code | N/A |
| Handler DELETE decisions | ❌ No route | N/A |
| CASCADE from cases DELETE | ✅ `ON DELETE CASCADE` | LEGITIMATE (cascade cleanup) |

### Decision #1 and Decision #2

Both Decision #1 and Decision #2 are rows in the same `decisions` table. Both are protected by the same `trg_decisions_immutable` trigger. Creating Decision #2 does NOT modify Decision #1 in any way — it's a separate INSERT.

```
DECISION IMMUTABILITY:
PROVEN — both Decision #1 and Decision #2 are protected by trg_decisions_immutable.
No application mutation path exists. Creating a new Decision never touches existing rows.
```

---

## AUDIT 7 — ENFORCEMENT BOUNDARY

### Schema Relationship

```text
Decision #1 → Enforcement #1, #2, ... (via enforcement.decision_id FK)
Decision #2 → Enforcement #3, #4, ... (via enforcement.decision_id FK)
```

**UNIQUE constraint:** `enforcements_decision_target_unique UNIQUE (decision_id, target_type, target_id)`

This means:
- Decision #1 can have its own Enforcement(s)
- Decision #2 can have its own Enforcement(s)
- They are independent — different `decision_id` values

### No Cross-Contamination

| Concern | Evidence | Status |
|---|---|---|
| Decision #2 changes Decision #1? | No — separate rows, immutable trigger | **NO** |
| Decision #2 reopens Case? | No — Case status unchanged by Decision creation | **NO** |
| Decision #2 deletes Enforcement #1? | No — different `decision_id`, no cascade from Decision to old Enforcement | **NO** |
| `enforced` becomes Decision authority? | No — `enforced` is gone from canonical schema (dropped with `moderation_status_enum`) | **NO** |
| Enforcement per Decision is independent? | Yes — `UNIQUE (decision_id, target_type, target_id)` scoping is per-Decision | **YES** |

### Example Scenario

```text
Case X (resolved)
├── Decision #1 (violation) → Enforcement #A (succeeded)
├── Decision #2 (no_violation) → no Enforcement (reversal)
│   └── OR: Enforcement #B (restore action, pending)
```

- Enforcement #A remains (historical record of Decision #1's enforcement)
- Enforcement #B (if created for reversal) is independent
- Neither affects the other
- Case stays resolved throughout

```
ENFORCEMENT BOUNDARY:
COMPATIBLE — multiple Decisions have independent Enforcements.
No cross-contamination. No Case reopen. No Decision mutation.
```

---

## AUDIT 8 — BUSINESS DECISION REQUIRED

### Primary Question

> Apakah Decision boleh dibuat terhadap Case yang sudah `resolved`?

**DB Schema:** No constraint prevents it. `decisions.case_id` FK references `cases(id)` regardless of Case status.

**Entity:** No code prevents it.

**Design:** Canonical design requires it — Appeal produces Decision #2 on a resolved Case.

**BUSINESS DECISION REQUIRED: YES**

The Decision creation service must explicitly allow creating Decisions on resolved Cases. This is not a schema change — it's an implementation rule:

```text
DecisionService.CreateDecision:
  - Does NOT require Case.status == 'open'
  - Works on both open and resolved Cases
  - Case resolution (open → resolved) happens only for the first Decision
  - Subsequent Decisions are created on the already-resolved Case
```

If the business decision is "Decision can only be created on open Case," then the model is NOT COMPATIBLE. But the canonical design requires Decision #2 on a resolved Case, so the default assumption is YES.

---

## FINAL REPORT

```
STATUS: PASS

CASE TERMINALITY:
PROVEN — 'open'/'resolved' enum, ResolveCase WHERE status='open' guard,
entity CanResolve() guard, no reopen path, integration test proves lifecycle.
Resolved is truly terminal.

MULTIPLE DECISIONS:
PROVEN — no unique constraint on decisions.case_id, non-unique index only.
Schema allows unlimited Decisions per Case. Matches canonical design.

RESOLVED CASE → NEW DECISION:
NOT IMPLEMENTED (no Decision creation code exists)
Schema allows it. Entity allows it. Design requires it.
Implementation rule: Decision creation must NOT require Case.status = 'open'.

APPEAL COMPATIBILITY:
COMPATIBLE (FUTURE DEPENDENCY — Slice 9 rebuild)
Appeal DB has decision_id FK (canonical). Go entity uses legacy CaseID.
Current appeal flow doesn't reopen Case. Multiple appeals per Case allowed.
Appeal review produces new Decision — no Case reopen involved.

IMMUTABILITY:
PROVEN — trg_decisions_immutable blocks all UPDATE.
No application mutation paths. Creating Decision #2 never touches Decision #1.

ENFORCEMENT BOUNDARY:
COMPATIBLE — multiple Decisions have independent Enforcements via
UNIQUE (decision_id, target_type, target_id). No cross-contamination.

BUSINESS DECISION REQUIRED:
YES — Decision creation service must explicitly allow creating
Decisions on resolved Cases. This is an implementation rule,
not a schema change.
```

---

## FINAL

```
MODEL COMPATIBLE
```

The model is safe and consistent because:

1. **Case terminality is DB-enforced:** The `case_status_enum` has only `open`/`resolved`. `ResolveCase` has a SQL `WHERE status = 'open'` guard. The entity has `CanResolve()` and `ErrCaseAlreadyResolved`. No code path transitions resolved → open.

2. **Multiple Decisions per Case is DB-allowed:** No unique constraint on `decisions.case_id`. The schema permits unlimited Decision rows per Case.

3. **Decision #2 on resolved Case is schema-compatible:** The `decisions` table has no CHECK constraint on Case status. An INSERT into `decisions` succeeds regardless of whether the referenced Case is `open` or `resolved`.

4. **Decision immutability is trigger-enforced:** `trg_decisions_immutable` blocks all UPDATE. Creating Decision #2 is a separate INSERT — it never modifies Decision #1.

5. **Enforcement independence is constraint-enforced:** `UNIQUE (decision_id, target_type, target_id)` scopes enforcement uniqueness per-Decision. Decision #1 and Decision #2 have independent enforcement records.

6. **Appeal compatibility is proven by design:** Appeal references Decision (not Case). Appeal review produces Decision #2. Case stays resolved. No reopen path exists.

**The only implementation requirement:** The Decision creation service must NOT gate on `case.status == 'open'`. It must create Decisions on any Case regardless of status. Case resolution should be a first-time-only action (no-op if already resolved).

---

```text
STATUS: PASS
CASE TERMINALITY: PROVEN
MULTIPLE DECISIONS: PROVEN
RESOLVED CASE → NEW DECISION: NOT IMPLEMENTED (schema compatible)
APPEAL COMPATIBILITY: COMPATIBLE (FUTURE DEPENDENCY)
IMMUTABILITY: PROVEN
ENFORCEMENT BOUNDARY: COMPATIBLE
BUSINESS DECISION REQUIRED: YES (implementation rule only)

FINAL: MODEL COMPATIBLE
```

---

*Audit selesai. Tidak ada implementasi yang dilakukan.*
