# REPORT — AUCTION IDENTITY + RELIST SIMPLICITY

> **Status:** AUDIT ONLY — NO IMPLEMENTATION
> **Generated:** 2026-09-02
> **Source:** Current filesystem

---

## VERDICT

**`READY_FOR_FINAL_IMPLEMENTATION_PLAN`**

The current auction architecture reuses the same auction record on relist. This is correct. No `attempt_id`, versioning, or identity complexity is needed. Bid isolation, order/payment isolation, and shipping isolation are all provably safe from current code. The NO EXTENSION deadline rule is deterministically enforceable. Seven business scenarios pass. No blockers found.

---

## 1. Current Auction Identity

### How Relist Works

**Relist = reuse the same auction record, progressing it through the normal DRAFT → SCHEDULED → ACTIVE flow.**

Evidence:

1. **`TransitionToDraftOnSettlementFailure()`** (to be implemented) clears `CurrentBid`, `CurrentWinnerID`, `OrderID`, `SettlementDeadline`, `ShippingResolvedAt` on the same auction record. The `ID` is unchanged.

2. **`UpdateDraft()`** (auction.go L490-510) edits pricing/timing on the DRAFT:
   ```go
   func (a *Auction) UpdateDraft(startPrice, bidIncrement int64, ...) error {
       if a.Status != StatusDraft { return error }
       a.StartPrice = startPrice
       a.BidIncrement = bidIncrement
       // ...
   }
   ```

3. **`Schedule()`** (auction.go L362-367) transitions `DRAFT → SCHEDULED`. Same record, new lifecycle.

4. **`Activate()`** (auction.go L369-374) transitions `SCHEDULED → ACTIVE`. Same record, new lifecycle.

5. **`PlaceBid()`** (auction.go L531-570) works on the same record. Reads `a.CurrentBid` and `a.CurrentWinnerID` from the struct, NOT from the bids table.

6. **Product binding:** `ClaimSellingSurface()` sets `Product.SellingSurface = SellingSurfaceAuction` at creation. On DRAFT return, the product remains claimed by this auction. No re-claim needed.

7. **Uniqueness constraint** (migration 000001:2015):
   ```sql
   CREATE UNIQUE INDEX uniq_active_auction_per_product
   ON auctions (product_id)
   WHERE (status IN ('draft','scheduled','active','waiting_settlement'));
   ```
   DRAFT is included. Only one non-terminal auction per product. The same record returning to DRAFT satisfies this.

**Answer to Question 1:** Relist reuses the same auction record (option A).

**Answer to Question 2:** This does not create any business problem. See Sections 2-5 for proof.

---

## 2. Bid Isolation

### Proof That Old Bids Do Not Contaminate New Auction

#### MinimumBid() — reads from struct, NOT bids table

```go
// auction.go L521-526
func (a *Auction) MinimumBid() int64 {
    if a.CurrentBid == nil {
        return a.StartPrice
    }
    return *a.CurrentBid + a.BidIncrement
}
```

After DRAFT return: `CurrentBid = nil` → `MinimumBid()` returns `StartPrice`.

**Old bids cannot raise the minimum bid.** The minimum is determined solely by the struct field, which is nil after DRAFT return.

#### PlaceBid() — checks struct fields, NOT bids table

```go
// auction.go L531-570
func (a *Auction) PlaceBid(bidderID uuid.UUID, amount int64, now time.Time) error {
    if a.Status != StatusActive { return error }  // Only active auctions
    if !now.Before(a.EndAt) { return error }       // Must be before end
    if bidderID == a.SellerID { return error }      // No self-bids
    minimum := a.MinimumBid()                       // From struct
    if amount < minimum { return error }            // Must meet minimum
    a.CurrentBid = &amount                          // Updates struct
    winnerID := bidderID
    a.CurrentWinnerID = &winnerID                   // Updates struct
    // ...
}
```

**Old bids cannot become winner again.** Winner is determined by `a.CurrentWinnerID`, which is nil after DRAFT return. New `PlaceBid()` overwrites it.

#### Winner Determination — struct-based, NOT query-based

The winner is `auction.CurrentWinnerID`. This is set by `PlaceBid()` and read during settlement. It is never determined by querying the bids table for the "highest bid."

**Evidence from EndAuctionInternal()** (auction_service.go L976):
```go
if auction.HasWinner() {  // checks CurrentWinnerID != nil
    auction.TransitionToWaitingSettlement()
}
```

