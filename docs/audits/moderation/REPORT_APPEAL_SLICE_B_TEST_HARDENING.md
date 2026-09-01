# Appeal Slice B — Test Hardening Report (F1–F3)

**Date:** 2026-09-01  
**Baseline Commit:** `e006676` (correctness recovery) / `eb864ec` (adversarial verification)  
**Scope:** F1–F3 test hardening ONLY  

---

## STATUS: **PASS**

---

## F1 — TRUE LATE-FAILURE ATOMICITY

### Root Cause (Previous)

`TestAppealSliceB_AtomicityRollback` used a non-existent `caseID` to force failure at step 1 of `CreateAppealDecision` (case validation). This failed BEFORE any writes, proving only empty-TX rollback.

### Fix

Replaced with `TestAppealSliceB_LateFailureAtomicity` using a **genuine late-failure injection**:

- Created `appealAtomicityAuditFault` — a fake `GovernanceAuditEmitter` that always returns error
- Wired into a SEPARATE `DecisionService` (`faultDS`) used only for the review path
- Decision #1 created with normal `setupDS` (nil audit = always succeeds)
- Review path uses `faultDS` → Decision #2 + Enforcement #2 + Outbox all INSERT against real PostgreSQL
- Audit emitter (step 5, LAST operation) fails → TX rolls back

### Failure injection mechanism

```go
type appealAtomicityAuditFault struct{}

func (f appealAtomicityAuditFault) GovernanceDecisionCreated(...) error {
    return fmt.Errorf("INJECTED: audit emission forced failure for atomicity proof")
}
```

Execution order inside `CreateAppealDecision`:
```
1. validate case (real PG SELECT)       → succeeds
2. INSERT Decision #2 (real PG)         → succeeds ← WRITTEN
3. INSERT Enforcement #2 (real PG)      → succeeds ← WRITTEN  
4. INSERT Outbox event (real PG)        → succeeds ← WRITTEN
5. INSERT Audit event                   → FAILS    ← INJECTED
→ TX ROLLBACK (automatic)
→ ALL of 2-4 UNDONE
```

### PostgreSQL proof

```
Decision #2    = 0  (INSERT succeeded but rolled back)
Enforcement #2 = 0  (INSERT succeeded but rolled back)
Outbox restore = 0  (INSERT succeeded but rolled back)
Appeal status  = pending (unchanged)
Decision #1    = violation (unchanged)
```

### Evidence

```
TestAppealSliceB_LateFailureAtomicity PASS (138s)
```

---

## F2 — STRONG CONCURRENCY PROOF

### Root Cause (Previous)

Used `LessOrEqual(d2Count, 1)` instead of exact assertions. Did not assert `successCount == 1`. Missing restoration event count assertion.

### Fix

Replaced all `LessOrEqual` with exact `assert.Equal(t, 1, ...)`:

```go
assert.Equal(t, 1, successCount, "Exactly one goroutine must succeed")
assert.Equal(t, 1, failureCount, "Exactly one goroutine must fail")
assert.Equal(t, 1, d2Count, "Exactly one Decision #2 must exist")
assert.Equal(t, 1, enf2Count, "Exactly one Enforcement #2 must exist")  // conditional
assert.Equal(t, 1, restoredEvtCount, "Exactly one restoration outbox")  // conditional
```

Conditional enforcement/outbox assertion based on which reviewer won:
- If approved (reversal) won → 1 enforcement + 1 restoration
- If rejected (upheld) won → 0 enforcement + 0 restoration

### PostgreSQL proof

```
err1=no rows in result set, err2=<nil>
Exactly one success, exactly one failure
Exactly one Decision #2
Appeal in final state (approved or rejected)
Enforcement #2 count matches reversal/upheld semantics
Restoration outbox count matches reversal/upheld semantics
Decision #1 unchanged
```

### Evidence

```
TestAppealSliceB_Concurrency PASS (131s)
```

---

## F3 — DECISION #2 ENFORCEMENT RETRY

### Root Cause (Previous)

No test proved worker retry lifecycle for Decision #2 enforcement. The existing E2E test proved retry for Decision #1 only.

### Fix

Added `TestAppealSliceB_EnforcementRetry` — full lifecycle proof:

```
Appeal created → Decision #2 (no_violation) via single-TX pattern
→ Enforcement #2 pending → restoration outbox exists
→ MarkProcessing → processing
→ MarkFailed → failed (attempt_count = 1)
→ MarkProcessing (retry) → processing
→ MarkSucceeded → succeeded (attempt_count = 2)
→ target restored
```

### Assertions

| Assertion | Result |
|-----------|--------|
| Enforcement #2 references Decision #2 | ✓ |
| attempt_count = 1 after first failure | ✓ |
| attempt_count = 2 after retry | ✓ |
| status = succeeded after retry | ✓ |
| No duplicate Enforcement for Decision #2 | ✓ |
| No duplicate restoration outbox | ✓ |
| Decision #1 unchanged (violation) | ✓ |
| Decision #2 immutable (UPDATE blocked) | ✓ |
| Appeal remains approved | ✓ |
| No Decision #3 or extra governance state | ✓ |

### Evidence

```
TestAppealSliceB_EnforcementRetry PASS (126s)
```

---

## REGRESSION RESULTS

| Command | Result |
|---------|--------|
| `go build ./...` | ✅ PASS |
| `go vet -tags integration ./internal/governance/moderation/...` | ✅ PASS |
| `go test -tags integration ./internal/governance/moderation/application/` | ✅ PASS (33/33) |
| `go test -tags integration ./internal/governance/moderation/delivery/http/` | ✅ PASS |
| `TestAppealSliceB_Reversal` | ✅ PASS |
| `TestAppealSliceB_Upheld` | ✅ PASS |
| `TestAppealSliceB_LateFailureAtomicity` | ✅ PASS (NEW) |
| `TestAppealSliceB_Concurrency` | ✅ PASS (HARDENED) |
| `TestAppealSliceB_StateMachine` | ✅ PASS |
| `TestAppealSliceB_EnforcementRetry` | ✅ PASS (NEW) |
| `TestGovernanceE2E` | ✅ PASS (4/4) |

---

## FILES CHANGED

| File | Change |
|------|--------|
| `tests/appeal_slice_b_integration_test.go` | Replace weak atomicity test with F1 (genuine late-failure injection); harden F2 concurrency assertions; add F3 enforcement retry test |

---

## REMAINING FINDINGS

None. All F1–F3 acceptance criteria met with real PostgreSQL evidence.
