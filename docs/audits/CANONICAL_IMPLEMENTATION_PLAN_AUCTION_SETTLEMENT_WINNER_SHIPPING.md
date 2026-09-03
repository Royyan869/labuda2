# CANONICAL IMPLEMENTATION PLAN — AUCTION SETTLEMENT + WINNER SHIPPING

> **Status:** PLANNING ONLY — No code has been changed.
> **Generated:** 2026-09-02
> **Source of Truth:** Current filesystem (Git history is backup only)

---

## A. Executive Summary

This document is the canonical implementation plan for the Auction Settlement + Winner Shipping redesign. It is based exclusively on the current filesystem as audited on 2026-09-02.

**Current reality (P0 gaps):**

1. **No `shipping_resolved_at` exists** anywhere in code or schema. The entire auction settlement flow bundles address submission, shipping selection, pricing token generation, and order creation into a single `/claim` endpoint. There is no separate shipping resolution step.

2. **Payment expiry is wrong for auctions.** `calculatePaymentExpiry("default", createdAt)` returns 30 minutes. Business truth requires `shipping_resolved_at + 24h`.

3. **`expired_bnr` is a terminal state.** The state machine has no path from `WAITING_SETTLEMENT → DRAFT`. The `AuctionSettlementWorker` transitions to `expired_bnr` (terminal) and emits `auction_bnr_detected`, but does NOT record a commerce violation, does NOT apply a restriction, and does NOT return the auction to `DRAFT`.

4. **`buyer_bnr_strikes` is the only violation/restriction authority.** It uses a bespoke strike-count ladder (0→1→2→3→4+ with decay and admin reset). `commerce_violations` and `commerce_restrictions` tables do not exist. The owner-locked business truth requires these new canonical tables with EXTEND stacking, no decay, no admin reset.

5. **No seller deadline worker exists.** The `AuctionSettlementWorker` only checks `settlement_deadline` (24h from auction end). It treats ALL expiry as "buyer BNR." There is no distinction between seller obligation (private quote) and buyer obligation (normal shipping selection). There is no seller-specific deadline enforcement.

6. **The `/claim` endpoint is a monolith.** It bundles winner validation, pricing token generation, order creation, and auction settlement into a single atomic operation. It does NOT separate destination resolution, shipping resolution, and payment readiness.

**What must change:**

- New `shipping_resolved_at` column on `auctions`
- New `auction_shipping_resolution` table for immutable shipping snapshots
- New `commerce_violations` and `commerce_restrictions` tables replacing `buyer_bnr_strikes`
- State machine change: `WAITING_SETTLEMENT → DRAFT` on settlement failure
- Removal of `expired_bnr` as a terminal state
- Payment deadline override for auction orders: `shipping_resolved_at + 24h`
- New seller deadline worker (Case A: outside coverage)
- New buyer shipping deadline worker (Case B: inside coverage)
- Decomposition of `/claim` into separate resolution endpoints
- Complete financial rollback audit and enhancement for BNR paths
- Full cleanup of obsolete `buyer_bnr_strikes`, `expired_bnr`, `settlement_deadline`, and old claim semantics

---

## B. Current-State Authority Map

### B.1 Auction State Machine

**File:** `backend/internal/commerce/auction/entity/auction.go`

```
Current states:  draft → scheduled → active → waiting_settlement → ended | expired_bnr | cancelled
```

| Transition | Exists | Correct per Spec |
|---|---|---|
| `active → waiting_settlement` | ✅ | ✅ |
| `active → ended` (no winner) | ✅ | ✅ |
| `waiting_settlement → ended` (settle) | ✅ | ✅ |
| `waiting_settlement → expired_bnr` | ✅ | ❌ Terminal state; must be `→ DRAFT` |
| `waiting_settlement → draft` | ❌ | ✅ Required |

**`SettlementDeadline`:** Set to `now + 24h` on `TransitionToWaitingSettlement()`. Currently used by `AuctionSettlementWorker` as a monolithic "settlement expired" trigger.

**`OrderID`:** Set atomically when order is created via `/claim`. Once set, prevents double settlement. `ReleaseUnpaidOrder()` clears it on unpaid expiry but auction stays `ended` (terminal).

### B.2 Current /claim Flow

**File:** `backend/internal/commerce/auction/delivery/http/auction_handler.go`

```
POST /api/v1/auctions/:id/claim
  Body: { address_id, shipping_option_id, discount_code?, use_coins? }

  1. Validate winner + deadline + not-settled (locks auction FOR UPDATE)
  2. Generate pricing token (same tx)
  3. Validate and lock token (same tx)
  4. Build pricing snapshot from token
  5. Create order from auction (same tx)
  6. Finalize token consumption
  7. Set auction.OrderID = order.ID, auction.Settle() → ended
```

**Problems:**
- No separate shipping resolution step
- No `shipping_resolved_at` marker
- Payment expiry calculated as `calculatePaymentExpiry("default", now)` = 30 minutes
- Seller private quote path not distinguished from normal shipping
- Winner's address is submitted at claim time, not resolved from primary address

### B.3 Payment Expiry Authority

**File:** `backend/internal/commerce/order/application/order_creation_service.go`

```go
func calculatePaymentExpiry(paymentMethod string, createdAt time.Time) time.Time {
    switch paymentMethod {
    case "instant": return createdAt.Add(15 * time.Minute)
    case "va":      return createdAt.Add(1 * time.Hour)
    case "retail":  return createdAt.Add(6 * time.Hour)
    default:        return createdAt.Add(30 * time.Minute)
    }
}
```

Auction orders use `PaymentMethod = "default"` → 30 minutes. Business truth requires `shipping_resolved_at + 24h`.

**Payment Expiry Worker:** `backend/internal/worker/payment_expiry_worker.go` — queries `payments WHERE status = 'pending' AND expired_at < NOW()`, calls `orderService.Expire()` which does:
1. `order.MarkExpired()` (pending → expired)
2. `restoreForSaleStock()` (for auctions: `releaseAuctionOrderBinding()` clears OrderID)
3. Release escrow + gateway refund
4. Refund coins
5. Reactivate shipping quote

**Critical:** The `Expire()` chain does NOT interact with the auction state machine. After `ReleaseUnpaidOrder()`, the auction remains `ended` (terminal). Under the new design, settlement failure must return the auction to `DRAFT`.

### B.4 BNR / Violation / Restriction System

**Current `buyer_bnr_strikes` table** (migration 000001):

```sql
CREATE TABLE buyer_bnr_strikes (
    id uuid PRIMARY KEY,
    buyer_id uuid REFERENCES users(id),
    auction_id uuid REFERENCES auctions(id) UNIQUE,
    struck_at timestamptz DEFAULT NOW(),
    decayed_at timestamptz,
    admin_reset boolean DEFAULT FALSE
);
```

**`BNRStrikeChecker`** (`backend/internal/commerce/auction/application/bnr_restriction.go`):
- 0 strikes → allow
- 1 strike → allow + warning
- 2 strikes → deny if `last_struck_at + 14d > now` (then allow again)
- 3 strikes → deny if `last_struck_at + 90d > now` (then allow again)
- 4+ → permanent ban

**`BNRDecayWorker`** (`backend/internal/worker/bnr_decay_worker.go`):
- If most recent active strike > 180 days old, decay the oldest strike
- Runs daily

**`BNRAdminResetter`** (`backend/internal/worker/bnr_admin_reset.go`):
- Sets `admin_reset = TRUE` on active strikes
- `ResetAllForBuyer()` and `ResetStrike()` methods

**`AuctionSettlementWorker`** (`backend/internal/worker/auction_settlement_worker.go`):
- Finds `waiting_settlement` auctions with `settlement_deadline <= NOW()`
- Transitions to `expired_bnr` (terminal)
- Emits `auction_bnr_detected` outbox event

**`BNRStrikeHandler`** (`backend/internal/worker/bnr_strike_handler.go`):
- Handles `auction_bnr_detected` events
- Inserts row in `buyer_bnr_strikes` (ON CONFLICT DO NOTHING)

**Business truth requires:**
- `commerce_violations` table (replaces `buyer_bnr_strikes`)
- `commerce_restrictions` table (new)
- Restriction ladder: 1st → 7d, 2nd → 15d, 3rd+ → 30d
- EXTEND stacking (not `now + N`)
- No decay, no admin reset, no permanent ban
- Buyer restriction → cannot transact For Sale + Auction
- Seller restriction → cannot sell For Sale + Auction

### B.5 Winner Address Authority

**File:** `backend/internal/identity/address/entity/address.go`

- `Address` entity with `IsPrimary` flag, `IsAvailableForCheckout` flag
- Purpose: `"shipping"` (buyer) or `"sender"` (seller)
- `AddressSnapshot` for immutable order storage

**Retrieval methods:**
- `AddressService.GetAddressForCheckout(ctx, tx, userID, addressID)` — validates ownership + availability
- `AddressRepositoryImpl.GetPrimaryByUserID(ctx, tx, userID)` — returns primary address
- `AddressRepositoryImpl.GetPrimaryByUserIDFiltered(ctx, tx, userID, purpose)` — filtered by purpose

**Current claim flow:** Winner explicitly submits `address_id` in the `/claim` request body. The winner's primary address is NOT automatically resolved.

