# FINAL CANONICAL CONVERGENCE AUDIT — AUCTION SETTLEMENT + WINNER SHIPPING

**Pass:** Final Canonical Convergence Audit (read-only — no implementation)
**Date:** 2026-09-02
**Authority:** Current filesystem implementation truth, locked business truth
**Verdict:** `NOT READY — TECHNICAL CORRECTION REQUIRED`

---

## 1. Verdict

**NOT READY — TECHNICAL CORRECTION REQUIRED**

Four P0 contradictions prevent implementation. The current auction state machine structurally cannot represent `waiting_settlement → DRAFT`. No commerce restriction system exists. No shipping resolution gate exists. Payment expiry timing is wrong.

---

## 2. Executive Finding

The locked business truth requires:
- Settlement failure → DRAFT (not terminal)
- Cross-commerce restriction (buyer + seller, For Sale + Auction)
- Shipping resolution before order creation
- Payment deadline = shipping_resolved_at + 24h

The current implementation has:
- `expired_bnr` as terminal state (no outgoing transitions)
- `buyer_bnr_strikes` (buyer-only, auction-only, wrong ladder)
- No `shipping_resolved_at` field (bundled in `/claim`)
- Method-based payment expiry (15min–6hr, not 24h)

**The recommended design direction (no intermediate state, direct `waiting_settlement → DRAFT`) is architecturally sound.** The state machine extension is straightforward. The main effort is replacing the BNR system with cross-commerce violations and decomposing the claim endpoint.

---

## 3. Locked Business Truth

### 3.1 Product Selling Surfaces

Product has exactly two selling surfaces: FOR SALE and AUCTION.

### 3.2 Settlement Failures → DRAFT

| Failure | Violating Actor | Restriction | Seller Can Relist |
|---------|----------------|-------------|-------------------|
| Buyer shipping selection failure | Buyer | Yes (7/15/30d) | YES (immediately) |
| Seller shipping default | Seller | Yes (7/15/30d) | NO (during restriction) |
| Buyer BNR | Buyer | Yes (7/15/30d) | YES (immediately) |

### 3.3 Deadlines

| Deadline | Anchor | Duration |
|----------|--------|----------|
| Buyer shipping selection | auction_end | 24h |
| Seller private quote | auction_end | 24h |
| Buyer payment | shipping_resolved_at | 24h |

### 3.4 Restriction

- One authority, buyer + seller, cross-commerce
- Ladder: 1st→7d, 2nd→15d, 3rd+→30d
- Cumulative, no automatic reset, no trust score, no permanent ban
- NOT account ban

### 3.5 No Automatic Relisting

### 3.6 No Automatic Winner Compensation

---

## 4. Auction State Machine Audit

### 4.1 Current States

**File:** `backend/internal/commerce/auction/entity/auction.go` lines 66–84

```
draft, scheduled, active, waiting_settlement, expired_bnr, ended, cancelled
```

**Schema:** `backend/migrations/000001_canonical_schema.up.sql` lines 36–44

```sql
CREATE TYPE auction_status_enum AS ENUM (
    'draft', 'scheduled', 'active', 'waiting_settlement',
    'expired_bnr', 'ended', 'cancelled'
);
```

### 4.2 transitionAllowed (Current)

```go
StatusDraft:             {StatusScheduled, StatusCancelled}
StatusScheduled:         {StatusActive, StatusCancelled, StatusDraft}
StatusActive:            {StatusWaitingSettlement, StatusEnded, StatusCancelled}
StatusWaitingSettlement: {StatusEnded, StatusExpiredBNR, StatusCancelled}
StatusExpiredBNR:        {}  // TERMINAL — no outgoing transitions
StatusEnded:             {}  // TERMINAL
StatusCancelled:         {}  // TERMINAL
```

### 4.3 Required Transition: `waiting_settlement → DRAFT`

**Business truth:** All three settlement failures return auction to DRAFT.

**Current state:** `StatusWaitingSettlement` can go to `ended`, `expired_bnr`, or `cancelled`. NOT to `draft`.

**Recommended direction:** Remove the intermediate terminal state. Settlement failure produces `waiting_settlement → DRAFT` atomically.

**Required change to `transitionAllowed`:**
```go
StatusWaitingSettlement: {StatusEnded, StatusDraft, StatusCancelled},
//                       Remove expired_bnr as target
//                       Add draft as target
```

**`expired_bnr` becomes obsolete.** No `expired_settlement` intermediate state needed.

### 4.4 `expired_bnr` Consumer Audit

| Consumer | File | Classification |
|----------|------|---------------|
| `StatusExpiredBNR` constant | `auction.go:77` | OBSOLETE (remove) |
| `transitionAllowed[StatusExpiredBNR]` | `auction.go:101` | OBSOLETE (remove) |
| `TransitionToExpiredBNR()` method | `auction.go:409` | OBSOLETE (remove) |
| `BNRAuctionRestrictedError` struct | `auction.go:222` | OBSOLETE (replace with CommerceRestrictedError) |
| `PublicLifecycle()` switch | `auction.go:136` | Must remove `StatusExpiredBNR` case |
| `IsRepostable()` | `auction.go:115` | Returns false — correct (DRAFT also returns false) |
| `IsPublicDiscoverable()` | `auction.go:162` | Returns false — correct |
| `AuctionSettlementWorker` | `auction_settlement_worker.go:31,292,313,319` | DECOMMISSION |
| `BNRStrikeHandler.Handle()` | `bnr_strike_handler.go:31` | DECOMMISSION |
| `notification_worker_commerce.go:handleAuctionBNRDetected` | notification handler | DECOMMISSION |
| `outbox_worker.go` registration | `outbox_worker.go:1125,1138` | DECOMMISSION |
| `outbox_event_registry.go` comment | `outbox_event_registry.go:141` | Remove |
| `moderation_event_handler.go:623,652` | moderation handler | Update (expired_bnr → draft transition) |
| `shared/view_access.go:23` | view access constant | REMOVE `auctionStatusExpiredBNR` |
| `shared/view_access.go:EvaluateAuctionViewAccess` | view access | Remove expired_bnr case |
| `auction_admin_cancel_test.go:105` | test | UPDATE |
| `auction_moderation_test.go:47,54,58` | test | UPDATE |
| `claim_error_test.go:41,141` | test | UPDATE |
| `admin_order_source_status_test.go:90` | test | UPDATE |
| Mobile `auction_status.dart:43,47,73,119,147` | Dart enum | RENAME/REMOVE |
| Mobile `auction_dto.dart:486-487` | Dart DTO | UPDATE |
| Admin `orders.ts:292,306` | TypeScript | RENAME label |
| `search_repository_impl.go:612` | search | Remove from comment |
| `publiccard/auction_card.go:18` | public card | Remove from comment |
| `bidding_service.go:160,199` | bidding | Update |
| `dependencies.go:1619` | bootstrap | Update |
| `.env.example:260` | config | Update |
| `dev-reset-data/main.go:86` | tool | No change (table name) |

### 4.5 `settlement_deadline` Consumer Audit

