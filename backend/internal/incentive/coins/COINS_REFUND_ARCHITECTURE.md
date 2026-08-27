# Coins Refund Flow - Hardened Architecture

## Summary

This document describes the hardened coins refund flow that ensures:
- **TRUE Single Entry Point**: All refunds flow through ONE handler ONLY
- **Idempotency**: Safe from double execution
- **Transaction-based**: Refunds based on actual spend transactions, not unreliable snapshots
- **Traceability**: All refunds logged and auditable

## CRITICAL RULE (ENFORCED)

**ALL coin refunds MUST go through `coins.refund_required` event and be handled exclusively by `CoinsRefundRequiredHandler`. No other component is allowed to perform coin refund.**

- ✅ ALLOWED: Emitting `coins.refund_required` events
- ❌ FORBIDDEN: Direct calls to `RefundCoinsInternal()` outside the handler
- ❌ FORBIDDEN: Direct balance mutations outside the handler
- ❌ FORBIDDEN: Creating `refund_earn` transactions outside the handler

---

## FLOW COMPARISON

### BEFORE (Multiple Entry Points - Ambiguous)

> ⚠️ HISTORICAL/REJECTED STATE — shown only to explain why the flow was
> hardened. Do NOT resurrect: direct `RefundCoinsInternal()` calls and
> reliance on the `order.coins_used` snapshot are forbidden.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        OLD REFUND FLOW                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  payment_webhook.go                 order_completion_service.go     │
│  ┌─────────────────────┐            ┌─────────────────────┐        │
│  │ Payment Failed      │            │ Order Expire()      │        │
│  │                     │            │                     │        │
│  │ → Calls            │            │ → Calls            │        │
│  │   RefundCoinsInternal()        │   RefundCoinsInternal()      │
│  └─────────────────────┘            └─────────────────────┘        │
│           │                                  │                     │
│           ▼                                  ▼                     │
│  ┌─────────────────────────────────────────────────────┐          │
│  │         Direct RefundCoinsInternal() Calls          │          │
│  │  • No unified entry point                           │          │
│  │  • Relies on order.coins_used snapshot              │          │
│  │  • Duplicate logic in multiple places               │          │
│  └─────────────────────────────────────────────────────┘          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### AFTER (Single Entry Point - Clear)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        NEW REFUND FLOW                              │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  payment_webhook.go         order_completion_service.go  money_refunded
│  ┌─────────────────────┐    ┌─────────────────────┐    ┌──────────┐ │
│  │ Payment Failed      │    │ Order Expire()      │    │ Handler  │ │
│  │                     │    │                     │    │          │ │
│  │ → Emits            │    │ → Emits            │    │ → Emits  │ │
│  │   coins.           │    │   coins.           │    │   coins.  │ │
│  │   refund_required  │    │   refund_required  │    │   refund_ │ │
│  └─────────────────────┘    └─────────────────────┘    │ required │ │
│           │                          │               └──────────┘ │
│           └──────────────┬───────────┘                          │
│                          ▼                                       │
│  ┌─────────────────────────────────────────────────────┐          │
│  │            Outbox Event Queue                       │          │
│  │  Event: coins.refund_required                       │          │
│  │  Payload: {order_id, user_id, reason, source}       │          │
│  └─────────────────────────────────────────────────────┘          │
│                          │                                         │
│                          ▼                                         │
│  ┌─────────────────────────────────────────────────────┐          │
│  │     CoinsRefundRequiredHandler (SINGLE ENTRY)       │          │
│  │  • Finds spend transaction (coins_transactions)     │          │
│  │  • Checks if already refunded (idempotency)         │          │
│  │  • Executes refund via RefundCoinsInternal()        │          │
│  │  • Logs all operations                              │          │
│  └─────────────────────────────────────────────────────┘          │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

**NOTE**: `MoneyRefundedEventHandler` now ALSO emits `coins.refund_required` events instead of
directly calling `RefundCoinsInternal()`. This ensures TRUE single entry point - ALL coin refunds,
whether from payment failure, order expiry, or post-payment refunds, flow through the SAME handler.

---

## EVENT SPECIFICATION

### `coins.refund_required`

**Purpose**: Triggers coins refund for ALL refund scenarios.

**Payload**:
```json
{
  "order_id": "uuid",
  "user_id": "uuid",
  "reason": "payment_failed" | "payment_expired" | "order_expired" | "order_cancelled" | "money_refunded" | "money_partially_refunded",
  "source": "payment_webhook" | "order_expire_worker" | "finance"
}
```

**CRITICAL**: Amount is NOT included - handler resolves from `coins_transactions`.

**Event Sources**:
- `payment_failed`: Payment was denied/cancelled (before payment success)
- `payment_expired`: Payment link expired (timeout)
- `order_expired`: Order expired without payment
- `order_cancelled`: Order was cancelled before payment
- `money_refunded`: Full refund after payment success (via `money.refunded` → `coins.refund_required`)
- `money_partially_refunded`: Partial refund after payment success (via `money.refunded` → `coins.refund_required`)

---

## HANDLER: CoinsRefundRequiredHandler