**Required:** Winner destination derived from winner's primary shipping address at settlement/shipping-resolution time.

### B.6 Shipping Domain

**Shipping Setup** (`backend/internal/commerce/shipping/entity/shipping_setup.go`): Reusable shipping option with coverage areas.

**Shipping Coverage** (`backend/internal/commerce/shipping/entity/shipping_coverage.go`): Province/city coverage for a shipping setup. `IsAvailable` flag.

**Shipping Quote** (`backend/internal/commerce/shipping/quote/entity/shipping_quote.go`): Manual seller-provided quote via chat.
- Statuses: ACTIVE → USED / EXPIRED / INVALID
- Has `ExpiresAt` (default 24h), `ReactivationCount`, `MaxReuse`
- Tied to: chat_id, product_id, source_type/source_id, seller_id, buyer_id
- Address locked at creation: `DestinationProvinceID`, `DestinationCityID`

**Shipping Service** (`backend/internal/commerce/shipping/application/shipping_service.go`):
- `CheckDeliveryAvailabilityForProduct()` — checks if shipping setup covers an address
- `ValidateSellableCreateShippingSelection()` — validates shipping options at auction creation

**Current coverage check:** At auction creation time, `ensureShippingCoverage()` verifies at least one shipping option has at least one active coverage row. At claim time, `CheckDeliveryAvailabilityForProduct()` checks if the selected option covers the buyer's address.

### B.7 Database Schema (Auctions)

**Table:** `auctions` (migration 000001)

```sql
CREATE TABLE auctions (
    id uuid PRIMARY KEY,
    seller_id uuid NOT NULL,
    listing_id uuid,          -- deprecated
    order_id uuid,
    settlement_deadline timestamptz,
    title text NOT NULL,      -- deprecated (now on products)
    description text NOT NULL, -- deprecated
    preparation_time preparation_time_enum,
    preparation_note text,    -- deprecated
    start_price bigint NOT NULL,
    bid_increment bigint NOT NULL,
    buy_now_price bigint,
    start_at timestamptz NOT NULL,
    end_at timestamptz NOT NULL,
    current_bid bigint,
    current_winner_id uuid,
    status auction_status_enum DEFAULT 'draft',
    created_at timestamptz DEFAULT NOW(),
    updated_at timestamptz DEFAULT NOW(),
    product_id uuid NOT NULL
);
```

**Missing columns:**
- `shipping_resolved_at timestamptz` — does NOT exist
- No auction-level shipping destination snapshot
- No shipping resolution mode (normal vs private quote)

**`settlement_deadline`:** Currently set to `now + 24h` on `TransitionToWaitingSettlement()`. Used by `AuctionSettlementWorker`. Must be repurposed or supplemented for the new dual-deadline system (seller deadline + buyer deadline).

---

## C. Canonical Target Architecture

### C.1 High-Level Flow

```
1. Auction ends (active → waiting_settlement)
2. Winner identified (existing: CurrentWinnerID)
3. System determines shipping requirement:
   a. Resolve winner destination from primary shipping address
   b. Check if destination is covered by auction's selected Shipping Setups
   c. If COVERED → Case B (buyer may select normal shipping OR seller may provide private quote)
   d. If NOT COVERED → Case A (seller MUST provide private quote)
4. Seller action if applicable (Case A: must provide quote within 24h)
5. Buyer action if applicable (Case B: must select shipping within 24h)
6. Shipping resolution (atomic, first-resolution-wins):
   a. shipping_resolved_at = NOW()
   b. Immutable shipping snapshot stored in auction_shipping_resolution
7. Order creation (after shipping resolved)
8. Payment deadline = shipping_resolved_at + 24h
9. Payment processing
10. Settlement success → ended, or failure → violation/restriction + DRAFT
```

### C.2 New State Machine

```
draft → scheduled → active → waiting_settlement → ended (settled)
                                                  → draft (settlement failure)
                                → cancelled
```

Key changes:
- Remove `expired_bnr` as a terminal state
- Add `waiting_settlement → draft` transition
- Failure reason lives in event/reason/audit mechanism, not in auction state

### C.3 New Tables

| Table | Purpose |
|---|---|
| `commerce_violations` | Immutable violation history |
| `commerce_restrictions` | Active restriction per user |
| `auction_shipping_resolution` | Immutable shipping snapshot per auction |

### C.4 Modified Tables

| Table | Changes |
|---|---|
| `auctions` | Add `shipping_resolved_at timestamptz`, remove `expired_bnr` from enum, add `waiting_settlement → draft` transition |

---

## D. State Machine Changes

### D.1 Current State Enum

```go
const (
    StatusDraft             Status = "draft"
    StatusScheduled         Status = "scheduled"
    StatusActive            Status = "active"
    StatusWaitingSettlement Status = "waiting_settlement"
    StatusExpiredBNR        Status = "expired_bnr"     // REMOVE
    StatusEnded             Status = "ended"
    StatusCancelled         Status = "cancelled"
)
```

### D.2 Target State Enum

```go
const (
    StatusDraft             Status = "draft"
    StatusScheduled         Status = "scheduled"
    StatusActive            Status = "active"
    StatusWaitingSettlement Status = "waiting_settlement"
    // StatusExpiredBNR REMOVED — not a canonical auction state
    StatusEnded             Status = "ended"
    StatusCancelled         Status = "cancelled"
)
```

### D.3 Target Transitions

```go
var transitionAllowed = map[Status][]Status{
    StatusDraft:             {StatusScheduled, StatusCancelled},
    StatusScheduled:         {StatusActive, StatusCancelled, StatusDraft},
    StatusActive:            {StatusWaitingSettlement, StatusEnded, StatusCancelled},
    StatusWaitingSettlement: {StatusEnded, StatusDraft, StatusCancelled},
    //                   NEW: ^^^^^^^^ DRAFT on settlement failure
    StatusEnded:             {},  // Terminal
    StatusCancelled:         {},  // Terminal
}
```

### D.4 Transition Logic

**`TransitionToDraft` (settlement failure):**
```go
func (a *Auction) TransitionToDraftOnSettlementFailure() error {
    if !canTransition(a.Status, StatusDraft) {
        return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusDraft}
    }
    a.Status = StatusDraft
    a.OrderID = nil           // Clear any order binding
    a.SettlementDeadline = nil
    a.CurrentWinnerID = nil   // Clear winner — fresh auction
    a.CurrentBid = nil        // Clear bid — fresh auction
    a.UpdatedAt = time.Now()
    return nil
}
```

**Critical invariant:** When returning to `DRAFT`:
- `OrderID` must be nil
- `SettlementDeadline` must be nil
- `CurrentWinnerID` must be nil (so new bids create a new winner)
- `CurrentBid` must be nil (so bidding restarts from `start_price`)
- Historical bids in `auction_bids` table remain intact (never deleted)

### D.5 Migration for State Enum

Migration must:
1. Remove `expired_bnr` from `auction_status_enum` after migrating any existing `expired_bnr` rows
2. Add transition path validation (application-level, not DB constraint)

**Data migration:** All existing `expired_bnr` auctions must be migrated to `draft`:
```sql
UPDATE auctions SET status = 'draft',
    order_id = NULL,
    settlement_deadline = NULL,
    current_winner_id = NULL,
    current_bid = NULL,
    updated_at = NOW()
WHERE status = 'expired_bnr';
```

---

## E. Winner Destination Flow

### E.1 Primary Address Authority

**Where:** `addresses` table, `is_primary = TRUE`, `purpose = 'shipping'`, `is_available_for_checkout = TRUE`

**Retrieval:** `AddressRepositoryImpl.GetPrimaryByUserIDFiltered(ctx, tx, winnerID, "shipping")`

**Winner identity mapping:** `auction.CurrentWinnerID → user_id → primary shipping address`

**Transaction/concurrency:** Address lookup within the same transaction as shipping resolution. No row lock needed on address (read-only snapshot). Address may change after resolution — this is acceptable because the snapshot is frozen at resolution time.

### E.2 Destination Resolution at Settlement Time

When the auction enters `waiting_settlement`:

1. System resolves winner's primary shipping address
2. If no primary shipping address exists → winner must add one before shipping can be resolved
3. System checks coverage of all auction's selected Shipping Setups against the winner's address
4. Determines Case A (outside coverage) or Case B (inside coverage)

### E.3 Immutable Shipping Snapshot

The resolved order MUST contain immutable shipping facts. These are stored in:

1. **`auction_shipping_resolution`** table (new):
   - `auction_id` (FK, unique)
   - `resolved_at` (timestamptz)
   - `resolution_mode` (enum: 'normal_shipping' | 'private_quote')
   - `shipping_setup_id` (nullable, for normal shipping)
   - `shipping_setup_name` (text, snapshot)
   - `shipping_transport_type` (text, snapshot)
   - `shipping_cost` (bigint, snapshot)
   - `shipping_quote_id` (nullable, for private quote)
   - `shipping_quote_price` (nullable bigint, snapshot)
   - `destination_snapshot` (jsonb, AddressSnapshot)
   - `origin_snapshot` (jsonb, AddressSnapshot)

