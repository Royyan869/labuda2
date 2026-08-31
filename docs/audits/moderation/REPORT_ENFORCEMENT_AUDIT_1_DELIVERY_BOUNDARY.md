# AUDIT — ENFORCEMENT + OUTBOX DELIVERY BOUNDARY

- **Tanggal audit:** 2026-08-31
- **Mode:** READ-ONLY — tidak ada implementasi
- **Satu-satunya artefak baru:** laporan ini
- **Baseline:** current filesystem (bukan git history)

---

## 1. Executive Verdict

**BLOCKED (3 P1 blockers remain)**

The Enforcement + Outbox delivery foundation had **one critical P1 blocker** (outbox retry broken) which has been **FIXED**. Three P1 blockers remain before canonical Enforcement runtime can be implemented. Additionally, the `enforcements` table exists in DB but has **zero application code** creating or managing enforcement records.

**Blockers:**
1. ~~**P1: Outbox retry broken**~~ ✅ **FIXED** — `MarkProcessing` now accepts both `pending` and `failed`.
2. **P1: No enforcement write-back** — `ModerationEventHandler` does not write enforcement results to the `enforcements` table.
3. **P1: Auction boundary unsafe** — `CancelForModeration` bypasses bid-state guards.

**Findings (non-blocking):**
- Admin UI uses legacy `enforced` vocabulary
- No audit trail for enforcement mutations
- Legacy residue (DomainAction, GovernanceCase, etc.)

---

## 2. Enforcement Schema

### 2.1 Table Definition (Migration 000055)

```sql
CREATE TABLE enforcements (
    id              uuid DEFAULT gen_random_uuid() NOT NULL,
    decision_id     uuid NOT NULL,
    target_type     moderation_target_type_enum NOT NULL,
    target_id       uuid NOT NULL,
    status          enforcement_status_enum DEFAULT 'pending' NOT NULL,
    attempt_count   integer DEFAULT 0 NOT NULL,
    requested_at    timestamptz NOT NULL DEFAULT now(),
    started_at      timestamptz,
    finished_at     timestamptz,
    last_error      text,
    next_attempt_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
```

### 2.2 Constraints

| Constraint | Type | Evidence |
|---|---|---|
| `enforcements_pkey` | PK on `id` | migration 000055 |
| `enforcements_decision_id_fkey` | FK → `decisions(id)` ON DELETE CASCADE | migration 000055 |
| `enforcements_attempt_count_nonneg` | CHECK `attempt_count >= 0` | migration 000055 |
| `enforcements_decision_target_unique` | UNIQUE `(decision_id, target_type, target_id)` | migration 000055 |

### 2.3 Status Enum

```sql
CREATE TYPE enforcement_status_enum AS ENUM ('pending', 'processing', 'succeeded', 'failed');
```

### 2.4 Schema Verdict

The schema is **correct and sufficient** for canonical Enforcement:
- ✅ FK to Decision (required, CASCADE delete)
- ✅ Lifecycle enum (pending → processing → succeeded/failed)
- ✅ Idempotency constraint (unique per decision_id + target)
- ✅ Execution tracking (attempt_count, started_at, finished_at, last_error)
- ✅ Retry support (next_attempt_at)
- ✅ Multiple enforcements per Decision (different targets)

**FINDING:** No DELETE trigger on enforcements. Non-blocking — defense-in-depth.

---

## 3. Enforcement Runtime Status

### 3.1 Application Code Search

| Component | File | Status |
|---|---|---|
| Entity | `entity/enforcement.go` | **ABSENT** |
| Repository interface | `infrastructure/repository/enforcement_repository.go` | **ABSENT** |
| Repository implementation | `infrastructure/repository/enforcement_repository_impl.go` | **ABSENT** |
| Service | `application/enforcement_service.go` | **ABSENT** |
| Worker write-back | `worker/moderation_event_handler.go` | **ABSENT** — no enforcement status update |
| Handler | `delivery/http/enforcement_handler.go` | **ABSENT** |

### 3.2 Insert Evidence

```
INSERT INTO enforcements
```

Found only in: `tests/migration_000055_canonical_moderation_foundation_test.go:199-219`
— Schema validation test only. No application code creates enforcement records.

### 3.3 Runtime Status

```
ENFORCEMENT RUNTIME: ABSENT
```

The `enforcements` table exists with correct schema, but no Go code reads from or writes to it.

---

## 4. Outbox Architecture Map

