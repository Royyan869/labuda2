# REPORT — AUCTION SETTLEMENT STATE / DEADLINE / RESOLUTION AUTHORITY

> **Status:** AUDIT ONLY — NO IMPLEMENTATION
> **Generated:** 2026-09-02
> **Source:** Current filesystem (Git history is backup only)

---

## 1. Verdict

**`READY_FOR_FINAL_IMPLEMENTATION_PLAN`**

All state semantics are unambiguous. All deadline ownership is unambiguous. All authority conflicts identified in the previous plan have been resolved. The corrections required are precise and enumerated in Section 18. No blocking authority ambiguity remains.

One OWNER DECISION REQUIRED item is identified (Section 9, "Quote Arrival vs Buyer Deadline") but it does not block implementation readiness — the safe default is marked and can be resolved before coding begins.

---

## 2. Executive Summary

The previous implementation plan contained three critical contradictions:

1. **`shipping_resolved_at` was called "immutable" but also proposed cleared on DRAFT return.** Resolved: it is a **current-attempt state marker**, not a historical event. Historical resolution lives in a separate `auction_shipping_resolution` table with per-attempt rows.

2. **`settlement_deadline` was proposed both kept and removed.** Resolved: **REMOVE**. It is always `end_at + 24h`, has no independent business meaning, and creates duplicate authority. Queries compute the deadline as `end_at + INTERVAL '24 hours'`.

3. **The payment phase model was inconsistent.** Resolved: The auction stays in `waiting_settlement` after order creation and only transitions to `ended` on payment success. This makes `WAITING_SETTLEMENT → DRAFT` the single failure path and eliminates the need for `ENDED → DRAFT`.

Additional findings:

- **`expired_bnr` migration** is proven safe. Existing rows migrate to DRAFT. Historical strikes migrate to `commerce_violations`.
- **`uniq_active_auction_per_product`** index includes DRAFT — compatible with DRAFT return.
- **`ReleaseUnpaidOrder`** must be extended to also transition auction to DRAFT (currently only clears OrderID).
- **One owner decision** remains: whether seller's late quote (e.g., at T+23h) extends the buyer's acceptance deadline.

---

## 3. Evidence Reviewed

| Source | Path | What it proves |
|---|---|---|
| Auction entity | `backend/internal/commerce/auction/entity/auction.go` | State machine, `TransitionToWaitingSettlement()`, `SettlementDeadline` set to `now+24h`, `Settle()`, `ReleaseUnpaidOrder()`, `TransitionToExpiredBNR()` |
| Auction service | `backend/internal/commerce/auction/application/auction_service.go` | `EndAuctionInternal()` calls `TransitionToWaitingSettlement()`, `GeneratePricingTokenForAuctionClaim()` checks `SettlementDeadline` |
| `/claim` handler | `backend/internal/commerce/auction/delivery/http/auction_handler.go` (L720-860) | Bundled: validate → pricing token → order creation → `Settle()` → ENDED, all in one tx |
| Settlement worker | `backend/internal/worker/auction_settlement_worker.go` | Fires on `settlement_deadline <= NOW()`, transitions to `expired_bnr`, emits `auction_bnr_detected` |
| BNR strike handler | `backend/internal/worker/bnr_strike_handler.go` | Inserts into `buyer_bnr_strikes` from `auction_bnr_detected` event |
| BNR restriction checker | `backend/internal/commerce/auction/application/bnr_restriction.go` | Strike-count ladder (0→4+) with decay and admin reset |
| BNR decay worker | `backend/internal/worker/bnr_decay_worker.go` | Decays strikes after 180 days |
| BNR admin reset | `backend/internal/worker/bnr_admin_reset.go` | Sets `admin_reset = TRUE` on strikes |
| Order creation | `backend/internal/commerce/order/application/order_creation_service.go` (L1-1092) | `calculatePaymentExpiry()` returns 15min–6hr; auction uses "default" = 30min |
| Order entity | `backend/internal/commerce/order/entity/order.go` | `PaymentExpiresAt` single source of truth, `MarkExpired()`, `NewOrderFromSource()` |
| Order completion | `backend/internal/commerce/order/application/order_completion_service.go` (L1008-1160) | `Expire()`: order→expired, `restoreForSaleStock()`→`releaseAuctionOrderBinding()`, escrow refund, coin refund |
| `releaseAuctionOrderBinding` | Same file (L2036-2054) | Locks auction, clears `OrderID` via `ReleaseUnpaidOrder()`. **Auction stays `ended`** |
| `ReleaseUnpaidOrder` | `auction.go` (L437-472) | Clears `OrderID`. Doc comment explicitly states: "Ended is a deliberate terminal state with no valid outgoing transition" |
| Payment expiry worker | `backend/internal/worker/payment_expiry_worker.go` | Queries `payments WHERE status='pending' AND expired_at < NOW()`, calls `orderService.Expire()` |
| Shipping quote entity | `backend/internal/commerce/shipping/quote/entity/shipping_quote.go` | Statuses: ACTIVE→USED/EXPIRED/INVALID. `ExpiresAt` default 24h |
| Shipping quote service | `backend/internal/commerce/shipping/quote/application/shipping_quote_service.go` (L1-100) | `DefaultShippingQuoteExpiryHours = 24`, `MaxShippingQuoteExpiryHours = 168` |
| Address entity | `backend/internal/identity/address/entity/address.go` | `IsPrimary`, `IsAvailableForCheckout`, `AddressSnapshot` for immutability |
| DB schema | `backend/migrations/000001_canonical_schema.up.sql` (L477-498, L575, L1965, L2015) | `auctions` table, `buyer_bnr_strikes`, `UNIQUE(auction_id)`, `uniq_active_auction_per_product` |
| Unique constraint | Migration 000001, L2015 | `uniq_active_auction_per_product` includes `draft` in the WHERE clause |
| Mobile auction status | `apps/mobile/lib/domains/commerce/catalog/auction/domain/entities/auction_status.dart` | Parses `expired_bnr`, `waiting_settlement` |
| Mobile auction DTO | `apps/mobile/lib/domains/commerce/catalog/auction/data/dto/auction_dto.dart` (L486-488) | Reads `settlement_deadline` from JSON |
| Chat projection | `backend/internal/serverboot/chat_auction_projection_resolver.go` (L136-139) | Reads `settlement_deadline` from auction for chat resource projection |

