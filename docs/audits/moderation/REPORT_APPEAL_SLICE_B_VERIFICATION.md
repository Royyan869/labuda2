# APPEAL SLICE B — ADVERSARIAL VERIFICATION REPORT

**Date:** 2026-09-01
**Verifier:** Buffy (adversarial pass)
**Status:** BLOCKED

---

## BASELINE

Slice A verification: PASS WITH FINDINGS
Slice B implementation report claimed: PASS

**This verification does NOT accept that verdict.**

---

## 1. CALL GRAPH

```
AdminReviewAppeal (HTTP Handler)
  ↓ h.db.WithTx(ctx, func(tx db.Tx) error {    ← TX1 BEGINS
  ↓   AppealService.GetAppeal(ctx, tx, appealID)
  ↓   AppealService.ReviewAppeal(ctx, tx, appealID, adminID, approved, adminResponse)
  ↓     ├─ AppealRepo.GetForUpdate(ctx, tx, appealID)     ← within TX1
  ↓     ├─ resolveAppealContext(ctx, tx, decisionID)       ← within TX1
  ↓     │    ├─ DecisionRepo.GetByID(ctx, tx, decisionID)
  ↓     │    └─ CaseRepo.GetByID(ctx, tx, caseID)
  ↓     ├─ DecisionService.CreateAppealDecision(ctx, input) ← ❌ DOES NOT RECEIVE TX1
  ↓     │    └─ s.db.WithTx(ctx, func(tx db.Tx) error {    ← TX2 BEGINS (SEPARATE!)
  ↓     │         ├─ CaseRepo.GetByID
  ↓     │         ├─ DecisionRepo.Create (Decision #2)
  ↓     │         ├─ EnforcementRepo.Create (Enforcement #2, if reversal)
  ↓     │         ├─ OutboxRepo.InsertEvent (if reversal)
  ↓     │         └─ AuditService.GovernanceDecisionCreated
  ↓     │    })                                              ← TX2 COMMITS
  ↓     ├─ appeal.Approve() or appeal.Reject()               ← in-memory
  ↓     └─ AppealRepo.Update(ctx, tx, appeal)                ← within TX1
  ↓ })                                                        ← TX1 COMMITS
```

**CRITICAL: `CreateAppealDecision` is called WITHOUT the transaction `tx`. It opens its own independent TX2 via `s.db.WithTx()`.**

---

## 2. TRANSACTION BOUNDARY — P1 ATOMICITY DEFECT

### The Problem

`ReviewAppeal` runs within handler TX1. But `CreateAppealDecision` opens its own TX2 because it receives `ctx` (not `tx`) and calls `s.db.WithTx(ctx, ...)`.

### Consequences

| Scenario | TX2 | TX1 | Result |
|----------|-----|-----|--------|
| Normal success | COMMIT | COMMIT | ✅ OK |
| TX2 fails | ROLLBACK | Error propagated → ROLLBACK | ✅ OK (appeal unchanged) |
| TX2 succeeds, appeal update fails | COMMIT | ROLLBACK | ❌ **ORPHANED Decision #2 + Enforcement #2** |
| Appeal already reviewed (state check) | N/A | Error → ROLLBACK | ✅ OK (decisionService not called) |

**Failure scenario is real:** If `appealRepo.Update()` fails after `CreateAppealDecision` succeeds, PostgreSQL will have:
- Decision #2 ✅
- Enforcement #2 ✅ 
- Outbox event ✅
- Audit event ✅
- Appeal status = still "pending" ❌

This creates a governance inconsistency where a Decision #2 and its Enforcement exist but the Appeal is not finalized.

### Severity: P1