| Consumer | File | Classification |
|----------|------|---------------|
| `Auction.SettlementDeadline` field | `auction.go:153` | OBSOLETE (remove) |
| `TransitionToWaitingSettlement()` sets it | `auction.go:363-365` | OBSOLETE (remove field set) |
| `AuctionSettlementWorker` queries it | `auction_settlement_worker.go:229` | DECOMMISSION |
| `GeneratePricingTokenForAuctionClaim` checks it | `auction_service.go:862` | UPDATE to `buyer_shipping_deadline` |
| `AuctionRepository.CreateTx` SQL | `auction_repository.go:33` | UPDATE (remove column) |
| `joinedAuctionColumns` SQL | `auction_repository.go:68` | UPDATE |
| `UpdateTx` SQL | `auction_repository.go:209` | UPDATE |
| `chat_auction_projection_resolver.go:138` | chat projection | UPDATE |
| `chat_resource_projection_aggregate_resolver_integration_test.go:216` | test | UPDATE |
| `chat_content_projection_resolver_integration_test.go:250` | test | UPDATE |
| `chat_auction_projection_resolver_integration_test.go:191` | test | UPDATE |
| Mobile `auction_dto.dart:486-487` | Dart DTO | UPDATE |
| Mobile `auction_status.dart:42` | Dart comment | UPDATE |
| DB constraint `auction_order_consistency` | migration:2436 | Unaffected (checks order_id) |

---

## 5. Auction Attempt Reset / DRAFT Relist Audit

### 5.1 What Happens When Auction Returns to DRAFT

When settlement fails, `waiting_settlement → DRAFT`:

| Attribute | Action | Reason |
|-----------|--------|--------|
| `ID` | RETAIN | Same auction attempt |
| `SellerID` | RETAIN | Same seller |
| `ProductID` | RETAIN | Same product |
| `OrderID` | CLEAR (set NULL) | No order exists |
| `SettlementDeadline` | CLEAR (set NULL) | No deadline |
| `StartPrice` | RETAIN | Seller's original pricing |
| `BidIncrement` | RETAIN | Seller's original increment |
| `BuyNowPrice` | RETAIN | Seller's original buy-now |
| `StartAt` | RETAIN | Original start time |
| `EndAt` | RETAIN | Original end time |
| `AntiSnipeExtensionTotal` | RETAIN | Already applied to EndAt |
| `CurrentBid` | CLEAR (set NULL) | No current bid |
| `CurrentWinnerID` | CLEAR (set NULL) | No winner |
| `Status` | SET to `draft` | Return to workspace |
| `CreatedAt` | RETAIN | Original creation time |
| `UpdatedAt` | SET to now | Mutation timestamp |
| `Product` | RETAIN | Read-only reference |

### 5.2 Bid History

| Data | Action | Reason |
|------|--------|--------|
| `auction_bids` rows | RETAIN | Audit trail. Not cleared. |
| `CurrentBid` on auction | CLEAR | Auction entity state |
| `CurrentWinnerID` on auction | CLEAR | Auction entity state |

Bid history rows are historical records. They must remain for audit. The auction entity's `CurrentBid` and `CurrentWinnerID` are attempt-specific state that must reset.

### 5.3 Product SellingSurface

The product's `SellingSurface` is set to `SellingSurfaceAuction` at auction creation (`ClaimSellingSurface`). When the auction returns to DRAFT, the product remains claimed by this auction. This is correct — only one non-terminal auction per product is allowed by `uniq_active_auction_per_product`.

**No Product row change needed.**

### 5.4 Shipping Quotes

Active shipping quotes for this auction (source_type="auction", source_id=auction_id) remain in their current state. If a quote was ACTIVE (not yet used), it remains ACTIVE. If it was USED, it remains USED.

When the auction is re-scheduled and re-activates, new quotes can be created. Old quotes will expire naturally (24h default).

**No shipping quote cleanup needed at DRAFT return.**

### 5.5 Chat References

Chat rooms created for the auction winner remain. They are not cleaned up. When the auction returns to DRAFT and a new winner is determined, a new chat room will be created for the new winner.

**No chat cleanup needed.**

### 5.6 Notifications

Existing notifications for the previous attempt remain in the notification table. New notifications will be sent for the new attempt.

**No notification cleanup needed.**

### 5.7 Uniqueness Constraint Compatibility

```sql
CREATE UNIQUE INDEX uniq_active_auction_per_product ON auctions (product_id)
WHERE (status = ANY (ARRAY['draft', 'scheduled', 'active', 'waiting_settlement']));
```

DRAFT is in the uniqueness condition. When the auction returns to DRAFT, only ONE auction per product can be in DRAFT/scheduled/active/waiting_settlement. This is correct — the seller cannot create a second auction for the same product while the first is in DRAFT.

**No constraint change needed.**

### 5.8 Compatibility with Current Implementation

The `ReleaseUnpaidOrder` method (called when order is cancelled/expired) clears `OrderID` but keeps status as `ended`. This is the current behavior for post-order-failure cleanup.

**With the new model:** Settlement failure happens BEFORE order creation (shipping resolution gate). No order exists at settlement failure time. `ReleaseUnpaidOrder` is not needed for settlement failure — only for post-order-failure cleanup (which remains unchanged).

### 5.9 DRAFT Return — State Machine Proof

```
waiting_settlement → DRAFT:
  1. FOR UPDATE lock on auction row
  2. Re-check: status == waiting_settlement AND order_id IS NULL
  3. Clear: order_id, settlement_deadline, current_winner_id, current_bid
  4. Set: status = draft, updated_at = now
  5. Emit outbox event: auction.settlement_failed
  6. Record violation in commerce_violations
  7. COMMIT
```

**Invariant:** `order_id IS NULL` at step 2 ensures no order was created. If an order was created (race with claim), the auction would have transitioned to `ended` (via `Settle()`), and step 2 would fail.

---

## 6. Shipping Resolution Authority Audit

### 6.1 Current State

No `shipping_resolved_at` field exists on the auction entity. Shipping resolution is bundled into the `/claim` endpoint as a single atomic operation.

### 6.2 Required Authority

`shipping_resolved_at` must be:
- Set exactly once per auction
- Immutable once set
- The single indicator that shipping has been resolved
- The anchor for payment deadline (shipping_resolved_at + 24h)

### 6.3 Resolution Paths

| Path | How `shipping_resolved_at` Is Set |
|------|----------------------------------|
| Normal: buyer selects Shipping Setup | Backend validates setup covers address → `shipping_resolved_at = NOW()` |
| Private: buyer accepts seller's quote | Backend validates quote is ACTIVE → `shipping_resolved_at = NOW()` |

### 6.4 Invariant: AT MOST ONE VALID RESOLUTION

**Proof:**
1. Both paths acquire `FOR UPDATE` on auction row
2. Both paths check `shipping_resolved_at IS NULL` after lock
3. First commit sets `shipping_resolved_at` to non-NULL
4. Second commit sees non-NULL and returns error/skips
5. No other code path sets `shipping_resolved_at`

**Authority:** The auction row itself is the serialization point. `FOR UPDATE` ensures exactly one writer.

### 6.5 Post-Resolution Immutability

Once `shipping_resolved_at != NULL`:
- Normal selection endpoint returns error ("shipping already resolved")
- Quote acceptance endpoint returns error ("shipping already resolved")
- Order creation checks `shipping_resolved_at IS NOT NULL` (gate)
- No UPDATE path exists that clears `shipping_resolved_at`

---

## 7. Case A Audit (Destination Not Covered)

### 7.1 Required Flow

