# AUCTION WINNER SHIPPING, SETTLEMENT FAILURE & RELIST LIFECYCLE — FINAL FORENSIC AUDIT

**Pass:** Final Forensic Audit (read-only — no implementation)
**Date:** 2026-09-02
**Scope:** Complete lifecycle audit of auction winner shipping, settlement, failure, restriction, relist
**Authority:** Current filesystem implementation truth, locked business truth, v3 technical design
**Verdict:** `NOT READY — TECHNICAL CORRECTION REQUIRED`

---

## 1. Executive Summary

The v3 technical design proposes a canonical architecture for auction winner shipping, settlement, payment, restriction, and relist. This audit examines the current filesystem implementation against the locked business truth and v3 design to determine whether the lifecycle can be represented without ambiguity or duplicate authority.

**Critical finding:** The architecture contains **multiple P0 contradictions** that prevent the locked business truth from being implemented without structural changes to the auction state machine, the restriction system, the claim flow, and the payment expiry model. The v3 design is partially correct but fails to resolve several fundamental incompatibilities between the current implementation and the locked business truth.

### Summary of Findings

| Area | Status | Severity |
|------|--------|----------|
| Failure → DRAFT transition | **IMPOSSIBLE with current state machine** | P0 |
| Commerce restriction system | **DOES NOT EXIST** | P0 |
| Shipping resolution gate | **DOES NOT EXIST** | P0 |
| Payment expiry timing | **DIVERGENT** | P0 |
| Case B (seller override) race | **UNGUARDED** | P1 |
| Seller deadline enforcement | **NOT IMPLEMENTED** | P1 |
| Duplicate violation prevention | **INCOMPLETE** | P1 |
| `expired_bnr` residue | **SEMANTIC CONFLICT** | P1 |
| Restriction overlap model | **UNDEFINED** | P2 |
| Private quote acceptance window | **OWNER DECISION REQUIRED** | P2 |
| `settlement_deadline` removal | **DEPENDENCIES EXIST** | P2 |
| `claim` → `settle` rename | **SEMANTIC IMPROVEMENT, NOT REQUIRED** | P3 |

---

## 2. Authority Map

| Document | Role | Weight |
|----------|------|--------|
| Current filesystem (Go source, migrations, Dart, TypeScript) | **Implementation truth** | Primary |
| Locked owner business decisions (in correction doc) | **Business truth** | Primary |
| REPORT_AUCTION_WINNER_SHIPPING_DESIGN_CORRECTED.md | **Supersedes** conflicting statements in design report | Primary |
| REPORT_AUCTION_WINNER_SHIPPING_TECHNICAL_DESIGN.md (v3) | **Proposal** to be audited, NOT assumed correct | Secondary |
| REPORT_AUCTION_WINNER_SHIPPING_DESIGN.md | **Forensic evidence** from prior audit | Secondary |
| PRD.md | Context/gambaran only | Tertiary |

### Contradiction Resolution

Where v3 contradicts the current filesystem, the filesystem wins as implementation truth. Where v3 contradicts the locked business truth, the business truth wins. Where the locked business truth cannot be represented by the current filesystem, this is documented as a technical contradiction requiring correction.

---

## 3. Locked Business Truth

### 3.1 Auction Failure → DRAFT

All three settlement failures return the auction to DRAFT:

| Failure | Violating Actor | Restriction | Seller Can Relist |
|---------|----------------|-------------|-------------------|
| Seller Shipping Default | Seller | Yes (7/15/30d) | NO (during restriction) |
| Buyer Shipping Selection Failure | Buyer | Yes (7/15/30d) | YES (immediately) |
| Buyer BNR | Buyer | Yes (7/15/30d) | YES (immediately) |

### 3.2 Deadlines

| Deadline | Anchor | Duration | Applies When |
|----------|--------|----------|--------------|
| Buyer shipping selection | `auction.end_at` | 24h | Always |
| Seller private quote | `auction.end_at` | 24h | When private quote required |
| Buyer payment | `shipping_resolved_at` | 24h | After shipping resolved |

### 3.3 Restriction

- One authority, buyer + seller, cross-commerce (For Sale + Auction)
- Ladder: 1st→7d, 2nd→15d, 3rd+→30d
- Cumulative, no automatic reset, no trust score, no permanent ban
- Not full account ban

### 3.4 No Automatic Relisting

"Seller may immediately relist" means seller is permitted to perform the normal relist/create flow. The system does NOT automatically relist.

---

## 4. Current Implementation Map

### 4.1 Auction State Machine

**File:** `backend/internal/commerce/auction/entity/auction.go`

```
States (enum):
  draft, scheduled, active, waiting_settlement, expired_bnr, ended, cancelled

Transitions:
  draft             → scheduled, cancelled
  scheduled         → active, cancelled, draft
  active            → waiting_settlement, ended, cancelled
  waiting_settlement → ended, expired_bnr, cancelled
  expired_bnr       → (terminal — NO outgoing transitions)
  ended             → (terminal)
  cancelled         → (terminal)
```

**Schema:** `backend/migrations/000001_canonical_schema.up.sql`
- `auction_status_enum`: draft, scheduled, active, waiting_settlement, expired_bnr, ended, cancelled
- `settlement_deadline` (timestamptz): set at `TransitionToWaitingSettlement()` as `now + 24h`
- `order_id` (uuid, nullable, unique): set atomically at order creation
- Constraint: `CHECK (order_id IS NULL OR status = 'ended')`

### 4.2 BNR System

| Component | File | Function |
|-----------|------|----------|
| `BNRStrikeChecker` | `bnr_restriction.go` | Evaluates active strikes: 0→allow, 1→warning, 2→14d, 3→90d, 4+→permanent |
| `BNRStrikeHandler` | `bnr_strike_handler.go` | Records strike: `INSERT INTO buyer_bnr_strikes ON CONFLICT (auction_id) DO NOTHING` |
| `BNRDecayWorker` | `bnr_decay_worker.go` | Daily decay of oldest strike after 180d inactivity |
| `BNRAdminResetter` | `bnr_admin_reset.go` | Sets `admin_reset = TRUE` on active strikes |
| `AuctionSettlementWorker` | `auction_settlement_worker.go` | Detects `settlement_deadline <= NOW()`, transitions to `expired_bnr`, emits `auction_bnr_detected` |

**Table:** `buyer_bnr_strikes`
- Columns: id, buyer_id, auction_id, struck_at, decayed_at, admin_reset
- Constraint: `UNIQUE(auction_id)` — one strike per auction
- Index: `idx_buyer_bnr_strikes_buyer_active` (buyer_id, struck_at) WHERE decayed_at IS NULL AND admin_reset = FALSE

**Enforcement:** `PlaceBid()` checks `BNRStrikeChecker.Check()` before processing bid. Fail-open on DB error.

**Scope:** Auction bidding only. Does NOT affect For Sale purchase, chat, social.

### 4.3 Claim Flow

**Endpoint:** `POST /api/v1/auctions/:id/claim`

Current one-shot flow:
```
1. Validate winner + deadline + not-settled (FOR UPDATE lock)
2. Generate pricing token (same tx)
3. Validate pricing token (same tx)
4. Create order from auction (same tx)
5. Mark pricing token consumed
6. Set auction.OrderID + transition to ended
```

**Critical observation:** The claim endpoint bundles shipping selection, pricing token generation, order creation, and settlement into a single atomic operation. There is NO separate shipping resolution step.

### 4.4 Payment Expiry

**File:** `order_creation_service.go`

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

**Issue:** For auction orders, `PaymentExpiresAt` is calculated as `order_created_at + method_expiry`, NOT `shipping_resolved_at + 24h`.

### 4.5 Shipping Quote Lifecycle

**Entity:** `backend/internal/commerce/shipping/quote/entity/shipping_quote.go`

- Status: ACTIVE → USED/EXPIRED/INVALID
- Default expiry: 24h, max 168h (7 days)
- Supersession: `SupersedeCurrentQuotes()` supersedes prior unsuperseded quotes for same canonical context
- Reactivation: USED → ACTIVE (max 2 reuses)
- Source: `source_type` (text), `source_id` (uuid) — supports "auction" and "for_sale"
- Destination lock: `destination_city_id`, `destination_province_id`
- **No acceptance mechanism** — buyer acceptance is implicit via checkout

**Validation for auction:** `validateAuctionForQuote()` checks:
- Auction exists, belongs to seller
- Status = waiting_settlement
- Winner is set
- Chat recipient is winner

**Does NOT check:**
- Whether `requires_private_quote` is true
- Whether seller deadline has passed
- Whether buyer has already selected normal shipping

### 4.6 `uniq_active_auction_per_product` Index

```sql
CREATE UNIQUE INDEX uniq_active_auction_per_product ON auctions (product_id)
WHERE (status = ANY (ARRAY['draft', 'scheduled', 'active', 'waiting_settlement']));
```

