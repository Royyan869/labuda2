# Auction Winner Shipping & Settlement — Forensic Design

**Pass:** Forensic Design (read-only — no implementation)  
**Date:** 2026-09-02  
**Scope:** Auction winner shipping integration, settlement flow, BNR/restriction, deadlines/workers, race conditions, chat capability  
**Verdict:** `DESIGN READY WITH OWNER DECISIONS`

---

## 1. Executive Summary

Auction Winner Shipping & Settlement is **partially implemented** with significant structural divergence from the locked business truth. The existing codebase already contains:

- **Implemented / Canonical:** Auction lifecycle state machine (draft → scheduled → active → waiting_settlement → ended/expired_bnr), BNR strike recording and evaluation, settlement deadline (24h) on auctions, order creation from auction via pricing token, shipping quote entity with auction source_type support, lazy direct room creation in chat, payment expiry workers.
- **Divergent:** Current BNR ladder (1→2→3→4+ strikes with 14d/90d/permanent) differs from locked truth (7/15/30-day cumulative ladder for buyer AND seller). Current settlement deadline applies to buyer only (winner claim window), not to seller shipping deadline. Current auction state machine has no concept of "seller shipping deadline" or "private quote required" flow.
- **Missing:** Seller Shipping Default mechanism (seller 24h deadline after auction ends), separate buyer payment deadline (24h after shipping resolved), seller-side restriction for shipping default, unified restriction ladder shared between buyer BNR and seller shipping default, commerce chat lazy-created specifically for auction winner with "Give Shipping Quote" entry point, shipping resolution prerequisite for buyer payment window.
- **Partially Implemented:** Private shipping quote (exists for for_sale, partially wired for auction via source_type="auction"), auction claim flow creates order directly without shipping resolution prerequisite.
- **Unknown:** Whether mobile client enforces settlement deadline display correctly in all edge cases; whether concurrent seller-creates-quote + buyer-places-order race conditions are fully guarded.

**Key architectural gap:** The current implementation collapses the auction settlement into a single "winner claim" step. The business truth requires a TWO-PHASE flow: (1) shipping resolution (either normal or private quote) THEN (2) buyer payment with 24h deadline. The current code creates the order at claim time, which bypasses the shipping resolution gate entirely.

---

## 2. Locked Business Truth

| Rule | Status |
|------|--------|
| Seller shipping deadline = 24h after auction ends | **LOCKED** |
| Buyer payment deadline = 24h after shipping resolved | **LOCKED** |
| Seller default → auction transaction fails | **LOCKED** |
| Buyer BNR → buyer gets transaction restriction | **LOCKED** |
| Same restriction ladder for buyer BNR and seller default | **LOCKED** |
| Ladder: #1→7d, #2→15d, #3+→30d | **LOCKED** |
| Cumulative, no automatic reset | **LOCKED** |
| No trust score | **LOCKED** |
| Lazy commerce chat (not auto-created for every auction) | **LOCKED** |
| Private quote does not mutate reusable Shipping Setup | **LOCKED** |
| One ACTIVE quote per buyer + commerce context | **LOCKED** |
| Commerce chat/interaction lazy-created when needed | **LOCKED** |

---

## 3. Current Auction Lifecycle

### Entity

**File:** `backend/internal/commerce/auction/entity/auction.go`

States:
```
draft → scheduled → active → waiting_settlement → ended
                                     ↓
                                 expired_bnr
```

Additional terminal states: `cancelled`

### Flow

1. **Creation:** `AuctionService.CreateDraft()` creates auction with product, immediately progresses to scheduled (or active for `StartModeNow`).
2. **Bidding:** `PlaceBid()` with idempotency key, BNR strike check, anti-sniping extension.
3. **Ending:** `AuctionEndWorker` polls every 30s for `active` auctions where `end_at <= NOW()`. Uses `FOR UPDATE SKIP LOCKED` for concurrent safety.
4. **Winner determination:** If `HasWinner() == true`, transitions to `waiting_settlement` with `SettlementDeadline = now + 24h`. If no winner, transitions to `ended`.
5. **Settlement:** Winner claims via `GeneratePricingTokenForAuctionClaim()` → order created via pricing token → auction transitions to `ended`.

### Authority

- **Winner authority:** `Auction.CurrentWinnerID` — set atomically during `PlaceBid()`.
- **Winner finality:** When `AuctionEndWorker` transitions to `waiting_settlement`.
- **Idempotency:** `EndAuctionInternal()` checks `Status == StatusActive` before processing. Worker uses `FOR UPDATE SKIP LOCKED`.
- **No winner:** Transitions to `ended` (terminal). No order created.
- **Worker retry:** Safe — status check prevents re-processing.
- **Concurrent ending:** `FOR UPDATE SKIP LOCKED` prevents two workers from processing the same auction.

### Schema

**Table:** `auctions`
- `settlement_deadline` (timestamptz) — set when entering waiting_settlement
- `order_id` (uuid, nullable, unique) — set atomically at order creation
- `status` (auction_status_enum)
- `current_winner_id` (uuid, nullable)
- `current_bid` (bigint, nullable)
- `anti_snipe_extension_total` (interval, added in migration 000004)

---

## 4. Current Winner Determination

**Authority:** `Auction.CurrentWinnerID` is set by `PlaceBid()` — last bidder wins.