```
Buyer submits address
→ Coverage check: no Shipping Setup covers address
→ requires_private_quote = true
→ Seller has 24h (auction_end + 24h) to create private quote
→ Buyer accepts quote
→ shipping_resolved_at = NOW()
```

### 7.2 Current Implementation Gap

| Requirement | Current | Gap |
|-------------|---------|-----|
| Address submission as separate step | Bundled in `/claim` | P0 |
| Coverage check at address submission | Not implemented | P0 |
| `requires_private_quote` field | Does not exist | P0 |
| Seller deadline at auction end | Not set | P1 |
| `shipping_resolved_at` field | Does not exist | P0 |
| Separate shipping resolution | Not implemented | P0 |
| Payment at shipping_resolved_at + 24h | Not implemented | P0 |

### 7.3 Case A Verdict

**CANNOT be represented** with current implementation. Requires endpoint decomposition and new fields.

---

## 8. Case B Audit (Seller Override)

### 8.1 Required Flow

```
Buyer submits address
→ Coverage check: Shipping Setup covers address
→ requires_private_quote = false (coverage exists)
→ Seller voluntarily creates private quote (special price)
→ Buyer accepts quote
→ shipping_resolved_at = NOW()
```

### 8.2 Existing ShippingQuote Lifecycle as Sole Authority

**File:** `backend/internal/commerce/shipping/quote/entity/shipping_quote.go`

The `ShippingQuote` entity can represent Case B:

| Capability | Evidence |
|------------|----------|
| Seller creates quote for auction | `validateAuctionForQuote()` checks `status == waiting_settlement` |
| Quote bound to auction | `source_type="auction"`, `source_id=auction_id` |
| Destination lock | `destination_city_id`, `destination_province_id` |
| One active quote per context | `SupersedeCurrentQuotes()` supersedes prior quotes |
| Quote expiry | 24h default, 168h max |
| Quote acceptance | Implicit via checkout (MarkUsed) |
| Quote reactivation | USED → ACTIVE on order failure |

**No new boolean/field needed.** The existence of an active private quote IS the Case B indicator.

### 8.3 Case B Race with Normal Selection

**Race:** Seller creates private quote while buyer selects normal shipping.

**Current state:** No guard. Both can proceed simultaneously.

**Fix:** `shipping_resolved_at` atomic guard on both paths. First to set `shipping_resolved_at` wins.

### 8.4 Case B Verdict

**CAN be represented** with existing `ShippingQuote` lifecycle + `shipping_resolved_at` atomic guard. No new fields needed beyond `shipping_resolved_at`.

---

## 9. Private Quote Acceptance Audit

### 9.1 Current Acceptance Mechanism

**There is NO explicit acceptance.** Quote acceptance is implicit:
1. Seller creates quote → quote is ACTIVE
2. Buyer proceeds to checkout with the quote
3. `validateShippingQuoteForOrder()` validates quote
4. Quote is marked USED atomically

### 9.2 Acceptance Window Analysis

**Proposed:** buyer has 24h from quote creation to accept.

**Current quote expiry:** 24h default from creation. This IS the acceptance window.

**Conflict with buyer_shipping_deadline:**
- If seller creates quote at T+23:59 (just before seller deadline)
- Quote expires at T+47:59
- But `buyer_shipping_deadline = auction_end + 24h = T+24:00`
- Buyer only has 1 minute!

**Resolution:** The `buyer_shipping_deadline` must be interpreted as "shipping must be resolved by this time." When the private quote path is active, the buyer's obligation is to accept the quote before it expires. The seller's obligation is to create the quote before the seller deadline.

If the seller creates a quote at T+23:59, the buyer has until T+47:59 (quote expiry) to accept. The `buyer_shipping_deadline` is not the binding constraint for the private quote path — the quote expiry is.

**This is a genuinely unresolved design question.** The business truth says "buyer has 24h from auction_end to select shipping." In the private quote path, the buyer cannot act until the seller provides a quote. The buyer's 24h obligation should be interpreted as "buyer must be ready to accept within 24h" — but if the seller provides a quote late, the buyer gets the quote's remaining lifetime.

### 9.3 Verdict

**OWNER DECISION REQUIRED.** The interaction between `buyer_shipping_deadline` and late-created private quotes is unresolved. The existing quote expiry (24h) provides a technical bound, but the business rule about buyer's 24h obligation needs clarification for the private quote path.

---

## 10. Deadline Audit

### 10.1 Three Distinct Clocks

| # | Deadline | Anchor | Duration | Worker |
|---|----------|--------|----------|--------|
| 1 | Buyer shipping selection | auction_end | 24h | BuyerShippingDeadlineWorker (NEW) |
| 2 | Seller private quote | auction_end | 24h | SellerShippingDeadlineWorker (NEW) |
| 3 | Buyer payment | shipping_resolved_at | 24h | OrderPaymentTimeoutWorker (EXISTING, modify) |

### 10.2 Current Settlement Deadline

**File:** `auction.go:363-365`
```go
func (a *Auction) TransitionToWaitingSettlement() error {
    // ...
    deadline := time.Now().Add(24 * time.Hour)
    a.SettlementDeadline = &deadline
}
```

**Current consumer:** `AuctionSettlementWorker` queries `settlement_deadline <= NOW()`.

**With new model:** `settlement_deadline` is replaced by `buyer_shipping_deadline` and `seller_shipping_deadline`. Both set at auction end as `auction_end + 24h`.

### 10.3 Payment Expiry for Auctions

**Current:** `calculatePaymentExpiry(method, now)` = 15min–6hr.

**Required:** `shipping_resolved_at + 24h`.

**For Sale:** UNCHANGED. `calculatePaymentExpiry(method, now)` remains correct.

**Implementation:** Auction order creation must bypass `calculatePaymentExpiry` and use explicit `payment_expires_at = shipping_resolved_at + 24h`.

### 10.4 Timezone / Boundary

All deadlines use `time.Time` (UTC). No timezone assumptions. Boundary at exactly deadline: `deadline <= NOW()` means the deadline has PASSED. Worker processes at the exact deadline moment.

### 10.5 Worker Ownership

| Worker | Deadline | Poll | Action |
|--------|----------|------|--------|
| `AuctionEndWorker` | Sets deadlines at auction_end | 30s | Set buyer/seller deadlines |
| `SellerShippingDeadlineWorker` (NEW) | seller_shipping_deadline | 30s | Detect seller default |
| `BuyerShippingDeadlineWorker` (NEW) | buyer_shipping_deadline | 30s | Detect buyer shipping failure |
| `OrderPaymentTimeoutWorker` | payment_expires_at | 2min | Detect BNR (auction orders) |

---

## 11. Failure Outcome Matrix

### 11.1 Buyer Shipping Selection Failure

| Attribute | Value |
|-----------|-------|
| TRIGGER | buyer_shipping_deadline <= NOW() AND shipping_resolved_at IS NULL AND requires_private_quote = false |
| ACTOR | Buyer (winner) |
| VIOLATION | `shipping_selection_failure` |
| AUCTION RESULT | `waiting_settlement → DRAFT` |
| RELIST PERMISSION | Seller may immediately relist |
| RESTRICTION EFFECT | Buyer receives 7/15/30d cross-commerce restriction |
| PAYMENT EFFECT | No order exists, no payment |
| SHIPPING EFFECT | No resolution, no quotes used |
| ORDER EFFECT | No order created |
| NOTIFICATION/EVENT | `auction.settlement_failed` (reason: shipping_selection_failure) |