**Terminal states excluded:** `ended`, `expired_bnr`, `cancelled` are NOT in the uniqueness condition. This means after a terminal state, a new auction (including DRAFT) can be created for the same product.

**DRAFT IS in the uniqueness condition.** This means only ONE non-terminal auction per product. If auction returns to DRAFT, a new auction cannot be created for the same product until the DRAFT auction progresses or is cancelled.

### 4.7 Capability / Restriction System

**File:** `backend/internal/identity/auth/role_checker_db.go`

`HasActiveSellerCapability()` checks:
1. Account is operational (not suspended/banned)
2. Seller profile exists
3. Subscription is active (`started_at <= NOW() < expires_at`)

**Does NOT check:**
- Commerce restriction
- BNR strikes
- Any transaction violation

**There is no `commerce_restriction`, `commerce_violations`, or `seller_restriction` table.**

---

## 5. Case A Audit (Destination Not Covered)

### 5.1 Required Flow

```
Address submitted
→ Coverage check
→ No applicable Shipping Setup
→ requires_private_quote = true
→ Seller must provide quote before seller_deadline (auction_end + 24h)
→ Buyer accepts
→ shipping_resolved_at set
```

### 5.2 Current Implementation Gap

| Requirement | Current State | Gap |
|-------------|---------------|-----|
| Address submission as separate step | Bundled in `/claim` endpoint | P0 — No separate address submission |
| Coverage check at address submission | Not implemented | P0 — No coverage check exists |
| `requires_private_quote` field | Does NOT exist on auction | P0 — No field |
| Seller deadline at auction end | Not set | P1 — No `seller_shipping_deadline` field |
| Buyer deadline at auction end | Not set | P1 — No `buyer_shipping_deadline` field |
| `shipping_resolved_at` field | Does NOT exist on auction | P0 — No field |
| Separate shipping resolution step | Not implemented | P0 — Bundled in claim |
| Payment at `shipping_resolved_at + 24h` | Not implemented | P0 — Uses method-based expiry |

### 5.3 Case A Verdict

**CANNOT be represented** with current implementation. The claim endpoint must be decomposed into separate steps: address submission → coverage check → shipping resolution → payment.

---

## 6. Case B Audit (Seller Override)

### 6.1 Required Flow

```
Address submitted
→ Coverage available (requires_private_quote = false)
→ Seller chooses special/private shipping
→ Seller creates private quote
→ Buyer accepts
→ shipping_resolved_at set
```

### 6.2 Existing Quote Lifecycle Can Represent Case B

The existing `ShippingQuote` entity with `source_type="auction"` and `validateAuctionForQuote()` can represent the quote lifecycle:

| Capability | Status |
|------------|--------|
| Seller creates quote for auction | ✅ IMPLEMENTED |
| Quote has destination lock | ✅ IMPLEMENTED |
| Quote supersedes prior quotes | ✅ IMPLEMENTED |
| Quote expiry (24h default) | ✅ IMPLEMENTED |
| Quote validation at checkout | ✅ IMPLEMENTED |
| Quote marked USED at order creation | ✅ IMPLEMENTED |
| Quote reactivation on order failure | ✅ IMPLEMENTED |

### 6.3 Case B Race Condition (P1)

**Race:** Seller creates private quote while buyer selects normal shipping.

**Current state:** No guard exists. Both paths can proceed simultaneously:
1. Buyer calls `/claim` with `shipping_option_id` → creates order with normal shipping
2. Seller creates quote via chat → quote is ACTIVE

**Result:** Order is created with normal shipping. Quote remains ACTIVE but unused. If order is later cancelled/reactivated, the quote could theoretically be used for a second order (reactivation allowed).

**Required guard:** Once `shipping_resolved_at` is set (by either path), the other path must be blocked. This requires:
- `shipping_resolved_at` field on auction
- Guard in both the normal selection path and the quote acceptance path
- `FOR UPDATE` on auction row before setting `shipping_resolved_at`

### 6.4 Case B Can Be Represented — With Required Fields

The existing quote lifecycle is sufficient for Case B, but ONLY if:
1. `shipping_resolved_at` exists on auction to gate both paths
2. Normal selection checks that no active quote has been created (or allows override)
3. Quote acceptance checks that normal shipping hasn't already resolved

**Minimum canonical representation required:**
- `shipping_resolved_at` (timestamptz, nullable) on `auctions`
- Guard: "Only one path may set `shipping_resolved_at`"

### 6.5 Case B Verdict

**CAN be represented** with the existing quote lifecycle, but **requires** `shipping_resolved_at` field and atomic guard.

---

## 7. Shipping Resolution Authority

### 7.1 `shipping_resolved_at` as Single Immutable Indicator

**Currently does NOT exist.** The v3 design proposes it as the single indicator that shipping has been resolved.

**Authority analysis:**

| Path | How `shipping_resolved_at` Would Be Set |
|------|----------------------------------------|
| Normal selection | Buyer selects Shipping Setup → backend validates → `shipping_resolved_at = NOW()` |
| Private quote acceptance | Buyer accepts quote → backend validates → `shipping_resolved_at = NOW()` |

**Immutability invariant:** Once `shipping_resolved_at != NULL`, no subsequent action may change it.

**Proof of safety:**
- `FOR UPDATE` on auction row serializes all mutations
- After lock acquired, re-check `shipping_resolved_at IS NULL`
- First commit wins; second sees non-NULL and skips
- Order creation checks `shipping_resolved_at IS NOT NULL` before proceeding

### 7.2 Post-Resolution Mutation Safety

| Action | Safety |
|--------|--------|
| Post-resolution quote mutation | Quote is USED; supersession blocked by validation |
| Post-resolution shipping mutation | No mutation path exists (shipping selection is single-use) |
| Post-resolution order mutation | Order exists; settlement guard prevents re-creation |
| Post-resolution payment | Payment worker checks order status, not `shipping_resolved_at` |

### 7.3 Verdict

**SAFE** if implemented with `FOR UPDATE` + null-check pattern. This is a well-understood pattern in the codebase (see `OrderID` binding pattern).

---

## 8. Deadline Audit

### 8.1 Buyer Shipping Selection Deadline

**Required:** `auction_end + 24h`

**Current implementation:** Does NOT exist. The `settlement_deadline` is set to `now + 24h` at `TransitionToWaitingSettlement()` (which runs at auction end), but it serves a different purpose (detecting BNR, not shipping selection failure).

**Contradiction:** `settlement_deadline` is a single deadline. The locked business truth requires TWO separate deadlines:
1. Buyer shipping selection: `auction_end + 24h`
2. Seller private quote: `auction_end + 24h`

These are the same value but represent different responsibilities. A single field conflates them.

### 8.2 Seller Private Quote Deadline

**Required:** `auction_end + 24h`

**Current implementation:** Does NOT exist as a separate field. The `settlement_deadline` serves this role but is not conditionally applied (it fires even when private quote is not needed).

**Contradiction with locked truth:** The seller deadline should only trigger when private quote is required. The current `settlement_deadline` triggers unconditionally for all `waiting_settlement` auctions.

### 8.3 Buyer Payment Deadline

**Required:** `shipping_resolved_at + 24h`

**Current implementation:** `PaymentExpiresAt` is calculated as `order_created_at + method_expiry` (15min/1hr/6hr/30min).

**Contradiction:** The payment window is 30 minutes (default) instead of 24 hours. This is a P0 divergence from locked business truth.

### 8.4 Deadline Field Analysis

| Field | Required By v3 | Currently Exists | Action |
|-------|---------------|-----------------|--------|
| `buyer_shipping_deadline` | Yes | No | ADD |
| `seller_shipping_deadline` | Yes | No | ADD |
| `shipping_resolved_at` | Yes | No | ADD |
| `requires_private_quote` | Yes | No | ADD |
| `settlement_deadline` | No (OBSOLETE) | Yes | DROP |

### 8.5 `settlement_deadline` Consumer Audit

| Consumer | File | Action | Required Change |
|----------|------|--------|----------------|
| `TransitionToWaitingSettlement()` | `auction.go` | Sets field | REMOVE field set |
| `AuctionSettlementWorker` | `auction_settlement_worker.go` | Queries `settlement_deadline <= NOW()` | DECOMMISSION or REWRITE |
| `GeneratePricingTokenForAuctionClaim` | `auction_service.go` | Checks deadline | UPDATE to `buyer_shipping_deadline` |
| `auction_repository.go` | Repository | Reads/writes column | UPDATE SQL |
| `chat_auction_projection_resolver.go` | Projection | Reads column | UPDATE projection |
| Mobile `auction_dto.dart` | Mobile | Parses `settlement_deadline` | UPDATE DTO |
| Test files | Various | Reference field | UPDATE tests |

### 8.6 Deadline Audit Verdict

