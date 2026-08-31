# SLICE 4 — CANONICAL DECISION RUNTIME IMPLEMENTATION REPORT

**Tanggal:** 2026-08-31
**Scope:** Decision runtime only — entity, repository, service, dependency wiring
**Last updated:** 2026-08-31 (real PostgreSQL execution proof added)

---

## 1. Implementation Summary

### 1.1 What Was Implemented

| Component | File | Status |
|---|---|---|
| Decision entity | `entity/decision.go` | ✅ NEW |
| Decision entity tests | `entity/decision_test.go` | ✅ NEW |
| Decision repository interface | `infrastructure/repository/decision_repository.go` | ✅ NEW |
| Decision repository implementation | `infrastructure/repository/decision_repository_impl.go` | ✅ NEW |
| Decision service | `application/decision_service.go` | ✅ NEW |
| Integration tests | `tests/decision_runtime_integration_test.go` | ✅ NEW |
| Dependency wiring | `serverboot/dependencies.go` | ✅ MODIFIED |
| Legacy GovernanceCase rename | `entity/governance_case.go` | ✅ MODIFIED (type rename) |

### 1.2 What Was NOT Changed

- Decision schema (migration 000055 unchanged)
- Enforcement runtime — not in scope
- Appeal runtime — not in scope
- Warning runtime — not in scope
- Admin UI — not in scope
- Mobile — not in scope
- Report/Case runtime — not in scope
- Outbox retry — not in scope

---

## 2. Canonical Contract

```text
Decision outcome ∈ {no_violation, violation}

Decision is immutable (append-only).
Decision belongs to a Case (case_id NOT NULL, FK).
One Case may have multiple immutable Decisions.
Decision creation resolves Case (open → resolved) on first Decision.
Decision creation on resolved Case succeeds (no-op on Case).
Decision does NOT create Enforcement (Enforcement is next slice).
Decision does NOT set enforcement status.
Decision does NOT mutate target content/auction/for_sale/user.
```

---

## 3. Proof Classification

### UNIT PROOF

| Test | What It Proves | Method |
|---|---|---|
| `TestNewDecision_Success` | Entity creation with valid inputs | Pure Go, no DB |
| `TestNewDecision_NoViolation` | no_violation outcome valid | Pure Go |
| `TestNewDecision_InvalidOutcome` | Invalid outcome rejected | Pure Go |
| `TestNewDecision_EmptyOutcome` | Empty outcome rejected | Pure Go |
| `TestDecisionOutcome_IsValid` | All valid/invalid outcomes | Pure Go |
| `TestNewDecision_EachHasUniqueID` | Unique IDs | Pure Go |
| `TestErrInvalidDecisionOutcome_Message` | Error message | Pure Go |
| `TestErrDecisionCaseNotFound_Message` | Error message | Pure Go |

**8/8 unit tests PASS** (0.6s)

### INTEGRATION PROOF (Real PostgreSQL)

All integration tests execute against a real PostgreSQL 16 instance via Docker (`labuda-postgres` container, `labuda_test` database). Tests use `testdb.SetupDB` which runs full migration chain against a disposable schema.

### POSTGRESQL RUNTIME PROOF

| Test | Proof | Result |
|---|---|---|
| **A.** `first_decision_on_open_case_resolves_case` | Open Case → Decision → resolved, closed_at set, Decision row exists | **PASS** (0.22s) |
| **B.** `second_decision_on_resolved_case_succeeds` | Resolved Case → Decision #2 → success, Case remains resolved, 2 rows | **PASS** (0.18s) |
| **C.** `multiple_decisions_all_exist` | 3 Decisions → all exist, all have correct case_id, Case resolved | **PASS** (0.13s) |
| **D.** `decision_immutable_update_rejected` | UPDATE → rejected by `trg_decisions_immutable`, Decision unchanged | **PASS** (0.06s) |
| **E.** `invalid_outcome_rejected` | `DecisionOutcome("enforce")` → `ErrInvalidDecisionOutcome` | **PASS** (0.03s) |
| **F.** `missing_case_rejected` | Non-existent case_id → `ErrDecisionCaseNotFound` | **PASS** (0.00s) |
| **G.** `decision_failure_does_not_mutate_case` | **Real atomicity proof** — see §4 below | **PASS** (0.01s) |
| **H.** `case_resolution_idempotent_across_decisions` | 3 Decisions on same Case, resolved once, no error on subsequent | **PASS** (0.28s) |
| **I.** `list_decisions_newest_first` | 3 Decisions → listed in reverse chronological order | **PASS** (0.20s) |