**Finality:** When `AuctionEndWorker.EndAuctionInternal()` transitions from `active` to `waiting_settlement`.

**Settlement deadline:** Set to `now + 24h` at transition time.

**BNR expiry:** `AuctionSettlementWorker` polls every 5 minutes for `waiting_settlement` auctions where `settlement_deadline <= NOW()`. Transitions to `expired_bnr`, emits `auction_bnr_detected` outbox event.

**BNR strike recording:** `BNRStrikeHandler` handles `auction_bnr_detected` events. Inserts strike row with `ON CONFLICT (auction_id) DO NOTHING`. Idempotent.

**BNR evaluation:** `BNRStrikeChecker.Check()` evaluates active (non-decayed, non-reset) strikes:
- 0 → allowed
- 1 → allowed + warning
- 2 → blocked for 14d from last strike
- 3 → blocked for 90d from last strike
- 4+ → permanent ban

**Scope:** Auction bidding only. Does not affect listing purchase, chat, social.

---

## 5. Current Order / Payment / Settlement Flow

### Order Creation from Auction

**Path:** `AuctionService.CreateOrderFromAuction()` → `OrderCreationService.CreateFromAuction()`

**Key flow:**
1. Load buyer address, validate ownership
2. Load canonical product
3. Check shipping options configured
4. Validate sale surface (synthetic ForSale-shaped view)
5. Check delivery availability (skip if using shipping quote)
6. Validate shipping quote (if applicable)
7. Create order with pricing snapshot
8. Mark shipping quote as USED (if applicable)
9. Emit outbox events

**Critical observation:** Order is created in `pending_payment` status with `PaymentExpiresAt` calculated based on payment method (default: 30 minutes for instant, varies for others). There is **no shipping resolution prerequisite** — the order is created immediately upon claim.

### Payment Flow

**Workers:**
- `PaymentExpiryWorker` (1min poll): Scans payments table for expired pending payments
- `OrderPaymentTimeoutWorker` (2min poll): Scans orders for orphan pending_payment orders where `payment_expires_at <= NOW()` AND no active payment row exists

**Payment expiry:** `PaymentExpiresAt` is single source of truth. Workers query this field directly.

### Settlement / Escrow

**Gateway-funded model:** Money lives at payment gateway clearing account. Buyer/seller wallet balances not mutated by escrow lifecycle events.

**Completion flow:** Order → Payment → Shipped → Completed → Escrow released to seller.

### Auction Buy-Now vs Winner Settlement

Both use `CreateFromAuction()`. The `AuctionSettlementType` field distinguishes:
- `buy_now`: Fixed-price checkout
- `bid_win`: Competitive final price

Both allow discounts and coins (owner canonical 2026-06-16).

---

## 6. Current Shipping Flow

### Shipping Setup

**Entity:** `ShippingSetup` — seller-configured shipping options (name, transport type, is_active).

### Shipping Coverage

**Entity:** `ShippingCoverage` — province-level coverage per shipping option (province code, rate, is_available).

### Product Shipping Setup

**Entity:** `ProductShippingSetup` — links products to shipping options they can use.

### Shipping Quote

**Entity:** `ShippingQuote` — manual shipping cost quote from seller.
- Fields: `source_type` (text, nullable), `source_id` (uuid, nullable) — supports "auction" and "for_sale"
- Status: ACTIVE → USED/EXPIRED/INVALID
- Address lock: `destination_city_id`, `destination_province_id`
- Reactivation: SUPPORTED (max 2 reuses)
- Expiry: Default 24h, max 168h (7 days)

### Order Integration

**Order fields:**
- `shipping_source`: "for_sale" or "shipping_quote"
- `shipping_quote_id`: Quote ID (nullable)
- `shipping_quote_price`: Quote cost snapshot
- `shipping_setup_id`: NULLABLE — nil when using shipping quote

### Residue

No significant legacy shipping residue detected. The `estimated_days` and `expedition_name` fields were dropped in migration 000014. The `listing_id` on auctions was dropped in migration 000010.

---

## 7. Current Private Quote Flow

### Create

**Service:** `ShippingQuoteService.CreateShippingQuote()`

**Validation:**
1. Chat exists, seller is participant
2. Auction exists, belongs to seller, status = waiting_settlement
3. Chat recipient is auction winner
4. Cost non-negative
5. Expires within bounds (default 24h)

**Supersession:** Prior unsuperseded quotes for same canonical context (chat_id, product_id, source_type, source_id, seller_id, buyer_id) are superseded.

**Chat message:** Sends `shipping_quote` type message with attachment JSON.

### Validate for Checkout

**Service:** `ShippingQuoteService.ValidateQuoteForCheckout()`

**Validations:**
1. Quote exists and is ACTIVE
2. Not expired
3. Belongs to buyer
4. Belongs to product/sale-surface
5. Checkout address matches locked destination

### Mark Used

Called during order creation to prevent reuse.

### Reactivation

**Service:** `ShippingQuoteService.ReactivateQuoteIfEligible()`

**Triggers:** Order expired, refunded, cancelled, cancelled_timeout.  
**Not triggered for:** Completed, shipped, partially_refunded, dispute_open.

### Architecture for Auction Quotes

The shipping quote system already supports `source_type="auction"` and `source_id=auction_id`. The `validateAuctionForQuote()` method validates:
- Auction exists and belongs to seller
- Status = waiting_settlement
- Winner is set
- Chat recipient is the winner