**P0 contradictions exist.** The current single `settlement_deadline` cannot represent the three distinct deadlines required by the locked business truth. Payment expiry timing is fundamentally divergent.

---

## 9. Private Quote Lifecycle

### 9.1 Existing Quote Lifecycle Audit

| Attribute | Current Implementation | v3 Requirement | Compatible? |
|-----------|----------------------|----------------|-------------|
| Quote creation | Via chat, seller-initiated | Same | ✅ |
| Quote status lifecycle | ACTIVE → USED/EXPIRED | Same | ✅ |
| Supersession | Prior unsuperseded quotes superseded | Same | ✅ |
| Destination lock | `destination_city_id`, `destination_province_id` | Same | ✅ |
| Expiry | 24h default, 168h max | Same (technical lifecycle) | ✅ |
| Reactivation | USED → ACTIVE, max 2 reuses | Not specified by business truth | ✅ (technical) |
| Acceptance mechanism | **NONE — implicit via checkout** | Explicit acceptance needed? | ⚠️ |
| Source binding | `source_type="auction"`, `source_id=auction_id` | Same | ✅ |

### 9.2 Quote Acceptance Timing (P2)

**Current behavior:** Quote acceptance is implicit. When buyer proceeds to checkout and the quote is validated and marked USED, that IS the acceptance.

**Business truth question:** How long does buyer have to accept a private quote?

**Analysis:**
- Quote has `expires_at` (default 24h from creation)
- If seller creates quote at T+23:59, quote expires at T+47:59
- Buyer has 24h from quote creation to accept (by quote expiry)
- But buyer also has `buyer_shipping_deadline` = auction_end + 24h

