# GOVERNANCE ADMIN BACKEND IMPLEMENTATION — SLICE 6

**Date:** 2026-09-01
**Baseline:** 0be0305 (Slice 5 correctness fix)
**Status:** PASS

---

## 1. API Contract

### Endpoints

| Endpoint | Method | Purpose | Capability |
|----------|--------|---------|------------|
| `GET /admin/governance/cases` | GET | List all Cases (open/resolved) | `moderation.case.read` |
| `GET /admin/governance/cases/:id` | GET | Case detail + Reports + Decisions + Enforcement | `moderation.case.read` |
| `POST /admin/governance/cases/:id/decisions` | POST | Create Decision (atomic: Decision + Case resolve + Enforcement + Outbox) | `moderation.case.resolve` |
| `GET /admin/governance/decisions/:id` | GET | Decision detail (immutable) | `moderation.case.read` |
| `GET /admin/governance/decisions/:id/enforcement` | GET | Enforcement status for a Decision | `moderation.case.read` |

### Query Parameters

**List Cases:**
- `status`: filter by `"open"` or `"resolved"` (optional, empty = all)
- `page`: page number (default 1)
- `limit`: items per page (default 20, max 100)

### Create Decision Request

```json
{
  "outcome": "violation" | "no_violation",
  "target_type": "content" | "comment" | "for_sale" | "auction" | "user",
  "target_id": "uuid",
  "decision_note": "optional string (max 2000)"
}
```

- `target_type` and `target_id` are REQUIRED when `outcome = "violation"`
- `target_type` and `target_id` are IGNORED when `outcome = "no_violation"`

### Response Shapes

**Case List:**
```json
{
  "cases": [
    {
      "id": "uuid",
      "subject_type": "content",
      "subject_id": "uuid",
      "status": "open",
      "created_at": "2026-09-01T...",
      "updated_at": "2026-09-01T...",
      "closed_at": null
    }
  ],
  "page": 1,
  "limit": 20,
  "count": 42
}
```

**Case Detail:**
```json
{
  "case": { "id": "uuid", "subject_type": "content", "subject_id": "uuid", "status": "open", "..." },
  "reports": [
    { "id": "uuid", "reporter_id": "uuid", "subject_type": "content", "reason_code": "prohibited_content", "..." }
  ],
  "decisions": [
    {
      "id": "uuid",
      "case_id": "uuid",
      "decided_by": "uuid",
      "outcome": "violation",
      "decision_note": "...",
      "created_at": "...",
      "enforcements": [
        {
          "id": "uuid",
          "decision_id": "uuid",
          "target_type": "content",
          "target_id": "uuid",
          "status": "pending",
          "attempt_count": 0,
          "requested_at": "...",
          "created_at": "...",
          "updated_at": "..."
        }
      ]
    }
  ]
}
```

**Decision Detail:**
```json
{
  "decision": {
    "id": "uuid",
    "case_id": "uuid",
    "decided_by": "uuid",
    "outcome": "violation",
    "decision_note": "...",
    "created_at": "...",
    "enforcements": [...]
  }
}
```

**Enforcement Status:**
```json
{
  "enforcements": [
    {
      "id": "uuid",
      "decision_id": "uuid",
      "target_type": "content",
      "target_id": "uuid",
      "status": "pending",
      "attempt_count": 0,
      "requested_at": "...",
      "started_at": null,
      "finished_at": null,
      "last_error": null,
      "next_attempt_at": null,
      "created_at": "...",
      "updated_at": "..."
    }
  ]
}
```

---

## 2. Authorization

| Endpoint | Capability | Defense |
|----------|-----------|---------|
| List Cases | `moderation.case.read` | Middleware + handler context |
| Case Detail | `moderation.case.read` | Middleware + handler context |
| Create Decision | `moderation.case.resolve` | Middleware + handler context |
| Decision Detail | `moderation.case.read` | Middleware + handler context |
| Enforcement Status | `moderation.case.read` | Middleware + handler context |

**Existing capability constants used:**
- `moderation.case.read` — read-only case/decision viewing
- `moderation.case.resolve` — create Decisions (write authority)

**No new capabilities invented.** Reuses existing capability registry from `platform/capability`.

---

## 3. Case List