**Key design:** Quotes are bound to chat_id + buyer_id + product + source. No separate `auction_id` column on the shipping_quotes table — the association is through `source_type`/`source_id`.

---

## 8. Current Auction Winner ↔ Commerce Chat Capability

### Chat Room

**Entity:** `ChatRoom` — supports `RoomTypeDirect` for buyer-seller 1:1 chat.

**Context:** `ContextJSON` (json.RawMessage) — optional commerce context for UI display only. Not used for business logic.

**Lazy creation:** `getOrCreateDirectRoomTx()` — creates room if not exists, updates context if room has no context. Handles race condition with `ErrDuplicateRoom`.

### Current Capability

- Direct room creation between any two users
- Commerce context via `ContextJSON`
- `LinkedOrderID` for order ↔ chat alignment
- `shipping_quote` message type exists

### Missing for Auction Winner

- **No automatic chat creation for every auction** (correct per business truth)
- **No specific "Give Shipping Quote" entry point** for seller → auction winner
- **No lazy creation triggered by "seller needs to provide private quote"** event
- The chat service already supports lazy room creation between seller and buyer — this is sufficient for the auction winner flow

---

## 9. Existing BNR / Restriction Implementation

### A. Current Implementation

**Table:** `buyer_bnr_strikes`
- `buyer_id`, `auction_id`, `struck_at`, `decayed_at`, `admin_reset`
- `UNIQUE(auction_id)` — prevents duplicate strikes per auction

**Strike recording:** `BNRStrikeHandler` → `INSERT ... ON CONFLICT (auction_id) DO NOTHING`

**Strike evaluation:** `BNRStrikeChecker.evaluate()`:
```
0 → allowed
1 → allowed + warning
2 → blocked if last_struck_at + 14d > now
3 → blocked if last_struck_at + 90d > now
4+ → permanent ban
```

**Decay:** `BNRDecayWorker` — daily, decays oldest active strike when MAX(struck_at) > 180 days old.

**Admin reset:** `BNRAdminResetter` — sets `admin_reset = TRUE` on active strikes.

**Telemetry:** `bnr_restriction_check_failed_total` counter for fail-open monitoring.

### B. Divergence from Locked Truth

| Aspect | Current | Locked Truth |
|--------|---------|-------------|
| Ladder | 0→1→2(14d)→3(90d)→4+(permanent) | #1→7d, #2→15d, #3+→30d |
| Scope | Buyer BNR only (auction bidding) | Buyer BNR AND seller shipping default, SAME ladder |
| Symmetry | Buyer only | Symmetric buyer + seller |
| Decay | 180-day automatic decay | **LOCKED: No automatic reset** |
| Trust score | None | **LOCKED: No trust score** |
| Cumulative | Yes (active strikes counted) | Yes (cumulative) |
| Permanent ban | 4+ strikes | **NOT LOCKED** — not mentioned in business truth |

### C. Required Changes for Implementation Pass

1. **Restructure ladder:** Replace evaluate() logic with 7/15/30-day ladder
2. **Remove decay:** Disable `BNRDecayWorker` (or leave but ignore strikes after restructuring)
3. **Add seller restriction:** New table/counter for seller shipping default violations
4. **Unify counters:** Both buyer BNR and seller shipping default use same counter/ladder
5. **Remove permanent ban:** Business truth specifies #3+ → 30d, no permanent ban mentioned
6. **Scope restriction:** Restriction applies to transaction/commerce activities, not full account ban

---

## 10. Existing Deadline / Worker Mechanisms

### Auction End Worker
- **Poll:** 30s
- **Query:** `active` AND `end_at <= NOW()` with `FOR UPDATE SKIP LOCKED`
- **Action:** Transitions to `waiting_settlement` (winner) or `ended` (no winner)

### Auction Settlement Worker (BNR Detection)
- **Poll:** 5min
- **Query:** `waiting_settlement` AND `settlement_deadline <= NOW()` with `FOR UPDATE SKIP LOCKED`
- **Action:** Transitions to `expired_bnr`, emits `auction_bnr_detected` event

### Payment Expiry Worker
- **Poll:** 1min
- **Query:** payments where `status = 'pending'` AND `expired_at < NOW()` with `FOR UPDATE SKIP LOCKED`
- **Action:** Marks payment as expired, cancels order via `OrderService.Expire()`

### Order Payment Timeout Worker
- **Poll:** 2min
- **Query:** orders where `status = 'pending_payment'` AND `payment_expires_at <= NOW()` AND no active payment row
- **Action:** Expires orphan orders via `OrderService.Expire()`

### Order Overdue Cancel Worker
- Cancels orders where seller hasn't shipped past deadline

### Required Workers for New Business Truth

1. **Seller Shipping Deadline Worker:** Detects `waiting_settlement` auctions where seller hasn't provided private quote within 24h
2. **Buyer Payment Deadline Worker:** Already exists as `OrderPaymentTimeoutWorker` (but needs alignment with "24h after shipping resolved" timing)

---

## 11. Current State Machine

### Auction State Machine (Current)

```
draft → scheduled → active → waiting_settlement → ended
                                          ↓
                                      expired_bnr
```

### Auction State Machine (Current — Detailed)