2. **Order entity** (existing, populated at order creation):
   - `ShippingSetupID` (snapshot)
   - `ShippingSetupName` (snapshot)
   - `ShippingTransportType` (snapshot)
   - `ShippingTotal` (snapshot)
   - `ShippingSource` ("for_sale" or "shipping_quote")
   - `ShippingQuoteID` (nullable, snapshot)
   - `ShippingQuotePrice` (nullable, snapshot)
   - `ShippingDestination` (jsonb AddressSnapshot)
   - `ShippingOrigin` (jsonb AddressSnapshot)

---

## F. Case A Flow — Outside Coverage

**Trigger:** Winner's destination is outside all selected Shipping Setups' coverage.

### F.1 Seller Obligation

- Seller MUST provide a private shipping quote
- Obligation starts from auction end
- Deadline: `auction_end + 24h`
- Seller provides quote via the existing shipping quote mechanism (chat-based)
- Quote is transaction-specific (tied to this auction)

### F.2 Timeline

```
T=0:    Auction ends → waiting_settlement
T=0:    System determines Case A (outside coverage)
T=0:    Seller obligation starts
T=24h:  Seller deadline
        - If quote provided: winner may accept
        - If no quote: seller violation + restriction + auction → DRAFT
T>24h:  (only if quote exists) Winner may accept
        - Acceptance sets shipping_resolved_at
        - Order created
        - Payment deadline = shipping_resolved_at + 24h
```

### F.3 Seller Deadline Worker

**New worker: `AuctionSellerDeadlineWorker`**

**Trigger condition:**
```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND shipping_resolved_at IS NULL
  AND no_seller_quote_pending = ???
  AND end_at + 24h <= NOW()
FOR UPDATE SKIP LOCKED
```

Actually, the eligibility query must determine that:
- Auction is in `waiting_settlement`
- `shipping_resolved_at IS NULL`
- The winner's destination is outside coverage (determined at auction end or lazily)
- No valid private quote exists for this auction
- `end_at + 24h <= NOW()`

**Simplification:** Rather than pre-computing Case A vs B at auction end, the seller deadline worker checks: if `shipping_resolved_at IS NULL` AND `end_at + 24h <= NOW()`, determine if seller obligation existed. The simplest approach is:

- At auction end, store a flag `seller_action_required` on the auction (computed from coverage check)
- Seller deadline worker checks `seller_action_required = TRUE AND shipping_resolved_at IS NULL AND end_at + 24h <= NOW()`

### F.4 Failure Path (Atomic)

```
1. Verify deadline: now >= auction.end_at + 24h
2. Lock auction (FOR UPDATE)
3. Verify status = waiting_settlement AND shipping_resolved_at IS NULL
4. Record seller violation (commerce_violations.insert)
5. Extend seller restriction (commerce_restrictions)
6. Transition auction → DRAFT
7. Clear: order_id, settlement_deadline, current_winner_id, current_bid, shipping_resolved_at
8. Emit outbox events
9. Commit atomically
```

### F.5 Seller Restriction

- Seller receives commerce violation
- Restriction applied per ladder (Section L)
- Seller cannot sell through For Sale + Auction while restricted
- Seller may relist after restriction expires

---

## G. Case B Flow — Inside Coverage

**Trigger:** Winner's destination IS covered by at least one selected Shipping Setup.

### G.1 Two Sub-Paths

**Path B1 — Normal Shipping:**
- Winner selects a Shipping Setup that covers their destination
- Selection resolves shipping
- No private quote needed

**Path B2 — Seller Special Quote (optional):**
- Seller MAY provide a private quote even though destination is covered
- The reusable Shipping Setups MUST NOT be mutated
- The private quote is transaction-specific
- Winner accepts the private quote
- Acceptance resolves shipping

### G.2 Buyer Deadline

**Deadline:** `auction_end + 24h` from auction end (for normal shipping selection)

**If buyer fails to select/resolve shipping within deadline:**
- Buyer commerce violation
- Buyer restriction
- Auction → DRAFT
- Seller may immediately relist/create commerce normally

**If seller provides private quote BEFORE buyer selects normal shipping:**
- Winner may accept the private quote
- If winner accepts → shipping resolved
- If winner does not accept before buyer deadline → buyer violation (unless seller quote arrival extended the window — **OWNER DECISION: quote arrival does NOT extend buyer deadline**)

### G.3 Buyer Shipping Deadline Worker

**New worker: `AuctionBuyerShippingDeadlineWorker`**

**Eligibility query:**
```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND shipping_resolved_at IS NULL
  AND end_at + 24h <= NOW()
  AND (seller_action_required = FALSE OR seller_quote_provided = TRUE)
FOR UPDATE SKIP LOCKED
```