**Conflict:** If seller creates quote at T+23:59:
- Quote expires at T+47:59 (quote's own expiry)
- Buyer's shipping deadline is T+24:00 (buyer_shipping_deadline)
- Buyer only has 1 minute to accept!

**This is a genuine technical design issue.** The quote expiry and the buyer's acceptance deadline are not synchronized. The v3 design correctly identifies this as `PRIVATE_QUOTE_ACCEPTANCE_WINDOW` requiring an Owner Decision.

### 9.3 Minimum Acceptance Window Analysis

The minimum time between quote creation and acceptance depends on:
1. When seller creates the quote (could be T+1 or T+23:59)
2. The `buyer_shipping_deadline` (auction_end + 24h)
3. The quote's own expiry

**If buyer_shipping_deadline = auction_end + 24h** and seller creates quote at T+23:59:
- Buyer has 1 minute before `buyer_shipping_deadline` expires
- Quote has 24h before it expires
- Business rule says buyer must complete shipping selection within 24h of auction end

**Resolution options:**
1. Seller deadline is `auction_end + 24h`, but buyer acceptance window extends beyond if seller creates quote late (quote expiry controls)
2. Seller deadline is earlier than 24h to guarantee buyer minimum acceptance window
3. Buyer acceptance window is separate from buyer shipping deadline (when private quote path is active)

**This is a genuinely unresolved technical design issue.** The owner must decide whether:
- The buyer_shipping_deadline is absolute (even for private quotes)
- The buyer gets a minimum window after late quote creation
- The quote expiry is the sole bound on acceptance time

### 9.4 Verdict

**OWNER DECISION GENUINELY REQUIRED** for private quote acceptance window. The existing quote expiry (24h) provides a technical bound, but the interaction with `buyer_shipping_deadline` creates a genuine ambiguity for late-created quotes.

---

## 10. Race / Concurrency Matrix

### Race 1: Buyer selects normal shipping while seller creates private quote

**Current state:** No guard. Both can proceed.
**Impact:** Order created via normal shipping. Quote remains unused. If order fails and quote is reactivated, buyer could theoretically use quote for a second order.
**Fix required:** `shipping_resolved_at` atomic guard on both paths.
**Business rule protected:** Only one shipping resolution path may succeed.

### Race 2: Seller creates private quote while buyer selects normal shipping

**Same as Race 1, different ordering.** Same fix required.

### Race 3: Seller creates quote exactly at seller deadline

**Current state:** `validateAuctionForQuote()` checks `auction.Status == waiting_settlement`. If deadline hasn't triggered transition yet, quote creation succeeds.
**Impact:** Seller is within deadline. Quote created successfully.
**Guard:** `FOR UPDATE` on auction row before checking deadline.
**Business rule protected:** Seller deadline is checked atomically with quote creation.

### Race 4: Seller default worker runs simultaneously with seller quote creation

**Current state:** `AuctionSettlementWorker` uses `FOR UPDATE SKIP LOCKED`. If worker acquires lock first, transitions to `expired_bnr`. If seller's quote creation request acquires lock first, quote is created.
**Impact:** First commit wins. If worker wins, seller is penalized even though they tried to create quote in time.
**Fix required:** Seller deadline worker must check quote existence AFTER acquiring lock (already proposed in v3 §15.1).
**Business rule protected:** If seller created quote before deadline, seller did NOT default.

### Race 5: Buyer accepts quote simultaneously with seller-default worker

**Current state:** No explicit acceptance mechanism exists. Quote acceptance is implicit via checkout.
**Impact:** If order is created while worker detects seller default, the order may conflict with the terminal `expired_bnr` state.
**Fix required:** `shipping_resolved_at` must be set atomically with quote acceptance. Worker must check `shipping_resolved_at` before recording seller default.
**Business rule protected:** If shipping is resolved, settlement proceeds normally.

### Race 6: Buyer shipping selection occurs exactly at buyer deadline

**Current state:** No buyer deadline mechanism exists.
**Fix required:** `FOR UPDATE` on auction, check `buyer_shipping_deadline > NOW()` before processing selection.
**Business rule protected:** Buyer must complete selection within deadline.

### Race 7: Buyer shipping failure worker vs successful shipping resolution

**Current state:** No buyer shipping failure worker exists.
**Fix required:** Worker must check `shipping_resolved_at IS NULL` after acquiring lock. If non-NULL, skip.
**Business rule protected:** Successful resolution prevents violation recording.

### Race 8: Payment vs payment expiry

**Current state:** `PaymentExpiryWorker` uses `FOR UPDATE SKIP LOCKED`. `OrderPaymentTimeoutWorker` checks `NOT EXISTS (active payment)`.
**Impact:** Payment webhook processes before expiry worker runs. Worker checks status after lock — if already paid, skips.
**Verdict:** SAFE with existing pattern. No change needed.

### Race 9: Payment vs BNR detection

**Current state:** `AuctionSettlementWorker` transitions to `expired_bnr` and emits `auction_bnr_detected`. Order creation is separate (claim flow).
**Impact:** In current implementation, BNR fires when `settlement_deadline` expires, BEFORE order exists. There is no order to conflict with.
**Fix required in new model:** BNR fires when `payment_deadline` expires (24h after `shipping_resolved_at`). Order already exists. Worker must check order status after acquiring lock.
**Business rule protected:** Payment success prevents BNR recording.

### Race 10: Settlement failure vs relist attempt

**Current state:** `expired_bnr` is terminal. Relist requires creating a NEW auction for the same product. `uniq_active_auction_per_product` allows this (terminal states excluded from index).
**Impact:** Seller can create new auction for same product while `expired_bnr` auction exists.
**Fix required for new model:** After failure → DRAFT, the auction IS the same auction (not a new one). Relist means progressing the DRAFT. Seller restriction must be checked at DRAFT → scheduled/active transition.
**Business rule protected:** Restricted seller cannot progress DRAFT to market.

---

## 11. Auction Failure Outcome

### 11.1 Current State Machine vs Locked Truth

**Locked truth:** All three failures return auction to DRAFT.

**Current state machine:**
```
expired_bnr → (terminal — NO outgoing transitions)
```

**Contradiction (P0):** The `transitionAllowed` map in `auction.go` line 101 shows:
```go
StatusExpiredBNR: {}, // Terminal state - winner didn't claim in time
```

There is NO transition from `expired_bnr` to `StatusDraft`. This is by design — `expired_bnr` is a terminal state with no valid outgoing transition.

**Even if renamed to `expired_settlement`:**
- The state is still terminal in the `transitionAllowed` map
- Renaming the enum value does NOT add an outgoing transition
- A new transition `expired_settlement → draft` must be explicitly added

### 11.2 DRAFT State Implications

If auction returns to DRAFT:
- Product must release its `SellingSurface` claim (currently set to `SellingSurfaceAuction` at auction creation)
- `uniq_active_auction_per_product` index includes DRAFT, so only one DRAFT auction per product
- Seller must be able to edit DRAFT fields and re-schedule
- Previous winner's bid history must be preserved or cleared

### 11.3 `ReleaseUnpaidOrder` Pattern

**Current implementation:** `ReleaseUnpaidOrder(orderID)` clears `OrderID` binding but does NOT change auction status. The auction stays in `StatusEnded` after release. This was deliberately designed:

```go
// ReleaseUnpaidOrder clears the auction's OrderID binding after its bound
// order was cancelled or expired before payment succeeded (PASS_20B).
//
// Both settlement paths (buy-now via End(), bid-win via Settle()) transition
// the auction to StatusEnded and set OrderID immediately at order-creation
// time... Making an Ended auction literally reopen for bidding would
// require extending the state machine, which is a distinct product/design
// decision left to a future pass...
```

**This comment explicitly acknowledges that reopening an ended auction requires state machine extension.** The same applies to reopening from `expired_bnr`/`expired_settlement`.

### 11.4 State Machine Extension Required

To support failure → DRAFT:

```go
var transitionAllowed = map[Status][]Status{
    StatusDraft:             {StatusScheduled, StatusCancelled},
    StatusScheduled:         {StatusActive, StatusCancelled, StatusDraft},
    StatusActive:            {StatusWaitingSettlement, StatusEnded, StatusCancelled},
    StatusWaitingSettlement: {StatusEnded, StatusExpiredBNR, StatusCancelled},
    StatusExpiredBNR:        {StatusDraft}, // NEW: Allow return to DRAFT
    StatusEnded:             {},
    StatusCancelled:         {},
}
```

And new entity methods:
```go
func (a *Auction) ReturnToDraft() error {
    if !canTransition(a.Status, StatusDraft) {
        return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusDraft}
    }
    a.Status = StatusDraft
    a.OrderID = nil
    a.SettlementDeadline = nil
    a.CurrentWinnerID = nil
    a.CurrentBid = nil
    a.UpdatedAt = time.Now()
    return nil
}
```

### 11.5 `expired_bnr` → `expired_settlement` Rename

**v3 proposes:** Rename `expired_bnr` to `expired_settlement` for semantic clarity.

**Analysis:** The rename is semantically correct but creates migration complexity (PostgreSQL enum value rename requires type recreation). More importantly, the rename alone does NOT solve the core problem — the state must also gain an outgoing transition to DRAFT.

**Verdict:** Rename is a good idea but is secondary to the state machine extension. Both must happen together.

### 11.6 Relist After Failure

| Failure | Can Seller Relist? | Mechanism |
|---------|-------------------|-----------|
| Seller Shipping Default | NO (during restriction) | Restriction check at DRAFT → scheduled/active |
| Buyer Shipping Selection Failure | YES (immediately) | No restriction on seller |
| Buyer BNR | YES (immediately) | No restriction on seller |

**Backend enforcement:** The `HasActiveSellerCapability()` check at `Schedule()` and `PlaceBid()` is NOT sufficient. It checks subscription status, NOT commerce restriction. A new commerce restriction gate must be added.

---

## 12. Relist Lifecycle

### 12.1 Current Relist Capability

**There is no explicit relist mechanism.** When an auction ends (terminal state), the seller can create a NEW auction for the same product (since terminal states are excluded from `uniq_active_auction_per_product`).

**With failure → DRAFT:** The same auction returns to DRAFT. Relist means editing and re-scheduling the DRAFT auction. This is a different flow from creating a new auction.

### 12.2 DRAFT → Scheduled/Active Gate

**Current gate at `Schedule()`:**
1. Ownership check (`IsSeller`)
2. Market authority check (`HasActiveSellerCapability`)
3. Shipping coverage check (`ensureShippingCoverage`)
4. State transition validation

**Missing gate:** Commerce restriction check. A seller under restriction must NOT be able to schedule or activate an auction.

### 12.3 Product SellingSurface Management

When auction returns to DRAFT, the product's `SellingSurface` must be handled:
- Currently set to `SellingSurfaceAuction` at auction creation
- If auction returns to DRAFT, product is still claimed by this auction
- No conflict with `uniq_active_auction_per_product` (DRAFT is included)
- Product cannot be used for a new auction while this DRAFT exists

### 12.4 Bid History After Return to DRAFT

When auction returns to DRAFT:
- Previous bids are in `auction_bids` table
- `CurrentBid` and `CurrentWinnerID` should be cleared
- Bid history should be preserved for audit but not affect new auction
- `AntiSnipeExtensionTotal` should be reset

### 12.5 Verdict

**DRAFT → Scheduled/Active is the correct enforcement point for seller restriction.** The existing `Schedule()` method already has the pattern; a commerce restriction check must be added as a new gate.

---

## 13. BNR Audit

### 13.1 Current BNR End-to-End Trace

```
AuctionSettlementWorker detects settlement_deadline <= NOW()
    → TransitionToExpiredBNR()
    → Emit auction_bnr_detected outbox event
    → BNRStrikeHandler.Handle()
    → INSERT INTO buyer_bnr_strikes (buyer_id, auction_id) ON CONFLICT (auction_id) DO NOTHING
    → BNRStrikeChecker.evaluate(count, lastStruckAt)
    → PlaceBid() checks BNRStrikeChecker before processing bid
```

### 13.2 BNR Ladder Divergence

| Aspect | Current | Locked Truth |
|--------|---------|-------------|
| 1st violation | Allowed + warning | 7-day restriction |
| 2nd violation | 14-day block | 15-day restriction |
| 3rd violation | 90-day block | 30-day restriction |
| 4th+ violation | Permanent ban | 30-day restriction |
| Decay | 180-day automatic decay | NO decay |
| Admin reset | YES | NOT YET DECIDED |
| Scope | Buyer auction bidding only | Buyer + Seller, cross-commerce |

### 13.3 BNR Scope Problem

**Current:** `buyer_bnr_strikes` tracks only buyer violations for auction bidding.

**Locked truth:** One restriction authority for buyer BNR + buyer shipping failure + seller shipping default. Cross-commerce (For Sale + Auction).

**Minimum change:** The `buyer_bnr_strikes` table CANNOT be reused as-is because:
1. It has `UNIQUE(auction_id)` — one strike per auction. But the new model needs one violation per source auction per actor per violation type.
2. It only has `buyer_id` — no `seller_id` column for seller violations.
3. It only has `auction_id` — no `source_type` for For Sale violations.
4. It has `decayed_at` and `admin_reset` columns that don't exist in the locked model.

### 13.4 `buyer_bnr_strikes` Schema vs Required `commerce_violations`

| Attribute | `buyer_bnr_strikes` (current) | `commerce_violations` (proposed) |
|-----------|-------------------------------|----------------------------------|
| Actor identity | `buyer_id` only | `actor_id` + `actor_type` (buyer/seller) |
| Source | `auction_id` only | `source_id` + `source_type` (auction/for_sale) |
| Violation type | Implicit (BNR) | Explicit (`bnr`, `shipping_selection_failure`, `shipping_default`) |
| Duplicate prevention | `UNIQUE(auction_id)` | `UNIQUE(source_id, source_type, violation_type)` |
| Decay | `decayed_at` column | NO decay |
| Admin reset | `admin_reset` column | TBD (owner decision) |
| Cumulative count | COUNT active strikes | COUNT all violations |

### 13.5 Duplicate Violation Prevention Analysis

**Proposed constraint:** `UNIQUE(source_id, source_type, violation_type)`

**Business invariant:** "One auction settlement attempt cannot produce duplicate violation consequences."

**Analysis:**
- For buyer violations: One auction can produce at most one `shipping_selection_failure` AND one `bnr` (different violation types). UNIQUE on `(source_id, source_type, violation_type)` allows both.
- For seller violations: One auction can produce at most one `shipping_default`. UNIQUE prevents duplicate.
- Cross-auction: Different auction IDs have different `source_id`. No conflict.

**But:** What about the same auction producing both a buyer violation AND a seller violation?
- Example: Seller default AND buyer BNR on the same auction.
- These are different `actor_type` values but the proposed constraint does NOT include `actor_type`.
- **CONTRADICTION:** `UNIQUE(source_id, source_type, violation_type)` would prevent the same auction from having both a buyer `bnr` and a seller `shipping_default` if the violation_type names collide. But they don't — `bnr` ≠ `shipping_default`.
- **However:** If a buyer gets `shipping_selection_failure` on auction X, and later a seller gets `shipping_default` on the same auction X (impossible in practice since the auction can only fail one way), the constraint would not conflict because actor_type differs conceptually but is not in the constraint.

**Verdict:** The `UNIQUE(source_id, source_type, violation_type)` constraint is **insufficient**. It should be `UNIQUE(source_id, source_type, violation_type, actor_type)` to correctly represent that the same auction can penalize both buyer and seller for different violation types. Without `actor_type` in the constraint, a buyer `bnr` on auction X would prevent a seller `shipping_default` on the same auction X IF they shared the same `violation_type` value (they don't today, but the schema should be future-proof).

**Actually, re-analyzing:** The violation types are distinct (`bnr`, `shipping_selection_failure`, `shipping_default`), so there is no collision today. But the constraint should still include `actor_type` for correctness: `UNIQUE(source_id, source_type, violation_type, actor_type)`.

### 13.6 BNR Migration Path

| Component | Action |
|-----------|--------|
| `buyer_bnr_strikes` table | DROP (after data migration) |
| `buyer_bnr_strikes` data | Migrate to `commerce_violations` |
| `BNRStrikeHandler` | REPLACE with `CommerceViolationHandler` |
| `BNRStrikeChecker` | REPLACE with `CommerceRestrictionChecker` |
| `BNRDecayWorker` | DECOMMISSION |
| `BNRAdminResetter` | REMOVE (pending owner decision on admin reset) |
| `BNRStrikeChecker.evaluate()` | REPLACE with 7/15/30 ladder |
| `BNRAuctionRestrictedError` | REPLACE with `CommerceRestrictedError` |
| `bnr_restriction_check_failed_total` metric | UPDATE name and scope |
| `auction_bnr_detected` event | RENAME to `auction.settlement_failed` |
| `expired_bnr` state | RENAME to `expired_settlement` |

---

## 14. Commerce Violation Authority Audit

### 14.1 `commerce_violations` Schema Evaluation

Proposed in v3:
```sql
CREATE TABLE commerce_violations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id        UUID NOT NULL,
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('buyer', 'seller')),
    violation_type  TEXT NOT NULL CHECK (violation_type IN ('bnr', 'shipping_selection_failure', 'shipping_default')),
    source_id       UUID NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'auction',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, source_type, violation_type)
);
```

### 14.2 Schema Sufficiency Analysis

| Requirement | Sufficient? | Notes |
|-------------|-------------|-------|
| Buyer shipping-selection failure | ✅ | `actor_type='buyer'`, `violation_type='shipping_selection_failure'` |
| Seller shipping default | ✅ | `actor_type='seller'`, `violation_type='shipping_default'` |
| Buyer BNR | ✅ | `actor_type='buyer'`, `violation_type='bnr'` |
| Cumulative count | ✅ | `COUNT(*) WHERE actor_id = X AND created_at > cutoff` |
| Actor identity | ✅ | `actor_id` + `actor_type` |
| Source auction | ✅ | `source_id` + `source_type` |
| Idempotency | ⚠️ | UNIQUE constraint prevents duplicate, but should include `actor_type` |
| Cross-commerce restriction | ✅ | `source_type` can be 'for_sale' or 'auction' |
| Admin intervention | ❓ | Owner decision pending |

### 14.3 Recommended Constraint Fix

```sql
UNIQUE (source_id, source_type, violation_type, actor_type)
```

This ensures:
- One auction can penalize both buyer (for BNR) and seller (for shipping default)
- Same violation type for same actor on same source is prevented
- Different actors on same source with same violation type are allowed

### 14.4 Restriction Lookup

```sql
-- Check if actor is currently restricted
SELECT COUNT(*) as violation_count
FROM commerce_violations
WHERE actor_id = $1
  AND actor_type = $2
  AND created_at > (
      SELECT MAX(created_at) - INTERVAL '30 days'
      FROM commerce_violations
      WHERE actor_id = $1 AND actor_type = $2
  );
```

The restriction duration depends on the cumulative count (7/15/30d). The query must count violations and compute the applicable restriction window.

### 14.5 Verdict

**Schema is sufficient** with the `actor_type` constraint fix. The `commerce_violations` table is the correct canonical representation.

---

## 15. Restriction Audit

### 15.1 Restriction Overlap Analysis

**Locked truth:** Cumulative violations, 7/15/30d ladder.

**Unresolved question:** When violation #2 occurs during active restriction from #1:

| Option | Result | Formula |
|--------|--------|---------|
| A: Latest violation resets clock | 15d from violation #2 | `restriction_end = violation_created_at + ladder_days(violation_count)` |
| B: Restrictions stack | Day 7 + 15d = Day 22 | `restriction_end = current_restriction_end + ladder_days(violation_count)` |

**Technical compatibility analysis:**

Both options are technically implementable with the `commerce_violations` table.

**Option A (Latest resets clock):**
```sql
-- Simple: just count violations and compute from latest
SELECT COUNT(*), MAX(created_at)
FROM commerce_violations
WHERE actor_id = $1 AND actor_type = $2;
-- restriction_end = MAX(created_at) + ladder(COUNT(*))
```

**Option B (Stack):**
```sql
-- Complex: must track each violation's contribution
-- Each violation extends the restriction by its ladder amount
-- Need to know the current restriction end to compute extension
```

**Locked truth constraint:** "No automatic reset, no permanent escalation." Both options satisfy this. The question is whether the restriction period is computed from the latest violation (Option A) or accumulated (Option B).

**Verdict:** **OWNER DECISION GENUINELY REQUIRED.** Both options are technically valid. The choice affects the restriction duration formula but not the schema.

---

## 16. Claim/Settle Semantic Audit

### 16.1 Current Claim Flow Analysis

**Endpoint:** `POST /api/v1/auctions/:id/claim`

**What "claim" actually means in current implementation:**
1. Validate winner identity
2. Validate settlement deadline
3. Validate not already settled
4. Generate pricing token (includes shipping validation)
5. Create order from auction (includes shipping quote validation if applicable)
6. Mark pricing token consumed
7. Set `auction.OrderID` and transition to `ended`

**In reality, this is "settle"** — it creates the order and settles the auction in one operation. The name "claim" is historical and misleading because:
- "Claim" implies the winner is "claiming" their prize
- "Settle" better describes the financial/settlement operation
- The endpoint does more than just claim — it creates the order, validates pricing, and settles

### 16.2 Rename Assessment

**v3 proposes:** Rename `claim` to `settle`.

**Analysis:**
- The rename is semantically correct
- The existing comment in `ReleaseUnpaidOrder` already uses "settlement" terminology
- The endpoint already does settlement work (order creation + auction state transition)
- Backward compatibility: Mobile app references `/claim` endpoint

**Verdict:** Rename is a good idea but is NOT a blocking issue. The current "claim" name is misleading but not incorrect. If renamed, mobile must be updated simultaneously.

---

## 17. Order/Payment Gate Audit

### 17.1 Shipping Resolution Gate

**Required:** Order creation cannot occur before `shipping_resolved_at != NULL`.

**Current implementation:** NO gate exists. The `/claim` endpoint bundles shipping selection and order creation in a single operation. There is no separate shipping resolution step.

**Evidence from `CreateFromAuction()`:**
```go
// Step 2.5: SHIPPING GUARD - Verify sale surface has shipping options configured.
shippingSetups, err := s.productShippingRepo.GetByProduct(ctx, tx, product.ID)
if len(shippingSetups) == 0 {
    return nil, fmt.Errorf("sale surface %s: %w", product.ID, shippingApp.ErrNoShippingSetups)
}
```

This checks that shipping options EXIST, not that shipping has been RESOLVED. There is no `shipping_resolved_at` check anywhere in the order creation path.

### 17.2 Payment Expiry for Auctions

**Required:** `payment_expires_at = shipping_resolved_at + 24h`

**Current implementation:**
```go
calculatePaymentExpiry(snapshot.PaymentMethod, time.Now())
// = order_created_at + method_expiry (15min/1hr/6hr/30min)
```

**Contradiction (P0):** The payment window is method-based (15min–6hr), NOT 24h from shipping resolved.

### 17.3 For Sale Independence

**Required:** For Sale must remain independent.

**Current implementation:** For Sale uses `calculatePaymentExpiry()` with method-based timing. This is correct and must NOT be changed.

**Gate implementation (required):**
```go
func (s *OrderCreationService) CreateFromAuction(...) {
    // ... existing validation ...
    
    // NEW: Shipping resolution gate
    if auction.ShippingResolvedAt == nil {
        return fmt.Errorf("shipping not resolved for auction %s", auction.ID)
    }
    
    // ... proceed with order creation ...
}
```

### 17.4 No Bypass Paths

**All auction order creation paths:**
1. `/claim` endpoint → `AuctionService.CreateOrderFromAuction()` → `OrderCreationService.CreateFromAuction()`
2. Pricing token validation → same path

**Both paths must check `shipping_resolved_at`.** The `/claim` endpoint is the only entry point today, but if new endpoints are added (e.g., separate settle endpoint), they must also check.

### 17.5 Verdict

**P0 contradiction.** The shipping resolution gate does NOT exist. Payment expiry timing is fundamentally wrong for auctions.

---

## 18. Worker Audit

### 18.1 Current Workers

| Worker | Poll | Purpose | Status in v3 |
|--------|------|---------|-------------|
| `AuctionEndWorker` | 30s | active → waiting_settlement | MODIFY: set deadlines |
| `AuctionSettlementWorker` | 5min | Detect settlement timeout → expired_bnr | DECOMMISSION |
| `BNRDecayWorker` | 24h | Decay old BNR strikes | DECOMMISSION |
| `PaymentExpiryWorker` | 1min | Expire pending payments | UNCHANGED |
| `OrderPaymentTimeoutWorker` | 2min | Expire orphan orders | MODIFY: add BNR emission |
| `OrderOverdueCancelWorker` | — | Cancel overdue shipped orders | UNCHANGED |

### 18.2 Workers Required by v3

| Worker | Purpose | New/Modified |
|--------|---------|-------------|
| `AuctionEndWorker` | Set `buyer_shipping_deadline`, `seller_shipping_deadline` at auction end | MODIFY |
| `SellerShippingDeadlineWorker` | Detect seller timeout (private quote not created) | NEW |
| `BuyerShippingDeadlineWorker` | Detect buyer timeout (shipping not resolved) | NEW |
| `PaymentExpiryWorker` | Unchanged | UNCHANGED |
| `OrderPaymentTimeoutWorker` | Modify to emit BNR event on auction payment timeout | MODIFY |
| `AuctionSettlementWorker` | DECOMMISSION | DECOMMISSION |
| `BNRDecayWorker` | DECOMMISSION | DECOMMISSION |

### 18.3 Worker Decommission Analysis

**`AuctionSettlementWorker`:**
- Currently detects `settlement_deadline <= NOW()` and transitions to `expired_bnr`
- Emits `auction_bnr_detected` event
- `BNRStrikeHandler` consumes event and records strike
- **Decommission:** Must be replaced by `SellerShippingDeadlineWorker` and `BuyerShippingDeadlineWorker`
- **Risk:** If decommissioned before new workers are deployed, settlement failures go undetected

**`BNRDecayWorker`:**
- Daily decay of old BNR strikes
- Locked truth says NO automatic decay
- **Decommission:** Safe to stop. Existing decayed strikes remain in DB for audit.
- **Risk:** None — locked truth explicitly prohibits decay.

### 18.4 Worker Migration Strategy

1. Deploy new fields (`buyer_shipping_deadline`, `seller_shipping_deadline`, `shipping_resolved_at`, `requires_private_quote`)
2. Deploy `SellerShippingDeadlineWorker` and `BuyerShippingDeadlineWorker`
3. Modify `AuctionEndWorker` to set new deadline fields
4. Modify `OrderPaymentTimeoutWorker` to emit BNR event for auction orders
5. Decommission `AuctionSettlementWorker`
6. Decommission `BNRDecayWorker`
7. Deploy `commerce_violations` table and `CommerceRestrictionChecker`
8. Migrate `buyer_bnr_strikes` data to `commerce_violations`
9. Drop `buyer_bnr_strikes` table

---

## 19. Mobile Contract Audit

### 19.1 Mobile Auction Status

**File:** `apps/mobile/lib/domains/commerce/catalog/auction/domain/entities/auction_status.dart`

Current states: `draft, scheduled, active, ended, waitingSettlement, expiredBNR, cancelled`

**Required changes:**
1. Rename `expiredBNR` to `expiredSettlement` (or add new value)
2. Add UI for buyer shipping selection (separate from claim)
3. Add countdown timers for 3 deadlines
4. Handle quote acceptance in chat
5. Handle `shipping_resolved_at` display

### 19.2 Mobile Settlement Deadline

**File:** `apps/mobile/lib/domains/commerce/catalog/auction/data/dto/auction_dto.dart`

```dart
settlementDeadline: json['settlement_deadline'] != null
    ? DateTime.parse(json['settlement_deadline'] as String)
```

**Required changes:**
- Replace `settlement_deadline` with `buyer_shipping_deadline`
- Add `seller_shipping_deadline`
- Add `shipping_resolved_at`
- Add `requires_private_quote`

### 19.3 Mobile Claim Flow

Currently: Winner sees "claim" button → single operation.

**Required:** Decompose into:
1. Address submission screen
2. Shipping selection screen (normal) OR wait for seller quote (private)
3. Payment screen (24h countdown)

### 19.4 Verdict

**Mobile requires significant UI changes** to support the decomposed claim flow. The state machine rename (`expired_bnr` → `expired_settlement`) requires Dart enum changes and all switch statements updated.

---

## 20. Admin Contract Audit

### 20.1 Admin Source Status Labels

**File:** `apps/admin/src/types/orders.ts`

```typescript
export const sourceStatusLabels: Record<string, string> = {
    // ...
    expired_bnr: 'Expired (BNR)',
    // ...
};
```

**Required changes:**
1. Rename `expired_bnr` to `expired_settlement` with label "Settlement Failed"
2. Add admin views for `commerce_violations` table
3. Add admin restriction management (if owner approves admin reset)

### 20.2 Admin BNR Management

**Current:** `BNRAdminResetter` allows admin to reset buyer BNR strikes.

**Locked truth:** Admin reset is a pending owner decision.

**Verdict:** Admin BNR management UI must be updated to work with `commerce_violations` table after migration.

---

## 21. Migration Impact

### 21.1 Required Migrations (ordered)

| # | Migration | Risk | Reversible? |
|---|-----------|------|-------------|
| 1 | ADD `buyer_shipping_deadline` (timestamptz) to auctions | Low | Yes (DROP column) |
| 2 | ADD `seller_shipping_deadline` (timestamptz) to auctions | Low | Yes |
| 3 | ADD `shipping_resolved_at` (timestamptz) to auctions | Low | Yes |
| 4 | ADD `requires_private_quote` (boolean, NOT NULL, default false) to auctions | Low | Yes |
| 5 | ADD `expired_settlement` to `auction_status_enum` | Low | Yes (but enum value remains) |
| 6 | CREATE `commerce_violations` table | Low | Yes (DROP table) |
| 7 | MIGRATE `buyer_bnr_strikes` data to `commerce_violations` | Medium | No (data migration) |
| 8 | ADD transition `expired_bnr → draft` in code (not migration) | N/A | Code change |
| 9 | RENAME `expired_bnr` rows to `expired_settlement` | Medium | Difficult (enum rename) |
| 10 | DROP `expired_bnr` from enum | Medium | No (enum value lost) |
| 11 | DROP `buyer_bnr_strikes` table | Low | No (data lost) |
| 12 | DROP `settlement_deadline` column | Low | Yes |

### 21.2 Migration Ordering Constraint

Steps 7-10 must be carefully ordered:
1. Add `expired_settlement` to enum (step 5) — no downtime
2. Update code to write `expired_settlement` instead of `expired_bnr` — dual-write period
3. Migrate existing `expired_bnr` rows to `expired_settlement` (step 9)
4. Remove `expired_bnr` from code paths
5. Drop `expired_bnr` from enum (step 10) — requires type recreation

### 21.3 `uniq_active_auction_per_product` Impact

The index condition includes `draft`:
```sql
WHERE (status = ANY (ARRAY['draft', 'scheduled', 'active', 'waiting_settlement']))
```

When auction returns to DRAFT, this index correctly prevents creating a second auction for the same product while the first is in DRAFT. No change needed.

---

## 22. Residue Audit

### 22.1 `expired_bnr` Residue

| Location | Type | Classification |
|----------|------|---------------|
| `auction_status_enum` (migration) | Schema | MUST rename to `expired_settlement` |
| `entity.StatusExpiredBNR` (Go) | Code | MUST rename |
| `transitionAllowed[StatusExpiredBNR]` | Code | MUST add outgoing transition to DRAFT |
| `PublicLifecycle()` | Code | Returns "unavailable" — correct |
| `IsRepostable()` | Code | Returns false — correct |
| `IsPublicDiscoverable()` | Code | Returns false — correct |
| `BNRAuctionRestrictedError` | Code | MUST rename to `CommerceRestrictedError` |
| Mobile `AuctionStatus.expiredBNR` | Dart | MUST rename |
| Mobile `parseAuctionStatus` | Dart | Handles `expired_bnr` — MUST update |
| Admin `sourceStatusLabels` | TypeScript | MUST update label |
| Admin `sourceStatusVariants` | TypeScript | MUST update variant |
| Tests referencing `expired_bnr` | Tests | MUST update |
| `dev-reset-data/main.go` | Tool | References table — no change needed |
| Documentation | Docs | Multiple references — update |

### 22.2 `settlement_deadline` Residue

| Location | Type | Classification |
|----------|------|---------------|
| `auctions.settlement_deadline` column | Schema | MUST drop |
| `entity.Auction.SettlementDeadline` | Code | MUST remove |
| `TransitionToWaitingSettlement()` | Code | MUST remove field set |
| `AuctionSettlementWorker` query | Worker | MUST decommission |
| `GeneratePricingTokenForAuctionClaim` | Service | MUST update to `buyer_shipping_deadline` |
| `auction_repository.go` SQL | Repository | MUST update |
| `chat_auction_projection_resolver.go` | Projection | MUST update |
| Mobile `auction_dto.dart` | Dart | MUST update |
| Tests | Tests | MUST update |

### 22.3 `buyer_bnr_strikes` Residue

| Location | Type | Classification |
|----------|------|---------------|
| `buyer_bnr_strikes` table | Schema | MUST drop (after migration) |
| `BNRStrikeHandler` | Worker | MUST decommission |
| `BNRStrikeChecker` | Service | MUST replace |
| `BNRDecayWorker` | Worker | MUST decommission |
| `BNRAdminResetter` | Worker | MUST remove or replace |
| `bnr_restriction.go` | Service | MUST replace |
| `bnr_telemetry.go` | Telemetry | MUST update |
| `buyer_bnr_strikes_buyer_active` index | Schema | Drops with table |
| `dev-reset-data/main.go` | Tool | References table |

### 22.4 `claim` Endpoint Residue

| Location | Type | Classification |
|----------|------|---------------|
| `POST /auctions/:id/claim` route | Route | Consider rename to `/settle` |
| `POST /auctions/:id/claim-token` route | Route | Consider rename |
| `ClaimAuction` handler method | Code | Consider rename |
| `ClaimAuctionRequest` struct | Code | Consider rename |
| Mobile references | Dart | Must update if renamed |
| Tests | Tests | Must update if renamed |

### 22.5 `auction_bnr_detected` Event Residue

| Location | Type | Classification |
|----------|------|---------------|
| Event type string | Outbox | MUST rename to `auction.settlement_failed` |
| `BNRStrikeHandler.Handle()` | Consumer | MUST replace |
| `notification_worker_commerce.go` | Consumer | MUST update |
| `outbox_event_registry.go` | Registry | MUST update |
| Test files | Tests | MUST update |

---

## 23. Contradiction Register

### C1: Failure → DRAFT Transition Impossible (P0)

```
CURRENT ASSUMPTION
→ expired_bnr is a terminal state with no outgoing transitions

EVIDENCE
→ transitionAllowed[StatusExpiredBNR] = {} (auction.go line 101)
→ Comment: "Terminal state - winner didn't claim in time"

CONTRADICTION
→ Locked truth requires ALL three failures to return auction to DRAFT
→ The state machine does not allow any transition from expired_bnr

BUSINESS IMPACT
→ Seller cannot relist after buyer failure
→ Seller cannot relist after seller default (after restriction expires)
→ Product remains stuck in terminal state

TECHNICAL IMPACT
→ State machine extension required (add expired_bnr → draft transition)
→ New entity method required (ReturnToDraft)
→ Bid history cleanup required
→ Product SellingSurface management required

REQUIRED CORRECTION
→ Add StatusDraft as valid target from StatusExpiredBNR
→ Add ReturnToDraft() method with bid cleanup logic
→ Or: introduce new state (e.g., StatusSettlementFailed) with DRAFT as target
```

### C2: No Commerce Restriction System (P0)

```
CURRENT ASSUMPTION
→ BNR system tracks buyer violations for auction bidding only
→ HasActiveSellerCapability() checks subscription only

EVIDENCE
→ buyer_bnr_strikes table exists (buyer-only, auction-only)
→ No commerce_restriction or seller_restriction table exists
→ No commerce_violations table exists
→ RoleCheckerDB.HasActiveSellerCapability() checks: account + profile + subscription

CONTRADICTION
→ Locked truth requires cross-commerce restriction (buyer + seller, For Sale + Auction)
→ No existing table or code path enforces commerce restriction

BUSINESS IMPACT
→ Restricted seller can still create For Sale listings
→ Restricted buyer can still purchase via For Sale
→ Seller shipping default has no enforcement mechanism

TECHNICAL IMPACT
→ New table: commerce_violations
→ New checker: CommerceRestrictionChecker
→ New enforcement gates at: PlaceBid, CreateFromAuction, CreateForSale, CreateAuction
→ Migration from buyer_bnr_strikes to commerce_violations

REQUIRED CORRECTION
→ Create commerce_violations table
→ Implement CommerceRestrictionChecker with 7/15/30 ladder
→ Add restriction gates at all commerce entry points
→ Migrate existing BNR data
```

### C3: No Shipping Resolution Gate (P0)

```
CURRENT ASSUMPTION
→ Order creation is bundled with shipping selection in /claim endpoint

EVIDENCE
→ ClaimAuction handler creates order in single atomic transaction
→ No shipping_resolved_at field on auction
→ No separate shipping resolution step

CONTRADICTION
→ Locked truth requires shipping resolution BEFORE order creation
→ Two distinct phases: (1) shipping resolution, (2) payment

BUSINESS IMPACT
→ Buyer cannot see final shipping cost before committing to order
→ No 24h payment window from shipping resolution
→ Payment expiry uses wrong timing (method-based vs 24h)

TECHNICAL IMPACT
→ New field: shipping_resolved_at on auctions
→ Decompose /claim into: /submit-address, /select-shipping OR /accept-quote, /settle
→ Gate order creation on shipping_resolved_at IS NOT NULL
→ Modify PaymentExpiresAt calculation for auction orders

REQUIRED CORRECTION
→ Add shipping_resolved_at field
→ Create separate endpoints for shipping resolution
→ Gate order creation
→ Fix payment expiry calculation
```

### C4: Payment Expiry Timing Divergent (P0)

```
CURRENT ASSUMPTION
→ PaymentExpiry = method-based (15min/1hr/6hr/30min)

EVIDENCE
→ calculatePaymentExpiry() in order_creation_service.go
→ Used by CreateFromAuction via pricing token

CONTRADICTION
→ Locked truth: payment_deadline = shipping_resolved_at + 24h
→ Current: payment_deadline = order_created_at + method_expiry

BUSINESS IMPACT
→ Buyer has only 30 minutes (default) instead of 24 hours to pay
→ This is a fundamental UX and business rule violation

TECHNICAL IMPACT
→ Modify calculatePaymentExpiry for auction orders
→ Or: bypass calculatePaymentExpiry with explicit payment_expires_at
→ OrderPaymentTimeoutWorker already queries payment_expires_at

REQUIRED CORRECTION
→ For auction orders: payment_expires_at = shipping_resolved_at + 24h
→ For For Sale orders: payment_expires_at = calculatePaymentExpiry(method, now) (unchanged)
```

### C5: Case B Race Condition Unguarded (P1)

```
CURRENT ASSUMPTION
→ Seller creates quote; buyer selects shipping independently

EVIDENCE
→ No guard prevents concurrent normal selection + private quote creation
→ /claim endpoint does not check for active quotes
→ validateAuctionForQuote() does not check for normal shipping selection

CONTRADICTION
→ Only one shipping resolution path may succeed
→ Both paths can currently proceed simultaneously

BUSINESS IMPACT
→ Duplicate shipping resolution possible
→ Inconsistent order pricing (normal vs private quote)

TECHNICAL IMPACT
→ shipping_resolved_at atomic guard required
→ Both paths must check shipping_resolved_at IS NULL before proceeding

REQUIRED CORRECTION
→ Add shipping_resolved_at null-check guard to both paths
→ FOR UPDATE on auction row before setting shipping_resolved_at
```

### C6: Seller Deadline Not Implemented (P1)

```
CURRENT ASSUMPTION
→ settlement_deadline serves as the only deadline

EVIDENCE
→ Only settlement_deadline field exists
→ No seller_shipping_deadline or buyer_shipping_deadline fields

CONTRADICTION
→ Locked truth requires separate seller and buyer deadlines
→ Seller deadline must be conditional (only when private quote required)
→ Current settlement_deadline fires unconditionally

BUSINESS IMPACT
→ Seller not penalized for failing to provide quote
→ Buyer not penalized for failing to select shipping

TECHNICAL IMPACT
→ New fields required
→ New workers required
→ Settlement worker must be decommissioned

REQUIRED CORRECTION
→ Add buyer_shipping_deadline and seller_shipping_deadline fields
→ Set both at auction end as auction_end + 24h
→ SellerShippingDeadlineWorker checks quote existence
→ BuyerShippingDeadlineWorker checks shipping_resolved_at
```

### C7: Duplicate Violation Constraint Insufficient (P1)

```
CURRENT ASSUMPTION
→ UNIQUE(auction_id) on buyer_bnr_strikes prevents duplicate strikes

EVIDENCE
→ buyer_bnr_strikes has UNIQUE(auction_id)
→ One strike per auction per buyer

CONTRADICTION
→ Locked truth requires separate violations per actor per type per source
→ UNIQUE(source_id, source_type, violation_type) without actor_type is insufficient

BUSINESS IMPACT
→ Same auction cannot penalize both buyer and seller if violation types overlap

TECHNICAL IMPACT
→ UNIQUE constraint must include actor_type

REQUIRED CORRECTION
→ UNIQUE(source_id, source_type, violation_type, actor_type)
```

### C8: `expired_bnr` Semantic Conflict (P1)

```
CURRENT ASSUMPTION
→ expired_bnr represents buyer BNR (Bid No Response)

EVIDENCE
→ State name: "expired_bnr"
→ Event name: "auction_bnr_detected"
→ Handler name: BNRStrikeHandler
→ Worker name: AuctionSettlementWorker (emits BNR event)

CONTRADICTION
→ Business truth has THREE failure types:
   1. Buyer shipping selection failure (NOT BNR)
   2. Seller shipping default (NOT BNR)
   3. Buyer BNR (failure to pay)
→ Using expired_bnr for all three conflates distinct business events

BUSINESS IMPACT
→ Seller shipping default incorrectly labeled as BNR
→ Buyer shipping failure incorrectly labeled as BNR
→ Notification messages may be misleading

TECHNICAL IMPACT
→ Rename expired_bnr to expired_settlement
→ Rename auction_bnr_detected to auction.settlement_failed
→ Update all handlers, notifications, tests

REQUIRED CORRECTION
→ Rename state, event, and handler to reflect settlement failure (not BNR specifically)
```

---

## 24. Owner Decision Register

### LOCKED

| # | Decision | Source |
|---|----------|--------|
| 1 | Seller deadline = auction_end + 24h | correction §5 |
| 2 | Buyer shipping selection = 24h from auction end | correction §3 |
| 3 | Buyer payment = shipping_resolved_at + 24h | correction §4 |
| 4 | Buyer shipping selection failure → buyer transaction violation | correction §3 |
| 5 | Seller shipping default → seller violation, winner not penalized | correction §5 |
| 6 | Cross-commerce restriction (For Sale + Auction) | correction §1 |
| 7 | Ladder: 1st→7d, 2nd→15d, 3rd+→30d | correction §2 |
| 8 | Cumulative, no automatic reset | correction §2 |
| 9 | No trust score | correction §2 |
| 10 | No permanent escalation | correction §2 |
| 11 | Lazy chat for private quote | correction §10 |
| 12 | Private quote does not mutate Shipping Setup | correction §10 |
| 13 | BNR = buyer failed to pay after shipping resolved | §3.3 |
| 14 | Seller default = seller didn't create quote by deadline | §3.1 |
| 15 | Auction failure returns to DRAFT | §4 locked truth |
| 16 | Seller cannot relist during restriction | §4 locked truth |
| 17 | Seller can relist immediately after buyer failure | §4 locked truth |

### OWNER DECISION REQUIRED

| # | Decision | Options | Impact |
|---|----------|---------|--------|
| 1 | **Private quote acceptance deadline** | A: Quote expiry controls; B: Fixed window; C: Tied to seller deadline | Worker, lifecycle stall |
| 2 | **Restriction overlap** | A: Latest resets clock; B: Stack restrictions | Restriction duration formula |
| 3 | **Admin manual violation reset** | A: Allow; B: Disallow | Admin mechanism |

---

## 25. Recommended Canonical Architecture

### 25.1 State Machine

```
draft → scheduled → active → waiting_settlement → ended
                                                 → expired_settlement → draft (NEW)
                                                 → cancelled
```

`expired_settlement` gains outgoing transition to `draft` with cleanup:
- Clear `OrderID`, `SettlementDeadline`, `CurrentWinnerID`, `CurrentBid`
- Clear `AntiSnipeExtensionTotal`
- Reset `Status` to `draft`
- Preserve bid history for audit

### 25.2 Auction Fields (New/Modified)

| Field | Type | Set When | Purpose |
|-------|------|----------|---------|
| `buyer_shipping_deadline` | timestamptz | Auction end (when winner) | = auction_end + 24h |
| `seller_shipping_deadline` | timestamptz | Auction end (when winner) | = auction_end + 24h |
| `shipping_resolved_at` | timestamptz | Shipping resolution | Immutable once set |
| `requires_private_quote` | boolean, NOT NULL, default false | After coverage check | Case A indicator |
| `settlement_deadline` | — | — | DROP |

### 25.3 New Table: `commerce_violations`

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
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, source_type, violation_type, actor_type)
);