```
active
  ↓ (worker detects end_at <= NOW)
  ├─ Has winner → waiting_settlement (settlement_deadline = now + 24h)
  └─ No winner → ended (terminal)

waiting_settlement
  ↓ (winner claims via pricing token)
  └─ ended (order_id set, terminal)

waiting_settlement
  ↓ (settlement_deadline expired)
  └─ expired_bnr (terminal, no order created)
```

### Order State Machine (Current)

```
pending_payment → paid → shipped → delivered → completed
pending_payment → expired (terminal)
pending_payment → cancelled (terminal)
paid → cancelled_timeout (terminal)
shipped → dispute_open → completed/refunded/partially_refunded
```

---

## 12. Gap Analysis

### Gap 1: Seller Shipping Deadline (MISSING)

**Current:** No concept of "seller must provide shipping quote within 24h."  
**Required:** New mechanism to detect and enforce seller shipping deadline.

**Approach:** Extend `waiting_settlement` state or add new state (e.g., `awaiting_seller_quote`) with a deadline field.

### Gap 2: Two-Phase Settlement (MISSING)

**Current:** Order created immediately at winner claim. No shipping resolution prerequisite.  
**Required:** Shipping must be resolved (normal OR private quote accepted) BEFORE order creation and payment window.

### Gap 3: Buyer Payment Deadline Timing (DIVERGENT)

**Current:** `PaymentExpiresAt` set at order creation (30min default for instant payment).  
**Required:** 24h after shipping resolved, not at order creation.

### Gap 4: Seller Restriction for Shipping Default (MISSING)

**Current:** `buyer_bnr_strikes` only tracks buyer violations. No seller-side mechanism.  
**Required:** Symmetric seller restriction for shipping default.

### Gap 5: Unified Restriction Ladder (DIVERGENT)

**Current:** Ladder is 0→1→2(14d)→3(90d)→4+(permanent).  
**Required:** #1→7d, #2→15d, #3+→30d. Same ladder for buyer and seller.

### Gap 6: Commerce Chat for Auction Winner (PARTIALLY IMPLEMENTED)

**Current:** Direct room creation exists. `shipping_quote` message type exists. Lazy creation exists.  
**Missing:** "Give Shipping Quote" entry point for seller → auction winner. No lazy creation triggered by private quote need.

### Gap 7: Shipping Resolution Gate (MISSING)

**Current:** Order can be created without shipping resolution.  
**Required:** Shipping must be resolved before order creation.

### Gap 8: Normal Shipping Selection Flow (MISSING)

**Current:** Winner claims with shipping setup selection at claim time.  
**Required:** Winner selects shipping setup AFTER winner is determined, as a separate step from payment.

---

## 13. Race / Idempotency Analysis

### Case A: Auction ends → winner → normal shipping available

**Current behavior:** `AuctionEndWorker` transitions to `waiting_settlement`. Winner claims via pricing token, selects shipping setup, order created.

**Expected:** Winner selects shipping setup → shipping resolved → order created → 24h payment window.

**Gap:** No separate shipping selection step. Order creation bundles shipping selection.

**Authority:** `Auction.CurrentWinnerID` is authoritative. `FOR UPDATE` prevents races.

**Idempotency:** `EndAuctionInternal()` checks status before processing. Pricing token prevents double-ordering.

### Case B: Auction ends → winner → private quote required → seller quotes → buyer accepts → payment

**Current behavior:** Seller creates shipping quote via `CreateShippingQuote()`. Quote sent in chat. Buyer accepts (implied by proceeding to checkout). Order created with quote.

**Expected:** Same as current, but with explicit "buyer accepts quote" step and 24h payment deadline.

**Gap:** No explicit acceptance step. Payment deadline is 30min (instant) not 24h.

**Authority:** Quote validation (`ValidateQuoteForCheckout()`) ensures quote is current, not expired, belongs to buyer, matches destination.

**Idempotency:** `MarkQuoteUsed()` prevents reuse. `SupersedeCurrentQuotes()` prevents multiple active quotes.

### Case C: Auction ends → winner → private quote required → seller does nothing for 24h

**Current behavior:** `AuctionSettlementWorker` detects `settlement_deadline <= NOW()`, transitions to `expired_bnr`. BNR strike recorded.

**Expected:** Seller Shipping Default → auction transaction fails, winner gets no penalty, seller gets restriction.

**Gap:** Current BNR strikes buyer (BNR = Bid No Response), not seller. No seller restriction mechanism.

**Authority:** `AuctionSettlementWorker` is authoritative for deadline detection.

**Idempotency:** `TransitionToExpiredBNR()` validates current status before transitioning.

### Case D: Seller quotes at hour 23:59

**Current behavior:** Quote created with default 24h expiry. Seller is within deadline.

**Expected:** Quote created, buyer has time to accept and pay.

**Gap:** Payment deadline should be 24h AFTER quote acceptance/shipping resolved, not 30min from order creation.

### Case E: Seller quote exists → buyer accepts → buyer does not pay for 24h

**Current behavior:** Order created with `PaymentExpiresAt` based on payment method (30min for instant). `OrderPaymentTimeoutWorker` expires order.

**Expected:** 24h payment deadline from shipping resolved. BNR strike if not paid.

**Gap:** Payment deadline is too short (30min vs 24h). BNR flow already exists but timing is wrong.

### Case F: Seller quote exists → buyer rejects / quote superseded