---

## 4. Auction State Authority

### Question A: What does `WAITING_SETTLEMENT` mean?

**Evidence:**

1. `StatusWaitingSettlement` comment (auction.go L74): "when auction has ended but order not yet created. Winner can claim auction to create order."

2. `TransitionToWaitingSettlement()` (auction.go L397-406): Sets status, sets `SettlementDeadline = now + 24h`.

3. `EndAuctionInternal()` (auction_service.go L976-984): If `auction.HasWinner()` → `TransitionToWaitingSettlement()`. If no winner → `End()` → `ended`.

4. `Settle()` (auction.go L414-419): `waiting_settlement → ended`. Comment: "Used after order is created via claim flow."

5. `TransitionToExpiredBNR()` (auction.go L409-412): `waiting_settlement → expired_bnr`. Terminal.

**Verdict:**

`WAITING_SETTLEMENT` means: **winner known, settlement incomplete.** The auction remains in this state until one of:
- Payment succeeds → `Settle()` → `ENDED`
- Settlement failure (deadline/seller default/buyer timeout/BNR) → `DRAFT`

The current implementation bundles settlement into `/claim` which immediately calls `Settle()` → `ENDED` after order creation. The new design separates shipping resolution from payment, keeping the auction in `WAITING_SETTLEMENT` until payment completes.

**Critical design decision:** The auction MUST NOT transition to `ENDED` at order creation time. It must stay in `WAITING_SETTLEMENT` with `OrderID` set. `ENDED` is reserved for payment success.

---

## 5. `shipping_resolved_at` Authority

### The Previous Plan's Contradiction

The previous plan states:
- Section H.1: "`shipping_resolved_at` is immutable once set"
- Section D.4: `TransitionToDraftOnSettlementFailure()` clears `shipping_resolved_at`
- Section Q: Cleanup targets include clearing shipping state on DRAFT return

These are contradictory. If the auction returns to DRAFT after settlement failure, `shipping_resolved_at` must be cleared for the next attempt. But calling it "immutable" is false.

### Analysis

When an auction returns to DRAFT after settlement failure:

1. The previous shipping resolution is **invalid** — it belongs to a failed attempt
2. The next attempt needs a **fresh** shipping resolution
3. Historical resolution data must be preserved for audit

Therefore: `shipping_resolved_at` on the `auctions` table is a **current-attempt state marker**. It is cleared when the auction returns to DRAFT.

### **CANONICAL: OPTION 1 — Current-Attempt State Marker**

`shipping_resolved_at` is a current-attempt state marker on the `auctions` table:
- Set when shipping is resolved (first resolution wins)
- Cleared when auction returns to DRAFT after settlement failure
- NOT globally immutable
- Historical resolution attempts live in a separate table

### Historical Resolution

The `auction_shipping_resolution` table stores **one row per settlement attempt** (NOT per auction lifetime):

```sql
CREATE TABLE auction_shipping_resolution (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auction_id uuid NOT NULL REFERENCES auctions(id),
    attempt_number integer NOT NULL,  -- 1, 2, 3, ... per auction
    resolved_at timestamptz NOT NULL,
    resolution_mode text NOT NULL,     -- 'normal_shipping' | 'private_quote'
    -- ... snapshot fields ...
    created_at timestamptz NOT NULL DEFAULT NOW(),

    UNIQUE(auction_id, attempt_number)  -- NOT UNIQUE(auction_id)
);
```

**Why `UNIQUE(auction_id, attempt_number)` not `UNIQUE(auction_id)`:**
- An auction returning to DRAFT may be settled again
- Each attempt creates a new resolution row
- `attempt_number` is monotonically increasing (1, 2, 3...)
- The current attempt's `attempt_number` is tracked on the auction entity

**Why this is correct:**
- Failed settlement → DRAFT → re-listed → new attempt → new resolution row
- Historical audit trail preserved
- No data loss on DRAFT return
- Clean separation of current-attempt state vs. history

---

## 6. Shipping Resolution History

### Model Decision

**Option B: One row per settlement attempt.**

The `auction_shipping_resolution` table:
- `UNIQUE(auction_id, attempt_number)` — NOT `UNIQUE(auction_id)`
- `attempt_number` tracks the settlement attempt count
- Failed attempt resolution rows remain in the table after DRAFT return
- Current attempt resolution is identified by `attempt_number = current_attempt_number` on the auction

**Proof:**
- Auction fails → DRAFT (attempt 1 resolution row preserved)
- Auction re-listed → new attempt (attempt 2)
- Buyer resolves shipping → new resolution row (attempt_number = 2)
- If attempt 2 also fails → DRAFT (both resolution rows preserved)
- Seller can see full history of resolution attempts

**Interaction with order binding:**
- Each resolution row may or may not have an associated order
- If order was created but payment expired: order is in `expired` status, resolution row remains
- The resolution row's `attempt_number` matches the order's creation context

---

## 7. Seller Deadline Authority

### A. How is Case A known?

**At auction end** (`EndAuctionInternal()`):
1. Winner is known (`CurrentWinnerID`)
2. System resolves winner's primary shipping address
3. System checks coverage of all auction's selected Shipping Setups against the winner's address
4. If destination is NOT covered by any Shipping Setup → **Case A** (seller must provide private quote)
5. If destination IS covered → **Case B** (buyer may select normal shipping)

**Storage:** A boolean flag `seller_action_required` on the `auctions` table, computed and set at the moment the auction enters `waiting_settlement`.

### B. When does seller obligation begin?

**Immediately when the auction enters `waiting_settlement`.**

The seller obligation is: provide a private shipping quote by `auction_end + 24h`.

### C. Where is seller obligation stored?

`auctions.seller_action_required = TRUE` (set at auction end, never changed during the settlement attempt).

### D. What proves the seller has fulfilled the obligation?

A valid shipping quote exists for this auction:
- `shipping_quotes.source_type = 'auction' AND shipping_quotes.source_id = auction_id`
- `shipping_quotes.status = 'ACTIVE'`
- `shipping_quotes.seller_id = auction.seller_id`
- `shipping_quotes.buyer_id = auction.current_winner_id`

**Once a valid quote exists, the seller is no longer defaulting.** The system MUST NOT later punish the seller merely because the buyer has not accepted the quote.