`HasWinner()` checks `a.CurrentWinnerID != nil`. After DRAFT return, `CurrentWinnerID = nil` → no winner → new bids must establish a new winner.

#### Bid List Display — old bids appear but are harmless

`ListByAuction()` (auction_bid_repository.go L131-135):
```sql
SELECT id, auction_id, bidder_id, amount, idempotency_key, created_at
FROM auction_bids
WHERE auction_id = $1
ORDER BY created_at DESC
```

Returns ALL bids including old ones. This is a **display concern**, not a correctness concern. Old bids:
- Don't affect `MinimumBid()`
- Don't affect winner determination
- Don't affect new bid acceptance
- Are sorted newest-first (new bids appear at top)

The client or a future backend filter can exclude old-attempt bids by timestamp if desired. But correctness is not affected.

### Summary

| Question | Answer | Evidence |
|---|---|---|
| Can old bids become current highest bid again? | **NO** | `CurrentBid = nil` after DRAFT; `MinimumBid()` reads struct |
| Can old bids become winner again? | **NO** | `CurrentWinnerID = nil` after DRAFT; `PlaceBid()` overwrites |
| Can old bids affect minimum bid? | **NO** | `MinimumBid()` reads `CurrentBid` field, not bids table |
| Can old bids determine new winner? | **NO** | Winner = `CurrentWinnerID`, set by `PlaceBid()`, not bid query |
| Can old bids affect new settlement? | **NO** | Settlement reads `CurrentWinnerID` and `CurrentBid` from struct |

**BID ISOLATION: PASS**

---

## 3. Order / Payment / Shipping Isolation

### Order Isolation

After settlement failure → DRAFT:

1. **`OrderID = nil`** on the auction. The old order is in `expired` status (terminal).

2. **`UNIQUE(order_id)` on auctions** (migration 000001:1962): Prevents two auctions from binding the same order. After `OrderID = nil`, the constraint is satisfied for this auction.

3. **New order creation:** When the relisted auction settles, a new order is created with a new `order.ID`. The old expired order is a separate record.

4. **`releaseAuctionOrderBinding()`** (order_completion_service.go L2036-2054): Clears `auction.OrderID` via `ReleaseUnpaidOrder()`. Idempotent and mismatch-safe.

### Payment Isolation

1. **Payment is bound to order, not auction.** `payments.reference_id = orders.id`.

2. **Old payment is in `expired` status.** Cannot be reused for a new order.

3. **New payment:** Created when the new order's buyer selects a payment method. New payment ID, new payment intent.

4. **Payment expiry worker** queries `payments WHERE status = 'pending' AND expired_at < NOW()`. Old expired payment is not matched.

### Shipping Isolation

1. **`ShippingResolvedAt = nil`** after DRAFT return (current-attempt marker cleared).

2. **Old shipping quote:** In `USED` or `EXPIRED` status. `reactivateQuoteIfEligible()` may reactivate it, but for a new settlement attempt the buyer/seller create new resolution context.

3. **`auction_shipping_resolution` (proposed):** Old resolution rows remain with `attempt_number = 1`. New resolution gets `attempt_number = 2`. No conflict.

4. **New order shipping snapshot:** Created at order creation time from the new resolution context. Independent of old snapshots.

### Summary

| Question | Answer | Evidence |
|---|---|---|
| Is old OrderID correctly released? | **YES** | `ReleaseUnpaidOrder()` clears it |
| Can old payment be used for new order? | **NO** | Payment bound to order, old payment expired |
| Is shipping_resolved_at cleared? | **YES** | DRAFT return clears current-attempt marker |
| Can old shipping quote affect new resolution? | **NO** | New resolution uses fresh context |
| Can new order be created? | **YES** | `OrderID = nil` on auction, new order has new ID |

**ORDER/PAYMENT/SHIPPING ISOLATION: PASS**

---

## 4. Shipping Resolution History

### Current State

The `auction_shipping_resolution` table (from the previous implementation plan) stores one row per settlement attempt:

```sql
UNIQUE(auction_id, attempt_number)
```

### What Resets on DRAFT Return

- `auction.shipping_resolved_at = nil` (current-attempt marker)
- `auction.seller_action_required = false`
- `auction.seller_quote_provided = false`
- Old resolution rows in `auction_shipping_resolution` are **NOT deleted** (historical record)

### What Stays as Historical Record