**Current behavior:** New quote supersedes old via `SupersedeCurrentQuotes()`.

**Expected:** Same. Supersession is correct.

**Authority:** Quote supersession is atomic within transaction.

### Case G: Seller and buyer actions near deadline

**Current behavior:** Worker uses `FOR UPDATE SKIP LOCKED` for concurrent safety. Entity validates status before transition.

**Expected:** Same pattern. Critical that worker actions and user actions are serialized via row locks.

**Authority:** `FOR UPDATE` on auction row serializes all mutations.

**Concern:** Payment success vs payment deadline race. If payment succeeds AND deadline expires simultaneously, the payment should win. Current `OrderPaymentTimeoutWorker` checks status in transaction — if already paid, skip.

### Case H: Worker retry after Seller Shipping Default

**Current behavior:** `AuctionSettlementWorker` re-checks status after acquiring lock. If already processed, skips.

**Expected:** Same idempotency pattern for seller shipping default.

### Case I: Worker retry after BNR

**Current behavior:** `BNRStrikeHandler` uses `ON CONFLICT (auction_id) DO NOTHING`. Idempotent.

**Expected:** Same pattern for seller restriction strikes.

### Case J: Payment success with payment deadline

**Current behavior:** `PaymentExpiryWorker` checks payment status after lock. If already settled, skips.

**Expected:** Same. Payment webhook processes before expiry worker runs.

**Mitigation:** Payment webhook uses idempotency keys. Expiry worker checks status before processing.

### Case K: Seller creates multiple quotes

**Current behavior:** `SupersedeCurrentQuotes()` supersedes prior unsuperseded quotes for same canonical context before inserting new one.

**Expected:** Same. One active quote per canonical context.

**Authority:** `SupersedeCurrentQuotes()` + `Create()` in same transaction.

### Case L: Buyer changes/has no destination while quote is being prepared

**Current behavior:** Quote has locked destination (province, city). Checkout validates against locked destination.

**Expected:** Same. Destination is locked at quote creation time.

**Concern:** If buyer changes address between quote creation and checkout, validation will fail. This is correct behavior — quote is destination-specific.

---

## 14. Proposed Canonical State Machine

### Auction State Machine (Proposed)

```
AUCTION ENDED
      ↓
WINNER CONFIRMED
      ↓
SHIPPING RESOLUTION
      │
      ├── NORMAL SHIPPING AVAILABLE
      │       ↓
      │   WINNER SELECTS SHIPPING SETUP
      │       ↓
      │   SHIPPING RESOLVED
      │
      └── PRIVATE QUOTE REQUIRED
              ↓
       SELLER ACTION WINDOW
         (24 HOURS FROM AUCTION END)
              │
          ┌───┴────┐
       QUOTE     TIMEOUT
       CREATED   (SELLER DEFAULT)
          ↓         ↓
     BUYER       AUCTION
     ACCEPTS     TERMINATED
          ↓       (seller restriction)
     SHIPPING
     RESOLVED
          ↓
   BUYER PAYMENT
     WINDOW
     (24 HOURS FROM SHIPPING RESOLVED)
          │
       ┌──┴───┐
      PAID   TIMEOUT
       ↓        ↓
    SUCCESS    BNR
               (buyer restriction)
```

### Mapping to Current Implementation

| Proposed State | Current Implementation | Change Required |
|---------------|----------------------|-----------------|
| Auction Ended | `StatusActive` → `StatusWaitingSettlement` | Already exists |
| Winner Confirmed | `StatusWaitingSettlement` | Already exists |
| Normal Shipping Available | N/A (checked at claim time) | Add shipping resolution check |
| Winner Selects Shipping | Part of claim flow | Separate into distinct step |
| Shipping Resolved | Part of claim flow | Add explicit state or gate |
| Seller Action Window | N/A | New: add `seller_shipping_deadline` field |
| Quote Created | Shipping quote exists | Already exists |
| Buyer Accepts | Implied by checkout | Add explicit acceptance step |
| Seller Default | N/A | New: extend settlement worker |
| Buyer Payment Window | `PaymentExpiresAt` (30min) | Change to 24h from shipping resolved |
| BNR | Already exists | Retime to fire on payment timeout |

### Proposed New States

Option A: Extend `waiting_settlement` with sub-states (via fields, not new enum values):
- `seller_shipping_deadline` field on auction
- `shipping_resolved_at` field on auction
- `requires_private_quote` boolean on auction

Option B: Add new auction states:
- `awaiting_seller_quote` — seller must provide quote within 24h
- `shipping_resolved` — shipping resolved, buyer can pay

**Recommendation:** Option A — avoid extending state machine with new states when fields on existing states can represent the same semantics. The `waiting_settlement` state already exists and is the correct semantic container.

---

## 15. Responsibility Boundary — Seller vs Buyer

### Seller Responsibilities

1. **Provide shipping quote** (if private quote required) within 24h of auction end
2. **Ship item** after payment confirmed (existing flow)
3. **Provide shipping proof** (existing flow)

### Buyer Responsibilities

1. **Select shipping setup** (normal shipping) within settlement deadline
2. **Accept shipping quote** (private quote) — implied by proceeding to payment
3. **Pay within 24h** of shipping resolved
4. **Confirm delivery** (existing flow)

### System Responsibilities