**9/9 Decision integration tests PASS** (150.4s including migration)

### REGRESSION PROOF (Real PostgreSQL)

| Suite | Tests | Result |
|---|---|---|
| `TestCanonicalCaseRuntime` (Slice 3) | 8 tests | **PASS** (144.3s) |
| `TestCanonicalReportRuntime` (Slice 2) | 11 tests | **PASS** (134.3s) |

**28/28 total integration tests PASS** against real PostgreSQL.

---

## 4. Atomicity Proof — Real Transaction Rollback

### The Problem

Previous test used invalid outcome (`"garbage"`) which was caught by application validation **before** entering the transaction. This proved validation, not transaction atomicity.

### The Fix

Introduced `caseRepoFault` — a minimal fault-injection wrapper around the real `CaseRepository`:

```go
type caseRepoFault struct {
    real     moderationRepo.CaseRepository
    faultErr error
}

func (r *caseRepoFault) ResolveCase(ctx context.Context, tx db.Tx, caseID uuid.UUID) error {
    if r.faultErr != nil {
        return r.faultErr
    }
    return r.real.ResolveCase(ctx, tx, caseID)
}
```

This wrapper:
- Delegates `GetByID` to the real repository (Case lookup succeeds)
- Delegates `ResolveCase` to the real repository but returns an injected error

### Execution Flow (Real PostgreSQL)

```text
DecisionService.CreateDecision
    ↓
s.db.WithTx(ctx, func(tx db.Tx) error {
    ↓
    1. caseRepo.GetByID(ctx, tx, caseID)
       → REAL PostgreSQL SELECT via tx
       → Case found, status = 'open'
       → SUCCESS
    ↓
    2. decRepo.Create(ctx, tx, decision)
       → REAL PostgreSQL INSERT INTO decisions via tx
       → SUCCESS (row exists in tx, not yet committed)
    ↓
    3. caseRepo.ResolveCase(ctx, tx, caseID)
       → caseRepoFault intercepts
       → Returns injected error WITHOUT calling real ResolveCase
       → ERROR returned to WithTx
    ↓
})
    ↓
WithTx sees error → pgxTx.Rollback(ctx)
    ↓
ROLLBACK issued to PostgreSQL
```

### Proof Assertions (Real PostgreSQL)

After the fault-injected `CreateDecision` returns error:

```sql
-- 1. Decision count = 0 (INSERT was rolled back)
SELECT COUNT(*) FROM decisions WHERE case_id = $1;
-- Result: 0

-- 2. Case status = open (resolution was rolled back)
SELECT status FROM cases WHERE id = $1;
-- Result: 'open'

-- 3. closed_at = NULL (resolution timestamp was rolled back)
SELECT closed_at FROM cases WHERE id = $1;
-- Result: NULL
```

**Zero persisted mutations. Real PostgreSQL transaction rollback confirmed.**

### Why This Is Real

- `caseRepoFault.GetByID` calls the **real** `CaseRepository.GetByID` with the **real** transaction — Case lookup is against PostgreSQL
- `decRepo.Create` calls the **real** `DecisionRepository.Create` with the **real** transaction — INSERT is executed against PostgreSQL
- The fault is injected **only** on `ResolveCase` — after the INSERT has been executed
- `WithTx` calls `pgxTx.Rollback(ctx)` — real PostgreSQL ROLLBACK
- Verification queries use `pool.QueryRow` — **direct PostgreSQL connection**, reads committed state only

---

## 5. Static Implementation Verification

### Transaction Flow

```text
DecisionService.CreateDecision
    ↓
s.db.WithTx(ctx, func(tx db.Tx) error {
    s.caseRepo.GetByID(ctx, tx, ...)   // same tx
    s.decRepo.Create(ctx, tx, ...)     // same tx
    s.caseRepo.ResolveCase(ctx, tx, ...) // same tx
})
```