**Implementation:** `CaseRepository.ListAll()` + `CaseRepository.CountAll()`

- Added `ListAll(ctx, tx, statusFilter, limit, offset)` to CaseRepository interface + impl
- Added `CountAll(ctx, tx, statusFilter)` to CaseRepository interface + impl
- Filters by canonical `status` column (`open` | `resolved`)
- Paginated with limit/offset
- Ordered by `created_at DESC` (newest first)

**Does NOT use:** `moderation_cases`, `GovernanceCase`, or legacy vocabulary.

---

## 4. Case Detail

**Implementation:** `CaseRepository.GetByID()` + `ReportRepository.ListByCaseID()` + `DecisionRepository.ListByCase()`

- Added `ListByCaseID(ctx, tx, caseID)` to ReportRepository interface + impl
- Returns Case with all related Reports, Decisions, and Enforcement status per Decision
- Uses canonical `cases` table, NOT `moderation_cases`

---

## 5. Report Association

Reports are linked to Cases via `reports.case_id` (nullable FK).

- `ReportRepository.ListByCaseID()` queries `WHERE case_id = $1`
- Admin Case detail shows all Reports correlated to the Case
- No cross-tenant leakage: Reports are fetched by case_id, not by reporter ownership

---

## 6. Decision Creation

**Implementation:** Pure delegation to existing `DecisionService.CreateDecision()`.

**Handler flow:**
```
Admin HTTP POST /admin/governance/cases/:id/decisions
  → validate request (outcome, target_type, target_id)
  → parse UUIDs
  → DecisionService.CreateDecision(ctx, input)
  → return Decision response
```

**No duplicate Decision authority.** The handler is a thin adapter that:
1. Parses HTTP request into `CreateDecisionInput`
2. Delegates to `DecisionService`
3. Maps domain errors to HTTP status codes
4. Converts entity to response DTO

**DecisionService already handles:**
- Case existence validation
- Decision creation (immutable append-only)
- Case resolution (open → resolved)
- Enforcement creation (if violation)
- Outbox event emission (if violation)
- All within a single atomic transaction

---

## 7. Decision Detail

Returns truthful immutable Decision information:
- `id`, `case_id`, `decided_by`, `outcome`, `decision_note`, `created_at`
- If violation: includes `enforcements` array with full enforcement lifecycle
- No edit/delete endpoints exposed
- Decision immutability enforced by `trg_decisions_immutable` trigger at DB level

---

## 8. Enforcement Status

**Implementation:** `EnforcementRepository.ListByDecision()`

Returns full enforcement lifecycle:
- `status`: pending | processing | succeeded | failed
- `attempt_count`, `started_at`, `finished_at`, `last_error`, `next_attempt_at`
- Only returned for violation decisions
- `no_violation` decisions return empty `enforcements` array

---

## 9. Error Semantics

| Error | HTTP Status | Condition |
|-------|-------------|-----------|
| Invalid request body | 400 | Malformed JSON, missing required fields |
| Invalid case ID | 400 | Non-UUID case_id parameter |
| Invalid outcome | 400 | Outcome not in {no_violation, violation} |
| Invalid target_type | 400 | Target type not in canonical set |
| Case not found | 404 | Case ID does not exist |
| Decision not found | 404 | Decision ID does not exist |
| Server error | 500 | Unexpected DB failure |

**Domain error mapping:**
- `ErrDecisionCaseNotFound` → 404
- `ErrInvalidDecisionOutcome` → 400
- `ErrInvalidEnforcementTargetType` → 400

---

## 10. Transaction Boundary

Admin Create Decision delegates to DecisionService which owns the canonical transaction:

```
DecisionService.CreateDecision:
  BEGIN
    validate Case exists
    INSERT Decision
    if violation: INSERT Enforcement + INSERT outbox event
    if Case open → UPDATE Case status = 'resolved'
  COMMIT
```

**Handler does NOT create its own transaction.** One business operation = one canonical authority.

---

## 11. Security / IDOR Protection

- All admin endpoints require authentication (middleware)
- All admin endpoints require capability-based authorization
- Case/Decision IDs in URL parameters are validated as UUIDs
- No user-scoped data leakage — admin sees ALL cases (governance authority)
- Non-admin users cannot access admin routes (middleware blocks)