1. **Detect auction end** → transition to waiting_settlement
2. **Determine if private quote required** → based on destination coverage
3. **Enforce seller deadline** → 24h for shipping quote
4. **Enforce buyer deadline** → 24h for payment after shipping resolved
5. **Record violations** → BNR strikes for buyer, seller restriction for default
6. **Create commerce chat** → lazy, when seller initiates "Give Shipping Quote"

---

## 16. Design Options

### Option A: Minimal Extension (Recommended)

**Approach:** Extend existing `waiting_settlement` state with additional fields. Add seller deadline detection worker. Modify order creation to require shipping resolution.

**Pros:**
- Minimal state machine changes
- Leverages existing worker patterns
- Leverages existing shipping quote infrastructure
- Leverages existing BNR infrastructure (with ladder changes)

**Cons:**
- `waiting_settlement` becomes overloaded (buyer claim window + seller shipping window)
- Requires careful field management

**Changes required:**
1. Add `seller_shipping_deadline` (timestamptz) to auctions table
2. Add `requires_private_quote` (boolean) to auctions table
3. Add `shipping_resolved_at` (timestamptz) to auctions table
4. Add `seller_default_count` table (mirror of buyer_bnr_strikes for sellers)
5. Modify `AuctionSettlementWorker` to handle both buyer BNR and seller default
6. Modify `OrderCreationService.CreateFromAuction()` to require shipping resolution
7. Modify payment expiry to use 24h from shipping resolved
8. Restructure BNR ladder to 7/15/30d

### Option B: Separate States

**Approach:** Add `awaiting_seller_quote` and `shipping_resolved` states to auction state machine.

**Pros:**
- Clear state semantics
- Explicit lifecycle stages

**Cons:**
- Larger state machine change
- More migration work
- Higher risk of transition bugs
- May require changes to mobile app state handling

### Option C: Order-Centric Shipping Resolution

**Approach:** Move shipping resolution into order lifecycle. Create order immediately but with `shipping_resolved = false`. Payment window starts when shipping resolved.

**Pros:**
- Order exists from claim time (simpler reference tracking)
- Shipping resolution becomes order lifecycle event

**Cons:**
- Order created before shipping resolved (violates business truth that buyer should see final amount before paying)
- Payment expiry logic more complex (two-phase)
- Requires order state machine extension

---

## 17. Recommended Design

**Option A: Minimal Extension** is recommended.

### Architecture Principles

1. **No parallel state machines** — Extend existing auction and order state machines
2. **No duplicate authority** — Shipping resolution is gate for order creation, not post-creation event
3. **Deterministic** — All deadlines are timestamp-based, worker-driven
4. **Idempotent** — All workers use FOR UPDATE SKIP LOCKED + status checks
5. **Leverages existing infrastructure** — Shipping quotes, chat rooms, payment workers, BNR infrastructure

### Key Design Decisions

1. **Seller shipping deadline:** Add `seller_shipping_deadline` field to auctions table. Set when auction transitions to `waiting_settlement`. Worker detects expiry.

2. **Shipping resolution gate:** `OrderCreationService.CreateFromAuction()` must verify `shipping_resolved_at IS NOT NULL` before creating order. This is the single gate that enforces the two-phase flow.

3. **Payment deadline:** `PaymentExpiresAt` must be calculated as `shipping_resolved_at + 24h`, not `order_created_at + 30min`. Modify order creation to accept explicit payment expiry.

4. **Unified restriction ladder:** Create `commerce_violations` table replacing both `buyer_bnr_strikes` and new seller restriction. Single counter, single ladder.

5. **Commerce chat lazy creation:** When seller clicks "Give Shipping Quote" on auction detail, system creates/finds direct room with winner and opens chat. No automatic creation for every auction.

### Trade-offs

**Trade-off 1: Separate tables vs unified violations table**
- Separate: Simpler queries, easier debugging, but duplicate code
- Unified: Single ladder logic, but more complex queries
- **Decision:** Unified table is cleaner for symmetric ladder

**Trade-off 2: Field on auction vs new state**
- Field: Minimal migration, but state semantics less clear
- New state: Clear semantics, but more complex state machine
- **Decision:** Field approach recommended for minimal change

**Trade-off 3: Order creation timing**
- Create order at claim (current): Simple but violates shipping resolution gate
- Create order after shipping resolved: Correct but requires flow restructure
- **Decision:** Create order after shipping resolved

---

## 18. Owner Decisions Still Required

### Decision 1: Terminal State After Seller Shipping Default

**Question:** When seller fails to provide shipping quote within 24h, what happens to the auction?

**Options:**
- A: Auction becomes `cancelled` (item not sold)
- B: Auction becomes `expired_bnr` (reusing existing terminal state with different semantics)
- C: New terminal state `seller_default` (explicit semantics)

**Recommendation:** Option C — New terminal state for clarity.

### Decision 2: Item Availability After Seller Default

**Question:** After seller shipping default, can the item be relisted?

**Options:**
- A: Yes — seller can create new auction for same product
- B: No — product is marked as sold/unavailable
- C: Depends on admin action

**Recommendation:** Option A — Product entity is separate from auction. New auction can reference same product.

### Decision 3: Winner Benefit After Seller Default

**Question:** Does winner get any benefit (priority, discount) when re-auctioned?

**Options:**
- A: No — clean slate, new auction
- B: Yes — winner gets priority or first-look
- C: Depends on platform policy