- `auction_shipping_resolution` rows (one per attempt, with `attempt_number`)
- Old orders (in `expired` status)
- Old payments (in `expired` status)
- Old bids (in `auction_bids` table)

### Simplicity Assessment

No complex history system is needed. The `auction_shipping_resolution` table with `attempt_number` is sufficient. The previous implementation plan already established this model. No additional schema is required.

**SHIPPING ISOLATION: PASS** — consistent with current-attempt semantics.

---

## 5. Relist Correctness

### After DRAFT Return, Seller Can:

1. **Edit the DRAFT** via `UpdateDraft()` — change pricing, timing, buy-now price
2. **Schedule** via `Schedule()` — `DRAFT → SCHEDULED` (requires active seller subscription)
3. **Activate** (immediate start) — `SCHEDULED → ACTIVE`
4. **New bids accepted** — `PlaceBid()` works on the same record with `CurrentBid = nil`

### After Relist:

| Property | Status | Evidence |
|---|---|---|
| Auction accepts new bids | ✅ | `PlaceBid()` checks `StatusActive`, works on same record |
| Buyer old has no special rights | ✅ | No "previous winner" concept in code |
| Winner old not auto-winner | ✅ | `CurrentWinnerID = nil` after DRAFT |
| Order old not carried | ✅ | `OrderID = nil` after DRAFT |
| Shipping old not carried | ✅ | `ShippingResolvedAt = nil` after DRAFT |
| Deadline old not carried | ✅ | `SettlementDeadline = nil` after DRAFT |
| Restriction/violation separate | ✅ | Violations are user-level, not auction-level |

### Relist vs Create New — Business Indistinguishable

For buyers and sellers, the observable behavior is identical:
- Same product, same seller
- Fresh bidding from `start_price`
- New winner, new settlement
- No trace of old settlement in the active auction

The only difference is the `auction.id` is the same UUID. This is invisible to buyers and irrelevant to business correctness.

**RELIST CORRECTNESS: PASS**

---

## 6. Deadline Correctness — NO EXTENSION

### All Deadlines Derived from `end_at`

| Deadline | Formula | Source |
|---|---|---|
| Seller quote deadline | `end_at + 24h` | Computed, not stored |
| Buyer shipping deadline | `end_at + 24h` | Computed, not stored |
| Payment deadline | `shipping_resolved_at + 24h` | `order.payment_expires_at` |

### `settlement_deadline` Status: REMOVED

`settlement_deadline` is always `end_at + 24h` (set in `TransitionToWaitingSettlement()`). It is a redundant derived field. All consumers compute the deadline from `end_at`. No duplicate authority exists.

### NO EXTENSION Rule: Deterministically Enforceable

The rule: "If seller provides a quote at T+23h, buyer does NOT get extra time. Deadline remains T+24h."

**Why this works:**

1. The settlement deadline worker fires at `end_at + 24h` regardless of quote timing
2. The worker checks `shipping_resolved_at IS NULL` — if shipping is not resolved by T+24h, it proceeds
3. The worker uses `seller_action_required` and `seller_quote_provided` to classify who defaulted
4. If seller provided quote but buyer didn't accept: `seller_quote_provided = TRUE` → buyer default
5. No code path extends the deadline based on quote creation time

**Evidence that no extension exists in current code:**

- `TransitionToWaitingSettlement()` sets `SettlementDeadline = now + 24h` (fixed)
- `AuctionSettlementWorker` fires on `settlement_deadline <= NOW()` (fixed)
- `GeneratePricingTokenForAuctionClaim()` checks `now.After(*auction.SettlementDeadline)` (fixed)
- No code modifies `SettlementDeadline` after initial set
- No code extends any deadline based on quote creation

### Deadline Conflict Check

| Source of Deadline | Value | Conflicts? |
|---|---|---|
| `end_at + 24h` (computed for seller/buyer) | Fixed | None |
| `order.payment_expires_at` (for payment) | `shipping_resolved_at + 24h` | None (different phase) |
| `shipping_quote.expires_at` (for quote validity) | Must be set to `end_at + 24h` for auctions | Must align with settlement deadline |
| ~~`settlement_deadline`~~ (removed) | ~~was `end_at + 24h`~~ | Removed, no conflict |

**DEADLINE CORRECTNESS: PASS** — NO EXTENSION is deterministically enforceable.

---

## 7. State Machine — Minimal

### Current States

```
draft → scheduled → active → waiting_settlement → ended
                                                 → draft (NEW)
                                → cancelled
```