### 4.1 Current Flow (Legacy)

```text
GovernanceCase.Enforce() (legacy)
    ↓
OutboxRepository.InsertEvent()
    ↓
outbox table (status=pending)
    ↓
OutboxWorker.processOutboxBatch()
    ↓
OutboxWorker.processSingleEvent()
    ↓
OutboxDispatcher.DispatchWithResult()
    ↓
ModerationEventHandler.Handle()
    ↓
target domain mutation (content/comment/for_sale/auction/user)
    ↓
MarkSucceeded() OR MarkFailedWithRetry() OR MoveToDeadLetter()
    ↓
(no enforcement write-back)
```

### 4.2 Authority at Each Stage

| Stage | Authority | Evidence |
|---|---|---|
| Event creation | GovernanceCase (legacy) | `moderation_service.go` (removed in Slice 2) |
| Outbox storage | OutboxRepository.InsertEvent | `outbox_repository.go:85-120` |
| Worker fetch | OutboxWorker.processOutboxBatch | `outbox_worker.go:245-270` |
| Handler dispatch | OutboxDispatcher | `outbox_worker.go:460-510` |
| Target mutation | ModerationEventHandler + target domain services | `moderation_event_handler.go` |
| Status update | OutboxWorker (MarkSucceeded/MarkFailed) | `outbox_worker.go:290-330` |
| **Enforcement record** | **NONE** | **No code writes to enforcements table** |

### 4.3 Critical Invariant

```
Outbox event emission ≠ Enforcement success
```

**CURRENT STATE:** The system has no enforcement records. The outbox status (`succeeded`) is the only execution state, but it is NOT the same as canonical Enforcement.

---

## 5. Outbox Retry — ~~P1 BLOCKER~~ ✅ FIXED

### 5.1 The Bug (RESOLVED)

**FILE:** `backend/internal/platform/outbox/infrastructure/repository/outbox_repository.go`

**Root Cause:** `FetchPendingBatch` fetched events with `status IN ('pending', 'failed')`, but `MarkProcessing` only accepted `WHERE status = 'pending'`. Failed events were fetched but could never be claimed.

### 5.2 Fix Applied

**Commit:** `fix(moderation): fix outbox retry — MarkProcessing accepts failed status`

**Change:** `MarkProcessing` now accepts both `pending` and `failed`:
```sql
UPDATE outbox
SET status = 'processing', updated_at = $2
WHERE id = $3 AND status IN ('pending', 'failed')
```

**Interface comment updated** in `platform/outbox/repository/outbox_repository.go`.

### 5.3 Proof (Real PostgreSQL)

```text
TestOutboxRetryLifecycle (7 subtests) — ALL PASS
├── A: pending → processing              PROVEN
├── B: failed → processing (retry)       PROVEN  ← THE CRITICAL FIX
├── C: dead_letter → rejected            PROVEN
├── D: processing → double-claim rejected PROVEN
├── E: succeeded → rejected              PROVEN
├── F: FetchPendingBatch returns pending+failed PROVEN
└── G: full lifecycle (pending→processing→failed→processing→succeeded) PROVEN

TestOutboxConcurrentClaimRaceSafety (2 subtests) — ALL PASS
├── concurrent claim on same event → exactly 1 succeeds  PROVEN
└── concurrent claims on different events → both succeed  PROVEN
```

### 5.4 Static Audit After Fix

| Caller | Accepts new behavior? | Impact |
|---|---|---|
| `outbox_worker.go:processEventInTx` | ✅ Handles ErrInvalidStatusTransition by skipping | No change |
| `realtime_worker.go:processEventInTx` | ✅ Returns nil on any MarkProcessing error | No change |
| `dependencies.go` adapter | ✅ Delegates to repo | No change |

### 5.5 Remaining Lifecycle After Fix

```text
pending
   ↓
processing
   ├── succeeded
   └── failed (retryable)
          ↓
       processing
          ↓
       succeeded / failed (retryable, up to MaxOutboxAttempts=20)
          ↓
       dead_letter (exhausted)
```

**STATUS: FIXED — P1 RESOLVED**

---

## 6. False Success

### 6.1 Current False-Success Path

**File:** `worker/moderation_event_handler.go`

After `ModerationEventHandler.Handle()` returns nil:
```go
// outbox_worker.go:290-310
if err := w.outboxRepo.MarkSucceeded(ctx, tx, event.ID); err != nil {
    return "", 0, fmt.Errorf("mark succeeded failed: %w", err)
}
```