**All three operations receive the identical `tx` object from `WithTx`.** No operation escapes the transaction boundary. No pool/global DB is used within the transaction.

### Evidence

```go
// decision_service.go:70-100
err := s.db.WithTx(ctx, func(tx db.Tx) error {
    kase, err := s.caseRepo.GetByID(ctx, tx, input.CaseID)  // tx
    // ...
    if err := s.decRepo.Create(ctx, tx, decision); err != nil { // tx
    // ...
    if err := s.caseRepo.ResolveCase(ctx, tx, input.CaseID); err != nil { // tx
    // ...
})
```

---

## 6. Legacy Rename

**Problem:** `entity/governance_case.go` declared `type Decision string` with constants `DecisionApprove/DecisionReject/DecisionEnforce`. This collided with the canonical `entity.Decision` struct.

**Resolution:** Renamed legacy type to `GovernanceCaseDecision` and constants to `GovernanceCaseDecisionApprove/Reject/Enforce`.

**Impact:** The old constants were only used inside `governance_case.go` itself — no external references. All existing tests pass without modification.

---

## 7. Files Changed

| File | Change |
|---|---|
| `entity/decision.go` | **NEW** — Canonical Decision entity |
| `entity/decision_test.go` | **NEW** — 8 unit tests |
| `entity/governance_case.go` | **MODIFIED** — Renamed `Decision` → `GovernanceCaseDecision` |
| `infrastructure/repository/decision_repository.go` | **NEW** — Repository interface |
| `infrastructure/repository/decision_repository_impl.go` | **NEW** — Repository implementation |
| `application/decision_service.go` | **NEW** — Decision service |
| `tests/decision_runtime_integration_test.go` | **NEW** — 9 integration test cases |
| `serverboot/dependencies.go` | **MODIFIED** — Wired DecisionRepository + DecisionService |

---

## 8. Exact Commands and Results

```bash
# Build (all packages)
go build ./...
# Result: PASS (exit code 0)

# Vet (moderation module)
go vet ./internal/governance/moderation/...
# Result: PASS (exit code 0)

# Vet (integration tests)
go vet -tags=integration ./tests/...
# Result: PASS (exit code 0)

# Unit tests (entity — 31 tests including 8 new Decision tests)
go test ./internal/governance/moderation/entity/... -count=1
# Result: PASS (31/31)

# All governance/moderation tests
go test ./internal/governance/moderation/... -count=1
# Result: PASS (4 packages)

# Integration tests — Decision (real PostgreSQL)
go test -tags=integration -v -run TestCanonicalDecisionRuntime -count=1 ./tests/
# Result: PASS (9/9 tests, 150.4s)

# Integration tests — Case regression (real PostgreSQL)
go test -tags=integration -v -run TestCanonicalCaseRuntime -count=1 ./tests/
# Result: PASS (8/8 tests, 144.3s)

# Integration tests — Report regression (real PostgreSQL)
go test -tags=integration -v -run TestCanonicalReportRuntime -count=1 ./tests/
# Result: PASS (11/11 tests, 134.3s)
```

---

## 9. Final Verdict

### **PASS**

**Evidence:**
1. ✅ Static implementation verified: all operations use same `tx` from `WithTx`
2. ✅ Real PostgreSQL integration suite executed: 28/28 tests PASS
3. ✅ Atomicity partial-failure proven: real transaction rollback via fault injection, zero persisted mutations
4. ✅ All 9 Decision proof cases (A-I) PASS against real PostgreSQL
5. ✅ All 8 Case regression tests PASS
6. ✅ All 11 Report regression tests PASS
7. ✅ Decision outcome ∈ {no_violation, violation} enforced by DB enum
8. ✅ Decision immutability enforced by `trg_decisions_immutable` trigger
9. ✅ Multiple Decisions per Case supported
10. ✅ Case resolution idempotent (no reopen)
11. ✅ Full build + vet clean
12. ✅ No Enforcement, Appeal, or Outbox changes
13. ✅ Legacy paths untouched except necessary rename