**Recommendation:** Option A — Clean slate. Winner can bid again.

### Decision 4: Payment Deadline Exact Timestamp

**Question:** When exactly does the 24h payment deadline start?

**Options:**
- A: When buyer accepts shipping quote (explicit action)
- B: When shipping resolution timestamp is recorded (system action)
- C: When buyer proceeds to checkout with resolved shipping

**Recommendation:** Option B — Deterministic, auditable, no user action dependency.

### Decision 5: Restriction Scope

**Question:** What does "transaction restriction" mean specifically?

**Options:**
- A: Cannot bid on auctions
- B: Cannot create orders (buy)
- C: Cannot sell (create listings/auctions)
- D: All of the above (full commerce restriction)

**Recommendation:** Needs clarification. Business truth says "membatasi aktivitas transaksi/commerce sesuai scope canonical."

### Decision 6: Normal Shipping Selection Window

**Question:** If normal shipping is available, how long does winner have to select shipping?

**Options:**
- A: Same 24h as seller shipping deadline
- B: Fixed 48h window
- C: No explicit deadline (uses settlement deadline)

**Recommendation:** Option C — Use existing settlement deadline (24h from auction end).

---

## 19. Implementation Acceptance Criteria

### Seller

- [ ] Seller has 24h from auction end to provide private shipping quote (if required)
- [ ] Seller default triggers: auction terminated, seller restriction recorded
- [ ] Seller restriction uses same ladder as buyer BNR (7/15/30d)
- [ ] Seller restriction is cumulative, no automatic reset
- [ ] Seller cannot bypass restriction by creating new account

### Buyer

- [ ] Shipping resolved prerequisite for order creation
- [ ] 24h payment deadline from shipping resolved (not order creation)
- [ ] BNR strike recorded if buyer fails to pay within deadline
- [ ] BNR uses same ladder as seller restriction (7/15/30d)
- [ ] BNR is cumulative, no automatic reset
- [ ] BNR applies to auction bidding (existing scope)

### Shipping

- [ ] Normal shipping setup selection available when destination is covered
- [ ] Private quote required when destination not covered OR seller chooses
- [ ] Quote acceptance triggers shipping resolution
- [ ] Quote supersession prevents multiple active quotes
- [ ] Destination validation on checkout matches quote-locked destination
- [ ] Shipping resolved timestamp recorded on auction

### Chat

- [ ] Commerce chat lazy-created when seller initiates "Give Shipping Quote"
- [ ] Chat participants: seller + auction winner
- [ ] Authorization: only seller can create quote, only winner can accept
- [ ] Shipping quote message type used for quote delivery
- [ ] No automatic chat creation for every auction

### Payment

- [ ] Final payable amount = winning bid + shipping cost (from quote or setup)
- [ ] Payment intent created after shipping resolved
- [ ] Payment expiry = shipping_resolved_at + 24h
- [ ] Existing Midtrans semantics preserved (definitive failure vs ambiguous)
- [ ] Escrow/ledger authority unchanged
- [ ] PaymentExpiryWorker uses payment_expires_at (already canonical)

### Workers

- [ ] Seller shipping deadline detection worker (new or extended)
- [ ] Buyer payment deadline worker (existing, retime)
- [ ] Idempotency via FOR UPDATE SKIP LOCKED + status checks
- [ ] Retry-safe via entity validation guards

### Race Conditions

- [ ] Concurrent timeout/action: Row lock serializes mutations
- [ ] Payment vs timeout: Payment webhook wins (check status after lock)
- [ ] Duplicate quote: Supersession prevents multiple active quotes
- [ ] Duplicate settlement: UNIQUE(auction_id) on orders + OrderID guard

### Cleanup

- [ ] Obsolete BNR ladder (14d/90d/permanent) replaced with 7/15/30d
- [ ] BNR decay worker disabled or removed (no automatic reset per locked truth)
- [ ] Payment expiry timing aligned to 24h after shipping resolved
- [ ] No compatibility code for old ladder

---

## 20. Cleanup / Residue Requirements

### Must Remove

1. **BNRDecayWorker logic** — Business truth: no automatic reset. The 180-day decay is incompatible with locked truth.

2. **Old BNR ladder** — 14d/90d/permanent replaced by 7/15/30d.

3. **Payment expiry default 30min** — Replaced by 24h from shipping resolved.

### Must Modify

1. **`BNRStrikeChecker.evaluate()`** — Replace ladder logic.
2. **`AuctionSettlementWorker`** — Add seller default detection.
3. **`OrderCreationService.CreateFromAuction()`** — Add shipping resolution gate.
4. **Order creation payment expiry calculation** — Use shipping_resolved_at + 24h.

### Must Add

1. **`commerce_violations` table** — Unified violation tracking for buyer and seller.
2. **`seller_shipping_deadline` field** on auctions — 24h deadline for seller.
3. **`shipping_resolved_at` field** on auctions — When shipping was resolved.
4. **`requires_private_quote` field** on auctions — Whether private quote is needed.
5. **Seller restriction enforcement** — Block seller from creating listings/auctions when restricted.
6. **"Give Shipping Quote" entry point** — UI/API for seller to initiate private quote for auction winner.

---

## 21. Evidence & Files Audited