### E. What happens after seller provides a quote?

1. `seller_quote_provided = TRUE` on the auction
2. The seller's obligation is discharged
3. The buyer may now accept the quote (which resolves shipping)
4. If the buyer does not accept before `auction_end + 24h`, it becomes a **buyer** failure (not seller)

---

## 8. Buyer Deadline Authority

### Complete Timeline State Table

| Time/State | Seller Obligation | Buyer Obligation | Can Seller Default? | Can Buyer Default? | Notes |
|---|---|---|---|---|---|
| **Auction ends** → `waiting_settlement` | Case A: provide quote by T+24h. Case B: none | Case B: select normal shipping by T+24h. Case A: wait | Yes (Case A only) | No (not yet) | `seller_action_required` set |
| **Case A / no quote / before T+24h** | Provide quote | Wait | Yes (still obligated) | No | Seller still in window |
| **Case A / no quote / at T+24h** | **DEFAULTED** | None | **VIOLATION + DRAFT** | None | Seller deadline worker fires |
| **Case A / quote provided (any time ≤ T+24h)** | Fulfilled | Accept quote by T+24h | No | Yes (after T+24h) | `seller_quote_provided = TRUE` |
| **Case A / quote at T+23h** | Fulfilled | Accept by T+24h (1h remaining) | No | Yes (after T+24h) | **OWNER DECISION: does late quote extend buyer window?** |
| **Case A / quote + T+24h passed / not accepted** | Fulfilled | **DEFAULTED** | No | **VIOLATION + DRAFT** | Buyer deadline worker fires |
| **Case B / normal shipping / before T+24h** | None | Select shipping setup | No | Yes (after T+24h) | Buyer in window |
| **Case B / normal shipping / at T+24h** | None | **DEFAULTED** | No | **VIOLATION + DRAFT** | Buyer deadline worker fires |
| **Case B + seller optional quote / before T+24h** | Quote optional | Select normal OR accept quote | No | Yes (after T+24h) | Both paths available |
| **Case B + seller optional quote / T+24h** | Quote optional | **DEFAULTED if not resolved** | No | **VIOLATION + DRAFT** | Whichever path, must resolve by T+24h |
| **Shipping resolved** | None | Pay within 24h | No | Yes (BNR after 24h) | `shipping_resolved_at` set, order created |
| **Payment succeeds** → `ended` | None | None | No | No | Terminal |
| **Payment expires** → BNR → `DRAFT` | None | **VIOLATION + DRAFT** | May relist (if not restricted) | Restricted | Financial rollback complete |
| **DRAFT after seller default** | Restricted | None | Cannot relist while restricted | None | May relist after restriction expires |
| **DRAFT after buyer failure** | None | Restricted | May relist immediately | Cannot transact | Seller unaffected |

### Critical Distinction

**"Buyer deadline"** always means `auction_end + 24h` from the auction end time. It does NOT change based on when the seller provides a quote. If the seller provides a quote at T+23h, the buyer has only 1 hour to accept.

This is the **safe default** because:
- The buyer's 24h window is the canonical deadline
- Extending it based on seller's late action creates a moving target
- The seller's obligation is to provide the quote by T+24h; the buyer's obligation is to resolve shipping by T+24h

**OWNER DECISION REQUIRED:** If the business wants the buyer's deadline to extend when the seller provides a late quote, this must be explicitly decided. The current locked business truth says "buyer has 24h from auction end" with no extension clause.

---

## 9. Private Quote Acceptance Window

### Current Technical Expiry

**Evidence:** `DefaultShippingQuoteExpiryHours = 24` (shipping_quote_service.go L28). A newly created shipping quote expires 24h after creation by default.

### Analysis

The existing 24h quote expiry is a **technical default**, not an auction settlement business rule. It was designed for the chat-based For Sale flow where quotes are ephemeral.

For auction settlement, the **canonical deadline** is `auction_end + 24h` — the same deadline that governs both seller obligation and buyer obligation. The technical quote expiry is coincidentally the same duration but operates independently.

### What Must Change

The private quote for auction settlement MUST:
1. Have `expires_at` set to `auction_end + 24h` (the canonical deadline), NOT `now + 24h` (the technical default)
2. If seller creates quote at T+23h, the quote's `expires_at` should be `auction_end + 24h` = T+24h (only 1h validity)
3. The quote expiry and the buyer deadline are the **same timestamp**

### What Must NOT Change

The technical `DefaultShippingQuoteExpiryHours = 24` for the For Sale chat flow remains unchanged. Auction settlement uses a different `expires_at` calculation.

### Buyer Acceptance Deadline

The buyer's acceptance deadline for a private quote is `auction_end + 24h` — the same canonical deadline. There is **no separate "acceptance window"** beyond this.

If the buyer does not accept by `auction_end + 24h`:
- It is a buyer shipping timeout (same as failing to select normal shipping)
- Buyer commerce violation
- Auction → DRAFT

### What Happens at Hour 24

At `auction_end + 24h`:
- Seller deadline worker checks: if `seller_action_required = TRUE` AND no valid quote → seller default
- Buyer deadline worker checks: if `shipping_resolved_at IS NULL` AND `seller_quote_provided = TRUE` (or Case B) → buyer default
- Both workers are idempotent and mutually exclusive via `shipping_resolved_at IS NULL` guard

---

## 10. `settlement_deadline` Verdict

### Evidence

1. **Set in `TransitionToWaitingSettlement()`** (auction.go L402-404):
   ```go
   deadline := time.Now().Add(24 * time.Hour)
   a.SettlementDeadline = &deadline
   ```
   Always `now + 24h` at the moment of transition.

2. **Used by `AuctionSettlementWorker`** (auction_settlement_worker.go L229):
   ```sql
   WHERE status = 'waiting_settlement' AND settlement_deadline <= NOW()
   ```
   Fires when `settlement_deadline` has passed.

3. **Used by `GeneratePricingTokenForAuctionClaim()`** (auction_service.go L1162):
   ```go
   if auction.SettlementDeadline != nil && now.After(*auction.SettlementDeadline) {
       return nil, fmt.Errorf("%w: deadline=%s", entity.ErrSettlementDeadlinePassed, ...)
   }
   ```

4. **Read by chat projection** (chat_auction_projection_resolver.go L138):
   ```sql
   a.settlement_deadline
   ```