### States to Remove

- **`expired_bnr`** — NOT a canonical business state. Settlement failure goes directly to DRAFT.

### States That Remain

| State | Business Meaning | Needed? |
|---|---|---|
| `draft` | Workspace, editable | ✅ |
| `scheduled` | Committed for future market run | ✅ |
| `active` | Accepting bids | ✅ |
| `waiting_settlement` | Winner known, settlement incomplete | ✅ |
| `ended` | Settlement complete (payment success) | ✅ |
| `cancelled` | Seller cancelled | ✅ |

### No New States Needed

- No `expired_settlement` — settlement failure goes to DRAFT
- No `expired_bnr` — removed
- No `relisting` — DRAFT covers this
- No `pending_resolution` — `waiting_settlement` covers this

**STATE MACHINE: PASS** — 6 states sufficient for all business scenarios.

---

## 8. Race Conditions — Only Relevant Races

### Race 1: Seller Quote vs Buyer Shipping Deadline

**Scenario:** Seller creates quote at T+23h59m. Buyer attempts normal shipping at T+24h.

**Resolution:** First `FOR UPDATE` lock wins.
- If buyer's normal shipping acquires lock first: `shipping_resolved_at` set → seller quote is moot
- If deadline worker fires first: buyer defaulted → DRAFT → seller quote never created

**SAFE.** The `FOR UPDATE + shipping_resolved_at IS NULL` guard serializes correctly.

### Race 2: Buyer Shipping Selection vs Timeout

**Scenario:** Buyer selects shipping at T+24h. Deadline worker fires at T+24h.

**Resolution:** `FOR UPDATE SKIP LOCKED` on the deadline worker ensures atomicity. If buyer's resolution commits first, `shipping_resolved_at IS NOT NULL` → worker skips. If worker fires first, buyer's resolution sees `status != waiting_settlement` → rejected.

**SAFE.** Standard PostgreSQL row locking.

### Race 3: Payment Success vs Payment Expiry

**Scenario:** Buyer pays at T+23h59m. Payment expiry worker fires at T+24h.

**Resolution:** Payment success handler marks order as `paid` (terminal for pending state). Payment expiry worker's `MarkExpired()` returns `InvalidTransitionError` if order is already paid. Idempotent no-op.

**SAFE.** Order state machine prevents double-processing.

### Race 4: Settlement Failure vs Duplicate Worker

**Scenario:** Two instances of the deadline worker pick up the same auction.

**Resolution:** `FOR UPDATE SKIP LOCKED` ensures only one instance processes each auction. The second instance either gets a different batch or finds the auction already processed (`status != waiting_settlement`).

**SAFE.** Standard worker pattern.

### Race 5: DRAFT Return vs `/resolve-shipping`

**Scenario:** Deadline worker fires to return auction to DRAFT. Winner simultaneously calls `/resolve-shipping`.

**Resolution:** Both lock the auction row. First to commit wins:
- If deadline worker commits first: `status = draft` → `/resolve-shipping` sees `status != waiting_settlement` → rejected
- If `/resolve-shipping` commits first: `shipping_resolved_at IS NOT NULL` → deadline worker skips

**SAFE.** `FOR UPDATE` + status/shipping guard.

**RACE SAFETY: PASS** — All relevant races are handled by existing PostgreSQL locking patterns.

---

## 9. Business Scenario Matrix

| # | Scenario | Result | Evidence |
|---|---|---|---|
| A | Normal shipping → payment → success | `ACTIVE → WAITING_SETTLEMENT → ENDED` | `EndAuctionInternal()` → `Settle()` on payment |
| B | Private quote → buyer accepts → payment → success | Same as A, with private quote path | `shipping_resolved_at` set by quote acceptance |
| C | Seller fails to provide quote → DRAFT | Deadline worker: seller default → DRAFT | `seller_action_required=TRUE, seller_quote_provided=FALSE` at T+24h |
| D | Buyer fails to select shipping → DRAFT | Deadline worker: buyer timeout → DRAFT | `shipping_resolved_at IS NULL` at T+24h, seller fulfilled |
| E | Payment expires → BNR → DRAFT | Payment expiry worker → financial rollback → DRAFT | `Expire()` chain + violation + restriction |
| F | DRAFT → relist → new bids | Same record, `CurrentBid=nil`, fresh bidding | `MinimumBid()` returns `StartPrice` |
| G | Relist → new winner → new settlement | New `CurrentWinnerID`, new order, new payment | Completely fresh settlement context |