### Backend — Auction Domain
- `backend/internal/commerce/auction/entity/auction.go` — State machine, entity, methods
- `backend/internal/commerce/auction/application/auction_service.go` — Service layer, order creation
- `backend/internal/commerce/auction/application/bnr_restriction.go` — BNR strike evaluation
- `backend/internal/commerce/auction/application/bnr_telemetry.go` — Metrics
- `backend/internal/commerce/auction/delivery/http/` — HTTP handlers

### Backend — Order Domain
- `backend/internal/commerce/order/entity/order.go` — Order entity, state machine
- `backend/internal/commerce/order/entity/order_status.go` — Status transitions
- `backend/internal/commerce/order/entity/auction_settlement_type.go` — Settlement types
- `backend/internal/commerce/order/application/order_creation_service.go` — Order creation from auction
- `backend/internal/commerce/order/ORDER_STATE_MACHINE_DIAGRAM.md` — Documentation

### Backend — Shipping Domain
- `backend/internal/commerce/shipping/entity/shipping_setup.go` — Shipping setup entity
- `backend/internal/commerce/shipping/entity/shipping_coverage.go` — Coverage entity
- `backend/internal/commerce/shipping/SHIPPING_DOMAIN_SCHEMA.md` — Schema documentation
- `backend/internal/commerce/shipping/quote/entity/shipping_quote.go` — Quote entity
- `backend/internal/commerce/shipping/quote/application/shipping_quote_service.go` — Quote service

### Backend — Chat Domain
- `backend/internal/interaction/chat/entity/chat_room.go` — Room entity
- `backend/internal/interaction/chat/entity/room_type.go` — Room types
- `backend/internal/interaction/chat/entity/message_type.go` — Message types
- `backend/internal/interaction/chat/application/chat_service.go` — Chat service, lazy room creation

### Backend — Workers
- `backend/internal/worker/auction_end_worker.go` — Auction end detection
- `backend/internal/worker/auction_settlement_worker.go` — BNR detection
- `backend/internal/worker/bnr_strike_handler.go` — Strike recording
- `backend/internal/worker/bnr_decay_worker.go` — Strike decay
- `backend/internal/worker/bnr_admin_reset.go` — Admin reset
- `backend/internal/worker/payment_expiry_worker.go` — Payment expiry
- `backend/internal/worker/order_payment_timeout_worker.go` — Order timeout
- `backend/internal/worker/seller_non_shipment_rule.go` — Seller non-shipment detection

### Database
- `backend/migrations/000001_canonical_schema.up.sql` — Base schema (auctions, buyer_bnr_strikes, shipping_quotes, orders)
- `backend/migrations/000004_auction_anti_sniping.up.sql` — Anti-sniping extension
- `backend/migrations/000010_product_sale_channel_canonicalization.up.sql` — Removed listing_id from auctions
- `backend/migrations/000046_product_content_authority_convergence.up.sql` — Removed title/description from auctions

### Mobile App
- `apps/mobile/lib/domains/commerce/catalog/auction/` — Auction domain (entity, data, presentation)
- `apps/mobile/lib/domains/commerce/transaction/checkout/` — Checkout flow

---

## 22. Final Verdict

**DESIGN READY WITH OWNER DECISIONS**

### What Is Canonical

1. Auction lifecycle state machine (draft → scheduled → active → waiting_settlement → ended/expired_bnr)
2. BNR strike recording and idempotent handling
3. Settlement deadline (24h) on auctions
4. Order creation from auction via pricing token
5. Shipping quote entity with auction source_type support
6. Lazy direct room creation in chat
7. Payment expiry workers (PaymentExpiryWorker + OrderPaymentTimeoutWorker)
8. Gateway-funded escrow model
9. ForSale/Auction/Order separation of concerns

### What Is Missing

1. Seller shipping deadline mechanism (24h from auction end)
2. Two-phase settlement (shipping resolution → payment)
3. Buyer payment deadline (24h from shipping resolved)
4. Seller restriction for shipping default
5. Unified restriction ladder (buyer + seller)
6. "Give Shipping Quote" entry point for seller → auction winner
7. Shipping resolution gate for order creation
8. Explicit buyer payment deadline timing (24h vs 30min)

### What Is Divergent

1. BNR ladder (14d/90d/permanent vs 7/15/30d)
2. BNR automatic decay (180d vs no reset)
3. Payment expiry timing (30min vs 24h from shipping resolved)
4. Settlement scope (buyer-only BNR vs symmetric buyer+seller)

### What Must Be Implemented

1. `commerce_violations` unified table
2. `seller_shipping_deadline` field on auctions
3. `shipping_resolved_at` field on auctions
4. `requires_private_quote` field on auctions
5. Seller shipping deadline worker
6. Shipping resolution gate in order creation
7. Payment expiry retime (24h from shipping resolved)
8. BNR ladder restructuring (7/15/30d)
9. Seller restriction enforcement
10. "Give Shipping Quote" UI/API entry point

### What Must Be Cleaned Up

1. BNRDecayWorker logic (remove automatic decay)
2. Old BNR ladder (14d/90d/permanent)
3. Payment expiry default (30min)
4. BNR permanent ban concept (not in locked truth)

### Owner Decisions Remaining

1. Terminal state after seller default (cancelled vs new state)
2. Item availability after seller default
3. Winner benefit after seller default
4. Payment deadline exact start timestamp
5. Restriction scope (auction-only vs full commerce)
6. Normal shipping selection window
