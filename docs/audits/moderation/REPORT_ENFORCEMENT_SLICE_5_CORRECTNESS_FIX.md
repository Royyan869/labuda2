# Enforcement Lifecycle Correctness Fix — F1 + F2 + Proof Hardening

**Date:** 2026-08-31  
**Baseline:** `d632cdd` (feat(moderation): implement canonical Enforcement runtime (Slice 5))  
**Scope:** F1 (MarkSucceeded guard), F2 (idempotent lifecycle), integration proof

---

## 1. F1 — MarkSucceeded / MarkFailed Status Guard

### Root Cause

The SQL in `MarkSucceeded` and `MarkFailed` had no `WHERE status = ...` guard:

```sql
-- BEFORE (vulnerable)
UPDATE enforcements
SET status = 'succeeded', finished_at = $2, updated_at = $3
WHERE id = $4
-- ^^^ No status guard — can transition from ANY state

UPDATE enforcements
SET status = 'failed', finished_at = $2, ...
WHERE id = $6
-- ^^^ No status guard — can transition from ANY state
```

This allowed:
- `pending → succeeded` (skipping processing entirely)
- `failed → succeeded` (skipping retry)
- `succeeded → failed` (reversing terminal state)
- `pending → failed` (skipping processing entirely)

### Fix

Added `AND status = 'processing'` to both SQL queries:

```sql
-- AFTER (guarded)
UPDATE enforcements
SET status = 'succeeded', finished_at = $2, updated_at = $3
WHERE id = $4 AND status = 'processing'

UPDATE enforcements
SET status = 'failed', finished_at = $2, ...
WHERE id = $6 AND status = 'processing'
```

Both methods now return nil for 0 rows affected (idempotent no-op for already-terminal states).

**File:** `backend/internal/governance/moderation/infrastructure/repository/enforcement_repository_impl.go`

### Lifecycle Invariant (After Fix)

```
pending  ──MarkProcessing──→  processing
processing  ──MarkSucceeded──→  succeeded  (terminal)
processing  ──MarkFailed──→  failed
failed  ──MarkProcessing──→  processing  (retry)
```

**Impossible transitions (enforced by SQL guards):**
- `pending → succeeded` ❌
- `pending → failed` ❌
- `failed → succeeded` ❌
- `succeeded → anything` ❌

### Proof

**Test K — MarkSucceeded from pending is REJECTED:**
1. Create violation Decision → Enforcement(pending)
2. Call `MarkSucceeded` directly (skipping MarkProcessing)
3. **ASSERT:** status remains `pending`

**Test L — MarkFailed from pending is REJECTED:**
1. Create violation Decision → Enforcement(pending)
2. Call `MarkFailed` directly
3. **ASSERT:** status remains `pending`

**Result: K ✅ L ✅** (real PostgreSQL)

---

## 2. F2 — Idempotent Target Execution

### Root Cause

In `handleForSaleRemoved` and `handleAuctionRemoved`, when the target domain returned `InvalidTransitionError` (target already in terminal state), the error was caught AFTER `enforceLifecycle` had already rolled back its transaction.

The flow was:
```
1. enforceLifecycle tx begins
2. MarkProcessing succeeds (pending → processing)
3. Withdraw/CancelForModeration returns InvalidTransitionError
4. enforceLifecycle tx ROLLS BACK (enforcement back to pending)
5. Error handler: new tx → MarkSucceeded directly
6. pending → succeeded (skipping processing!)
```

Step 5-6 created `pending → succeeded` without going through `processing`.

### Fix

Moved `InvalidTransitionError` handling INSIDE the target function, so the lifecycle always completes atomically:

```go
// BEFORE (broken — separate tx for idempotent path)
err := h.db.WithTx(ctx, func(tx db.Tx) error {
    return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
        return h.forSaleService.Withdraw(ctx, tx, forSaleID)
    })
})
if err != nil {
    var ite *forSaleEntity.InvalidTransitionError
    if errors.As(err, &ite) {
        // Separate tx → MarkSucceeded directly (BROKEN)
        _ = h.db.WithTx(ctx, func(tx db.Tx) error {
            _ = h.enfRepo.MarkSucceeded(ctx, tx, *enforcementID)
            return nil
        })
    }
}

// AFTER (correct — same tx)
err := h.db.WithTx(ctx, func(tx db.Tx) error {
    return h.enforceLifecycle(ctx, tx, enforcementID, func() error {
        withdrawErr := h.forSaleService.Withdraw(ctx, tx, forSaleID)
        if withdrawErr != nil {
            var ite *forSaleEntity.InvalidTransitionError
            if errors.As(withdrawErr, &ite) {
                return nil // Skip mutation, lifecycle proceeds to MarkSucceeded
            }
            return withdrawErr // Real failure → TX rolls back
        }
        return nil
    })
})
```

Same pattern applied to `handleAuctionRemoved`.

**Files:**
- `backend/internal/worker/moderation_event_handler.go`

### Idempotency Behavior (After Fix)

| Scenario | Flow | Result |
|----------|------|--------|
| First execution | MarkProcessing → target mutation → MarkSucceeded | ✅ One mutation |
| Duplicate delivery | MarkProcessing (0 rows, idempotent) → target mutation (idempotent) → MarkSucceeded (0 rows) | ✅ One mutation |
| Target already terminal | MarkProcessing → InvalidTransitionError → nil → MarkSucceeded | ✅ Processing→succeeded |
| Real failure | MarkProcessing → target error → TX rollback | ✅ Outbox retries |

---

## 3. Test Evidence (Real PostgreSQL)

### New Tests Added (K-P)

| Test | What it proves | Result |
|------|---------------|--------|
| K | `pending → succeeded` rejected by SQL guard | ✅ PASS |
| L | `pending → failed` rejected by SQL guard | ✅ PASS |
| M | `MarkSucceeded` on already-succeeded is idempotent | ✅ PASS |
| N | `MarkFailed` on already-failed is idempotent | ✅ PASS |
| O | `attempt_count` incremented correctly across retries | ✅ PASS |
| P | Decision + Enforcement + Outbox + Case atomicity | ✅ PASS |

### Regression Evidence

| Test Suite | Result |
|-----------|--------|
| TestCanonicalReportRuntime | 11/11 PASS |
| TestCanonicalCaseRuntime | 8/8 PASS |
| TestCanonicalDecisionRuntime | 9/9 PASS |
| TestEnforcementRuntime | 16/16 PASS (10 original + 6 new) |
| TestOutboxRetryLifecycle | 7/7 PASS |
| TestOutboxConcurrentClaimRaceSafety | 2/2 PASS |
| go vet (moderation) | CLEAN |
| go vet (worker) | CLEAN |

---

## 4. Files Changed

| File | Change |
|------|--------|
| `backend/internal/governance/moderation/infrastructure/repository/enforcement_repository_impl.go` | MarkSucceeded + MarkFailed SQL guard |
| `backend/internal/worker/moderation_event_handler.go` | Idempotent handling moved inside enforceLifecycle |
| `backend/tests/enforcement_runtime_integration_test.go` | 6 new lifecycle guard tests + outbox repo wiring |

---

## 5. Remaining Findings

**None from F1/F2 scope.**

Pre-existing findings from Audit 3 remain:
- F5: `chat_message` handler is dead code (LEGACY RESIDUE)
- F6: Admin UI uses legacy vocabulary (PRE-EXISTING, outside scope)
- F7: No `audit_events` integration for enforcement lifecycle (P2)

---

## 6. Verdict

**PASS** — Both F1 and F2 are fixed and proven with real PostgreSQL integration tests. The enforcement lifecycle is now canonical: no transition can skip `processing`, and idempotent execution always goes through the proper lifecycle path.