**Outbox status becomes `succeeded`.**

But this means:
- Content was soft-deleted ✓
- Comment was soft-deleted ✓
- ForSale was withdrawn ✓
- Auction was cancelled (with unsafe bid state — P1) ⚠️
- User was suspended ✓

The outbox `succeeded` status **does** correspond to actual target mutation in most cases. But:

1. **No enforcement record is created** — the `enforcements` table is never written to
2. **No enforcement result is persisted** — the outbox status is the only execution state
3. **Admin UI shows `enforced` badge** — legacy vocabulary that conflates outbox success with enforcement authority

### 6.2 Classification

```
CURRENT STATE: PARTIAL FALSE SUCCESS

Content/Comment/ForSale/User: Outbox succeeded = target mutated (CORRECT)
Auction: Outbox succeeded ≠ safe mutation (UNSAFE — P1)
Enforcement record: Never created (MISSING)
```

---

## 7. Transaction Boundaries

### 7.1 Current (Legacy)

```
GovernanceCase.Enforce() [REMOVED in Slice 2]
    ↓
OutboxRepository.InsertEvent() — same TX as case mutation
    ↓
COMMIT
```

The legacy path inserted outbox events atomically with the case mutation. But:
- No enforcement record was created
- No enforcement status was tracked

### 7.2 Canonical Expected

```
TX_A (Decision creation):
  INSERT INTO decisions
  INSERT INTO enforcements (status=pending)
  INSERT INTO outbox (event referencing enforcement_id)
COMMIT

TX_B (Worker execution):
  BEGIN
    MarkProcessing
    target domain mutation
    MarkSucceeded / MarkFailed
  COMMIT
```

### 7.3 Current Gap

**Decision creation does not create enforcement records.** The `DecisionService.CreateDecision()` (Slice 4) only:
1. Creates Decision
2. Resolves Case (if open)

It does NOT create enforcement records or outbox events.

**Gap:** The canonical Transaction Boundary (Decision + Enforcement + Outbox atomic) is NOT implemented.

---

## 8. Target-Domain Executor Matrix

| Target | Executor | Idempotent | Transaction | Result Observable |
|---|---|---|---|---|
| content | `ContentService.SoftDeleteForModeration` | ✅ (deleted_at IS NULL guard) | own tx | ✅ (content status) |
| comment | `CommentService.SoftDeleteForModeration` | ✅ (deleted_at IS NULL guard) | own tx | ✅ (comment status) |
| for_sale | `ForSaleService.Withdraw` | ✅ (terminal state = success) | own tx | ✅ (for_sale status) |
| auction | `AuctionService.CancelForModeration` | ✅ (terminal state = success) | own tx | ⚠️ (bid state unsafe) |
| user | `UserRepository.Update` | ✅ (already suspended = skip) | own tx | ✅ (user account_status) |

### 8.1 Content

**File:** `social/content/application/content_service.go`
- `SoftDeleteForModeration(ctx, tx, contentID)` — sets `deleted_at`, `deletion_reason`
- Idempotent: checks `deleted_at IS NULL` before mutation
- Safe boundary: only soft-deletes, no financial mutation

### 8.2 Comment

**File:** `social/content/application/comment_service.go`
- `SoftDeleteForModeration(ctx, tx, commentID)` — sets `deleted_at`
- Idempotent: checks `deleted_at IS NULL`
- Safe boundary

### 8.3 ForSale

**File:** `commerce/forsale/application/for_sale_service.go`
- `Withdraw(ctx, tx, forSaleID)` — sets `status=withdrawn`
- Idempotent: terminal state returns `InvalidTransitionError` → treated as success
- Safe boundary: no order/payment mutation

### 8.4 Auction — P1 UNSAFE

**File:** `commerce/auction/application/auction_service.go:1289-1320`

`CancelForModeration(ctx, tx, auctionID)`:
- Bypasses `CanCancel()` bid guard
- Does NOT handle bid state (refunds, voids)
- Does NOT notify bidders
- Does NOT handle order state
- **UNSAFE:** Auction cancelled with active bids, winner, or order

### 8.5 User

**File:** `worker/moderation_event_handler.go:597-666`
- Directly sets `user.AccountStatus = "suspended"` via `userRepo.Update`
- Idempotent: checks `account_status == "suspended"` before mutation
- **FINDING:** Uses repo directly, not user domain service (Audit 3 P1)