The practical risk is low (Update after successful Decision #2 is unlikely to fail), but the architectural invariant is broken. The report's claim of "atomic" is false.

### Required Fix (Slice C)

`ReviewAppeal` must either:
- Option A: Receive `db.Tx` and pass it to `CreateAppealDecision` (making `CreateAppealDecision` accept a pre-existing tx)
- Option B: Move `CreateAppealDecision` into the same `WithTx` block as the appeal update, with all operations sharing one transaction

---

## 3. DECISION #2 SEMANTICS

### Reversal (approved)

```go
decisionOutcome = entity.DecisionOutcomeNoViolation
```

**Correct:** Reversal creates Decision #2 with `outcome = no_violation`, which triggers Enforcement #2 creation + outbox event for restoration.

### Upheld (rejected)

```go
decisionOutcome = entity.DecisionOutcomeViolation
```

**Correct:** Upheld creates Decision #2 with `outcome = violation`, no Enforcement, no outbox.

### Decision #2 fields

- `CaseID` = same Case as Decision #1 ✅
- `DecidedBy` = reviewing admin ✅
- `Outcome` = correct per review result ✅
- `DecisionNote` = admin response ✅
- `TargetType/TargetID` = set for reversal, zero for upheld ✅

### Verdict: DECISION #2 SEMANTICS — CORRECT (but transactional safety is compromised)

---

## 4. REAL POSTGRESQL REVERSAL PROOF

### Result: ❌ NOT PROVEN

**Zero integration tests exist that:**
1. Create a real Case row
2. Create a real Decision #1 row 
3. Create a real Appeal row
4. Call ReviewAppeal against real PostgreSQL
5. Verify Decision #2 exists
6. Verify Enforcement #2 exists
7. Verify outbox event exists

### Evidence

All existing integration tests use **mock repositories** despite the `//go:build integration` tag. The test file imports pgx/pgconn but uses `mockAppealRepository`, `mockDecisionRepository`, `mockCaseRepository` — none of which touch PostgreSQL.

The 5 tests that PASS (`TestCreateAppeal_DecisionNotFound_ReturnsError`, etc.) are **mock-based tests** that happen to have correct mock setup. They are NOT real PostgreSQL proof.

### What Real PostgreSQL Would Prove

Without real PostgreSQL, we cannot verify:
- FK constraint: `appeals.decision_id → decisions.id`
- Unique constraint: one pending appeal per Decision
- CHECK constraint: `status IN ('pending', 'approved', 'rejected')`
- `trg_decisions_immutable` trigger on Decision #2
- Outbox event persistence
- Enforcement lifecycle persistence
- Audit event persistence

---

## 5. REAL POSTGRESQL UPHELD PROOF

### Result: ❌ NOT PROVEN

No integration test creates a real upheld review scenario.

---

## 6. ATOMICITY FAULT INJECTION

### Result: ❌ NOT PROVEN

The report claimed atomicity. The verification proves it false (Section 2).

No fault injection test exists to prove rollback behavior.

---

## 7. ORIGINAL DECISION IMMUTABILITY

### Code-Level Verification: ✅ CORRECT

The `trg_decisions_immutable` trigger (created in migration 000055) prevents UPDATE on decisions table. Decision #2 is created via `entity.NewDecision()` which generates a new UUID. Decision #1 is never referenced for mutation in `ReviewAppeal`.

### DB-Level Verification: ❌ NOT PROVEN (no real PostgreSQL test)

---

## 8. CONCURRENT REVIEW

### Code-Level: ✅ APPEARS SAFE

- `GetForUpdate` acquires `SELECT ... FOR UPDATE` lock on the appeal row
- `Appeal.Approve/Reject` checks `CanAppealTransition(from, to)` which requires `from == pending`
- Two concurrent reviews: first commits appeal update, second gets `ErrAppealAlreadyReviewed`

### BUT: The `CreateAppealDecision` opens a separate TX2, so two concurrent reviews could:
1. Both acquire FOR UPDATE lock (within TX1, which blocks)
2. TX1 serialization means only one proceeds

Actually — since both operations share TX1 via `h.db.WithTx()`, and PGX uses `pool.Begin()` which gets a dedicated connection, the FOR UPDATE lock DOES serialize concurrent reviews at the database level. ✅

### DB-Level Concurrency Proof: ❌ NOT PROVEN (no real PostgreSQL concurrency test)

---

## 9. WORKER RETRY

### Result: ❌ NOT PROVEN

No integration test proves:
- pending → processing → failed → processing → succeeded lifecycle
- attempt_count increment
- No duplicate target mutation

The worker code itself (`ModerationEventHandler.handleRestoration`) appears correct for retry, but it was NOT exercised by Slice B tests.

---

## 10. LEGACY RESTORATION SEARCH

### Direct InsertEvent in AppealService

```
grep for "s.outboxRepo" in appeal_service.go → ZERO hits (only struct field declaration)
```

**Result:** ✅ AppealService no longer directly emits restoration events.

### Canonical Enforcement Path

The new path goes through `DecisionService.CreateAppealDecision` → `OutboxRepo.InsertEvent` within the same TX2. The `moderation.<type>.restored` event naming is preserved, matching the existing worker dispatcher.

### Worker Consumer

`SetupModerationHandlers` registers `moderation.content.restored`, `moderation.comment.restored`, etc. These are active and will handle events from the canonical Enforcement path.

### Verdict: ✅ No parallel authority — old direct-emission path is removed

---

## 11. API CONTRACT

### Backend Response

Handler `appealToResponse` returns:
```json
{
  "id": "...",
  "decision_id": "...",    // ← canonical field name
  "status": "pending",
  "message": "...",
  "created_at": "..."
}
```

### Admin UI Frontend

`apps/admin/src/types/moderation.ts:22` defines `Appeal` with `report_id: string`.
`AppealsPage.tsx:161` renders `{appeal.report_id.slice(0, 8)}`.
`AppealDetailModal.tsx:201` renders `{appeal.report_id}`.

**⚠️ P1 CONTRACT MISMATCH:** Admin UI reads `report_id`, backend returns `decision_id`. The field is undefined in the TypeScript type, so `appeal.report_id` will be `undefined`.

### Mobile Consumer

`appeal_dto.dart:37` reads `json['case_id']`.
`appeal_dto.dart:70` sends `{'case_id': caseId, 'message': message}`.

**⚠️ P1 CONTRACT MISMATCH:** Mobile sends `case_id`, backend expects `decision_id` (`CreateAppealRequest.DecisionID` is bound from JSON field `decision_id`).

---

## 12. HANDLER TESTS — DON'T COMPILE

```
go test -tags integration ./internal/governance/moderation/delivery/http/ → BUILD FAILED
```

Errors:
1. `undefined: fakeAdminAuditLogger` — test helper type not found
2. `unknown field CaseID in struct literal of type CreateAppealRequest` — Slice A renamed the field but tests weren't updated

**The entire handler test file doesn't compile with `integration` tag.** This means:
- No handler-level integration tests run
- `TestAppealToResponse` asserts `resp["case_id"]` but handler returns `decision_id`
- `TestAdminReviewAppeal_*` tests (capability tests) don't compile

### Verdict: ❌ BLOCKED — Handler test suite is broken

---

## 13. APPLICATION TESTS — 12 FAIL

```
go test -tags integration ./internal/governance/moderation/application/
```

| Test | Result | Reason |
|------|--------|--------|
| TestCreateAppeal_DecisionNotFound | ✅ PASS | Correct mock setup |
| TestCreateAppeal_NoViolationNotAppealable | ✅ PASS | Correct mock setup |
| TestCreateAppeal_NotResourceOwner | ✅ PASS | Correct mock setup |
| TestCreateAppeal_DuplicatePendingAppeal | ✅ PASS | Correct mock setup |
| TestCreateAppeal_ValidOwner_RemovedCase | ✅ PASS | Correct mock setup |
| TestCreateAppeal_ValidOwner_RejectedCase | ❌ FAIL | `decision not found` — mock not returning decision |
| TestCreateAppeal_CommentResource | ❌ FAIL | Same |
| TestReviewAppeal_ApproveNonRemovedCase | ❌ FAIL | Same — old GovernanceCase-based setup, decisionRepo empty |
| TestReviewAppeal_RejectAppeal | ❌ FAIL | Same |
| TestReviewAppeal_ApproveRemovedCase | ❌ FAIL | Same |
| TestReviewAppeal_RestorationEventEmittedBeforeStateChange | ❌ FAIL | Expects ErrRestorationEventFailed (legacy), gets ErrDecisionNotFound |
| TestReviewAppeal_RestorationFailureLeavesAppealPending | ❌ FAIL | Same |
| TestCreateAppeal_AllowsNewAppealAfterResolved | ❌ FAIL | Same |
| TestCreateAppeal_ForSaleResource_ValidSeller | ❌ FAIL | Same |
| TestCreateAppeal_ForSaleResource_NonOwner | ❌ FAIL | Same |
| TestCreateAppeal_AuctionResource_ValidSeller | ❌ FAIL | Same |
| TestCreateAppeal_UserSuspension_ValidUser | ❌ FAIL | Same |
| TestCreateAppeal_UserSuspension_OtherUser | ❌ FAIL | Same |

### Root Cause

The failing tests create `mockDecisionRepository{}` with NO `getByIDFunc` set. The default returns `nil, nil` (decision not found). The tests need to configure `getByIDFunc` to return a real Decision, but they weren't updated for Slice A's Decision→Case resolution path.

Additionally, `TestReviewAppeal_*` tests reference `entity.NewGovernanceCase()` and `kase.Enforce()` which are legacy patterns. These tests should now use canonical Decision + Case setup.

**4 review-specific tests still reference `ErrRestorationEventFailed`** — a legacy error type that no longer applies after Slice B.

### Classification: PRE-EXISTING + SLICE B REGRESSION

The review tests are **Slice B regressions** because they test behavior that changed in Slice B (review now creates Decision #2 instead of emitting direct restoration events).

---

## 14. NON-EXISTING RECEIVERS

The `ReviewAppeal` method handles `resolveAppealContext` returning errors, but if the decision does not exist, the error message will be "decision not found: <uuid>". This is correct behavior — you can't review an appeal for a non-existent Decision.

---

## 15. REMAINING LEGACY RESIDUE

| Artifact | Status | Action |
|----------|--------|--------|
| `entity/governance_case.go` | Still exists | Defer to cleanup slice |
| `ModerationRepository` interface | Still exists | Defer to cleanup slice |
| `ModerationRepositoryImpl` | Still exists | Defer to cleanup slice |
| `GetAppealWithCase` | Dead stub | Defer to cleanup slice |
| `outboxRepo` field in AppealService | Unused (no direct calls) | Should be removed |
| `ErrRestorationEventFailed` | Still in entity package | Can be removed (dead after Slice B) |
| Handler tests (`fakeAdminAuditLogger`, `CaseID`) | Broken | Must be fixed |
| Review tests (GovernanceCase, ErrRestorationEventFailed) | Broken | Must be fixed |

---

## 16. SLICE B PRECONDITIONS

For Slice B to be considered complete, the following MUST be true:

| Criterion | Status |
|-----------|--------|
| Appeal review creates Decision #2 | ✅ Correct code path |
| Decision #1 remains immutable | ✅ Code correct, DB trigger exists |
| Decision #2 belongs to same Case | ✅ Correct |
| Decision #2 has correct reviewer | ✅ Correct |
| Reversal creates Enforcement #2 | ✅ Correct |
| Enforcement #2 uses canonical lifecycle | ✅ Correct |
| Restoration goes through Outbox + Worker | ✅ Correct |
| AppealService no longer directly restores | ✅ Verified |
| No parallel restoration authority | ✅ Verified |
| Upheld review does not restore target | ✅ Correct |
| Review is atomic | ❌ **FALSE — two separate transactions** |
| Audit is canonical and transactional | ⚠️ Within TX2 only (not TX1) |
| Duplicate/concurrent review is safe | ✅ Code correct (unproven against real DB) |
| Worker retry is safe | ⚠️ Code appears correct (unproven) |
| Real PostgreSQL proof exists | ❌ **NO** |
| Existing tests remain green | ❌ **12 tests fail** |
| No commerce/payment mutation | ✅ Correct |
| No legacy authority reintroduced | ✅ Verified |

---

## FINAL VERDICT: BLOCKED

### Blockers

1. **P1 — Transaction Boundary Deception**: `CreateAppealDecision` opens separate TX2, breaking claimed atomicity. Appeal status update and Decision #2 creation are NOT in the same transaction.

2. **P1 — Zero Real PostgreSQL Proof**: No integration test creates Decision → Appeal → Review → Decision #2 against a real database. The "5/5 integration tests PASS" claim was incorrect — those were mock-based tests with correct mock setup.

3. **P1 — 12 Application Integration Tests Fail**: All `TestReviewAppeal_*` tests and several `TestCreateAppeal_*` tests fail because they use legacy mock setup (empty `mockDecisionRepository`) instead of the new Decision→Case resolution path.

4. **P1 — Handler Tests Don't Compile**: `appeal_handler_test.go` has compilation errors: `undefined: fakeAdminAuditLogger`, `unknown field CaseID`. Zero handler-level tests run.

5. **P2 — Admin/Mobile Contract Mismatch**: Admin UI reads `report_id` (undefined), Mobile sends `case_id` (ignored by backend). User-facing Appeal flows are non-functional.

### Required Before Slice B Can Be Called PASS

1. Fix the transaction boundary: `ReviewAppeal` must share a single transaction with `CreateAppealDecision`
2. Fix all 12 failing application tests with proper Decision→Case mock setup
3. Fix handler test compilation errors (`fakeAdminAuditLogger`, `CaseID` → `DecisionID`)
4. Update review tests to test Decision #2 creation instead of ErrRestorationEventFailed
5. Write at least one real PostgreSQL integration test proving the full review flow

### Classification of Previous Report

The Slice B report (`REPORT_APPEAL_SLICE_B_REVIEW_RUNTIME.md`) is **partially correct** on code structure but **incorrect** on:
- Atomicity claim (two transactions, not one)
- "5/5 integration tests PASS" (5 error-path tests pass; 12 tests fail)
- "No Slice B regressions" (12 review tests broken)
- "Real PostgreSQL proof: deferred" (still deferred, but listed as non-blocking)

### Recommended Next Steps

1. **Fix transaction boundary** — pass `db.Tx` into `CreateAppealDecision` so all operations share one transaction
2. **Fix tests** — update mock setup, remove legacy GovernanceCase references from review tests, fix handler test compilation
3. **Write real PostgreSQL integration test** — create Case + Decision + Appeal + Review against real DB
4. **Document consumer mismatches** for the consumer slice

---

*Report generated by adversarial verification pass. No code was modified.*