---

## 12. Integration Tests

**File:** `backend/tests/governance_admin_integration_test.go`

| Test | Description | Status |
|------|-------------|--------|
| A | Case list returns open + resolved cases | PASS |
| B | Case detail returns case with related reports | PASS |
| C | Case detail returns case with decisions | PASS |
| D | Violation Decision creates Enforcement pending + outbox | PASS |
| E | No_violation Decision creates no Enforcement | PASS |
| F | Decision is immutable (UPDATE trigger blocks) | PASS |
| G | Enforcement status lifecycle is truthful | PASS |
| H | GetByID returns nil for non-existent case | PASS |
| I | GetDecision returns nil for non-existent decision | PASS |
| J | Multiple decisions on same case are canonical | PASS |
| K | Case resolved stays resolved on second Decision | PASS |
| L | Case count with status filter works correctly | PASS |

**12/12 PASS against real PostgreSQL.**

---

## 13. Regression Tests

| Test Suite | Status |
|-----------|--------|
| TestEnforcementRuntime (16/16) | PASS |
| TestCanonicalDecisionRuntime (9/9) | PASS |
| TestCanonicalCaseRuntime (8/8) | PASS |
| TestOutboxRetryLifecycle (7/7) | PASS |
| Unit tests (moderation packages) | PASS |
| go vet (moderation packages) | PASS |
| go build ./... | PASS |

**ZERO regressions.**

---

## 14. UI Contract Readiness

The API contract is designed for Slice 7 (Admin UI rebuild):

- `GET /admin/governance/cases` → Case list page with status filter
- `GET /admin/governance/cases/:id` → Case detail page with Reports, Decisions, Enforcement
- `POST /admin/governance/cases/:id/decisions` → Decision creation form
- `GET /admin/governance/decisions/:id` → Decision detail page
- `GET /admin/governance/decisions/:id/enforcement` → Enforcement status display

No UI-specific fake fields. Every response field maps to canonical data.

---

## 15. Legacy Endpoint Non-Restoration

The following legacy endpoints remain REMOVED and are NOT restored:

- `GET /admin/moderation/cases` ❌
- `GET /admin/moderation/cases/:id` ❌
- `GET /admin/moderation/cases/:id/evidence` ❌
- `POST /admin/moderation/cases/:id/action` ❌

These belong to the rejected GovernanceCase runtime. Canonical endpoints live under `/admin/governance/`.

---

## 16. Files Changed

| File | Change |
|------|--------|
| `entity/canonical_case.go` | No change (interface already clean) |
| `infrastructure/repository/case_repository.go` | Added `ListAll`, `CountAll` interface methods |
| `infrastructure/repository/case_repository_impl.go` | Added `ListAll`, `CountAll` implementations |
| `infrastructure/repository/report_repository.go` | Added `ListByCaseID` interface method |
| `infrastructure/repository/report_repository_impl.go` | Added `ListByCaseID` implementation |
| `delivery/http/governance_admin_handler.go` | **NEW** — canonical admin governance handler |
| `serverboot/dependencies.go` | Added `GovernanceAdminHandler` to struct + wiring |
| `cmd/core_server/routes_core.go` | Added `/admin/governance/*` routes |
| `tests/governance_admin_integration_test.go` | **NEW** — 12 integration tests |
| `tests/decision_runtime_integration_test.go` | Added `ListAll`, `CountAll` to fault-injected mock |
| `application/report_service_test.go` | Added `ListByCaseID` to mock |

---

## 17. Remaining Findings

| Finding | Severity | Notes |
|---------|----------|-------|
| No admin retry endpoint for failed enforcement | P3 | Correct — retry is automatic via outbox, no manual retry business rule exists |
| Appeal system still uses legacy GovernanceCase | P2 | Pre-existing, Slice 9 dependency |
| Admin UI still uses legacy vocabulary | P1 | Pre-existing, Slice 7 dependency |

---

## 18. Verdict

**PASS**

- Canonical API contract ✅
- Correct authorization (existing capabilities) ✅
- No duplicate Decision authority (pure adapter to DecisionService) ✅
- Real PostgreSQL proof (12/12 integration tests) ✅
- No false-success ✅
- No legacy endpoint restoration ✅
- All regression tests pass ✅