**Simplification:** The buyer deadline worker fires when:
- Auction is in `waiting_settlement`
- `shipping_resolved_at IS NULL`
- `end_at + 24h <= NOW()`
- Either destination is covered (Case B) OR seller has provided a quote (Case A with quote, buyer hasn't accepted)

**Key guard:** Do NOT punish the buyer when seller action is still pending (Case A without quote). The buyer deadline worker must check that the seller is NOT still obligated. This is resolved by the `seller_action_required` flag.

### G.4 Failure Path (Atomic)

```
1. Verify deadline: now >= auction.end_at + 24h
2. Lock auction (FOR UPDATE)
3. Verify status = waiting_settlement AND shipping_resolved_at IS NULL
4. Verify seller is NOT still pending action (Case A seller deadline not yet reached)
5. Record buyer violation (commerce_violations.insert)
6. Extend buyer restriction (commerce_restrictions)
7. Transition auction → DRAFT
8. Clear all settlement state
9. Emit outbox events
10. Commit atomically
```

### G.5 Case B with Seller Special Quote

**Race prevention:**
- Both normal shipping selection and private quote acceptance check `shipping_resolved_at IS NULL` as a guard
- First to set `shipping_resolved_at` wins
- The quote entity's existing lifecycle (ACTIVE → USED) provides double protection
- The `FOR UPDATE` lock on the auction row + `shipping_resolved_at IS NULL` guard provides atomicity

---

## H. Shipping Resolution Concurrency

### H.1 First-Resolution-Wins Mechanism

```sql
-- Atomic resolution attempt (in Go code):
-- 1. Lock auction FOR UPDATE
-- 2. Check: shipping_resolved_at IS NULL
-- 3. If NULL → set shipping_resolved_at = NOW(), commit
-- 4. If NOT NULL → reject (another path already resolved)
```

```sql
-- Database-level guard (belt and suspenders):
UPDATE auctions
SET shipping_resolved_at = NOW()
WHERE id = $1
  AND shipping_resolved_at IS NULL
  AND status = 'waiting_settlement'
```

If `rows_affected = 0`, resolution failed (another path won or auction moved).

### H.2 Concurrent Scenarios

| Scenario | Result |
|---|---|
| Buyer selects normal shipping; seller quote accepted concurrently | First commit wins. Second sees `shipping_resolved_at IS NOT NULL` → rejected. |
| Two normal shipping selections concurrently | First commit wins. |
| Seller creates quote; buyer already accepted different resolution | Quote ACCEPTANCE is the resolution trigger, not quote creation. |
| Worker fires buyer deadline while seller is resolving | Worker checks `shipping_resolved_at IS NULL` → if already set, no-op. |

### H.3 Concurrency Guard Requirements

For every resolution path:

1. `SELECT ... FOR UPDATE` on auction row
2. Check `shipping_resolved_at IS NULL`
3. Check `status = 'waiting_settlement'`
4. Set `shipping_resolved_at = NOW()`
5. Insert `auction_shipping_resolution` row
6. Create order (or defer to separate step)
7. Commit

**After resolution is set:**
- All later resolution attempts fail safely
- No overwrite
- No second order
- No second payment deadline
- No contradictory shipping snapshot

---

## I. Payment Deadline Changes

### I.1 Current Payment Expiry Authority

**File:** `backend/internal/commerce/order/application/order_creation_service.go`

```go
func calculatePaymentExpiry(paymentMethod string, createdAt time.Time) time.Time {
    // instant=15min, va=1hr, retail=6hr, default=30min
}
```

Auction orders currently use `PaymentMethod = "default"` → 30 minutes.

**`Order.PaymentExpiresAt`:** Single source of truth for payment expiry. Set at order creation. Queried by `PaymentExpiryWorker`.

**`PaymentExpiryWorker`:** Queries `payments WHERE status = 'pending' AND expired_at < NOW()`. On expiry: `orderService.Expire()` → order → expired, escrow refund, coin refund, stock restore.

### I.2 Current Auction Order Expiry Path

1. Order created via `/claim` with `PaymentExpiresAt = calculatePaymentExpiry("default", now)` = now + 30min
2. `PaymentExpiryWorker` finds expired payment
3. Calls `orderService.Expire()`:
   a. `order.MarkExpired()` (pending → expired)
   b. `restoreForSaleStock()` → for auctions: `releaseAuctionOrderBinding()` clears `auction.OrderID`
   c. Escrow release + gateway refund
   d. Coin refund
   e. Shipping quote reactivation
4. **Auction stays `ended` (terminal)** — OrderID cleared but no state transition

### I.3 Required Changes

**For auction orders:** `PaymentExpiresAt` must be `shipping_resolved_at + 24h` (NOT method-based).

**For For Sale orders:** `calculatePaymentExpiry()` remains unchanged.

**Implementation:**

1. Modify order creation to accept an explicit `paymentExpiresAt` parameter for auction orders:
   ```go
   // In order_creation_service.go:
   func calculateAuctionPaymentExpiry(shippingResolvedAt time.Time) time.Time {
       return shippingResolvedAt.Add(24 * time.Hour)
   }
   ```

2. At order creation time for auctions, pass `shipping_resolved_at + 24h` as `PaymentExpiresAt`.

3. Ensure `PaymentExpiryWorker` correctly handles auction order expiry:
   - After order expires: `releaseAuctionOrderBinding()` (existing)
   - **NEW:** Transition auction from `ended` to `draft` (after financial rollback)
   - This requires adding the `ended → draft` transition OR handling the rollback before settling

**Wait — architectural issue:** Currently, the order is created AND the auction is settled (→ ended) in the same transaction. If the order later expires, the auction is `ended` with `OrderID = nil`. Under the new design, settlement failure → `DRAFT`. So:

- **BNR path (payment expires after shipping resolved):** The `Expire()` chain must transition the auction from `ended` to `draft`. This requires the `ended → draft` transition or a new dedicated settlement-failure path.
- **Alternative:** Do NOT settle the auction to `ended` at order creation time. Keep it in `waiting_settlement` with `OrderID` set. Only transition to `ended` after payment succeeds.

**Recommended approach:**

```
waiting_settlement → (shipping resolved, order created, OrderID set) → still waiting_settlement
  → payment succeeds → ended
  → payment expires → violation/restriction + DRAFT
```

This means the auction stays in `waiting_settlement` even after order creation, and only transitions to `ended` on payment success. This is cleaner because:
- `waiting_settlement → draft` is already in the new state machine
- No need for `ended → draft` (which would be unusual)
- The auction's `OrderID` being non-nil prevents double settlement

### I.4 Revised Payment Deadline Flow

```
1. shipping_resolved_at set (first resolution wins)
2. Order created: PaymentExpiresAt = shipping_resolved_at + 24h
3. Auction stays in waiting_settlement (OrderID set)
4. PaymentExpiryWorker finds expired payment for auction order:
   a. orderService.Expire() → order → expired, escrow refund, coin refund
   b. NEW: releaseAuctionOrderBinding() (existing)
   c. NEW: Transition auction → DRAFT (new path: waiting_settlement → draft)
   d. NEW: Record buyer commerce violation
   e. NEW: Apply buyer restriction
5. On payment success:
   a. Order → paid
   b. Auction → ended (settle)
```

---

## J. BNR Financial Rollback

### J.1 Current Expire() Chain

**File:** `backend/internal/commerce/order/application/order_completion_service.go` (line 1008)

```
1. GetForUpdate(order) — lock order
2. order.MarkExpired() — pending → expired
3. restoreForSaleStock() — for auctions: releaseAuctionOrderBinding()
4. GetEscrowForOrder() — check if escrow was held
5. If escrow: InitiateGatewayRefundForOrder() + RefundToBuyer()
6. If coins used: emit coins.refund_required event
7. Reactivate shipping quote if eligible
8. UpdateStatusTx() — persist expired status
9. Emit order.expired outbox event
```

### J.2 What is Already Correct

| Component | Status |
|---|---|
| Order state transition (pending → expired) | ✅ Correct |
| Escrow release for auction orders | ✅ Correct (if escrow held) |
| Gateway refund initiation | ✅ Correct |
| Coin refund via outbox event | ✅ Correct |
| Shipping quote reactivation | ✅ Correct |
| Auction OrderID release | ✅ Correct (ReleaseUnpaidOrder) |
| Outbox event emission | ✅ Correct |

### J.3 What Must Change

| Component | Current | Required |
|---|---|---|
| Auction state after expire | Stays `ended` (terminal) | Transitions to `draft` |
| Violation recording | None | Record in `commerce_violations` |
| Restriction application | None | Apply in `commerce_restrictions` |
| Payment deadline authority | `calculatePaymentExpiry("default")` = 30min | `shipping_resolved_at + 24h` |

### J.4 Auction-Order Binding

**Current:** `ReleaseUnpaidOrder()` clears `auction.OrderID` but auction stays `ended`.

**Required:** After financial rollback completes, transition auction from `waiting_settlement` to `draft`. This means:

- The `Expire()` chain must include: `auction.TransitionToDraftOnSettlementFailure()`
- This is within the same transaction as the financial rollback
- The auction lock (FOR UPDATE) must be acquired during the Expire transaction

### J.5 Pricing Token

The pricing token is already consumed at order creation (`FinalizeOrderConsumption`). On order expiry, the token has already been consumed and cannot be reused. This is correct — the buyer gets a new pricing token on relisting.

### J.6 Escrow

Escrow is correctly handled: `GetEscrowForOrder()` → `RefundToBuyer()`. The `RefundEscrow` method is idempotent. If no escrow exists, it's a no-op.

### J.7 Ledger

The ledger is only written to on payment success (not at order creation). On expiry, no ledger entries need reversal. The escrow refund is the only financial reversal.

### J.8 Complete BNR Financial Rollback

After valid BNR:
1. Order → expired (via `MarkExpired()`)
2. Stock restored / auction binding released (via `restoreForSaleStock()`)
3. Escrow refunded to buyer (via `RefundToBuyer()`)
4. Coins refunded (via `coins.refund_required` event)
5. Shipping quote reactivated if applicable
6. **NEW:** Commerce violation recorded
7. **NEW:** Commerce restriction applied/extended
8. **NEW:** Auction → draft (via `TransitionToDraftOnSettlementFailure()`)

All within one atomic transaction.

---

## K. Commerce Violation Authority

### K.1 Current State

- Only `buyer_bnr_strikes` exists (buyer-only, auction-only)
- No `commerce_violations` table
- No `commerce_restrictions` table
- No seller violation mechanism for settlement failures
- `BNRStrikeHandler` records strikes in `buyer_bnr_strikes`

### K.2 New `commerce_violations` Table

```sql
CREATE TABLE commerce_violations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    violation_type text NOT NULL,  -- 'buyer_bnr', 'buyer_shipping_timeout', 'seller_shipping_default', 'seller_quote_default'
    source_type text NOT NULL,     -- 'auction', 'for_sale'
    source_id uuid NOT NULL,       -- auction_id or for_sale_id
    reason text,                   -- Human-readable reason
    metadata jsonb,                -- Additional context (deadline, timestamps, etc.)
    created_at timestamptz NOT NULL DEFAULT NOW()
);

-- Immutable: no UPDATE, no DELETE (append-only)
-- UNIQUE constraint not on source_id because same auction can produce
-- buyer AND seller violations (different users)
```

### K.3 Violation Types

| Type | Actor | When |
|---|---|---|
| `buyer_bnr` | Buyer | Buyer fails to pay after shipping resolved (BNR) |
| `buyer_shipping_timeout` | Buyer | Buyer fails to select/resolve shipping within deadline |
| `seller_shipping_default` | Seller | Seller fails to provide private quote within deadline (Case A) |
| `seller_quote_default` | Seller | Seller fails to provide quote (covered by seller_shipping_default) |

### K.4 Violation Atomicity

Every failure path records the violation atomically with the restriction and auction rollback:

```go
// In a single transaction:
tx.Begin()
// 1. Lock auction
// 2. Verify deadline and state
// 3. Insert commerce_violations row
// 4. Upsert commerce_restrictions (EXTEND)
// 5. Transition auction → DRAFT
// 6. Emit outbox events
tx.Commit()
```

---

## L. Commerce Restriction Authority

### L.1 New `commerce_restrictions` Table

```sql
CREATE TABLE commerce_restrictions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    violation_count integer NOT NULL DEFAULT 0,  -- Running count of violations
    restricted_until timestamptz NOT NULL,        -- When restriction expires
    last_violation_id uuid NOT NULL REFERENCES commerce_violations(id),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),

    UNIQUE(user_id)  -- One active restriction per user
);
```

### L.2 Restriction Ladder (Locked)

| Violation # | Restriction Duration |
|---|---|
| 1st | 7 days |
| 2nd | 15 days |
| 3rd+ | 30 days |

### L.3 Restriction Stacking — EXTEND Semantics

```go
// Calculate new restricted_until
func calculateRestrictedUntil(currentRestrictedUntil *time.Time, violationCount int) time.Time {
    duration := restrictionDuration(violationCount)  // 7d, 15d, or 30d

    if currentRestrictedUntil == nil || currentRestrictedUntil.Before(time.Now()) {
        // No active restriction or expired: new restriction starts now
        return time.Now().Add(duration)
    }
    // Active restriction: EXTEND from current expiry
    return currentRestrictedUntil.Add(duration)
}

func restrictionDuration(violationCount int) time.Duration {
    switch {
    case violationCount <= 1:
        return 7 * 24 * time.Hour
    case violationCount == 2:
        return 15 * 24 * time.Hour
    default:
        return 30 * 24 * time.Hour
    }
}
```

**Example:**
- Current restriction has 4 days remaining
- New violation grants 15 days
- `new_restricted_until = current_restricted_until + 15 days`

### L.4 Restriction Scope

- **Buyer restriction:** Cannot transact on For Sale + Auction (cannot buy)
- **Seller restriction:** Cannot sell through For Sale + Auction (cannot list/sell)

### L.5 Restriction Check

```go
func IsUserRestricted(ctx context.Context, tx db.Tx, userID uuid.UUID) (bool, *time.Time, error) {
    // SELECT restricted_until FROM commerce_restrictions
    //   WHERE user_id = $1
    // If restricted_until > NOW() → restricted
    // If no row or restricted_until <= NOW() → not restricted
}
```

### L.6 No Admin Reset

Violation history is immutable. There is NO admin reset mechanism. If an exceptional override is ever required, it must be a separate audited capability and is OUT OF SCOPE.

---

## M. Restriction Stacking

### M.1 Calculation

```
current_restricted_until = SELECT restricted_until FROM commerce_restrictions WHERE user_id = $1
new_duration = restrictionDuration(current_violation_count + 1)

if current_restricted_until is NULL or current_restricted_until <= NOW():
    new_restricted_until = NOW() + new_duration
else:
    new_restricted_until = current_restricted_until + new_duration
```

### M.2 Concurrency

The restriction upsert must be atomic:

```sql
INSERT INTO commerce_restrictions (user_id, violation_count, restricted_until, last_violation_id)
VALUES ($1, 1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
SET violation_count = commerce_violations.violation_count + 1,
    restricted_until = CASE
        WHEN commerce_violations.restricted_until > NOW()
        THEN commerce_violations.restricted_until + $4
        ELSE NOW() + $4
    END,
    last_violation_id = $3,
    updated_at = NOW()
```

**Note:** The above SQL has a subtlety — the `violation_count` in the ON CONFLICT refers to the existing row. The duration calculation must be based on the NEW violation count (`violation_count + 1`).

**Better approach (application-level, within transaction):**

```go
func ApplyRestriction(ctx context.Context, tx db.Tx, userID uuid.UUID, violationID uuid.UUID) error {
    // 1. Lock existing restriction (FOR UPDATE) or use INSERT ... ON CONFLICT
    existing, err := getRestrictionForUpdate(ctx, tx, userID)

    var newCount int
    var newUntil time.Time

    if existing == nil {
        // First violation
        newCount = 1
        newUntil = time.Now().Add(7 * 24 * time.Hour)
    } else {
        newCount = existing.ViolationCount + 1
        duration := restrictionDuration(newCount) // uses NEW count
        if existing.RestrictedUntil.After(time.Now()) {
            newUntil = existing.RestrictedUntil.Add(duration) // EXTEND
        } else {
            newUntil = time.Now().Add(duration)
        }
    }

    // 2. Upsert restriction
    return upsertRestriction(ctx, tx, userID, newCount, newUntil, violationID)
}
```

---

## N. API / Application Changes

### N.1 New Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/auctions/:id/resolve-shipping` | Resolve shipping (normal selection or quote acceptance) |
| POST | `/auctions/:id/provide-quote` | Seller provides private shipping quote for settlement |
| POST | `/auctions/:id/accept-quote` | Winner accepts seller's private quote |
| GET | `/auctions/:id/settlement-status` | Get current settlement status (which case, deadlines, etc.) |

### N.2 Modified Endpoints

| Endpoint | Change |
|---|---|
| `POST /auctions/:id/claim` | **REPLACE** with `resolve-shipping` + separate order creation. Remove bundled `/claim` flow. |
| `POST /auctions/:id/claim-token` | **REMOVE** — replaced by resolution endpoints |

### N.3 New Flow

```
1. POST /auctions/:id/settlement-status
   → Returns: case (A/B), coverage info, deadlines, seller/buyer obligations

2. [If Case A] POST /auctions/:id/provide-quote
   → Seller provides: cost, note, destination validation
   → System creates shipping_quote with expires_at

3. POST /auctions/:id/resolve-shipping
   → Buyer submits: address_id + either shipping_option_id OR shipping_quote_id
   → System:
     a. Resolves winner's primary address (if address_id not provided)
     b. Validates coverage (normal) or quote (private)
     c. Atomic: set shipping_resolved_at + insert auction_shipping_resolution
     d. Creates order with PaymentExpiresAt = shipping_resolved_at + 24h
     e. Auction stays in waiting_settlement (NOT settled to ended yet)
   → Returns: order_id

4. [After payment] Auction transitions to ended
   [On payment expiry] Auction transitions to draft + violation + restriction
```

### N.4 `/resolve-shipping` Detailed Behavior

```go
func ResolveShipping(ctx context.Context, tx db.Tx, input ResolveShippingInput) error {
    // 1. Lock auction FOR UPDATE
    auction := getForUpdate(input.AuctionID)

    // 2. Validate state
    if auction.Status != StatusWaitingSettlement { return ErrNotClaimable }
    if auction.OrderID != nil { return ErrAlreadySettled }
    if auction.ShippingResolvedAt != nil { return ErrShippingAlreadyResolved }

    // 3. Validate caller is winner
    if auction.CurrentWinnerID != input.UserID { return ErrNotWinner }

    // 4. Validate deadline (24h from auction end)
    if time.Now().After(auction.EndAt.Add(24 * time.Hour)) { return ErrDeadlinePassed }

    // 5. Resolve destination (from provided address_id or primary address)
    address := resolveAddress(input.UserID, input.AddressID)

    // 6. Determine resolution mode
    if input.ShippingQuoteID != nil {
        // Private quote path
        quote := validateAndLockQuote(input.ShippingQuoteID, auction, input.UserID)
        resolutionMode = "private_quote"
        shippingCost = quote.Cost
        // Mark quote as USED
    } else {
        // Normal shipping path
        deliveryOption := validateShippingOption(input.ShippingSetupID, auction, address)
        resolutionMode = "normal_shipping"
        shippingCost = deliveryOption.Cost
    }

    // 7. First-resolution-wins guard
    if auction.ShippingResolvedAt != nil { return ErrAlreadyResolved }

    // 8. Set shipping_resolved_at
    auction.ShippingResolvedAt = &now

    // 9. Insert auction_shipping_resolution
    insertResolution(auction, resolutionMode, address, shippingCost, ...)

    // 10. Create order (with PaymentExpiresAt = shipping_resolved_at + 24h)
    order := createOrder(auction, input, shippingCost, shippingResolvedAt.Add(24*time.Hour))

    // 11. Bind order to auction
    auction.OrderID = &order.ID
    // NOTE: Auction stays in waiting_settlement (NOT settled yet)

    // 12. Emit outbox events
    // 13. Commit
}
```

### N.5 Mobile App Changes

The mobile app currently:
1. Shows claim modal with address + shipping option selection
2. Calls `POST /auctions/:id/claim`

Must change to:
1. Show settlement status screen (case A/B, deadlines)
2. For Case A: wait for seller quote notification
3. For resolution: call `POST /auctions/:id/resolve-shipping`
4. After resolution: proceed to payment (order already created, payment window = 24h)

---

## O. Worker Changes

### O.1 Existing Workers to Modify

| Worker | Current Behavior | Required Change |
|---|---|---|
| `AuctionEndWorker` | Ends expired active auctions → waiting_settlement | Add: determine Case A/B, set `seller_action_required` flag, start seller/buyer deadline clocks |
| `AuctionSettlementWorker` | Transitions to `expired_bnr` | **REPLACE** with deadline enforcement for both seller and buyer paths |
| `PaymentExpiryWorker` | Expires unpaid orders | Add: transition auction → draft after financial rollback |
| `BNRStrikeHandler` | Inserts into `buyer_bnr_strikes` | **REPLACE** with `commerce_violations` recording + restriction application |
| `BNRDecayWorker` | Decays strikes after 180 days | **REMOVE** (no decay in new system) |
| `BNRAdminResetter` | Resets strikes via admin | **REMOVE** (no admin reset in new system) |

### O.2 New Workers

#### O.2.1 AuctionSellerDeadlineWorker

**Purpose:** Enforce seller deadline for Case A (outside coverage, private quote required).

**Trigger:** `auction_end + 24h`

**Eligibility query:**
```sql
SELECT id, seller_id, current_winner_id
FROM auctions
WHERE status = 'waiting_settlement'
  AND shipping_resolved_at IS NULL
  AND seller_action_required = TRUE
  AND seller_quote_provided = FALSE
  AND end_at + INTERVAL '24 hours' <= NOW()
FOR UPDATE SKIP LOCKED
LIMIT 50
```

**Idempotency:** Check `shipping_resolved_at IS NULL` and `status = 'waiting_settlement'` after lock.

**Failure path (per auction, atomic):**
```
1. Verify deadline (belt-and-suspenders)
2. Lock auction FOR UPDATE
3. Double-check: status = waiting_settlement AND shipping_resolved_at IS NULL
4. Record seller commerce violation
5. Apply/extend seller restriction
6. Transition auction → DRAFT
7. Clear settlement state
8. Emit outbox events
9. Commit
```

#### O.2.2 AuctionBuyerShippingDeadlineWorker

**Purpose:** Enforce buyer deadline for normal shipping selection (Case B).

**Trigger:** `auction_end + 24h`

**Eligibility query:**
```sql
SELECT id, seller_id, current_winner_id
FROM auctions
WHERE status = 'waiting_settlement'
  AND shipping_resolved_at IS NULL
  AND end_at + INTERVAL '24 hours' <= NOW()
  AND seller_action_required = FALSE
FOR UPDATE SKIP LOCKED
LIMIT 50
```

**Key guard:** `seller_action_required = FALSE` ensures we don't punish buyer when seller is still obligated.

**Failure path:** Same as seller deadline but records buyer violation + buyer restriction.

### O.3 Worker Design Decisions

**Should the seller deadline worker own the full responsibility?**

No. The seller deadline and buyer deadline are separate concerns:
- Seller deadline: Case A only, when `seller_action_required = TRUE`
- Buyer deadline: Case B (or Case A after seller provided quote and buyer didn't accept)

**Should existing `AuctionSettlementWorker` be repurposed?**

Yes. The `AuctionSettlementWorker` currently fires on `settlement_deadline` which is `end_at + 24h`. This is exactly the right time for both deadlines. The worker should be refactored to:

1. Find all `waiting_settlement` auctions with deadline passed and `shipping_resolved_at IS NULL`
2. For each, determine if it's a seller deadline (Case A) or buyer deadline (Case B)
3. Apply the appropriate violation + restriction + DRAFT rollback

This is simpler than having two separate workers. The determination of seller vs buyer deadline can be done by checking `seller_action_required` flag on the auction.

### O.4 Revised AuctionSettlementWorker

```go
func (w *AuctionSettlementWorker) processDeadline(ctx context.Context, auctionID uuid.UUID) error {
    return w.db.WithTx(ctx, func(tx db.Tx) error {
        // 1. Lock auction
        auction := w.auctionSvc.GetForUpdate(ctx, tx, auctionID)

        // 2. Double-check
        if auction.Status != StatusWaitingSettlement { return nil }
        if auction.ShippingResolvedAt != nil { return nil } // Already resolved
        if time.Now().Before(auction.EndAt.Add(24 * time.Hour)) { return nil }

        // 3. Determine who defaulted
        var violatedUserID uuid.UUID
        var violationType string
        if auction.SellerActionRequired {
            violatedUserID = auction.SellerID
            violationType = "seller_shipping_default"
        } else {
            violatedUserID = *auction.CurrentWinnerID
            violationType = "buyer_shipping_timeout"
        }

        // 4. Record violation + apply restriction (atomic)
        violationID := recordViolation(ctx, tx, violatedUserID, violationType, auction.ID)
        applyRestriction(ctx, tx, violatedUserID, violationID)

        // 5. Transition auction → DRAFT
        auction.TransitionToDraftOnSettlementFailure()
        w.auctionRepo.UpdateTx(ctx, tx, auction)

        // 6. Emit outbox events
        // 7. Commit
    })
}
```

---

## P. Database Changes

### P.1 Schema Changes

#### P.1.1 Modify `auctions` table

```sql
-- Add shipping resolution marker
ALTER TABLE auctions ADD COLUMN shipping_resolved_at timestamptz;

-- Add seller action flag (computed at auction end)
ALTER TABLE auctions ADD COLUMN seller_action_required boolean DEFAULT FALSE;

-- Add seller quote provided flag
ALTER TABLE auctions ADD COLUMN seller_quote_provided boolean DEFAULT FALSE;
```

**Authority:** Shipping resolution is a first-class auction lifecycle event. `shipping_resolved_at` is the single marker for when shipping was resolved.

**Consumers:**
- `/resolve-shipping` endpoint (sets the value)
- Deadline workers (check IS NULL)
- Order creation (gate: must be set)
- Payment expiry (anchor for 24h deadline)

#### P.1.2 Remove `settlement_deadline` from `auctions`

**No.** `settlement_deadline` is currently used by `AuctionSettlementWorker`. Under the new design, the deadline is implicitly `end_at + 24h`. We could either:
- Keep `settlement_deadline` for backward compatibility (set to `end_at + 24h` at auction end)
- Remove it and compute deadline as `end_at + 24h` in queries

**Decision:** Keep `settlement_deadline` as a denormalized field for query simplicity. Set it to `end_at + 24h` on `TransitionToWaitingSettlement()` (already done).

#### P.1.3 New `auction_shipping_resolution` table

```sql
CREATE TABLE auction_shipping_resolution (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    auction_id uuid NOT NULL REFERENCES auctions(id) UNIQUE,
    resolved_at timestamptz NOT NULL,
    resolution_mode text NOT NULL CHECK (resolution_mode IN ('normal_shipping', 'private_quote')),

    -- Normal shipping fields (nullable)
    shipping_setup_id uuid,
    shipping_setup_name text,
    shipping_transport_type text,

    -- Private quote fields (nullable)
    shipping_quote_id uuid,
    shipping_quote_price bigint,

    -- Shipping cost (from either source)
    shipping_cost bigint NOT NULL,

    -- Immutable snapshots
    destination_snapshot jsonb NOT NULL,
    origin_snapshot jsonb NOT NULL,

    created_at timestamptz NOT NULL DEFAULT NOW()
);

-- UNIQUE on auction_id ensures first-resolution-wins
CREATE UNIQUE INDEX idx_auction_shipping_resolution_auction ON auction_shipping_resolution(auction_id);
```

**Authority:** This is the canonical immutable shipping record for auction settlement. Once inserted, it is never updated or deleted.

**Consumers:**
- Order creation reads from here instead of re-computing
- Financial audit trail
- Fulfillment reference

#### P.1.4 New `commerce_violations` table

```sql
CREATE TABLE commerce_violations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id),
    violation_type text NOT NULL,
    source_type text NOT NULL,
    source_id uuid NOT NULL,
    reason text,
    metadata jsonb,
    created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commerce_violations_user ON commerce_violations(user_id, created_at DESC);
```

**Authority:** Immutable violation history. Append-only. No UPDATE, no DELETE.

#### P.1.5 New `commerce_restrictions` table

```sql
CREATE TABLE commerce_restrictions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) UNIQUE,
    violation_count integer NOT NULL DEFAULT 1,
    restricted_until timestamptz NOT NULL,
    last_violation_id uuid NOT NULL REFERENCES commerce_violations(id),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commerce_restrictions_active ON commerce_restrictions(user_id)
    WHERE restricted_until > NOW();
```

**Authority:** Current active restriction per user. One row per user (upserted on each violation).

#### P.1.6 Modify `auction_status_enum`

```sql
-- Remove expired_bnr from enum (after data migration)
ALTER TYPE auction_status_enum DROP VALUE IF EXISTS 'expired_bnr';
```

**Note:** PostgreSQL does not support `DROP VALUE` for enums. The approach must be:
1. Create new enum type without `expired_bnr`
2. Migrate data
3. Alter column to use new type
4. Drop old type

### P.2 Consumers of Each Change

| Change | Consumers |
|---|---|
| `auctions.shipping_resolved_at` | `/resolve-shipping`, deadline workers, order creation, payment expiry |
| `auctions.seller_action_required` | Auction end worker (set), deadline workers (read) |
| `auctions.seller_quote_provided` | Seller deadline worker (read), buyer deadline worker (read) |
| `auction_shipping_resolution` | Order creation (read), financial audit, fulfillment |
| `commerce_violations` | Violation recording, admin read, audit |
| `commerce_restrictions` | Restriction checks (PlaceBid, CreateOrder, CreateDraft) |
| `expired_bnr` removal | State machine, all status references, moderation, search |

### P.3 Obsolete Schema to Drop

| Table/Column | Reason |
|---|---|
| `buyer_bnr_strikes` | Replaced by `commerce_violations` + `commerce_restrictions` |
| `buyer_bnr_strikes.decayed_at` | No decay in new system |
| `buyer_bnr_strikes.admin_reset` | No admin reset in new system |
| `auction_status_enum.expired_bnr` | Not a canonical auction state |
| `auctions.listing_id` | Already deprecated |
| `auctions.title`, `auctions.description` | Deprecated (on products now) |
| `auctions.preparation_time`, `auctions.preparation_note` | Deprecated (on products now) |

---

## Q. Cleanup / Residue Removal

### Q.1 Code Cleanup Targets

| Target | Location | Action |
|---|---|---|
| `StatusExpiredBNR` constant | `auction/entity/auction.go` | Remove |
| `TransitionToExpiredBNR()` method | `auction/entity/auction.go` | Remove |
| `expired_bnr` in `transitionAllowed` | `auction/entity/auction.go` | Remove |
| `expired_bnr` in `PublicLifecycle()` | `auction/entity/auction.go` | Remove |
| `expired_bnr` in `IsPublicDiscoverable()` | `auction/entity/auction.go` | Remove |
| `expired_bnr` in `IsRepostable()` | `auction/entity/auction.go` | Remove (if referenced) |
| `BNRStrikeChecker` | `auction/application/bnr_restriction.go` | Remove entirely |
| `BNRStrikeChecker.evaluate()` | `auction/application/bnr_restriction.go` | Remove |
| `BNRStrikeHandler` | `worker/bnr_strike_handler.go` | Remove entirely |
| `BNRDecayWorker` | `worker/bnr_decay_worker.go` | Remove entirely |
| `BNRAdminResetter` | `worker/bnr_admin_reset.go` | Remove entirely |
| `BNRStrikeHandler` wiring | `serverboot/dependencies.go` | Remove |
| `BNRDecayWorker` wiring | `serverboot/dependencies.go` | Remove |
| `BNRAdminResetter` wiring | `serverboot/dependencies.go` | Remove |
| `AuctionBNRDetectedEvent` struct | `worker/auction_settlement_worker.go` | Remove |
| `emitBNREvent` method | `worker/auction_settlement_worker.go` | Remove |
| `AuctionSettlementWorker.expireAuctionSettlement()` | `worker/auction_settlement_worker.go` | Replace with DRAFT rollback |
| `AuctionSettlementWorker.emitBNREvent()` | `worker/auction_settlement_worker.go` | Remove |
| `buyer_bnr_strikes` references | Multiple files | Remove all |
| `/claim` endpoint | `auction/delivery/http/auction_handler.go` | Replace with `/resolve-shipping` |
| `/claim-token` endpoint | `auction/delivery/http/auction_handler.go` | Remove |
| `GeneratePricingTokenForClaim` | `auction/application/auction_service.go` | Replace |
| `ClaimAuctionRequest` struct | `auction/delivery/http/auction_handler.go` | Replace |
| `buildClaimPricingSnapshot` | `auction/delivery/http/auction_handler.go` | Move to resolution flow |
| `SetBNRStrikeChecker` | `auction/application/auction_service.go` | Remove |
| `bnrStrikeChecker` field | `auction/application/auction_service.go` | Remove |
| `BNR` in PlaceBid | `auction/application/auction_service.go` | Replace with restriction check |
| `BNRAuctionRestrictedError` | `auction/entity/auction.go` | Replace with generic restriction error |
| `IsBNRAuctionRestricted` | `auction/entity/auction.go` | Remove |
| Old BNR tests | Multiple `*_test.go` files | Replace with violation/restriction tests |
| `expired_bnr` in moderation handler | `worker/moderation_event_handler.go` | Remove |
| `expired_bnr` in search/discovery | `discovery/search/` | Remove |
| `expired_bnr` in commerce/shared view | `commerce/shared/view_access.go` | Remove |
| `expired_bnr` in public card | `pkg/publiccard/auction_card.go` | Remove |
| Mobile: claim modal | `auction_claim_shipping_modal.dart` | Replace with resolution UI |
| Mobile: claim API call | `auction_remote_datasource.dart` | Replace with resolution endpoint |
| Mobile: claim test | `auction_contract_p1_test.dart` | Update |
| Mobile: seller settlement monitor | `auction_seller_settlement_monitor.dart` | Update for new states |
| `buyer_bnr_strikes` migration | `migrations/000001_canonical_schema.up.sql` | Keep for history but add drop migration |

### Q.2 Obsolete Test Files

- `auction_settlement_worker_test.go` — rewrite for new deadline logic
- `bnr_strike_handler_test.go` — remove
- `bnr_decay_worker_test.go` — remove
- `bnr_admin_reset_test.go` — remove
- `bnr_restriction_test.go` — rewrite as restriction tests
- `release_unpaid_order_test.go` — update for DRAFT transition
- All `expired_bnr` references in test seeds/fixtures

### Q.3 Obsolete Migrations

No migrations need to be dropped. The cleanup happens via new migrations that:
1. Create new tables (`commerce_violations`, `commerce_restrictions`, `auction_shipping_resolution`)
2. Add new columns (`shipping_resolved_at`, `seller_action_required`, `seller_quote_provided`)
3. Migrate data (`expired_bnr` → `draft`)
4. Create new enum type (without `expired_bnr`)
5. Drop `buyer_bnr_strikes` table

---

## R. Test & Regression Proof Plan

### R.1 Backend Unit Tests

| Test | What It Proves |
|---|---|
| `TestResolveShipping_NormalCoverage` | Normal shipping selection resolves shipping |
| `TestResolveShipping_PrivateQuote` | Private quote acceptance resolves shipping |
| `TestResolveShipping_FirstResolutionWins` | Concurrent resolution: first wins, second rejected |
| `TestResolveShipping_AlreadyResolved` | Re-resolution rejected after first resolution |
| `TestResolveShipping_NotWinner` | Non-winner cannot resolve |
| `TestResolveShipping_DeadlinePassed` | Resolution rejected after deadline |
| `TestSellerDeadlineWorker_CaseA` | Seller deadline worker fires on Case A |
| `TestSellerDeadlineWorker_AlreadyResolved` | No-op when shipping already resolved |
| `TestBuyerDeadlineWorker_CaseB` | Buyer deadline worker fires on Case B |
| `TestBuyerDeadlineWorker_SellerPending` | Does NOT fire when seller still obligated |
| `TestViolationAtomicity_SellerDefault` | Seller violation + restriction + DRAFT in one tx |
| `TestViolationAtomicity_BuyerTimeout` | Buyer violation + restriction + DRAFT in one tx |
| `TestRestrictionStacking_Extend` | EXTEND semantics work correctly |
| `TestRestrictionStacking_FromExpired` | New restriction from expired state starts fresh |
| `TestPaymentExpiry_AuctionOrder` | Auction order expires at shipping_resolved_at + 24h |
| `TestPaymentExpiry_AuctionOrder_Rollback` | Financial rollback + DRAFT transition |
| `TestSettleAuction_PaymentSuccess` | Payment success → auction → ended |
| `TestTransitionToDraft` | WAITING_SETTLEMENT → DRAFT works |
| `TestTransitionToDraft_ClearsState` | All settlement state cleared on DRAFT return |
| `TestBiddingAfterDraft` | New bids work after relisting from DRAFT |
| `TestHistoricalBidsIntact` | Old bids remain after DRAFT return |
| `TestRestrictionCheck_BlocksBidding` | Restricted buyer cannot bid |
| `TestRestrictionCheck_BlocksCheckout` | Restricted buyer cannot buy (For Sale + Auction) |
| `TestSellerRestriction_BlocksListing` | Restricted seller cannot create/list |
| `TestCommerceViolation_Immutable` | No UPDATE/DELETE on violations |
| `TestShippingResolutionSnapshot` | Snapshot frozen correctly |

### R.2 Integration Tests

| Test | What It Proves |
|---|---|
| `TestFullFlow_NormalShipping` | End-to-end: auction ends → normal shipping → payment → completion |
| `TestFullFlow_PrivateQuote` | End-to-end: auction ends → seller quote → buyer accepts → payment → completion |
| `TestFullFlow_BuyerBNR` | End-to-end: shipping resolved → payment timeout → BNR → DRAFT |
| `TestFullFlow_SellerDefault` | End-to-end: Case A → seller timeout → violation → DRAFT |
| `TestFullFlow_RelistAfterBNR` | End-to-end: BNR → DRAFT → seller relists → new auction |
| `TestFullFlow_RelistWhileRestricted` | Cannot relist while restricted |
| `TestFullFlow_RelistAfterRestrictionExpires` | Can relist after restriction expires |
| `TestRaceCondition_ConcurrentResolution` | Two resolution attempts: only one succeeds |

### R.3 Database Tests

| Test | What It Proves |
|---|---|
| `TestMigrationReplay` | Clean migration from zero produces correct schema |
| `TestSchemaValidation` | All constraints and indexes exist |
| `TestDataMigration_ExpiredBNR` | All `expired_bnr` rows migrated to `draft` |

### R.4 Mobile Tests

| Test | What It Proves |
|---|---|
| API contract tests | New endpoints return expected shapes |
| Shipping flow tests | Resolution UI works for both cases |
| Auction settlement tests | Settlement status screen displays correctly |
| Payment readiness tests | 24h payment window displayed correctly |
| Regression tests | Existing For Sale commerce unaffected |

### R.5 Critical Scenarios (Must Explicitly Test)

1. ✅ Normal shipping inside coverage
2. ✅ Outside coverage → seller private quote → buyer accepts
3. ✅ Inside coverage + seller special quote
4. ✅ Buyer shipping selection timeout
5. ✅ Seller quote timeout (Case A)
6. ✅ Quote acceptance → shipping resolved
7. ✅ Simultaneous normal shipping / private quote
8. ✅ First-resolution-wins
9. ✅ Payment within 24h → success → ended
10. ✅ BNR after 24h (payment timeout)
11. ✅ BNR financial rollback (escrow, coins, quote)
12. ✅ Seller restriction (Case A default)
13. ✅ Buyer restriction (BNR / shipping timeout)
14. ✅ Restriction stacking (EXTEND)
15. ✅ DRAFT after failure
16. ✅ Immediate relist after buyer failure
17. ✅ Blocked relist while seller restricted
18. ✅ Relist after seller restriction expires
19. ✅ Historical bids remain isolated
20. ✅ No duplicate violation
21. ✅ No duplicate order
22. ✅ No duplicate payment
23. ✅ No duplicate financial ledger effect

---

## S. Migration Replay / Runtime Proof Plan

### S.1 Migration Sequence

```
1. CREATE commerce_violations table
2. CREATE commerce_restrictions table
3. CREATE auction_shipping_resolution table
4. ALTER auctions ADD COLUMN shipping_resolved_at
5. ALTER auctions ADD COLUMN seller_action_required
6. ALTER auctions ADD COLUMN seller_quote_provided
7. CREATE new auction_status_enum (without expired_bnr)
8. Migrate expired_bnr rows to draft
9. ALTER auctions ALTER COLUMN status TYPE new_enum
10. DROP old auction_status_enum
11. DROP buyer_bnr_strikes table
```

### S.2 Migration Proof

```bash
# From zero:
make dev-reset-data    # or equivalent
make migrate           # Run all migrations

# Verify schema:
psql -c "\d auctions" | grep shipping_resolved_at
psql -c "\d commerce_violations"
psql -c "\d commerce_restrictions"
psql -c "\d auction_shipping_resolution"
psql -c "SELECT enum_range(NULL::auction_status_enum)"  # Should NOT include expired_bnr
psql -c "\d buyer_bnr_strikes"  # Should fail (table dropped)
```

### S.3 Runtime Proof

1. Start server
2. Create seller account + subscription
3. Create auction with shipping options
4. Place bids
5. End auction → waiting_settlement
6. Verify Case A/B determination
7. Execute resolution flow
8. Verify payment deadline = shipping_resolved_at + 24h
9. Complete payment → verify auction → ended
10. Repeat with BNR scenario → verify DRAFT return

---

## T. Implementation Order

### Phase 1: Authority / Schema Foundation

1. Create `commerce_violations` table migration
2. Create `commerce_restrictions` table migration
3. Create `auction_shipping_resolution` table migration
4. Add `shipping_resolved_at`, `seller_action_required`, `seller_quote_provided` to auctions
5. Create new `auction_status_enum` without `expired_bnr`
6. Data migration: `expired_bnr` → `draft`
7. Drop `buyer_bnr_strikes` table
8. Write Go entity types for new tables

### Phase 2: State Machine Foundation

9. Remove `StatusExpiredBNR` from entity
10. Remove `TransitionToExpiredBNR()` method
11. Add `TransitionToDraftOnSettlementFailure()` method
12. Update `transitionAllowed` map
13. Update `PublicLifecycle()`, `IsPublicDiscoverable()`, `IsRepostable()`
14. Write state machine tests

### Phase 3: Commerce Violation / Restriction Authority

15. Implement `CommerceViolationService` (insert-only)
16. Implement `CommerceRestrictionService` (upsert with EXTEND)
17. Implement `IsUserRestricted()` check
18. Wire restriction checks into: PlaceBid, CreateDraft, CreateOrder
19. Remove `BNRStrikeChecker`, `BNRStrikeHandler`, `BNRDecayWorker`, `BNRAdminResetter`
20. Write violation/restriction tests

### Phase 4: Shipping Resolution Authority

21. Implement `ShippingResolutionService` (resolve, snapshot)
22. Implement `/resolve-shipping` endpoint
23. Implement `/settlement-status` endpoint
24. Implement `/provide-quote` endpoint (seller)
25. Implement `/accept-quote` endpoint (winner)
26. Write resolution concurrency tests

### Phase 5: Auction Settlement Flow

27. Modify `ClaimAuction` → replace with `ResolveShipping`
28. Modify order creation: accept `paymentExpiresAt` parameter for auctions
29. Set `PaymentExpiresAt = shipping_resolved_at + 24h` for auction orders
30. Modify auction settle: only transition to `ended` on payment success (not at order creation)
31. Add settlement flow tests

### Phase 6: Seller / Buyer Deadline Enforcement

32. Refactor `AuctionSettlementWorker` to handle both seller and buyer deadlines
33. Implement `seller_action_required` flag computation at auction end
34. Implement deadline worker: violation + restriction + DRAFT rollback
35. Write deadline worker tests

### Phase 7: Payment Deadline & BNR Integration

36. Modify `PaymentExpiryWorker` to handle auction → DRAFT on expiry
37. Modify `OrderCompletionService.Expire()` for auction settlement failure
38. Add `TransitionToDraftOnSettlementFailure()` call to Expire chain
39. Write payment expiry integration tests

### Phase 8: API / Mobile Consumers

40. Update mobile auction detail screen (new settlement status)
41. Update mobile claim modal → resolution UI
42. Update mobile auction API data sources
43. Write mobile API contract tests
44. Write mobile shipping flow tests

### Phase 9: Cleanup

45. Remove all `expired_bnr` references from code
46. Remove all `buyer_bnr_strikes` references from code
47. Remove `BNRStrikeChecker`, `BNRDecayWorker`, `BNRAdminResetter`
48. Remove old `/claim` and `/claim-token` endpoints
49. Remove obsolete test files
50. Remove obsolete DTO fields
51. Remove obsolete comments/docs

### Phase 10: Regression Proof

52. Run full backend test suite
53. Run `go vet`
54. Run build
55. Run migration replay from zero
56. Run mobile tests
57. Execute all 23 critical scenarios
58. Performance/load testing on resolution endpoint

---

## U. Risk / Dependency Matrix

| Risk | Impact | Mitigation |
|---|---|---|
| Concurrent resolution race condition | Double order / double payment | `shipping_resolved_at IS NULL` guard + FOR UPDATE lock |
| Payment expiry worker does not transition to DRAFT | Auction stuck in waiting_settlement | Atomic transaction in Expire() chain |
| Restriction stacking overflow | Unreasonably long restriction | 30-day cap per violation; practical limit |
| `expired_bnr` data migration loses history | Historical data incomplete | Migrate to `draft` with audit trail in `commerce_violations` |
| Mobile app backward compatibility | Old app breaks with new endpoints | Deploy backend first with compatibility shims, then update mobile |
| Seller deadline worker fires after resolution | No-op (idempotent check) | Double-check `shipping_resolved_at IS NULL` after lock |
| Order created but auction not settled | Payment success needs to settle | Separate settle step on payment success |
| New enum type migration fails | Data loss | Test migration in staging first |

### Dependencies

| Change | Depends On |
|---|---|
| `/resolve-shipping` endpoint | `auction_shipping_resolution` table, state machine |
| Deadline workers | `commerce_violations`, `commerce_restrictions`, state machine |
| Payment expiry for auctions | State machine (DRAFT transition), violation/restriction |
| Mobile updates | Backend API changes |
| Cleanup | All new code deployed and verified |

---

## V. Final Invariants

These invariants MUST hold at all times after implementation:

1. **Settlement failure → DRAFT:** An auction settlement failure returns the auction to `DRAFT`.

2. **No `expired_bnr`:** `expired_bnr` is not a canonical auction state.

3. **No `expired_settlement`:** `expired_settlement` is not a canonical auction state.

4. **Shipping before payment:** Shipping must resolve before payment becomes available.

5. **`shipping_resolved_at` immutable:** Once set, `shipping_resolved_at` is never cleared or overwritten.

6. **First resolution wins:** First valid shipping resolution wins. Subsequent attempts are rejected.

7. **Immutable shipping snapshot:** Resolved shipping facts in the order and `auction_shipping_resolution` are immutable.

8. **Payment deadline = shipping_resolved_at + 24h:** Payment deadline is always `shipping_resolved_at + 24h` for auction orders.

9. **Seller quote deadline:** Seller quote deadline is `auction_end + 24h` when seller action is required (Case A).

10. **Buyer shipping deadline:** Buyer normal-shipping deadline is `auction_end + 24h` (Case B).

11. **Seller failure ≠ buyer violation:** Seller failure is never converted into buyer violation.

12. **Shipping timeout ≠ BNR:** Buyer shipping-selection failure is never classified as BNR.

13. **BNR only after payment ability:** BNR only occurs after the buyer is able to pay.

14. **BNR rollback completeness:** BNR rollback must release/reverse all relevant financial reservations/effects.

15. **Buyer restriction scope:** Buyer restriction blocks For Sale + Auction transactions.

16. **Seller restriction scope:** Seller restriction blocks For Sale + Auction selling.

17. **EXTEND stacking:** Restriction stacking uses EXTEND semantics.

18. **Immutable violations:** Violation history is immutable.

19. **No admin reset:** No normal admin reset exists.

20. **No automatic compensation:** No automatic winner compensation exists.

21. **No automatic relisting:** No automatic relisting exists.

22. **Immediate relist (buyer):** Seller may relist immediately after buyer-caused settlement failure.

23. **Blocked relist (seller restricted):** Seller cannot relist while seller restriction is active.

24. **Relist after expiry:** Seller may relist normally after seller restriction expires.

25. **Bid isolation:** Historical bids remain intact and isolated.

26. **No duplicate authority:** No duplicate violation/restriction authority remains after cleanup.

---

## IMPLEMENTATION READINESS

**`READY_FOR_IMPLEMENTATION`**

> "This document is an implementation plan only. No code has been changed."

All business decisions are locked. All required dependencies are identified. All concurrency semantics are defined. The financial rollback chain is proven. The schema impact is clear. The cleanup impact is enumerated.

The following items from the PRD/audit are explicitly out of scope:
- Admin reset capability (future audited capability if needed)
- Trust score (not in locked business truth)
- Automatic permanent ban (not in locked business truth)
- `expired_settlement` state (explicitly rejected by business truth)
- `attempt_id` for bid isolation (not required per current audit)