---

## 9. Auction Boundary — P1 BLOCKER

### 9.1 Finding

`AuctionService.CancelForModeration` (`auction_service.go:1289-1320`):

```go
func (s *AuctionService) CancelForModeration(ctx context.Context, tx db.Tx, auctionID uuid.UUID) error {
    // Bypasses CanCancel() — which checks bid count
    // Directly sets status = cancelled
    // Does NOT:
    //   - refund bidders
    //   - void active bids
    //   - handle order state
    //   - notify bidders
    //   - clean up settlement
}
```

### 9.2 Impact

If enforcement runs `CancelForModeration` on an auction with active bids:
- Bid money is not refunded
- Winner notification is not sent
- Settlement/claim flow is broken
- Bid state is inconsistent

### 9.3 Classification

```
P1 BLOCKER for Enforcement implementation
```

The auction command boundary MUST be changed to Auction-Domain-owned before canonical Enforcement can safely enforce on auctions.

---

## 10. Consumer/Producer Reachability

### 10.1 Registered Moderation Consumers

| Event Type | Enforcement Handler | Notification Handler | Fanout |
|---|---|---|---|
| `moderation.content.removed` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.comment.removed` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.for_sale.removed` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.auction.removed` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.user.suspended` | ✅ ModerationEventHandler | ✅ NotificationEventHandler + WS Eviction | ✅ |
| `moderation.content.restored` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.comment.restored` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.for_sale.restored` | ✅ ModerationEventHandler | ✅ + PromotionEventHandler | ✅ |
| `moderation.auction.restored` | ✅ (no-op) | ✅ NotificationEventHandler | ✅ |
| `moderation.user.restored` | ✅ ModerationEventHandler | ✅ NotificationEventHandler | ✅ |
| `moderation.chat_message.hidden` | ✅ ModerationEventHandler | — | sole |
| `moderation.chat_message.restored` | ✅ ModerationEventHandler | — | sole |

### 10.2 Producer Status

**NO PRODUCER EXISTS.** The legacy `GovernanceCase.Enforce()` (which emitted these events) was removed in Slice 2. Currently, no code inserts `moderation.*.removed` or `moderation.*.restored` events into the outbox.

**Classification:** All moderation outbox events are **DEAD** — consumers exist but events are never produced.

### 10.3 Canonical Enforcement Path (Future)

When canonical Enforcement runtime is implemented:
1. `DecisionService.CreateDecision` → INSERT enforcement + outbox event (atomic)
2. OutboxWorker picks up event → ModerationEventHandler handles it
3. Handler writes back enforcement status

The existing consumers (ModerationEventHandler + NotificationEventHandler) are the correct canonical enforcement executors. They just need enforcement write-back added.

---

## 11. Audit Trail

### 11.1 Current State

| Event | Audit Mechanism | Reliability |
|---|---|---|
| Decision created | `audit_events` (available but not wired) | N/A — not implemented |
| Enforcement created | **NONE** | **MISSING** |
| Enforcement processing | **NONE** | **MISSING** |
| Enforcement succeeded | **NONE** | **MISSING** |
| Enforcement failed | **NONE** | **MISSING** |

### 11.2 Available Infrastructure

`audit_events` table with `AuditEventRepository` (`governance/audit/repository/audit_event_repository.go`):
- `Emit(ctx, tx, event)` — reliable append-only, in-transaction
- Supports `event_type`, `entity_type`, `entity_id`, `actor_type`, `actor_id`, `payload_json`

**RECOMMENDATION:** Enforcement mutations should use `audit_events` (not LogSafe). The infrastructure exists and is reliable.

---

## 12. Legacy/Zombie Classification

| Component | Location | Classification |
|---|---|---|
| `GovernanceCase` entity | `entity/governance_case.go` | **FUTURE DEPENDENCY** (appeal Slice 9) |
| `ModerationRepository` | `infrastructure/repository/moderation_repository.go` | **DEAD/ZOMBIE** (reads dropped table) |
| `DomainAction` entity | `entity/domain_action.go` | **PARKED/ZOMBIE** (no migration, no code) |
| `DomainActionWorker` | `worker/domain_action_worker.go` | **PARKED/ZOMBIE** (never instantiated) |
| `AppealReversalService` | `application/appeal_reversal_service.go` | **PARKED/ZOMBIE** (never instantiated) |
| `ModerationService` | Removed in Slice 2 | **DELETED** |
| Legacy admin routes | Removed in Slice 2 | **DELETED** |