### 11.2 Seller Shipping Default

| Attribute | Value |
|-----------|-------|
| TRIGGER | seller_shipping_deadline <= NOW() AND no active quote exists AND requires_private_quote = true |
| ACTOR | Seller |
| VIOLATION | `shipping_default` |
| AUCTION RESULT | `waiting_settlement → DRAFT` |
| RELIST PERMISSION | Seller CANNOT relist during restriction |
| RESTRICTION EFFECT | Seller receives 7/15/30d cross-commerce restriction |
| PAYMENT EFFECT | No order exists, no payment |
| SHIPPING EFFECT | No resolution |
| ORDER EFFECT | No order created |
| NOTIFICATION/EVENT | `auction.settlement_failed` (reason: seller_default) |

### 11.3 Buyer BNR

| Attribute | Value |
|-----------|-------|
| TRIGGER | payment_expires_at <= NOW() AND order.status = pending_payment AND source_type = auction |
| ACTOR | Buyer (winner) |
| VIOLATION | `bnr` |
| AUCTION RESULT | Order expires. Auction status depends on current state. |
| RELIST PERMISSION | Seller may immediately relist |
| RESTRICTION EFFECT | Buyer receives 7/15/30d cross-commerce restriction |
| PAYMENT EFFECT | Order expires, escrow refunded if held |
| SHIPPING EFFECT | Shipping quote reactivated if eligible |
| ORDER EFFECT | Order → expired, OrderID released from auction |
| NOTIFICATION/EVENT | `order.expired` + `auction.bnr_detected` (NEW event from order expiry) |

**Critical:** BNR currently fires from `AuctionSettlementWorker` (settlement_deadline). In the new model, BNR fires from `OrderPaymentTimeoutWorker` when auction order payment expires. The order must exist for BNR to fire.

### 11.4 Buyer Private Quote Acceptance Failure

**Currently unresolved.** If the buyer does not accept the quote before the quote expires:
- The quote becomes EXPIRED
- No `shipping_resolved_at` is set
- The `buyer_shipping_deadline` has likely also passed
- This falls under buyer shipping selection failure (11.1)

**Verdict:** No separate failure type needed. Quote expiry + buyer deadline expiry = buyer shipping selection failure.

---

## 12. Restriction Authority Audit

### 12.1 Current BNR System

| Component | File | Purpose |
|-----------|------|---------|
| `buyer_bnr_strikes` table | migration:575 | Strike storage |
| `BNRStrikeChecker` | `bnr_restriction.go` | Evaluate: 0→allow, 1→warning, 2→14d, 3→90d, 4+→permanent |
| `BNRStrikeHandler` | `bnr_strike_handler.go` | Record strike: INSERT ON CONFLICT(auction_id) DO NOTHING |
| `BNRDecayWorker` | `bnr_decay_worker.go` | Daily decay after 180d |
| `BNRAdminResetter` | `bnr_admin_reset.go` | Admin reset strikes |
| `AuctionSettlementWorker` | `auction_settlement_worker.go` | Detect timeout, emit event |

### 12.2 Incompatibility with Locked Truth

| Aspect | Current | Required |
|--------|---------|----------|
| Ladder | 0→1→2(14d)→3(90d)→4+(permanent) | 1→7d, 2→15, 3+→30d |
| Scope | Buyer auction bidding only | Buyer + Seller, For Sale + Auction |
| Decay | 180d automatic | NO decay |
| Permanent ban | 4+ strikes | NO permanent ban |
| Symmetry | Buyer only | Symmetric buyer + seller |
| Source | auction_bnr_detected event | Multiple failure types |

### 12.3 `buyer_bnr_strikes` Schema vs Required

| Attribute | `buyer_bnr_strikes` | Required |
|-----------|---------------------|----------|
| Actor | `buyer_id` only | `actor_id` + `actor_type` (buyer/seller) |
| Source | `auction_id` only | `source_id` + `source_type` (auction/for_sale) |
| Violation type | Implicit (BNR) | Explicit (`bnr`, `shipping_selection_failure`, `shipping_default`) |
| Duplicate | `UNIQUE(auction_id)` | `UNIQUE(source_id, source_type, violation_type, actor_type)` |
| Decay | `decayed_at` column | None |
| Admin reset | `admin_reset` column | TBD (owner decision) |

### 12.4 Obsolete BNR Components

| Component | Classification | Action |
|-----------|---------------|--------|
| `buyer_bnr_strikes` table | OBSOLETE | DROP (after migration) |
| `BNRStrikeChecker` | OBSOLETE | REPLACE with `CommerceRestrictionChecker` |
| `BNRStrikeHandler` | OBSOLETE | REPLACE with `CommerceViolationHandler` |
| `BNRDecayWorker` | OBSOLETE | DECOMMISSION |
| `BNRAdminResetter` | OBSOLETE | REMOVE or REPLACE (pending owner decision) |
| `BNRAuctionRestrictedError` | OBSOLETE | REPLACE with `CommerceRestrictedError` |
| `bnr_restriction_check_failed_total` metric | OBSOLETE | UPDATE name/scope |
| `auction_bnr_detected` event | OBSOLETE | RENAME to `auction.settlement_failed` |
| `buyer_bnr_strikes_buyer_active` index | OBSOLETE | DROP with table |
| Ladder 14d/90d/permanent | OBSOLETE | REPLACE with 7/15/30d |

---

## 13. Restriction Stacking Gap

### 13.1 Proposed Behavior

Restrictions stack cumulatively.

### 13.2 Open Question

When violation #2 occurs during active restriction from #1:

**Option A:** New restriction period starts from violation #2 date.
```
restriction_end = violation_2_created_at + ladder_days(count=2)
```

**Option B:** New restriction period stacks on top of existing.
```
restriction_end = current_restriction_end + ladder_days(count=2)
```

### 13.3 Technical Implications

**Option A (latest resets clock):**
- Simple formula: `restriction_end = MAX(created_at) + ladder(COUNT(*))`
- Each violation independently extends from its own timestamp
- After restriction expires, next violation starts fresh from its date

**Option B (stacking):**
- Complex: must track current restriction end
- Each violation adds to remaining time
- Requires computing `current_restriction_end` before adding

### 13.4 Verdict

**OWNER DECISION REQUIRED.** Both options are technically implementable. The choice affects the restriction duration formula. Option A is simpler. Option B is stricter.

---

## 14. Commerce Violation Authority Audit

### 14.1 Candidate: `commerce_violations` Table