CREATE INDEX idx_commerce_violations_actor ON commerce_violations (actor_id, created_at DESC);
```

### 25.4 API Endpoints (New/Modified)

```
Normal shipping path:
  POST /auctions/:id/submit-address     → coverage check, set requires_private_quote
  POST /auctions/:id/select-shipping    → shipping resolution, set shipping_resolved_at
  POST /auctions/:id/settle             → create order (gated on shipping_resolved_at)

Private quote path:
  POST /auctions/:id/submit-address     → coverage check, set requires_private_quote
  POST /auctions/:id/shipping-quote     → seller creates quote (via chat)
  POST /auctions/:id/shipping/accept-quote → buyer accepts, set shipping_resolved_at
  POST /auctions/:id/settle             → create order (gated on shipping_resolved_at)
```

### 25.5 Workers (New/Modified)

| Worker | Purpose |
|--------|---------|
| `AuctionEndWorker` (MODIFY) | Set buyer/seller deadlines at auction end |
| `SellerShippingDeadlineWorker` (NEW) | Detect seller timeout, record violation, transition to expired_settlement |
| `BuyerShippingDeadlineWorker` (NEW) | Detect buyer timeout, record violation, transition to expired_settlement |
| `OrderPaymentTimeoutWorker` (MODIFY) | Emit BNR event for auction payment timeout |
| `AuctionSettlementWorker` (DECOMMISSION) | Replaced by new workers |
| `BNRDecayWorker` (DECOMMISSION) | No decay in locked truth |

---

## 26. Implementation Preconditions

Before implementation can begin, the following P0 contradictions must be resolved:

1. **State machine extension:** Add `expired_bnr → draft` transition (or new state). This requires:
   - Entity method: `ReturnToDraft()` with cleanup logic
   - Transition map update
   - Bid history handling decision
   - Product SellingSurface management

2. **Commerce restriction system:** Create `commerce_violations` table and enforcement. This requires:
   - Schema design (approved above with constraint fix)
   - `CommerceRestrictionChecker` implementation
   - Enforcement gates at all commerce entry points
   - Migration from `buyer_bnr_strikes`

3. **Shipping resolution decomposition:** Break `/claim` into separate steps. This requires:
   - New `shipping_resolved_at` field
   - New API endpoints
   - Mobile UI decomposition
   - Payment expiry recalculation

4. **Payment expiry fix:** For auction orders, use `shipping_resolved_at + 24h`. This requires:
   - Conditional payment expiry calculation (auction vs For Sale)
   - Backward compatibility for in-flight orders

---

## 27. Final Verdict

```
NOT READY — TECHNICAL CORRECTION REQUIRED
```

### Blocking Issues (P0)

1. **State machine cannot represent failure → DRAFT.** The `expired_bnr` state is terminal with no outgoing transition. This is a structural contradiction with the locked business truth that all three settlement failures must return the auction to DRAFT.

2. **No commerce restriction system exists.** The `buyer_bnr_strikes` table only tracks buyer auction violations. The locked business truth requires a cross-commerce restriction system for both buyer and seller violations across For Sale and Auction.

3. **No shipping resolution gate exists.** Order creation is bundled with shipping selection in the `/claim` endpoint. The locked business truth requires shipping resolution as a prerequisite for order creation, with a separate 24h payment window.

4. **Payment expiry timing is wrong.** Auction orders use method-based expiry (15min–6hr) instead of `shipping_resolved_at + 24h`. This is a fundamental business rule violation.

### Non-Blocking Issues (P1–P3)

5. Case B race condition (P1) — fixable with `shipping_resolved_at` atomic guard.
6. Seller deadline not implemented (P1) — fixable with new fields and workers.
7. Duplicate violation constraint insufficient (P1) — fixable with `actor_type` in UNIQUE.
8. `expired_bnr` semantic conflict (P1) — fixable with rename.
9. Restriction overlap undefined (P2) — owner decision required.
10. Private quote acceptance window (P2) — owner decision required.
11. `settlement_deadline` removal (P2) — straightforward after new fields deployed.
12. `claim` → `settle` rename (P3) — optional semantic improvement.

### Architecture Assessment

The v3 technical design correctly identifies most gaps and proposes reasonable solutions. However, v3 fails to adequately address:

1. The state machine extension required for failure → DRAFT (v3 proposes `expired_settlement` but does not address the terminal state problem)
2. The complete decomposition of the `/claim` endpoint
3. The commerce restriction enforcement gates at all entry points
4. The payment expiry timing fix for auction orders

The v3 design's proposed `commerce_violations` schema is correct but needs the `actor_type` constraint fix. The v3 deadline model is correct. The v3 worker design is correct.

### SOURCE CODE CHANGED: NO

This report is audit-only. No backend, mobile, admin, migration, or test files were modified.