**Location**: `internal/worker/coins_refund_handler.go`

**Responsibility**: The SINGLE SOURCE OF TRUTH for all coins refunds.

**Logic**:
1. **Find Original Spend Transaction**
   - Queries `coins_transactions` for `type='spend'` AND `reference_type='order_spend'`
   - If not found → SKIP (no coins to refund)

2. **Check Already Refunded**
   - Queries for existing `type='earn'` AND `reference_type='refund_earn'`
   - If exists → SKIP (already refunded, idempotent)

3. **Execute Refund**
   - Calls `RefundCoinsInternal()` with INSERT-FIRST pattern
   - Database UNIQUE constraint prevents double refund
   - Returns success on duplicate (idempotent)

**Idempotency Guarantee**:
- Handler can be called multiple times safely
- No double refund possible (database constraint)
- No side effects on retry

---

## FILES MODIFIED

### 1. NEW FILES CREATED

| File | Purpose |
|------|---------|
| `internal/worker/coins_refund_handler.go` | Single entry point handler for all coins refunds |

### 2. FILES MODIFIED

| File | Changes |
|------|---------|
| `internal/worker/outbox_worker.go` | Added `SetupCoinsRefundRequiredHandler()` registration method |
| `internal/domain/payment/application/payment_webhook.go` | Replaced `RefundCoinsInternal()` call with `coins.refund_required` event emission |
| `internal/domain/order/application/order_completion_service.go` | Replaced `RefundCoinsInternal()` call in `Expire()` with `coins.refund_required` event emission |

### 3. FILES REMOVED/DEPENDENCIES CLEANED

- Removed `coinsService` dependency from `PaymentWebhookService` (no longer needed for direct refunds)

---

## PROOF OF SINGLE ENTRY POINT

### All Refund Paths Now Flow Through:

```
coins.refund_required event → CoinsRefundRequiredHandler → RefundCoinsInternal()
```

### Entry Point Sources:

| Source | Trigger | Event Emitted | Handler |
|--------|---------|---------------|---------|
| Payment Failure (deny/cancel) | Payment webhook receives deny/cancel | `coins.refund_required` | CoinsRefundRequiredHandler |
| Payment Expiry | Payment link expires (timeout) | `coins.refund_required` | CoinsRefundRequiredHandler |
| Order Expiry | Order expires without payment | `coins.refund_required` | CoinsRefundRequiredHandler |
| Money Refunded (full/partial) | After successful payment refund | `coins.refund_required` | CoinsRefundRequiredHandler |

### Single Entry Point Proof:

To verify, search for all calls to `RefundCoinsInternal`:
```bash
grep -r "RefundCoinsInternal" backend/
```

**Expected Result**: Should ONLY appear in:
- `internal/worker/coins_refund_handler.go` - The handler implementation
- `internal/incentive/coins/application/coins_service.go` - The method definition

Any other occurrence is a VIOLATION of single entry point.

---

## IDEMPOTENCY VERIFICATION

### Database Constraints

```sql
-- coins_transactions has UNIQUE constraint:
-- (user_id, reference_type, reference_id)
```

### Handler Logic

1. **INSERT-FIRST Pattern**: Attempts to create refund transaction first
2. **Duplicate Detection**: If UNIQUE constraint violation, treats as success
3. **Pre-Check Optimization**: Checks for existing refund before attempting insert

### Test Scenarios

| Scenario | Expected Result |
|----------|----------------|
| Event called 2x | Only 1 refund (idempotent) |
| No coins spent | Skip (no-op) |
| Already refunded | Skip (no-op) |
| Handler fails | Event retried by outbox worker |

---

## FAILURE HANDLING

### On Handler Failure

1. Event remains in `status='failed'`
2. Outbox worker retries with exponential backoff
3. After max attempts, moves to `status='dead_letter'`

### Future: coins.refund_failed Event

For manual recovery, a `coins.refund_failed` event can be emitted containing:
```json
{
  "order_id": "uuid",
  "user_id": "uuid",
  "error_message": "string"
}
```

This would trigger an alert for manual intervention.

---

## MIGRATION NOTES

### For Deployment

1. Register the new handler in worker initialization:
```go
worker.SetupCoinsRefundRequiredHandler(db, coinsService)
```

2. No database migration required (uses existing tables)

3. Existing `money.refunded` flow continues to work unchanged

### Rollback Plan

If issues arise:
1. Remove `coins.refund_required` handler registration
2. Direct `RefundCoinsInternal()` calls can be restored from git history

---

## ARCHITECTURE BENEFITS

1. **Single Source of Truth**: All refunds traceable to one handler
2. **Idempotency**: Safe from double execution at database level
3. **Transaction-Based**: Refunds based on actual spend, not snapshot
4. **Observability**: All refunds logged via outbox events
5. **Recovery**: Failed refunds retryable via outbox worker
6. **Clean Separation**: Refund logic isolated from order/payment logic

---

## REFERENCES

- **Coins README**: `internal/incentive/coins/README.md`
- **Handler**: `internal/worker/coins_refund_handler.go`
- **Worker Registration**: `internal/worker/outbox_worker.go`