```sql
CREATE TABLE commerce_violations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id        UUID NOT NULL REFERENCES users(id),
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('buyer', 'seller')),
    violation_type  TEXT NOT NULL CHECK (violation_type IN (
        'bnr', 'shipping_selection_failure', 'shipping_default'
    )),
    source_id       UUID NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'auction',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 14.2 Required Constraint

```sql
UNIQUE (source_id, source_type, violation_type, actor_type)
```

**NOT** `UNIQUE(source_id, source_type, violation_type)` — that would prevent the same auction from penalizing both buyer (for BNR) and seller (for shipping default) if they shared a violation type. Including `actor_type` prevents this.

### 14.3 Invariant: One Terminal Violation Per Auction Per Actor Per Type

The UNIQUE constraint guarantees:
- Same source + same actor_type + same violation_type = ONE violation maximum
- Different actors on same source = allowed (buyer + seller on same auction)
- Same actor on different sources = allowed (different auctions)

**This is sufficient for the locked business truth.**

### 14.4 Relation to Restriction

The violation table records violations. The restriction is DERIVED:
```sql
-- Check if actor is restricted
-- Count violations, compute restriction end based on ladder
SELECT COUNT(*) as violation_count
FROM commerce_violations
WHERE actor_id = $1 AND actor_type = $2;
-- restriction_end depends on count (7/15/30d) and stacking model
```

### 14.5 Duplicate Authority Check

| Potential Duplicate | Resolution |
|--------------------|-----------|
| `buyer_bnr_strikes` + `commerce_violations` | `buyer_bnr_strikes` is OBSOLETE. Migrate data, then drop. |
| `AuctionSettlementWorker` + new deadline workers | `AuctionSettlementWorker` is DECOMMISSIONED. |
| `BNRStrikeChecker` + `CommerceRestrictionChecker` | `BNRStrikeChecker` is REPLACED. |
| Subscription check + restriction check | Different authorities. Both needed. Subscription ≠ restriction. |

### 14.6 Verdict

**`commerce_violations` is sufficient as canonical authority** with the `actor_type` constraint fix. No duplicate authority survives after cleanup.

---

## 15. Seller Capability Enforcement Audit

### 15.1 `HasActiveSellerCapability()` — Current

**File:** `backend/internal/identity/auth/role_checker_db.go:53`

Checks:
1. Account operational (not suspended/banned)
2. Seller profile exists
3. Subscription active (`started_at <= NOW() < expires_at`)

**Does NOT check:** Commerce restriction.

### 15.2 Required Enforcement Points

| Gate | Currently Checks | Must Also Check |
|------|-----------------|----------------|
| `Schedule()` (auction) | Ownership + subscription + shipping coverage | + commerce restriction |
| `PlaceBid()` (auction) | BNR strike check | + commerce restriction (buyer) |
| `CreateDraft()` (auction) | Account status | (workspace — no restriction needed) |
| For Sale creation | Subscription | + commerce restriction (seller) |
| For Sale checkout | Account status | + commerce restriction (buyer) |

### 15.3 Verdict

`HasActiveSellerCapability()` must be extended to include commerce restriction check. Or a separate `CommerceRestrictionChecker` must be added as an independent gate.

---

## 16. Payment Deadline Audit

### 16.1 Current Payment Expiry

**File:** `order_creation_service.go:33-44`

```go
func calculatePaymentExpiry(paymentMethod string, createdAt time.Time) time.Time {
    switch paymentMethod {
    case PaymentMethodInstant: return createdAt.Add(15 * time.Minute)
    case PaymentMethodVA:      return createdAt.Add(1 * time.Hour)
    case PaymentMethodRetail:  return createdAt.Add(6 * time.Hour)
    default:                   return createdAt.Add(30 * time.Minute)
    }
}
```

### 16.2 Auction Payment Expiry Must Be

```
payment_expires_at = shipping_resolved_at + 24h
```

### 16.3 For Sale Payment Expiry — UNCHANGED

```
payment_expires_at = calculatePaymentExpiry(method, now)
```

### 16.4 Implementation

Auction order creation must bypass `calculatePaymentExpiry`:
```go
// In CreateFromAuction:
if input.AuctionSettlementType == AuctionSettlementBidWin || input.AuctionSettlementType == AuctionSettlementBuyNow {
    order.PaymentExpiresAt = shippingResolvedAt.Add(24 * time.Hour)
} else {
    order.PaymentExpiresAt = calculatePaymentExpiry(snapshot.PaymentMethod, time.Now())
}
```

### 16.5 BNR Detection from Payment Expiry

**Current:** `AuctionSettlementWorker` detects settlement_deadline expiry → BNR.

**Required:** `OrderPaymentTimeoutWorker` detects payment_expires_at expiry for auction orders → BNR.

The order must exist for BNR to fire. BNR fires AFTER order creation, not before.

---

## 17. Worker / Retry / Concurrency Audit

### 17.1 Worker Migration

| Worker | Current | Required |
|--------|---------|----------|
| `AuctionEndWorker` | Sets `settlement_deadline` | Set `buyer_shipping_deadline` + `seller_shipping_deadline` |
| `AuctionSettlementWorker` | Detects timeout → expired_bnr | DECOMMISSION |
| `BNRDecayWorker` | Daily decay | DECOMMISSION |
| `SellerShippingDeadlineWorker` | DOES NOT EXIST | NEW: detect seller default |
| `BuyerShippingDeadlineWorker` | DOES NOT EXIST | NEW: detect buyer shipping failure |
| `OrderPaymentTimeoutWorker` | Expires orphan orders | MODIFY: emit BNR event for auction orders |
| `PaymentExpiryWorker` | Expires pending payments | UNCHANGED |

### 17.2 SellerShippingDeadlineWorker

```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND requires_private_quote = true
  AND seller_shipping_deadline <= NOW()
  AND shipping_resolved_at IS NULL
  AND order_id IS NULL
FOR UPDATE SKIP LOCKED
```

**Guard after lock:** Re-check status, check no quote exists, check no order exists.

### 17.3 BuyerShippingDeadlineWorker

```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND requires_private_quote = false
  AND shipping_resolved_at IS NULL
  AND buyer_shipping_deadline <= NOW()
  AND order_id IS NULL