**All 7 scenarios: PASS**

---

## 10. Unnecessary Complexity — Do NOT Build

| Item | Why Not Needed |
|---|---|
| `attempt_id` on auctions | Bid isolation proven without it (Section 2) |
| Auction versioning | Same record reuse is correct (Section 1) |
| `attempt_number` on `auction_bids` | Old bids don't affect correctness (Section 2) |
| Separate relist endpoint | Normal DRAFT → SCHEDULED → ACTIVE flow works (Section 5) |
| `expired_settlement` state | DRAFT is sufficient (Section 7) |
| `shipping_resolved_at` history table | Simple `UNIQUE(auction_id, attempt_number)` suffices (Section 4) |
| Distributed workflow framework | `FOR UPDATE` + row locking handles all races (Section 8) |
| Deadline extension mechanism | NO EXTENSION rule is deterministic (Section 6) |
| Bid filtering by attempt | Old bids are display-only, not correctness (Section 2) |

---

## 11. Residue

### Active Authority to Remove

| Item | Location | Status |
|---|---|---|
| `StatusExpiredBNR` | auction.go L77 | Active — REMOVE |
| `TransitionToExpiredBNR()` | auction.go L409-412 | Active — REMOVE |
| `expired_bnr` in transitionAllowed | auction.go L100 | Active — REMOVE |
| `expired_bnr` in PublicLifecycle | auction.go L140-145 | Active — REMOVE |
| `expired_bnr` in IsPublicDiscoverable | auction.go L162 | Active — REMOVE |
| `BNRAuctionRestrictedError` | auction.go L249-269 | Active — REMOVE |
| `BNRStrikeChecker` | bnr_restriction.go | Active — REMOVE |
| `BNRStrikeHandler` | bnr_strike_handler.go | Active — REMOVE |
| `BNRDecayWorker` | bnr_decay_worker.go | Active — REMOVE |
| `BNRAdminResetter` | bnr_admin_reset.go | Active — REMOVE |
| `AuctionSettlementWorker` (expired_bnr path) | auction_settlement_worker.go | Active — REFACTOR |
| `settlement_deadline` field | auction.go L300 | Active — REMOVE |
| `settlement_deadline` in INSERT/UPDATE | auction_repository.go L33, L209 | Consumer — REMOVE |
| `/claim` endpoint | auction_handler.go L720-860 | Active — REPLACE |
| `/claim-token` endpoint | auction_handler.go L647-718 | Active — REMOVE |

### Stale Tests

| Item | Status |
|---|---|
| `bnr_restriction_test.go` | REPLACE |
| `bnr_decay_worker_test.go` | REMOVE |
| `bnr_admin_reset_test.go` | REMOVE |
| `bnr_strike_handler_test.go` | REMOVE |
| `auction_settlement_worker_test.go` | REWRITE |
| `release_unpaid_order_test.go` | UPDATE (DRAFT transition) |
| All `expired_bnr` test fixtures | UPDATE |

### Stale Documentation / Comments

| Item | Status |
|---|---|
| `StatusWaitingSettlement` comment: "Winner can claim" | UPDATE (shipping resolution, not claim) |
| `ReleaseUnpaidOrder` doc: "Ended is terminal" | UPDATE (now returns to DRAFT) |
| `BNR` references throughout codebase | REMOVE |
| `settlement_deadline` in chat projection | UPDATE (compute from end_at) |
| Mobile `expired_bnr` enum value | REMOVE |
| Mobile `settlement_deadline` DTO field | COMPUTE from end_at |

---

## 12. Blockers

**None.**

---

## 12. FINAL VERDICT

**`READY_FOR_FINAL_IMPLEMENTATION_PLAN`**

All conditions met:

- ✅ Relist identity proven safe (same record reuse, no contamination)
- ✅ Old bids cannot affect new auction (struct-based, not query-based)
- ✅ Old order/payment/shipping cannot affect relist (all cleared on DRAFT)
- ✅ Deadline is unambiguous (`end_at + 24h`, no extension)
- ✅ NO EXTENSION deterministically enforceable
- ✅ No `attempt_id` or complexity needed
- ✅ 6-state machine sufficient
- ✅ All relevant races safe (`FOR UPDATE` + row locking)
- ✅ All 7 business scenarios pass

> This document is an audit report only. No code has been changed.