5. **Read by mobile DTO** (auction_dto.dart L486-488):
   ```dart
   settlementDeadline: json['settlement_deadline'] != null
       ? DateTime.parse(json['settlement_deadline'] as String)
       : null,
   ```

6. **Written by auction repository** (auction_repository.go L33, L209):
   Included in INSERT and UPDATE queries.

### Analysis

`settlement_deadline` is **always** `auction.end_at + 24h` (because it's set in `TransitionToWaitingSettlement()` which is called right after `EndAuctionInternal()` which is called when `end_at` has passed). It has no independent business meaning.

The only question is whether removing it causes harm:
- Worker queries can compute `end_at + INTERVAL '24 hours'`
- Claim validation can compute `end_at + INTERVAL '24 hours'`
- Chat projection and mobile can compute it on read

### **VERDICT: REMOVE**

`settlement_deadline` is a redundant derived field. All consumers can compute it as `end_at + INTERVAL '24 hours'`. Keeping it creates duplicate authority (stored value vs. computed truth). Removing it eliminates a class of inconsistency bugs.

**Impact on chat projection:** The chat projection query must be updated to compute `a.end_at + INTERVAL '24 hours' AS settlement_deadline` instead of reading `a.settlement_deadline`.

**Impact on mobile:** The DTO must compute the deadline from `end_at` instead of reading `settlement_deadline`. Or the API response must compute and return it.

---

## 11. Worker Authority

### Model Decision: One Worker with Deterministic Branching

**Option A: One settlement deadline worker with deterministic branching.**

**Why NOT separate workers:**

1. The trigger is the same for all paths: `auction_end + 24h`
2. The query is the same: `waiting_settlement` auctions where deadline passed AND `shipping_resolved_at IS NULL`
3. The branching logic (who defaulted) is deterministic from the auction state
4. Two workers would need to coordinate to avoid both firing on the same auction
5. One worker with `FOR UPDATE SKIP LOCKED` naturally serializes

### Worker Logic

```
AuctionSettlementDeadlineWorker:

1. Find auctions:
   SELECT id FROM auctions
   WHERE status = 'waiting_settlement'
     AND shipping_resolved_at IS NULL
     AND end_at + INTERVAL '24 hours' <= NOW()
   FOR UPDATE SKIP LOCKED

2. For each auction, determine who defaulted:
   IF seller_action_required = TRUE AND seller_quote_provided = FALSE:
     → SELLER DEFAULT
   ELSE:
     → BUYER SHIPPING TIMEOUT (either Case A buyer didn't accept, or Case B buyer didn't select)

3. Execute atomic rollback:
   a. Record violation (commerce_violations)
   b. Apply restriction (commerce_restrictions)
   c. Transition auction → DRAFT
   d. Clear settlement state
   e. Emit outbox events
```

### Why This Is Correct

The `shipping_resolved_at IS NULL` guard ensures:
- Already-resolved auctions are skipped (shipping resolved, payment in progress)
- Only unresolved auctions past deadline are processed
- `seller_action_required` and `seller_quote_provided` flags deterministically classify the failure

### Payment Expiry Worker (Separate)

The `PaymentExpiryWorker` is a **separate worker** that handles payment expiry after shipping has been resolved:

```
PaymentExpiryWorker:
  1. Find payments WHERE status = 'pending' AND expired_at < NOW()
  2. For each: expire payment → expire order → financial rollback
  3. For auction orders: transition auction → DRAFT + buyer violation
```

This is correct because:
- Payment expiry is after shipping resolution (different phase)
- Payment expiry handles escrow, coins, gateway refund (financial concerns)
- Payment expiry is a different worker from settlement deadline

---

## 12. First-Resolution-Wins Concurrency

### Concurrency Model

**`FOR UPDATE + NULL guard` is sufficient.** Here is the proof:

#### Path 1: Buyer selects normal shipping

```go
// 1. Lock auction
auction := auctionRepo.GetForUpdate(ctx, tx, auctionID)

// 2. Guard: shipping not yet resolved
if auction.ShippingResolvedAt != nil { return ErrAlreadyResolved }

// 3. Validate shipping setup covers address
// 4. Set shipping_resolved_at
auction.ShippingResolvedAt = &now

// 5. Insert resolution row
insertResolution(attempt_number, "normal_shipping", ...)

// 6. Create order (PaymentExpiresAt = shipping_resolved_at + 24h)
order := createOrder(...)

// 7. Bind order
auction.OrderID = &order.ID

// 8. Persist (same tx)
auctionRepo.UpdateTx(ctx, tx, auction)
```

#### Path 2: Buyer accepts private quote

```go
// 1. Lock auction
auction := auctionRepo.GetForUpdate(ctx, tx, auctionID)

// 2. Guard: shipping not yet resolved
if auction.ShippingResolvedAt != nil { return ErrAlreadyResolved }

// 3. Validate quote (ACTIVE, not expired, ownership)
// 4. Mark quote as USED (FOR UPDATE on quote row)
// 5. Set shipping_resolved_at
auction.ShippingResolvedAt = &now

// 6. Insert resolution row
insertResolution(attempt_number, "private_quote", ...)

// 7. Create order
// 8. Bind order
// 9. Persist
```

#### Race: Both paths arrive concurrently

1. Both call `GetForUpdate()` — **PostgreSQL serializes** (one gets lock, other waits)
2. First to acquire lock: `ShippingResolvedAt IS NULL` → proceeds, sets `ShippingResolvedAt`
3. Second acquires lock: `ShippingResolvedAt IS NOT NULL` → **rejected** (ErrAlreadyResolved)
4. First transaction commits. Second transaction rolls back (or returns error).

#### Belt and Suspenders: DB-Level Guard

```sql
UPDATE auctions
SET shipping_resolved_at = NOW()
WHERE id = $1
  AND shipping_resolved_at IS NULL
  AND status = 'waiting_settlement'
```

If `rows_affected = 0`, resolution failed. This provides defense-in-depth even if application logic has a bug.

#### Quote Lifecycle Protection

Quote creation alone does NOT resolve shipping. Only **acceptance** resolves shipping. So:
- Seller creates quote (status = ACTIVE) → shipping NOT resolved
- Buyer accepts quote → quote status = USED + shipping_resolved_at = NOW() → shipping resolved
- Concurrent normal shipping selection → `shipping_resolved_at IS NULL` check fails → rejected

**UNIQUE constraint:** `UNIQUE(auction_id, attempt_number)` on `auction_shipping_resolution` prevents duplicate resolution rows.

### Required Invariant

Only one successful shipping resolution may produce one settlement order. No:
- Double order (protected by `UNIQUE(order_id)` on auctions + `OrderID != nil` guard)
- Double payment (one order = one payment intent)
- Contradictory shipping (first-resolution-wins)
- Overwritten destination (immutable snapshot in resolution row)
- Second payment deadline (single `PaymentExpiresAt` on order)

---

## 13. Payment Phase

### Current Flow (Bundled)

```
/claim → validate + pricing token + order creation + Settle() → ENDED
         (all in one transaction)
```

After this, auction is `ended` with `OrderID` set. If payment expires:
- `Expire()` → order→expired → `releaseAuctionOrderBinding()` clears `OrderID` → auction stays `ended`

### New Flow (Separated)

```
1. Auction ends → waiting_settlement
2. Shipping resolved → shipping_resolved_at set, order created, OrderID set
   Auction STAYS in waiting_settlement (NOT settled to ENDED)
3. Payment succeeds → order→paid → auction.Settle() → ENDED
4. Payment expires → order→expired → financial rollback → auction→DRAFT
```

### Why This Is Correct

**The auction MUST NOT transition to ENDED at order creation time** because:
- `ENDED` is terminal with no outgoing transitions
- If the order later expires, we need to return to DRAFT
- `ENDED → DRAFT` would be an unusual and confusing state machine path
- Staying in `WAITING_SETTLEMENT` is semantically correct: settlement IS still waiting (waiting for payment)

**The state machine becomes:**

```
waiting_settlement → ended (payment success)
waiting_settlement → draft (settlement failure)
```

Both are outgoing transitions from `WAITING_SETTLEMENT`. Clean, simple, no backtracking.

### Compatibility Check

| Component | Compatible? | Notes |
|---|---|---|
| Auction state machine | ✅ | `waiting_settlement → ended` and `waiting_settlement → draft` |
| Order lifecycle | ✅ | Order goes through normal lifecycle (pending → paid → shipped → completed) |
| Payment expiry worker | ✅ | Finds expired payments, calls `Expire()` |
| Settlement deadline worker | ✅ | Finds unresolved auctions past deadline |
| Order binding | ✅ | `OrderID` set at order creation, cleared on DRAFT return |
| Payment success handler | ✅ | Calls `Settle()` on auction, transitions to ENDED |
| Payment expiry handler | ✅ | Calls DRAFT return + financial rollback |
| `uniq_active_auction_per_product` | ✅ | `waiting_settlement` included in index |

### Settlement Worker vs Payment Expiry Worker

These are **two separate concerns**:

| Worker | Fires when | Handles |
|---|---|---|
| Settlement deadline worker | `end_at + 24h` AND shipping NOT resolved | Seller/buyer shipping default → DRAFT |
| Payment expiry worker | Payment `expired_at < NOW()` | Payment timeout → financial rollback → DRAFT |

They are mutually exclusive because:
- Settlement deadline worker only fires when `shipping_resolved_at IS NULL`
- Payment expiry worker only fires when an order exists (which requires `shipping_resolved_at IS NOT NULL`)
- An auction cannot be in both states simultaneously

---

## 14. BNR Rollback

### Transaction Boundary

```
BEGIN
  -- 1. Lock payment (FOR UPDATE)
  payment := paymentRepo.GetByIDForUpdate(paymentID)

  -- 2. Validate payment is expired
  IF time.Now().Before(payment.ExpiredAt) → ROLLBACK

  -- 3. Mark payment as expired (pending → expired)
  paymentRepo.UpdateStatus(paymentID, pending, expired)

  -- 4. Lock order (FOR UPDATE)
  order := orderRepo.GetForUpdate(orderID)

  -- 5. Validate order status transition
  order.MarkExpired()  // pending → expired

  -- 6. Restore stock / release auction binding
  IF order.SourceType == auction:
    auction := auctionRepo.GetForUpdate(order.SourceID)
    auction.ReleaseUnpaidOrder(order.ID)  // clears OrderID
    // NEW: transition auction → DRAFT
    auction.TransitionToDraftOnSettlementFailure()
    // NEW: clear settlement state
    auction.CurrentWinnerID = nil
    auction.CurrentBid = nil
    auction.ShippingResolvedAt = nil
    auction.SellerActionRequired = false
    auction.SellerQuoteProvided = false
    auctionRepo.UpdateTx(auction)

  -- 7. Release escrow (if any)
  escrow := walletService.GetEscrowForOrder(order.ID)
  IF escrow != nil:
    paymentService.InitiateGatewayRefundForOrder(...)
    paymentService.RefundToBuyer(...)

  -- 8. Refund coins (if any)
  IF order.CoinsUsed > 0:
    outboxRepo.InsertEvent("coins.refund_required", ...)

  -- 9. Reactivate shipping quote (if any)
  IF order.ShippingQuoteID != nil:
    shippingQuoteService.ReactivateQuoteIfEligible(...)

  -- 10. Record buyer violation (NEW)
  violationID := commerceViolationService.Record(ctx, tx,
    userID: order.BuyerID,
    type: "buyer_bnr",
    sourceType: "auction",
    sourceID: order.SourceID,
  )

  -- 11. Apply buyer restriction (NEW)
  commerceRestrictionService.Apply(ctx, tx, order.BuyerID, violationID)

  -- 12. Update order status
  orderRepo.UpdateStatusTx(order)

  -- 13. Emit outbox events
  outboxRepo.InsertEvent("order.expired", order.ID, ...)
  outboxRepo.InsertEvent("auction.returned_to_draft", auction.ID, ...)

COMMIT
```

### What Is Proven Correct

| Component | Status |
|---|---|
| Order → expired | ✅ `MarkExpired()` is correct |
| Escrow release | ✅ `RefundToBuyer()` is idempotent |
| Gateway refund | ✅ `InitiateGatewayRefundForOrder()` handles payment gateway |
| Coin refund | ✅ Via outbox event `coins.refund_required` |
| Shipping quote reactivation | ✅ `ReactivateQuoteIfEligible()` is correct |
| Auction OrderID release | ✅ `ReleaseUnpaidOrder()` is idempotent |
| Outbox event emission | ✅ Correct |

### What Must Change

| Component | Current | Required |
|---|---|---|
| Auction state after expire | Stays `ended` (via `ReleaseUnpaidOrder`) | Transitions to `DRAFT` (via `TransitionToDraftOnSettlementFailure`) |
| Violation recording | None | Record in `commerce_violations` |
| Restriction application | None | Apply in `commerce_restrictions` |
| Settlement state clearing | None | Clear `CurrentWinnerID`, `CurrentBid`, `ShippingResolvedAt`, `SellerActionRequired`, `SellerQuoteProvided` |

### Idempotency

The entire rollback is idempotent:
- `MarkExpired()` returns `InvalidTransitionError` if order already expired → handled as no-op
- `ReleaseUnpaidOrder()` returns nil if `OrderID` already nil → no-op
- `RefundToBuyer()` is documented as idempotent
- `ReactivateQuoteIfEligible()` checks quote status before acting
- Commerce violation insert can use `ON CONFLICT` for safety
- Commerce restriction upsert is naturally idempotent

---

## 15. `expired_bnr` Migration

### Can Existing Rows Safely Become DRAFT?

**Yes, with the following conditions:**

1. **Auction fields cleared:**
   ```sql
   UPDATE auctions
   SET status = 'draft',
       order_id = NULL,
       settlement_deadline = NULL,
       current_winner_id = NULL,
       current_bid = NULL,
       seller_action_required = FALSE,
       seller_quote_provided = FALSE,
       shipping_resolved_at = NULL,
       updated_at = NOW()
   WHERE status = 'expired_bnr';
   ```

2. **Historical bids preserved:** `auction_bids` rows are NOT deleted. They remain as historical records. The auction's `current_bid = NULL` means no active bid, but old bid rows are intact.

3. **Product binding preserved:** The product's `SellingSurface` remains `SellingSurfaceAuction`. The auction is the same entity returning to DRAFT. No product change needed.

4. **`uniq_active_auction_per_product` compatible:** DRAFT is included in the unique index. Since there's only one `expired_bnr` auction per product (the index previously excluded terminal states, allowing new auctions), after migration there will be one DRAFT auction per product. If the seller had already created a NEW auction for the same product while the old one was `expired_bnr`, there will be a UNIQUE constraint violation.

**Mitigation:** The migration must check for conflicts. If a product already has a non-terminal auction, the `expired_bnr` row should be migrated to `cancelled` instead of `draft` (since the seller already moved on).

### Historical BNR Fact Preservation

**`buyer_bnr_strikes` data:**

The `buyer_bnr_strikes` table has:
- `buyer_id`, `auction_id` (UNIQUE), `struck_at`, `decayed_at`, `admin_reset`

Every `expired_bnr` auction should have a corresponding strike (emitted by `AuctionSettlementWorker` via `auction_bnr_detected`). But partial failures are possible (outbox event not processed).

**Migration plan:**
1. Insert `commerce_violations` rows from `buyer_bnr_strikes`:
   ```sql
   INSERT INTO commerce_violations (user_id, violation_type, source_type, source_id, reason, created_at)
   SELECT buyer_id, 'buyer_bnr', 'auction', auction_id,
          'Historical BNR strike (migrated from buyer_bnr_strikes)',
          struck_at
   FROM buyer_bnr_strikes
   WHERE admin_reset = FALSE;  -- Exclude admin-reset strikes
   ```

2. Insert `commerce_restrictions` from strike counts:
   - For each buyer with active strikes: compute restriction based on count and most recent strike
   - Use the new ladder (7d/15d/30d) for any currently-restricted buyers
   - For buyers whose strikes were decayed or admin-reset: no restriction

3. **Do NOT delete `buyer_bnr_strikes`** until migration is verified. Drop it in a subsequent migration after validation.

### Migration Faithfulness

The migration CAN faithfully transform old history because:
- `buyer_bnr_strikes` has `buyer_id`, `auction_id`, `struck_at` — enough for `commerce_violations`
- The strike count per buyer determines the restriction level
- Admin-reset and decayed strikes are excluded (they don't carry restriction in the new system)
- The `UNIQUE(auction_id)` on strikes ensures one violation per auction

**No migration blocker identified.**

---

## 16. Definitive State/Timeline Matrix

| State | Seller Obligation | Buyer Obligation | Shipping | Payment | Failure | Next States |
|---|---|---|---|---|---|---|
| **waiting_settlement / Case A / no quote / <T+24h** | Provide quote by T+24h | Wait | unresolved | unavailable | seller default at T+24h | → draft (seller) |
| **waiting_settlement / Case A / quote provided** | Fulfilled | Accept quote by T+24h | unresolved | unavailable | buyer timeout at T+24h | → draft (buyer) |
| **waiting_settlement / Case A / quote + buyer accepted** | Fulfilled | Pay | resolved | available (24h) | BNR at payment expiry | → ended (success) or → draft (BNR) |
| **waiting_settlement / Case B / normal / <T+24h** | None | Select shipping by T+24h | unresolved | unavailable | buyer timeout at T+24h | → draft (buyer) |
| **waiting_settlement / Case B / seller optional quote / <T+24h** | Quote optional | Select OR accept by T+24h | unresolved | unavailable | buyer timeout at T+24h | → draft (buyer) |
| **waiting_settlement / resolved / payment pending** | None | Pay within 24h | resolved | available | BNR at payment expiry | → ended or → draft (BNR) |
| **DRAFT after seller default** | Restricted (cannot sell) | None | failed attempt | none | — | → scheduled (after restriction) |
| **DRAFT after buyer failure** | May relist immediately | Restricted (cannot buy) | failed attempt | none | — | → scheduled → active |
| **DRAFT after BNR** | May relist immediately | Restricted (cannot buy) | resolved | failed attempt | — | → scheduled → active |
| **ENDED** | None | None | resolved | paid | terminal | — |
| **CANCELLED** | None | None | none | none | terminal | — |

---

## 17. Definitive Authority Map

| Concern | Canonical Authority | Legacy/Duplicate | Action |
|---|---|---|---|
| Auction lifecycle | `auction.status` state machine | — | Add `waiting_settlement → draft` |
| Settlement phase | `auction.status = waiting_settlement` | `settlement_deadline` | REMOVE `settlement_deadline` |
| Seller deadline | `auction.end_at + 24h` (computed) | `settlement_deadline` (stored) | REMOVE `settlement_deadline` |
| Buyer shipping deadline | `auction.end_at + 24h` (computed) | `settlement_deadline` (stored) | REMOVE `settlement_deadline` |
| Shipping resolution (current) | `auction.shipping_resolved_at` | — | ADD column (current-attempt marker) |
| Shipping resolution (history) | `auction_shipping_resolution` per attempt | — | NEW table |
| Private quote lifecycle | `shipping_quotes` (existing entity) | — | Extend for auction settlement |
| Payment deadline | `order.payment_expires_at` (existing) | `calculatePaymentExpiry()` for auctions | Override for auctions: `shipping_resolved_at + 24h` |
| Violation | `commerce_violations` (new) | `buyer_bnr_strikes` | REPLACE |
| Restriction | `commerce_restrictions` (new) | `BNRStrikeChecker` ladder | REPLACE |
| BNR | `commerce_violations` where `type = 'buyer_bnr'` | `buyer_bnr_strikes` + `BNRStrikeChecker` | REPLACE |

---

## 18. Corrections Required to Previous Plan

### Section A (Executive Summary)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "`shipping_resolved_at` immutable once set" | Contradicts DRAFT return clearing it | "`shipping_resolved_at` is a current-attempt marker, cleared on DRAFT return" |
| "`settlement_deadline` kept as denormalized field" | Contradicts cleanup section suggesting removal | "REMOVE `settlement_deadline` — redundant derived field" |

### Section D (State Machine Changes)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| `TransitionToDraftOnSettlementFailure()` clears `shipping_resolved_at` | Fine, but Section H calls it immutable | Consistent: shipping_resolved_at is current-attempt, cleared on DRAFT |
| `CurrentWinnerID = nil` on DRAFT return | Correct | No change needed |
| `CurrentBid = nil` on DRAFT return | Correct | No change needed |

### Section E (Winner Destination Flow)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "`auction_shipping_resolution` table with `UNIQUE(auction_id)`" | Wrong — prevents DRAFT return + re-attempt | `UNIQUE(auction_id, attempt_number)` — one row per attempt |

### Section H (Shipping Resolution Concurrency)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "shipping_resolved_at is immutable once set" | Contradicts DRAFT return | "shipping_resolved_at is a current-attempt marker, cleared on DRAFT return. Historical data in auction_shipping_resolution." |

### Section I (Payment Deadline Changes)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "Auction stays in waiting_settlement (NOT settled to ended yet)" | Was unclear, mixed with "Settle()" on payment success | Clear: auction stays in `waiting_settlement` with OrderID set. `Settle()` only called on payment success. |

### Section J (BNR Financial Rollback)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "NEW: Transition auction → draft (after financial rollback)" | Good, but the transaction boundary was not fully specified | Full transaction boundary specified in Section 14 above |

### Section N (API/Application Changes)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "`/resolve-shipping` creates order AND resolves shipping in one step" | Good | No change — correct |

### Section O (Worker Changes)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "Refactor `AuctionSettlementWorker`" | Was ambiguous about whether one or two workers | "One `AuctionSettlementDeadlineWorker` with deterministic branching based on `seller_action_required` and `seller_quote_provided`" |

### Section P (Database Changes)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "Keep `settlement_deadline` for query convenience" | Contradicts cleanup section | "REMOVE `settlement_deadline` — compute as `end_at + INTERVAL '24 hours'`" |
| "`UNIQUE(auction_id)` on auction_shipping_resolution" | Prevents DRAFT + re-attempt | `UNIQUE(auction_id, attempt_number)` |

### Section Q (Cleanup)

| Current Statement | Problem | Corrected Statement |
|---|---|---|
| "`settlement_deadline` listed in both KEEP and REMOVE" | Contradiction | "REMOVE `settlement_deadline`" |

---

## 19. Residue Audit

### `expired_bnr`

| Location | Classification | Action |
|---|---|---|
| `auction.go` StatusExpiredBNR | Active authority | REMOVE |
| `auction.go` transitionAllowed map | Active authority | REMOVE entry |
| `auction.go` TransitionToExpiredBNR() | Active authority | REMOVE method |
| `auction.go` PublicLifecycle() | Active authority | REMOVE case |
| `auction.go` IsPublicDiscoverable() | Active authority | REMOVE case |
| `auction.go` IsRepostable() | Active authority | REMOVE (if referenced) |
| `auction.go` BNRAuctionRestrictedError | Active authority | REMOVE (replace with generic) |
| `auction_settlement_worker.go` | Active authority | REFACTOR (no more expired_bnr transition) |
| `moderation_event_handler.go:623,652` | Consumer | UPDATE (remove expired_bnr handling) |
| `shared/view_access.go:23` | Consumer | UPDATE (remove expired_bnr case) |
| `search_repository_impl.go:612` | Consumer | UPDATE (remove expired_bnr exclusion) |
| `publiccard/auction_card.go:18` | Consumer | UPDATE (remove expired_bnr reference) |
| `bidding_service.go:160,199` | Consumer | UPDATE (remove expired_bnr from queries) |
| `outbox_worker.go:1116-1121` | Consumer (BNR handler) | REPLACE (commerce_violations) |
| `dependencies.go:1617-1620` | Wiring | UPDATE |
| `migration 000001:41` | Migration (enum definition) | MIGRATE (new enum without expired_bnr) |
| `migration 000001:494` | Migration (column default) | MIGRATE |
| All test files with `expired_bnr` | Tests | UPDATE/REPLACE |
| Mobile `auction_status.dart` | Mobile consumer | UPDATE (remove expiredBNR enum value) |
| Mobile `auction_dto.dart` | Mobile consumer | UPDATE (remove expired_bnr parsing) |

### `settlement_deadline`

| Location | Classification | Action |
|---|---|---|
| `auction.go` SettlementDeadline field | Active authority | REMOVE from struct |
| `auction.go` TransitionToWaitingSettlement() | Active authority | REMOVE deadline assignment |
| `auction.go` ErrSettlementDeadlinePassed | Active authority | KEEP (error still valid, just computed differently) |
| `auction_repository.go` INSERT/UPDATE | Consumer | REMOVE from queries |
| `chat_auction_projection_resolver.go:138` | Consumer | UPDATE (compute from end_at) |
| `auction_service.go:1162` | Consumer | UPDATE (compute from end_at) |
| `auction_settlement_worker.go:229` | Consumer | UPDATE (compute from end_at) |
| Mobile `auction_dto.dart:486-488` | Mobile consumer | UPDATE (compute from end_at or API computes) |
| Test INSERT statements | Tests | UPDATE |

### `buyer_bnr_strikes`

| Location | Classification | Action |
|---|---|---|
| `migration 000001:575` | Migration (table creation) | KEEP for history, DROP in new migration |
| `migration 000001:1864` | Migration (PK) | KEEP for history |
| `migration 000001:1965` | Migration (UNIQUE) | KEEP for history |
| `migration 000001:2028` | Migration (index) | KEEP for history |
| `migration 000001:2293-2294` | Migration (FK) | KEEP for history |
| `bnr_strike_handler.go` | Active authority | REMOVE entirely |
| `bnr_decay_worker.go` | Active authority | REMOVE entirely |
| `bnr_admin_reset.go` | Active authority | REMOVE entirely |
| `bnr_restriction.go` (BNRStrikeChecker) | Active authority | REMOVE entirely |
| `bnr_telemetry.go` | Active authority | REMOVE |
| `dependencies.go` wiring | Wiring | REMOVE BNR wiring |
| `outbox_worker.go:1116-1121` | Consumer | REPLACE |

### `BNRStrikeChecker`

| Location | Classification | Action |
|---|---|---|
| `bnr_restriction.go` | Active authority | REMOVE entirely |
| `bnr_restriction_test.go` | Test | REPLACE with restriction tests |
| `auction_service.go: SetBNRStrikeChecker()` | Wiring | REMOVE |
| `auction_service.go: bnrStrikeChecker` field | Wiring | REMOVE |
| `auction_service.go: PlaceBid()` BNR check | Consumer | REPLACE with restriction check |

### `BNRDecayWorker`

| Location | Classification | Action |
|---|---|---|
| `bnr_decay_worker.go` | Active authority | REMOVE entirely |
| `bnr_decay_worker_test.go` | Test | REMOVE |
| `dependencies.go` wiring | Wiring | REMOVE |

### `BNRAdminResetter`

| Location | Classification | Action |
|---|---|---|
| `bnr_admin_reset.go` | Active authority | REMOVE entirely |
| `bnr_admin_reset_test.go` | Test | REMOVE |
| `dependencies.go` wiring | Wiring | REMOVE |

### `/claim` and `/claim-token`

| Location | Classification | Action |
|---|---|---|
| `auction_handler.go: ClaimAuction()` | Active authority | REPLACE with `/resolve-shipping` |
| `auction_handler.go: GeneratePricingTokenForClaim()` | Active authority | REMOVE |
| `routes_core.go: POST /:id/claim` | Wiring | REPLACE |
| `routes_core.go: POST /:id/claim-token` | Wiring | REMOVE |
| `auction_claim_gate_test.go` | Test | UPDATE |
| `auction_claim_error_test.go` | Test | UPDATE |
| Mobile `auction_remote_datasource.dart` | Mobile consumer | UPDATE |
| Mobile `auction_repository_impl.dart` | Mobile consumer | UPDATE |
| Mobile `auction_contract_p1_test.dart` | Mobile test | UPDATE |

### `TransitionToExpiredBNR`

| Location | Classification | Action |
|---|---|---|
| `auction.go:409-412` | Active authority | REMOVE |
| `auction_settlement_worker.go:320` | Consumer | REMOVE |
| All test references | Tests | UPDATE |

---

## 20. Remaining Owner Decisions

### Decision 1: Quote Arrival vs Buyer Deadline (OWNER DECISION REQUIRED)

**Question:** When the seller provides a private quote at T+23h (1 hour before the deadline), does the buyer's acceptance deadline extend beyond `auction_end + 24h`?

**Current locked truth:** "buyer has 24 hours from auction end to resolve shipping." No extension clause.

**Safe default (recommended):** NO extension. The buyer's deadline is always `auction_end + 24h` regardless of when the seller provides the quote. This keeps the deadline fixed and predictable.

**If extension is desired:** The implementation would need to track `quote_created_at` and set the buyer's deadline to `quote_created_at + N hours` where N is the extension window. This adds complexity and a moving deadline.

### Decision 2: Case B Seller Optional Quote Timing

**Question:** Can the seller provide an optional special quote at any time during the 24h window, or only before the buyer selects normal shipping?

**Current locked truth:** "A seller may still choose to provide a transaction-specific private quote even when the destination is covered."

**Safe default:** Seller may create a quote at any time while `shipping_resolved_at IS NULL`. If the buyer has already resolved shipping via normal selection, the quote is moot (shipping already resolved).

---

## 21. Final Implementation Gate

### Checklist

- [x] State semantics unambiguous: `WAITING_SETTLEMENT` = winner known, settlement incomplete. Failure → DRAFT. Success → ENDED.
- [x] Deadline ownership unambiguous: `end_at + 24h` for both seller and buyer. `settlement_deadline` removed.
- [x] Private quote lifecycle unambiguous: Quote creation ≠ resolution. Acceptance = resolution. Deadline = `end_at + 24h`.
- [x] Shipping resolution authority singular: `auction_shipping_resolution` with `UNIQUE(auction_id, attempt_number)`.
- [x] History/attempt semantics correct: One resolution row per attempt. `shipping_resolved_at` is current-attempt marker.
- [x] `settlement_deadline` verdict: REMOVE. Compute as `end_at + INTERVAL '24 hours'`.
- [x] Payment phase proven: Auction stays in `waiting_settlement` until payment success (→ ENDED) or expiry (→ DRAFT).
- [x] BNR rollback boundary proven: Full transaction boundary specified (Section 14).
- [x] Migration semantics proven: `expired_bnr` → DRAFT, `buyer_bnr_strikes` → `commerce_violations`.
- [x] No unresolved technical authority duplication.

### **VERDICT: `READY_FOR_FINAL_IMPLEMENTATION_PLAN`**

> This document is an audit report only. No code has been changed.