FOR UPDATE SKIP LOCKED
```

**Guard after lock:** Re-check status, check `shipping_resolved_at IS NULL`.

### 17.4 Concurrency Proof

**Invariant:** One auction = one terminal settlement outcome.

**Proof:**
1. `FOR UPDATE` on auction row serializes all mutations
2. After lock: re-check `status == waiting_settlement`
3. After lock: re-check `order_id IS NULL` (no order created by claim)
4. After lock: re-check `shipping_resolved_at IS NULL` (no resolution)
5. Terminal action: set `status = draft`, clear settlement fields, record violation
6. COMMIT

**First commit wins.** Second sees non-NULL `shipping_resolved_at` or non-WAITING_SETLEMENT status and skips.

### 17.5 Race: Worker vs Claim

If claim succeeds (order created, auction → ended) while worker is processing:
- Worker acquires lock
- Re-checks status → sees `ended` (not `waiting_settlement`)
- Skips

If worker succeeds (auction → draft) while claim is processing:
- Claim acquires lock
- Re-checks status → sees `draft` (not `waiting_settlement`)
- Returns `ErrNotClaimable`

---

## 18. Chat / Notification Consumer Audit

### 18.1 Current Event Consumers

| Event | Consumer | Action |
|-------|----------|--------|
| `auction.waiting_settlement` | `handleAuctionWaitingSettlement` | Notify winner + seller |
| `auction_bnr_detected` | `handleAuctionBNRDetected` | Notify seller + winner |
| `auction_bnr_detected` | `BNRStrikeHandler` | Record strike |
| `order.expired` | `handleOrderExpired` | Notify buyer |

### 18.2 Required Event Changes

| Old Event | New Event | Consumer Change |
|-----------|-----------|----------------|
| `auction_bnr_detected` | `auction.settlement_failed` | Update payload (add reason: bnr/shipping_default/shipping_selection_failure) |
| — (NEW) | `auction.settlement_failed` | Notify affected parties based on reason |
| `order.expired` | `order.expired` (UNCHANGED) | Add BNR emission for auction orders |

### 18.3 Missing Notifications

| Event | Currently Notified | Required |
|-------|--------------------|----------|
| Seller shipping default | Via `auction_bnr_detected` → `auction.bnr_seller` | Rename to `auction.seller_default` |
| Buyer shipping selection failure | Not notified | Add notification |
| Auction returned to DRAFT | Not notified | Add notification (seller) |

### 18.4 Verdict

Existing notification infrastructure is sufficient. Events need renaming and payload enrichment. New notification types needed for seller default and buyer shipping failure.

---

## 19. Mobile Contract Audit

### 19.1 Auction Status

**File:** `apps/mobile/lib/domains/commerce/catalog/auction/domain/entities/auction_status.dart`

| Current | Required |
|---------|----------|
| `expiredBNR` | REMOVE (no intermediate state) |
| `waitingSettlement` | RETAIN (still used) |

**Parse function:** Must handle backend removing `expired_bnr` from responses.

### 19.2 Auction DTO

**File:** `apps/mobile/lib/domains/commerce/catalog/auction/data/dto/auction_dto.dart`

| Field | Current | Required |
|-------|---------|----------|
| `settlementDeadline` | Parsed from JSON | RENAME to `buyerShippingDeadline` |
| `status` (expired_bnr) | Parsed | REMOVE handling |
| — | — | ADD: `sellerShippingDeadline` |
| — | — | ADD: `shippingResolvedAt` |
| — | — | ADD: `requiresPrivateQuote` |

### 19.3 Claim Flow

**Current:** Single `/claim` endpoint (address + shipping + payment).

**Required:** Decomposed flow:
1. Submit address → coverage check
2. Select shipping (normal) OR wait for seller quote (private)
3. Accept quote (private)
4. Settle (create order)

Mobile must implement new screens for decomposed flow.

### 19.4 Countdowns

**Current:** Single `settlementDeadline` countdown.

**Required:** Three countdowns:
1. Buyer shipping selection deadline
2. Seller private quote deadline (conditional)
3. Buyer payment deadline (after shipping resolved)

---

## 20. Admin Contract Audit

### 20.1 Source Status Labels

**File:** `apps/admin/src/types/orders.ts:291-307`

| Current | Required |
|---------|----------|
| `expired_bnr: 'Expired (BNR)'` | REMOVE |
| `expired_bnr: 'error'` | REMOVE |
| — | ADD: handle DRAFT status in source context |

### 20.2 Admin BNR Management

**Current:** `BNRAdminResetter` allows admin reset.

**Locked truth:** Admin reset is pending owner decision.

**Verdict:** If admin reset is approved, it must work with `commerce_violations` table, not `buyer_bnr_strikes`.

---

## 21. Database / Migration Audit

### 21.1 Schema Changes Required

| Change | Table | Type | Risk |
|--------|-------|------|------|
| ADD `buyer_shipping_deadline` | auctions | timestamptz | Low |
| ADD `seller_shipping_deadline` | auctions | timestamptz | Low |
| ADD `shipping_resolved_at` | auctions | timestamptz | Low |
| ADD `requires_private_quote` | auctions | boolean, NOT NULL, default false | Low |
| DROP `settlement_deadline` | auctions | column | Low (from-zero) |
| CREATE `commerce_violations` | new table | — | Low |
| ADD `expired_settlement` to enum | auction_status_enum | enum value | Low |
| MIGRATE `buyer_bnr_strikes` data | — | data migration | Medium |
| DROP `buyer_bnr_strikes` | table | — | Low |
| RENAME `expired_bnr` rows | auctions | data migration | Medium |
| DROP `expired_bnr` from enum | auction_status_enum | type recreation | Medium |

### 21.2 Enum Migration (expired_bnr → DRAFT direct)

Since the recommended direction is NO intermediate state, the `expired_bnr` enum value is simply removed after all rows are migrated. No `expired_settlement` is needed.

**Migration order:**
1. Add new columns (additive, no downtime)
2. Create `commerce_violations` table
3. Add DRAFT as valid transition from `waiting_settlement` (code change)
4. Migrate existing `expired_bnr` rows to `draft` (or `ended` if they have orders)
5. Remove `expired_bnr` from `transitionAllowed` (code change)
6. Drop `expired_bnr` from enum (requires type recreation)
7. Migrate `buyer_bnr_strikes` data to `commerce_violations`
8. Drop `buyer_bnr_strikes` table
9. Drop `settlement_deadline` column

### 21.3 CHECK Constraint

```sql
ALTER TABLE auctions ADD CONSTRAINT auction_order_consistency
CHECK ((order_id IS NULL) OR (status = 'ended'));
```

**With DRAFT return:** When auction returns to DRAFT, `order_id` must be NULL (no order exists at settlement failure). The CHECK constraint is satisfied: `order_id IS NULL` is true.

**No constraint change needed.**

### 21.4 `uniq_active_auction_per_product` Index

DRAFT is included. When auction returns to DRAFT, only one auction per product can be in non-terminal states. Correct.

**No index change needed.**

---

## 22. Residue Audit

### 22.1 `expired_bnr`

| Location | File | Classification |
|----------|------|---------------|
| `StatusExpiredBNR` constant | `auction.go:77` | OBSOLETE |
| `transitionAllowed` entry | `auction.go:101` | OBSOLETE |
| `TransitionToExpiredBNR()` | `auction.go:409` | OBSOLETE |
| `BNRAuctionRestrictedError` | `auction.go:222` | OBSOLETE |
| `PublicLifecycle()` case | `auction.go:136` | OBSOLETE |
| `AuctionSettlementWorker` | `auction_settlement_worker.go` | OBSOLETE |
| `BNRStrikeHandler` | `bnr_strike_handler.go` | OBSOLETE |
| `handleAuctionBNRDetected` | `notification_worker_commerce.go` | OBSOLETE |
| `outbox_worker.go` registration | `outbox_worker.go:1125,1138` | OBSOLETE |
| `moderation_event_handler.go` | lines 623,652 | STALE (update) |
| `shared/view_access.go` | constant + switch | OBSOLETE |
| `auction_admin_cancel_test.go` | test | STALE TEST |
| `auction_moderation_test.go` | test | STALE TEST |
| `claim_error_test.go` | test | STALE TEST |
| `admin_order_source_status_test.go` | test | STALE TEST |
| Mobile `auction_status.dart` | Dart enum | OBSOLETE |
| Mobile `auction_dto.dart` | Dart DTO | STALE |
| Admin `orders.ts` | TypeScript | STALE |
| `search_repository_impl.go` | comment | STALE DOC |
| `publiccard/auction_card.go` | comment | STALE DOC |
| `bidding_service.go` | code | STALE |
| `dependencies.go` | bootstrap | STALE |
| `.env.example` | config | STALE |
| `dev-reset-data/main.go` | tool | CANONICAL (table name) |
| DB enum `expired_bnr` | migration | OBSOLETE |
| `product_public_availability_stage6b_integration_test.go` | test | STALE TEST |

### 22.2 `settlement_deadline`

| Location | File | Classification |
|----------|------|---------------|
| `Auction.SettlementDeadline` field | `auction.go:153` | OBSOLETE |
| `TransitionToWaitingSettlement()` | `auction.go:363-365` | OBSOLETE |
| `AuctionSettlementWorker` query | `auction_settlement_worker.go:229` | OBSOLETE |
| `GeneratePricingTokenForAuctionClaim` | `auction_service.go:862` | STALE (update) |
| `AuctionRepository` SQL | `auction_repository.go:33,68,209` | STALE (update) |
| `chat_auction_projection_resolver.go` | `chat_auction_projection_resolver.go:138` | STALE (update) |
| 3 integration tests | serverboot tests | STALE TEST |
| Mobile `auction_dto.dart` | Dart DTO | STALE |
| Mobile `auction_status.dart` | Dart comment | STALE |
| DB column `settlement_deadline` | migration | OBSOLETE |

### 22.3 `buyer_bnr_strikes`

| Location | File | Classification |
|----------|------|---------------|
| DB table | migration:575 | OBSOLETE |
| `BNRStrikeHandler` SQL | `bnr_strike_handler.go:72` | OBSOLETE |
| `BNRDecayWorker` SQL | `bnr_decay_worker.go:175-192` | OBSOLETE |
| `BNRAdminResetter` SQL | `bnr_admin_reset.go:52,87` | OBSOLETE |
| `BNRStrikeChecker` query | `bnr_restriction.go:110` | OBSOLETE |
| `bnr_telemetry.go` | metric | OBSOLETE |
| `dev-reset-data/main.go` | tool | STALE |
| 2 test files | tests | STALE TEST |
| `outbox_worker.go:1120` | comment | STALE DOC |

### 22.4 `AuctionSettlementWorker`

| Location | File | Classification |
|----------|------|---------------|
| Worker struct + methods | `auction_settlement_worker.go` | OBSOLETE |
| Worker test | `auction_settlement_worker_test.go` | STALE TEST |
| Bootstrap wiring | `dependencies.go:1619` | STALE |
| `.env.example` | config | STALE |

### 22.5 `BNRDecayWorker`

| Location | File | Classification |
|----------|------|---------------|
| Worker struct + methods | `bnr_decay_worker.go` | OBSOLETE |
| Worker test | `bnr_decay_worker_test.go` | STALE TEST |
| Bootstrap wiring | `dependencies.go` | STALE |

### 22.6 `BNRAdminResetter`

| Location | File | Classification |
|----------|------|---------------|
| Resetter struct + methods | `bnr_admin_reset.go` | OBSOLETE |
| Resetter test | `bnr_admin_reset_test.go` | STALE TEST |
| Bootstrap wiring | `dependencies.go` | STALE |

---

## 23. Duplicate Authority Audit

### 23.1 Settlement Timeout Detection

| Authority 1 | Authority 2 | Conflict? |
|-------------|-------------|-----------|
| `AuctionSettlementWorker` (settlement_deadline) | `SellerShippingDeadlineWorker` + `BuyerShippingDeadlineWorker` (new) | YES — must decommission old |

### 23.2 BNR / Restriction Check

| Authority 1 | Authority 2 | Conflict? |
|-------------|-------------|-----------|
| `BNRStrikeChecker` (buyer_bnr_strikes) | `CommerceRestrictionChecker` (commerce_violations) | YES — must replace old |

### 23.3 Seller Capability

| Authority 1 | Authority 2 | Conflict? |
|-------------|-------------|-----------|
| `HasActiveSellerCapability()` (subscription) | Commerce restriction (new) | NO — complementary, both needed |

### 23.4 Payment Expiry

| Authority 1 | Authority 2 | Conflict? |
|-------------|-------------|-----------|
| `calculatePaymentExpiry()` (method-based) | `shipping_resolved_at + 24h` (auction) | YES for auctions — must use new for auctions, keep old for For Sale |

### 23.5 Verdict

All duplicate authorities identified. Each has a clear resolution path. No undetected duplicates found.

---

## 24. P0 Findings

### P0-1: State Machine Cannot Return to DRAFT

**File:** `auction.go:101`
**Evidence:** `StatusExpiredBNR: {}` — terminal, no outgoing transitions.
**Business Impact:** Settlement failure cannot return auction to DRAFT. Seller cannot relist after buyer failure.
**Technical Impact:** State machine extension required. Add `StatusWaitingSettlement → StatusDraft` transition.
**Required Correction:** Modify `transitionAllowed` and add `ReturnToDraft()` method.

### P0-2: No Commerce Restriction System

**Evidence:** No `commerce_violations` or equivalent table exists. `buyer_bnr_strikes` is buyer-only, auction-only. `HasActiveSellerCapability()` checks subscription only.
**Business Impact:** Restricted seller can still create For Sale. Restricted buyer can still purchase. Seller shipping default has no enforcement.
**Technical Impact:** New table, new checker, new enforcement gates.
**Required Correction:** Create `commerce_violations` table with `CommerceRestrictionChecker`.

### P0-3: No Shipping Resolution Gate

**Evidence:** `CreateFromAuction()` has no `shipping_resolved_at` check. `/claim` bundles shipping selection and order creation.
**Business Impact:** Order created without shipping resolution. No separate payment window.
**Technical Impact:** New fields, endpoint decomposition, gate in order creation.
**Required Correction:** Add `shipping_resolved_at` field and gate.

### P0-4: Payment Expiry Timing Wrong

**File:** `order_creation_service.go:33-44`
**Evidence:** `calculatePaymentExpiry()` uses method-based timing (15min–6hr).
**Business Impact:** Buyer has 30min (default) instead of 24h to pay.
**Technical Impact:** Conditional payment expiry for auction orders.
**Required Correction:** For auction orders: `payment_expires_at = shipping_resolved_at + 24h`.

---

## 25. P1 Findings

### P1-1: Case B Race Condition

**Evidence:** No guard prevents concurrent normal selection + private quote creation.
**Fix:** `shipping_resolved_at` atomic guard on both paths.

### P1-2: Seller Deadline Not Implemented

**Evidence:** No `seller_shipping_deadline` field exists.
**Fix:** Add field, set at auction end, new worker.

### P1-3: Duplicate Violation Constraint

**Evidence:** `UNIQUE(source_id, source_type, violation_type)` without `actor_type` is insufficient.
**Fix:** `UNIQUE(source_id, source_type, violation_type, actor_type)`.

### P1-4: BNR Detection from Wrong Source

**Evidence:** BNR fires from `AuctionSettlementWorker` (settlement_deadline). Should fire from `OrderPaymentTimeoutWorker` (payment expiry).
**Fix:** Modify `OrderPaymentTimeoutWorker` to emit BNR event for auction orders.

### P1-5: `expired_bnr` Semantic Conflict

**Evidence:** All three failure types conflated into one "BNR" state.
**Fix:** Remove `expired_bnr`. Use `waiting_settlement → DRAFT` directly.

---

## 26. P2 Findings

### P2-1: Admin Reset of Violations

**Evidence:** `BNRAdminResetter` exists for `buyer_bnr_strikes`.
**Status:** OWNER DECISION REQUIRED — whether admin reset is allowed for `commerce_violations`.

### P2-2: Private Quote Acceptance Window

**Evidence:** Quote expiry (24h) is the technical bound. Interaction with `buyer_shipping_deadline` for late-created quotes is unresolved.
**Status:** OWNER DECISION REQUIRED.

### P2-3: Restriction Stacking

**Evidence:** Two options (latest resets clock vs stacking). Both technically valid.
**Status:** OWNER DECISION REQUIRED.

### P2-4: Mobile/Admin Contract Updates

**Evidence:** `expired_bnr`, `settlement_deadline`, `claim` references throughout mobile and admin.
**Status:** STALE — must update after backend changes.

---

## 27. Owner Decisions Still Required

| # | Decision | Options | Impact |
|---|----------|---------|--------|
| 1 | **Private quote acceptance window** | A: Quote expiry controls (24h from creation); B: Fixed window after quote creation; C: Tied to buyer_shipping_deadline | Worker, lifecycle stall |
| 2 | **Restriction stacking** | A: Latest violation resets clock; B: Stack restrictions | Restriction duration |
| 3 | **Admin manual violation reset** | A: Allow; B: Disallow | Admin mechanism |

All three are genuinely business decisions, not technical design choices.

---

## 28. Exact Implementation Scope After Approval

### 28.1 Backend Changes

1. **State machine:** Add `StatusWaitingSettlement → StatusDraft` in `transitionAllowed`
2. **Entity:** Add `ReturnToDraft()` method (clears OrderID, SettlementDeadline, CurrentWinnerID, CurrentBid)
3. **New fields:** `buyer_shipping_deadline`, `seller_shipping_deadline`, `shipping_resolved_at`, `requires_private_quote` on `auctions`
4. **Drop field:** `settlement_deadline`
5. **New table:** `commerce_violations`
6. **New service:** `CommerceRestrictionChecker` with 7/15/30 ladder
7. **New endpoints:** `/submit-address`, `/select-shipping`, `/accept-quote`, `/settle`
8. **Modify:** `/claim` → `/settle` (or keep `/claim` as alias)
9. **Gate:** Order creation checks `shipping_resolved_at IS NOT NULL`
10. **Fix:** Auction payment expiry = `shipping_resolved_at + 24h`
11. **New workers:** `SellerShippingDeadlineWorker`, `BuyerShippingDeadlineWorker`
12. **Modify:** `AuctionEndWorker` sets new deadline fields
13. **Modify:** `OrderPaymentTimeoutWorker` emits BNR event for auction orders
14. **Enforcement:** Add commerce restriction check to `Schedule()`, `PlaceBid()`, For Sale creation, For Sale checkout
15. **Event rename:** `auction_bnr_detected` → `auction.settlement_failed` (with reason)

### 28.2 Mobile Changes

1. Remove `expiredBNR` from `AuctionStatus` enum
2. Update `AuctionDto` for new fields
3. Implement decomposed claim flow (address → shipping → settle)
4. Add three countdown timers
5. Handle quote acceptance in chat
6. Update all `expired_bnr` and `settlement_deadline` references

### 28.3 Admin Changes

1. Update `sourceStatusLabels` (remove `expired_bnr`)
2. Update `sourceStatusVariants`
3. Add `commerce_violations` management view (if admin reset approved)
4. Update all `expired_bnr` and `settlement_deadline` references

---

## 29. Cleanup Scope After Implementation

| Component | Action |
|-----------|--------|
| `expired_bnr` enum value | DROP from enum |
| `settlement_deadline` column | DROP from auctions |
| `buyer_bnr_strikes` table | DROP (after data migration) |
| `AuctionSettlementWorker` | DELETE file + tests |
| `BNRDecayWorker` | DELETE file + tests |
| `BNRAdminResetter` | DELETE file + tests (or保留 if admin reset approved) |
| `BNRStrikeHandler` | DELETE file + tests |
| `BNRStrikeChecker` | DELETE file + tests |
| `BNR telemetry` | UPDATE metric names |
| `expired_bnr` in mobile | DELETE enum value + update parse function |
| `expired_bnr` in admin | DELETE label/variant entries |
| `settlement_deadline` in mobile | DELETE field + update DTO |
| `settlement_deadline` in admin | N/A (not in admin types) |
| `settlement_deadline` in chat projection | UPDATE SQL |
| `settlement_deadline` in auction repository | UPDATE SQL |
| `claim` endpoint (optional) | RENAME to `settle` |
| Stale tests | UPDATE or DELETE |
| Stale docs | UPDATE |

---

## 30. Regression Proof Required

### 30.1 State Machine

- [ ] `waiting_settlement → DRAFT` transition works for all three failure types
- [ ] `DRAFT → scheduled → active` re-entry works after DRAFT return
- [ ] `uniq_active_auction_per_product` constraint satisfied after DRAFT return
- [ ] Bid history preserved but not active after DRAFT return
- [ ] Order creation blocked when status is DRAFT

### 30.2 Shipping Resolution

- [ ] `shipping_resolved_at` set exactly once per auction
- [ ] Concurrent normal selection + private quote creation: first wins
- [ ] Order creation blocked when `shipping_resolved_at IS NULL`
- [ ] Post-resolution: no shipping mutation possible

### 30.3 Payment

- [ ] Auction payment expires at `shipping_resolved_at + 24h`
- [ ] For Sale payment expiry unchanged
- [ ] BNR fires from order payment timeout (not settlement deadline)

### 30.4 Restriction

- [ ] Buyer shipping failure → buyer restriction, seller unrestricted
- [ ] Seller default → seller restriction, buyer unrestricted
- [ ] Buyer BNR → buyer restriction, seller unrestricted
- [ ] Restriction blocks For Sale + Auction commerce
- [ ] Restriction does NOT block non-commerce activity
- [ ] Cumulative count correct across failure types
- [ ] Ladder: 1→7d, 2→15d, 3+→30d

### 30.5 Concurrency

- [ ] FOR UPDATE serializes all settlement mutations
- [ ] Worker retry is idempotent (re-check after lock)
- [ ] Duplicate violation prevented by UNIQUE constraint
- [ ] Order creation and settlement failure cannot both succeed

---

## 31. Final Verdict

**NOT READY — TECHNICAL CORRECTION REQUIRED**

### Blocking Issues (P0)

1. **State machine:** `expired_bnr` is terminal. Cannot return to DRAFT. Fix: add `waiting_settlement → DRAFT` transition, remove `expired_bnr`.
2. **No commerce restriction:** `buyer_bnr_strikes` is buyer-only, auction-only. Fix: create `commerce_violations` with `CommerceRestrictionChecker`.
3. **No shipping resolution gate:** `/claim` bundles everything. Fix: add `shipping_resolved_at` field and gate.
4. **Payment expiry wrong:** Method-based (15min–6hr). Fix: `shipping_resolved_at + 24h` for auctions.

### Can We Now Implement Without Creating Another Branch of Legacy?

**NO.** The four P0 contradictions must be resolved first. After resolution:

**YES** — the smallest exact implementation scope is:
- State machine extension (1 file change + 1 new method)
- 4 new fields on auctions (1 migration)
- 1 new table `commerce_violations` (1 migration)
- 1 new service `CommerceRestrictionChecker` (1 new file)
- Endpoint decomposition (modify existing + add new)
- 2 new workers (2 new files)
- 4 enforcement gate additions (modify existing)
- Mobile/admin contract updates (update existing)

**Total new files:** ~5 (2 workers, 1 service, 2 migrations)
**Total modified files:** ~15 (entity, service, handler, workers, mobile, admin)
**Total deleted files:** ~8 (old BNR system, settlement worker, decay worker)

The architecture is clean. No hidden state. No duplicate authority. No inconsistent business logic — after the corrections.

### SOURCE CODE CHANGED: NO

This report is audit-only. No backend, mobile, admin, migration, or test files were modified.