---

## 13. Mobile/Admin Legacy Vocabulary

### 13.1 Admin UI

**File:** `apps/admin/src/types/moderation.ts`

```typescript
export type ModerationCaseStatus = 'pending' | 'approved' | 'rejected' | 'enforced'
export type CaseAction = 'approve' | 'reject' | 'enforce'
export const moderationCaseStatusLabels = {
  enforced: 'Enforced',  // ← FALSE SUCCESS badge
}
```

**Impact:** Admin UI displays `Enforced` as a final badge. In canonical model, `enforced` should be split into:
- Decision outcome: `violation` / `no_violation`
- Enforcement status: `pending` / `processing` / `succeeded` / `failed`

**Classification:** LEGACY — must be replaced when admin workflow is rebuilt.

### 13.2 Mobile

**File:** `apps/mobile/lib/domains/system/report/domain/entities/report.dart`

```dart
case ReportAction.contentRemoved:  // ← legacy vocabulary
```

**Impact:** Mobile uses `contentRemoved` as a report action — this is legacy vocabulary.

**Classification:** LEGACY — out of scope for Enforcement implementation.

---

## 14. Blockers for Enforcement Implementation

### P1 (Must fix before Enforcement)

| # | Blocker | Evidence | Status |
|---|---|---|---|
| 1 | ~~**Outbox retry broken**~~ | ~~`MarkProcessing` only accepts `pending`~~ | ✅ **FIXED** |
| 2 | **No enforcement write-back** | `ModerationEventHandler` does not update `enforcements` table | PENDING |
| 3 | **Auction boundary unsafe** | `CancelForModeration` bypasses bid-state guards | PENDING |
| 4 | **No enforcement creation path** | `DecisionService.CreateDecision` does not create enforcement records | PENDING |

### P2 (Should fix, non-blocking)

| # | Finding | Evidence |
|---|---|---|
| 5 | **No audit trail for enforcement** | No `audit_events` writes for enforcement lifecycle |
| 6 | **Admin UI legacy vocabulary** | `enforced` badge conflates Decision + Enforcement |
| 7 | **User suspension via repo directly** | `ModerationEventHandler` uses `userRepo.Update`, not user domain service |
| 8 | **Outbox payload lacks enforcement_id** | Current payload has `case_id`, not `enforcement_id`/`decision_id` |

---

## 15. Recommended Bounded Implementation Sequence

### Phase 1: Fix Outbox Retry (P1)

**Scope:** One file change.
- `outbox_repository.go`: `MarkProcessing` accepts `IN ('pending', 'failed')`
- Verify with integration test

### Phase 2: Enforcement Creation + Write-Back (P1)

**Scope:**
- `entity/enforcement.go` — Enforcement entity
- `infrastructure/repository/enforcement_repository.go` — interface
- `infrastructure/repository/enforcement_repository_impl.go` — implementation
- `application/decision_service.go` — add enforcement INSERT in Decision creation TX
- `worker/moderation_event_handler.go` — add enforcement write-back after target mutation
- Integration tests

### Phase 3: Outbox Payload Redesign (P2)

**Scope:**
- Change outbox event payload to include `enforcement_id`, `decision_id`
- Update `ModerationEventHandler` to read new payload format

### Phase 4: Admin Workflow (P2)

**Scope:**
- Replace legacy admin types (`enforced` → `violation`/`no_violation` + enforcement status)
- Add enforcement status display
- Add enforcement retry endpoint

---

```text
VERDICT: BLOCKED (3 P1 blockers remain)

P1 BLOCKERS:
1. ~~Outbox retry broken~~ ✅ FIXED
2. No enforcement write-back (enforcements table never written)
3. Auction boundary unsafe (CancelForModeration bypasses bid guards)
4. No enforcement creation path (Decision doesn't create enforcement)

NON-BLOCKING FINDINGS:
- Admin UI legacy vocabulary
- No audit trail for enforcement
- User suspension via repo directly
- Outbox payload lacks enforcement_id

SCHEMA: PASS (enforcements table correct)
OUTBOX RETRY: ✅ FIXED (pending→processing, failed→processing both proven)
ENFORCEMENT RUNTIME: ABSENT
TARGET EXECUTORS: MOSTLY SAFE (auction unsafe)
CONSUMERS: EXIST BUT DEAD (no producers)
```
