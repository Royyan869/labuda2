# AUCTION SETTLEMENT CANONICAL AUTHORITY — FINAL AUDIT

**Audit Date:** September 2, 2026
**Mode:** FORENSIC AUDIT ONLY — NO IMPLEMENTATION
**Authority:** Current filesystem is the ONLY implementation truth

---

## 1. Verdict

# **NOT READY — TECHNICAL CORRECTION REQUIRED**

Four P0 structural blockers prevent the canonical auction settlement lifecycle from being implemented with the current architecture. The recommended design direction is sound, but the current implementation has no shipping resolution gate, no commerce restriction system, a terminal state machine that prevents DRAFT return, and incorrect payment expiry for auctions.

---

## 2. Executive Summary

The audit answers the final question:

> "With primary address guaranteed before bidding, can Labuda now implement the canonical auction settlement flow as: BID → AUCTION END → WINNER PRIMARY ADDRESS → NORMAL SHIPPING OR PRIVATE QUOTE → SHIPPING RESOLVED → 24H PAYMENT → SUCCESS with every failure returning the auction to DRAFT?"

**NO.** Even assuming primary address enforcement (which itself requires implementation), five P0 blockers remain:

1. **PlaceBid has NO primary address check.** Backend does not validate that a bidder has a primary address before accepting a bid. `PlaceBidInput` has no address field. Zero references to address in `PlaceBid` or `PlaceBidInput`.

2. **State machine cannot return to DRAFT.** `StatusExpiredBNR: {}` is terminal with zero outgoing transitions. `StatusWaitingSettlement` can only go to `ended`, `expired_bnr`, or `cancelled`. NOT to `draft`.

3. **No shipping resolution gate.** No `shipping_resolved_at` field exists anywhere. The `/claim` endpoint bundles address submission, pricing token, and order creation atomically. There is no separate shipping resolution step.

4. **No commerce restriction system.** `buyer_bnr_strikes` only tracks buyer auction violations with a 14d/90d/permanent ladder. `HasActiveSellerCapability()` checks subscription only. No cross-commerce restriction exists.

5. **Payment expiry is wrong for auctions.** `calculatePaymentExpiry()` uses method-based timing (15min–6hr). Must be `shipping_resolved_at + 24h` for auctions.

---

## 3. Primary Address Authority Audit

### Evidence

| Component | Evidence | Finding |
|-----------|----------|---------|
| Address entity | `address.go:154-160` — `PrimaryAddressAlreadyExistsError` enforces single primary | Model is sound |
| Address repo | `address_repository_impl.go:317` — `GetPrimaryByUserID` returns nil if no primary | Deterministic lookup |
| DB constraint | `address_primary_invariant_integration_test.go:358` — unique index on primary | DB enforces single primary |
| PlaceBid service | `auction_service.go:690-789` — `PlaceBidInput{AuctionID, BidderID, Amount, IdempotencyKey}` | **NO address field** |
| PlaceBid handler | `auction_handler.go` PlaceBid endpoint — no address validation | **NO address check** |
| PlaceBid entity | `auction.go:556-590` — PlaceBid checks status, end time, self-bid, minimum | **NO address check** |
| Viewer capabilities | `auction_viewer_capabilities.go` — `canBid = canChat && isActive` | **NO address check** |
| Mobile | `auction_dto.dart` — `PlaceBidDto{amount, idempotencyKey}` | No address on bid |

### Finding

**Backend does not validate primary address before accepting a bid.** The address model is sound (deterministic, DB-enforced uniqueness), but PlaceBid does not use it. Mobile has no "Atur alamat sebelum melakukan bid" guard in the auction bid flow.

**Primary address must be added as a backend gate in PlaceBid.** This is a P0 blocker because without it, a winner may have no primary address, and settlement cannot derive a destination.

---

## 4. Bid Eligibility Audit

### Current PlaceBid Gates (auction_service.go:690-789)

1. ✅ Idempotency key required
2. ✅ Account active (`accountStatus.EnsureActive`)
3. ✅ BNR strike check (fail-open on error)
4. ✅ Idempotency lookup
5. ✅ Auction locked FOR UPDATE
6. ✅ Seller market authority (`HasActiveSellerCapability`)
7. ✅ Entity PlaceBid validation (status active, not ended, not self-bid, minimum amount)

### Missing Gates

| Gate | Status | Evidence |
|------|--------|----------|
| Primary address exists | ❌ MISSING | `PlaceBidInput` has no address; `PlaceBid` service has no address query |
| Buyer commerce restriction | ❌ MISSING | No `CommerceRestrictionChecker` exists; only `BNRStrikeChecker` |
| Anti-snipe extension | ✅ Present | `applyAntiSnipingExtension` |

### Bypass Analysis

- No alternative PlaceBid path exists (single handler, single service method)
- Admin cannot place bids (no admin bid endpoint)
- Test helpers bypass business logic (acceptable)
- **No bypasses found beyond the missing primary address gate**

---

## 5. Auction-End Winner Destination Audit

### Current Flow

```
AuctionEndWorker.endAuction()
  → AuctionService.EndAuctionInternal()
    → auction.TransitionToWaitingSettlement()
    → settlement_deadline = time.Now().Add(24h)
```

### Winner Address Resolution

**Not implemented.** The current flow:
1. Auction end worker ends auction
2. Transitions to `waiting_settlement` with `settlement_deadline = now + 24h`
3. Winner is `auction.CurrentWinnerID`
4. Winner's primary address is NEVER resolved at auction end

### Destination Authority

| Question | Answer | Evidence |
|----------|--------|----------|
| When is primary address read? | At claim time (by winner) | `ClaimAuctionRequest{AddressID}` |
| Is it resolved at auction end? | NO | `EndAuctionInternal` has no address logic |
| Can winner change address before claim? | YES | Address is read at claim, not at auction end |
| Is snapshot needed? | **YES** — for Case A/B detection | Without snapshot at auction end, coverage cannot be determined |

### P0 Finding

**Winner destination is not resolved at auction end.** The system cannot determine Case A vs Case B because it doesn't know the winner's address at auction end time. This means:
- No seller deadline can be triggered (seller doesn't know if private quote is required)
- No shipping coverage check is performed against winner's address
- The seller default timer cannot start

**Resolution required:** Winner's primary address must be resolved at auction end (or at earliest settlement entry) and stored as destination authority.

---

## 6. Case A — Outside Shipping Coverage

### Current State

**Case A does not exist in the current implementation.** There is no mechanism to:
1. Check if winner's destination is outside shipping coverage
2. Trigger a seller obligation to provide private quote
3. Enforce seller default if quote not provided
4. Return auction to DRAFT on seller default

### Evidence

| Component | Finding |
|-----------|---------|
| Shipping coverage | `ensureShippingCoverage` only runs at Schedule time — checks product has ANY coverage |
| Auction end | No destination coverage check against winner address |
| Shipping quote service | `validateAuctionForQuote` requires `status == waiting_settlement` + winner match — but no coverage check |
| Settlement worker | Only detects expired `settlement_deadline` — no coverage logic |
| No `requires_private_quote` | Correct — should be derived, not persisted |

### Verdict

Case A requires a new flow that:
1. Resolves winner's primary address at auction end
2. Checks each product's `ShippingCoverage` against winner's province
3. If no coverage matches → seller MUST provide quote → seller deadline starts
4. Seller default worker enforces seller deadline

The existing `ShippingCoverage` entity (`GetByOptionAndProvince`) provides sufficient authority for coverage detection. No new field needed for Case A detection itself — it can be derived from `winner_address.province + product_shipping_setup.coverage`.

---

## 7. Case B — Seller Special Quote

### Current State

**Case B has partial infrastructure but no auction-specific integration.**

| Component | Evidence | Finding |
|-----------|----------|---------|
| Quote creation | `shipping_quote_service.go:154` — `CreateShippingQuote` validates auction is `waiting_settlement` and chat recipient is winner | Partially works |
| Quote validation | `validateAuctionForQuote` — checks auction status + seller ownership + winner match | Works for Case B |
| Quote expiry | Default 24h expiry from creation | Provides acceptance window |
| Supersession | `SupersedeCurrentQuotes` — one active quote per context | Works |
| Race with normal shipping | **NO GUARD** | Buyer can select normal shipping while seller creates private quote |

### Case B Race Condition

```
Timeline:
T0: Auction ends, winner known, destination covered by Shipping Setup
T1: Buyer opens claim page, sees normal shipping option
T2: Seller opens chat, creates private ShippingQuote (source_type=auction)
T3: Buyer submits claim with normal ShippingSetupID
T4: Order created with normal shipping
T5: Seller's private quote remains ACTIVE but unused
```

**No guard prevents concurrent resolution.** The current `/claim` endpoint does not check for active private quotes before proceeding with normal shipping.

### Verdict

ShippingQuote lifecycle is sufficient for Case B representation. No new field needed. But **a concurrency guard is needed**: if an active private quote exists for this auction/winner, normal shipping selection must be blocked (or vice versa — first valid resolution wins).

---

## 8. Shipping Resolution Authority Audit

### Current State

**There is NO shipping resolution authority.** The concept does not exist in the current implementation.

| Component | Finding |
|-----------|---------|
| `shipping_resolved_at` | **DOES NOT EXIST** — zero references in codebase |
| `/claim` endpoint | Bundles address + pricing token + order creation atomically |
| Auction entity | No `ShippingResolvedAt` field |
| Order creation | `CreateFromAuction` has no shipping resolution check |

### Current Claim Flow (auction_handler.go ClaimAuction)

```
1. Validate winner (FOR UPDATE lock)
2. Generate pricing token (same tx)
3. Validate pricing token (same tx)
4. Create order from auction (same tx)
5. Mark pricing token consumed (same tx)
6. Settle auction (same tx)
```

**All steps are atomic.** There is no separate "shipping resolved" step.

### Required Flow

```
AUCTION END
→ Winner primary address resolved
→ Shipping path determined (normal vs private)
→ SHIPPING RESOLVED (shipping_resolved_at set ONCE)
→ Order created
→ Payment deadline = shipping_resolved_at + 24h
```

### P0 Finding

The current `/claim` endpoint cannot be trivially decomposed because it is atomic by design. The canonical flow requires a separate shipping resolution step that:
1. Sets `shipping_resolved_at` exactly once
2. Blocks all alternative resolution paths after setting
3. Creates the order after resolution
4. Sets payment deadline from resolution timestamp

---

## 9. Private Quote Acceptance Window

### Evidence

| Component | Finding |
|-----------|---------|
| Default expiry | `DefaultShippingQuoteExpiryHours = 24` (shipping_quote_service.go:23) |
| Max expiry | `MaxShippingQuoteExpiryHours = 168` (7 days) |
| Quote created | `expiresAt = time.Now().Add(expiryHours * time.Hour)` |
| `IsExpiredAt` | `!now.Before(*q.ExpiresAt)` — exact boundary |
| `IsBuyerUsableAt` | `IsCurrent() && ExpiresAt != nil && UsedAt == nil && !IsExpiredAt(now)` |

### Analysis

The existing quote expiry provides the acceptance window. Default 24h from quote creation = buyer acceptance window.

**Critical question answered:** If seller creates quote at T+23:59 (near seller deadline), buyer gets 24h from quote creation (until T+47:59). This is correct — the buyer's acceptance window is NOT shortened by the seller's late submission.

### Verdict

**Private quote acceptance window is provided by existing expiry mechanism.** No gap.

**PROPOSED — NEEDS OWNER LOCK:** `DefaultShippingQuoteExpiryHours = 24` is the acceptance window. This should be explicitly confirmed as the canonical business rule.

---

## 10. Payment Deadline Authority

### Current Implementation

| Component | Evidence | Finding |
|-----------|----------|---------|
| `calculatePaymentExpiry` | `order_creation_service.go:33-44` | Method-based: 15min–6hr |
| Auction orders | `PaymentMethodDefault = "default"` → `createdAt.Add(30 * time.Minute)` | **WRONG** — must be 24h |
| For Sale orders | Same `calculatePaymentExpiry` | Correct for For Sale |
| `settlement_deadline` | `auction.go:403` — `time.Now().Add(24 * time.Hour)` | Used by settlement worker, NOT by payment |

### P0 Finding

Auction payment expiry uses `calculatePaymentExpiry("default", order.CreatedAt)` = 30 minutes. The locked business truth requires `shipping_resolved_at + 24h`.

**No `shipping_resolved_at` exists.** Even if it did, `calculatePaymentExpiry` does not read it.

### Required

Auction order creation must set `payment_expires_at = shipping_resolved_at + 24h` instead of using `calculatePaymentExpiry`. For Sale must remain independent.

---

## 11. Concurrency / First-Resolution-Wins Proof

### Current State

**No first-resolution-wins mechanism exists.** The `/claim` endpoint is atomic but does not guard against concurrent resolution paths.

### Required Invariant

Once `shipping_resolved_at != NULL`:
- No alternative shipping resolution may succeed
- Concurrent normal selection + private quote acceptance must not produce two valid resolutions

### Proposed Mechanism

```
BEGIN
  SELECT ... FOR UPDATE FROM auctions WHERE id = $1
  -- guard: shipping_resolved_at IS NULL
  UPDATE auctions SET shipping_resolved_at = NOW() WHERE id = $1 AND shipping_resolved_at IS NULL
  -- affected_rows = 1 → first resolution wins
  -- affected_rows = 0 → second resolution rejected
COMMIT
```

**Business predicate:** `shipping_resolved_at IS NULL` is the guard. Not `FOR UPDATE` alone.

### Evidence Gap

This mechanism does not exist. Must be implemented.

---

## 12. Settlement Failure → DRAFT

### Current State Machine (auction.go:95)

```go
var transitionAllowed = map[Status][]Status{
    StatusDraft:             {StatusScheduled, StatusCancelled},
    StatusScheduled:         {StatusActive, StatusCancelled, StatusDraft},
    StatusActive:            {StatusWaitingSettlement, StatusEnded, StatusCancelled},
    StatusWaitingSettlement: {StatusEnded, StatusExpiredBNR, StatusCancelled},
    StatusExpiredBNR:        {},  // TERMINAL — no outgoing transitions
    StatusEnded:             {},
    StatusCancelled:         {},
}
```

### P0 Finding

**`StatusExpiredBNR: {}` is terminal.** There is NO outgoing transition to DRAFT. The locked business truth demands all three settlement failures return the auction to DRAFT, but the state machine structurally prevents this.

### Required Transitions

```
StatusWaitingSettlement → StatusDraft  (settlement failure)
```

### Current `TransitionToWaitingSettlement` (auction.go:397-407)

Sets `settlement_deadline = time.Now().Add(24 * time.Hour)`. This is the claim deadline, NOT the shipping resolution deadline. With the new model, this deadline concept changes.

---

## 13. Auction Attempt Reset Audit

### What Happens When Auction Returns to DRAFT

| Field | Current Value | After DRAFT Return | Evidence |
|-------|---------------|-------------------|----------|
| `current_winner_id` | Winner UUID | **MUST clear** | Old winner cannot contaminate new attempt |
| `current_bid` | Winning amount | **MUST clear** | New attempt starts fresh |
| `order_id` | nil (no order yet in settlement failure) | nil | Correct — no order exists |
| `settlement_deadline` | 24h from entry | **MUST clear** | New attempt has new deadline |
| `status` | `waiting_settlement` / `expired_bnr` | `draft` | Core transition |
| `product_id` | Product UUID | **UNCHANGED** | Product identity preserved |
| `seller_id` | Seller UUID | **UNCHANGED** | Ownership preserved |
| `start_at` / `end_at` | Original timing | **MUST reset** | New attempt needs new timing |
| `anti_snipe_extension_total` | Cumulative | **MUST reset** | Clean slate |

### Bid History

| Table | Records | After DRAFT Return | Evidence |
|-------|---------|-------------------|----------|
| `auction_bids` | All bids from attempt | **PRESERVE as history** | Historical audit data |

### Uniqueness Constraint

```sql
CREATE UNIQUE INDEX uniq_active_auction_per_product ON auctions (product_id)
WHERE (status = ANY (ARRAY['draft', 'scheduled', 'active', 'waiting_settlement']));
```

DRAFT is included. Only one non-terminal auction per product. **Compatible with DRAFT return.**

### Attempt Boundary

**Current model has NO explicit attempt_id.** The question is whether auction state transitions provide sufficient boundary.

**Verdict: `SAFE_WITHOUT_ATTEMPT_ID`** — provided that:
1. `current_winner_id` and `current_bid` are cleared on DRAFT return
2. All bid queries are scoped by auction_id (they are — `ListByAuction`)
3. Winner calculation reads current auction state, not historical bids
4. Anti-snipe extension is reset

The auction status transition itself (waiting_settlement → draft → active) serves as the attempt boundary. All settlement-specific state is cleared on DRAFT return.

---

## 14. Bid History vs Active Attempt

### Current Queries

| Query | Scope | Finds Old Bids? | Impact |
|-------|-------|----------------|--------|
| `ListByAuction` | auction_id + limit | YES — all bids | **Historical display only** |
| `GetByAuctionAndIdempotencyKey` | auction_id + bidder_id + key | Per-bidder scoped | No contamination |
| Winner calculation | `CurrentWinnerID` field on auction | Cleared on DRAFT return | Clean |
| Highest bid | `CurrentBid` field on auction | Cleared on DRAFT return | Clean |
| Anti-snipe | `EndAt` + `AntiSnipeExtensionTotal` | Reset on DRAFT return | Clean |

### Verdict

Bid history is safe to preserve because:
- All active-attempt queries use auction fields (`CurrentBid`, `CurrentWinnerID`), not bid table queries
- Bid table queries are for display/history only
- No bid table query feeds into winner calculation or settlement

---

## 15. DRAFT Reactivation Audit

### State Machine Gap

`StatusWaitingSettlement → StatusDraft` is NOT in `transitionAllowed`. Must be added.

### What Must Happen on DRAFT Return

1. **Clear** `current_winner_id` → NULL
2. **Clear** `current_bid` → NULL
3. **Clear** `settlement_deadline` → NULL
4. **Reset** `start_at` and `end_at` (seller must reconfigure timing)
5. **Reset** `anti_snipe_extension_total` → 0
6. **Preserve** `product_id` (product identity stays)
7. **Preserve** `seller_id` (ownership stays)
8. **Set** `status` → `draft`
9. **Emit** outbox event `auction.settlement_failed`
10. **Record** violation (separate table)

### Order/Payment References

**No order exists at settlement failure time.** The canonical flow is:
- Shipping resolved → order created → payment → settlement success/failure

But settlement failure occurs BEFORE payment in the new model:
- Buyer shipping failure → no order → DRAFT
- Seller default → no order → DRAFT
- Buyer BNR → order exists but payment failed → need `ReleaseUnpaidOrder`

### `ReleaseUnpaidOrder` Pattern

When buyer BNR occurs after order creation:
1. Order must be released/expired
2. Escrow must be released
3. Auction returns to DRAFT

This pattern exists in `order_completion_service.go` for order expiry but not for auction settlement failure.

---

## 16. Relist Eligibility Audit

### Business Rule

| Failure Type | Seller Restricted? | Seller Can Relist? |
|-------------|-------------------|-------------------|
| Buyer shipping failure | NO | Immediately |
| Buyer BNR | NO | Immediately |
| Seller shipping default | YES | After restriction expires |

### Current Enforcement

| Gate | Evidence | Finding |
|------|----------|---------|
| `HasActiveSellerCapability` | `role_checker_db.go:53-107` | Checks subscription only |
| Commerce restriction | **DOES NOT EXIST** | No restriction gate |
| DRAFT → Scheduled | `scheduleAuctionInternal` | Checks `HasActiveSellerCapability` only |

### P0 Finding

**No commerce restriction gate exists.** A seller who is commerce-restricted can still schedule/activate auctions because `HasActiveSellerCapability` only checks subscription.

The canonical enforcement requires:
```
ACTIVE SELLER CAPABILITY (subscription)
AND
NOT COMMERCE RESTRICTED (restriction system)
```

---

## 17. Commerce Violation Authority

### Current System: `buyer_bnr_strikes`

| Schema | Evidence |
|--------|----------|
| Table | `buyer_bnr_strikes(id, buyer_id, auction_id, struck_at, decayed_at, appeal_id, admin_reset)` |
| Scope | Buyer-only, auction-only |
| Ladder | 1→warning, 2→14d, 3→90d, 4+→permanent (bnr_restriction.go:53-95) |
| Decay | `BNRDecayWorker` — decay logic exists |
| Admin reset | `admin_reset` boolean, `BNRAdminResetter` |

### Business Truth vs Current Implementation

| Dimension | Business Truth | Current | Gap |
|-----------|---------------|---------|-----|
| Actor types | buyer + seller | buyer only | ❌ No seller violation |
| Commerce scope | For Sale + Auction | Auction only | ❌ No cross-commerce |
| Ladder | 7/15/30 days | 14/90/permanent | ❌ Wrong durations |
| Cumulative | Yes, stacking | Count-based | ⚠️ Partially compatible |
| Decay | No decay | `BNRDecayWorker` exists | ❌ Business truth says no decay |
| Permanent | No permanent escalation | 4+ = permanent | ❌ Business truth says no permanent |
| Table | `commerce_violations` (candidate) | `buyer_bnr_strikes` | ❌ Different schema |

### `UNIQUE(source_id, source_type, violation_type)` Audit

Proposed uniqueness: `UNIQUE(source_id, source_type, violation_type)`

**Insufficient.** Consider:
- Same auction can have buyer shipping failure AND buyer BNR (different failure stages)
- Same auction can have seller default AND buyer BNR
- Need `actor_type` dimension: `UNIQUE(source_id, source_type, violation_type, actor_type)`

Wait — actually the business truth says one auction produces AT MOST ONE violation (not multiple per actor type). Let me re-examine:

- Seller default → one seller violation per auction
- Buyer shipping failure → one buyer violation per auction
- Buyer BNR → one buyer violation per auction

But buyer shipping failure and buyer BNR are mutually exclusive (shipping failure happens before payment, BNR happens after). So `UNIQUE(source_id, source_type, violation_type)` may be sufficient if violation_type distinguishes shipping_failure from bnr.

**Actually:** `UNIQUE(source_id, source_type, violation_type)` IS sufficient because:
- Each violation type is unique per auction
- source_type = "auction" identifies the domain
- violation_type distinguishes the failure

But we need `actor_id` + `actor_type` for cross-commerce counting. The uniqueness constraint ensures idempotency; the actor dimensions ensure correct restriction calculation.

---

## 18. Commerce Restriction Authority

### Current Implementation

**No commerce restriction system exists.**

| Component | Finding |
|-----------|---------|
| `buyer_bnr_strikes` | Buyer-only, auction-only, wrong ladder |
| `HasActiveSellerCapability` | Subscription only |
| No `commerce_violations` table | Candidate only |
| No restriction table | Must be created |
| No enforcement at checkout | `actorResolver.CanCheckout()` checks account status, not restriction |

### Required

1. `commerce_violations` table (immutable history)
2. `commerce_restrictions` table (active restriction per actor)
3. `CommerceRestrictionChecker` (replaces `BNRStrikeChecker`)
4. Seller gate: `HasActiveSellerCapability AND NOT restricted`
5. Buyer gate: `NOT restricted` at PlaceBid and checkout

---

## 19. Restriction Stacking Audit

### Business Truth

Restrictions stack cumulatively. 1st=7d, 2nd=15d, 3rd+=30d.

### Unresolved Question

If user has restriction with 4 days remaining and gets new violation (15 days):
- A) New restriction = violation date + 15 days (replace)?
- B) New restriction = remaining 4 + 15 = 19 days from violation (extend)?
- C) New restriction = current expiry + 15 days (stack on top)?

### Verdict

**OWNER DECISION REQUIRED.** The business truth says "cumulative" and "stack" but does not specify the exact formula. This must be explicitly locked before implementation.

---

## 20. Admin Reset Audit

### Current Machinery

| Component | Evidence | Finding |
|-----------|----------|---------|
| `admin_reset` column | `buyer_bnr_strikes` schema | Boolean flag |
| `BNRAdminResetter` | `bnr_admin_reset.go` | Sets `admin_reset = TRUE` |
| Query filter | `WHERE admin_reset = FALSE` | Resets effectively remove strikes |

### Business Truth

**No normal admin reset.** Violations are immutable history.

### Verdict

**OWNER DECISION REQUIRED.** Existing admin reset machinery must be evaluated:
- Should admin have exceptional override capability?
- If yes, what authority/capability is required?
- If no, `BNRAdminResetter` and `admin_reset` column are obsolete

---

## 21. Seller Capability Enforcement

### Current Gate: `HasActiveSellerCapability`

```
GATE 1: Account status (EnsureActive)
GATE 2: Seller profile exists
GATE 3: Active subscription (started_at <= now < expires_at)
```

### Required Gate (with commerce restriction)

```
ACTIVE SELLER CAPABILITY (all 3 gates above)
AND
NOT COMMERCE RESTRICTED (new restriction check)
```

### Enforcement Points

| Operation | Current Gate | Required Additional Gate |
|-----------|-------------|------------------------|
| Create auction | Account active only | Seller commerce restriction |
| Schedule auction | HasActiveSellerCapability | + seller commerce restriction |
| PlaceBid (seller check) | HasActiveSellerCapability | + seller commerce restriction |
| Create For Sale | HasActiveSellerCapability | + seller commerce restriction |
| Create Promotion | HasActiveSellerCapability | + seller commerce restriction |

---

## 22. Buyer Commerce Enforcement

### Current Gates

| Operation | Buyer Gate | Gap |
|-----------|-----------|-----|
| PlaceBid | Account active + BNRStrikeChecker | ❌ No primary address, no cross-commerce restriction |
| Checkout (For Sale) | Account active + email verified | ❌ No commerce restriction |
| Checkout (Auction claim) | Winner check | ❌ No commerce restriction |

### Required

Buyer commerce restriction must be checked at:
1. PlaceBid (replaces BNRStrikeChecker)
2. For Sale checkout
3. Auction claim

---

## 23. Worker / Retry Audit

### Current Workers

| Worker | Purpose | Finding |
|--------|---------|---------|
| `AuctionEndWorker` | Ends expired active auctions | Correct — transitions to waiting_settlement |
| `AuctionSettlementWorker` | Detects expired settlement_deadline | **OBSOLETE** in new model — must be replaced |
| `OrderPaymentTimeoutWorker` | Expires unpaid orders | Correct for For Sale; auction path needs adjustment |
| `BNRStrikeHandler` | Handles `auction_bnr_detected` events | **OBSOLETE** — must be replaced by commerce violation handler |
| `BNRDecayWorker` | Decays BNR strikes | **OBSOLETE** — business truth says no decay |
| `BNRAdminResetter` | Admin resets BNR | **OWNER DECISION** — may be obsolete |

### New Workers Required

| Worker | Purpose |
|--------|---------|
| `SellerShippingDefaultWorker` | Detects seller deadline expiry (Case A), transitions to DRAFT, records seller violation |
| `BuyerShippingFailureWorker` | Detects buyer shipping deadline expiry, transitions to DRAFT, records buyer violation |
| `CommerceRestrictionExpiryWorker` | Detects expired restrictions, removes restriction (not the violation) |

### Duplicate Worker Analysis

- `AuctionSettlementWorker` currently handles ALL settlement timeout as single "BNR" event
- New model requires DIFFERENT workers for DIFFERENT failure types (seller default vs buyer shipping failure vs buyer BNR)
- Old worker must be decomposed, not replaced

---

## 24. Mobile Contract Audit

### Current Mobile DTOs

| DTO | Field | Finding |
|-----|-------|---------|
| `AuctionDto` | `settlementDeadline` | Parses from API — correct |
| `AuctionDto` | `status` | Supports all current statuses |
| `AuctionDto` | `canBid` | Computed from backend `can_bid` |
| `PlaceBidDto` | `{amount, idempotencyKey}` | **NO address field** |

### Stale Contracts

| Contract | Finding |
|----------|---------|
| `settlementDeadline` on AuctionDto | Will become obsolete when `settlement_deadline` is removed |
| `expired_bnr` status handling | Must be updated to handle DRAFT return |
| No shipping resolution UI | Must be added for Case A/B flows |
| No "Atur alamat sebelum melakukan bid" | Must be added |

### Missing Mobile Contracts

| Missing | Impact |
|---------|--------|
| Shipping resolution screen | Buyer cannot select normal shipping or accept private quote |
| Seller private quote creation flow | Seller cannot create auction-specific quotes |
| Seller default notification | Seller not notified of default |
| Buyer shipping failure notification | Buyer not notified of violation |
| DRAFT return notification | Seller not notified auction returned to DRAFT |

---

## 25. Admin Contract Audit

### Current Admin Types

| Type | Finding |
|------|---------|
| `apps/admin/src/types/orders.ts` | Order status types — no auction settlement types |
| Admin auction list | Filter by status — must support DRAFT return |
| Admin restriction view | **DOES NOT EXIST** for commerce restriction |

### Stale Admin Contracts

| Contract | Finding |
|----------|---------|
| `expired_bnr` display | Must be updated |
| `settlement_deadline` display | Will be removed |
| BNR admin reset button | Must be evaluated per owner decision |

---

## 26. Database / Migration Audit

### Current Schema

| Table | Column | Finding |
|-------|--------|---------|
| `auctions` | `settlement_deadline` | **OBSOLETE** — must be dropped |
| `auctions` | `status` enum includes `expired_bnr` | **OBSOLETE** — must be dropped (or kept for transition) |
| `buyer_bnr_strikes` | Full table | **OBSOLETE** — must be migrated to `commerce_violations` |
| `auction_bids` | Schema | **COMPATIBLE** — no changes needed |
| `shipping_quotes` | Schema | **COMPATIBLE** — sufficient for Case B |

### Required New Schema

| Table | Purpose |
|-------|---------|
| `commerce_violations` | Immutable violation history (actor_id, actor_type, violation_type, source_id, source_type, created_at) |
| `commerce_restrictions` | Active restrictions (actor_id, actor_type, restricted_until, violation_count) |

### Required Schema Changes

| Change | Table | Priority |
|--------|-------|----------|
| Add `shipping_resolved_at` | `auctions` | P0 |
| Add `ReturnToDraft` transition | Code only | P0 |
| Add `commerce_violations` | New table | P0 |
| Add `commerce_restrictions` | New table | P0 |
| DROP `settlement_deadline` | `auctions` | Cleanup |
| DROP `expired_bnr` from enum | `auction_status_enum` | Cleanup |
| DROP `buyer_bnr_strikes` | Table | Cleanup |

### `uniq_active_auction_per_product` Impact

```sql
WHERE (status = ANY (ARRAY['draft', 'scheduled', 'active', 'waiting_settlement']))
```

DRAFT is included. Compatible with DRAFT return. **No index change needed.**

---

## 27. Residue Audit

### `expired_bnr`

| Location | Type | Finding |
|----------|------|---------|
| `auction.go` StatusExpiredBNR | Active code | OBSOLETE after DRAFT return |
| `auction.go` TransitionToExpiredBNR | Active code | OBSOLETE after DRAFT return |
| `auction_settlement_worker.go` | Active code | OBSOLETE — entire worker replaced |
| `auction.go` transitionAllowed | Active code | Must remove expired_bnr entry |
| `auction_viewer_capabilities.go` test A6 | Test | STALE after removal |
| `auction_dto.dart` | DTO | Mobile parses — stale after removal |
| `admin/types` | Admin types | Must update |
| `report.go` | Moderation | `commerce_violation` report type — separate concern |

### `settlement_deadline`

| Location | Type | Finding |
|----------|------|---------|
| `auction.go` SettlementDeadline field | Active code | OBSOLETE after new model |
| `auction_settlement_worker.go` query | Active code | OBSOLETE |
| `auction_dto.dart` settlementDeadline | Mobile DTO | STALE after removal |
| Canonical schema migration | Schema | Will be dropped |

### `buyer_bnr_strikes`

| Location | Type | Finding |
|----------|------|---------|
| `bnr_restriction.go` BNRStrikeChecker | Active code | REPLACED by CommerceRestrictionChecker |
| `bnr_restriction.go` queryActiveStrikes | Active code | OBSOLETE |
| `bnr_strike_handler.go` | Active code | REPLACED |
| `bnr_decay_worker.go` | Active code | OBSOLETE (no decay in new model) |
| `bnr_admin_reset.go` | Active code | OWNER DECISION |
| `buyer_bnr_strikes` schema | Schema | OBSOLETE — migrate to commerce_violations |
| `BNRAuctionRestrictedError` | Error type | REPLACED by CommerceRestrictedError |

### `AuctionSettlementWorker`

| Location | Type | Finding |
|----------|------|---------|
| `auction_settlement_worker.go` | Active code | REPLACED by separate seller-default and buyer-failure workers |
| Worker registration in bootstrap | Bootstrap | Must update |

### Claim/Semantics

| Location | Type | Finding |
|----------|------|---------|
| `ClaimAuction` endpoint | Active code | Must be decomposed into shipping resolution + order creation |
| `claim-token` endpoint | Active code | Part of claim decomposition |
| `GeneratePricingTokenForAuctionClaim` | Active code | Part of claim decomposition |

---

## 28. Duplicate Authority Audit

### Current Duplicate Authorities

| Authority | Location | Duplicate Of |
|-----------|----------|-------------|
| `calculatePaymentExpiry` for auctions | order_creation_service.go:33-44 | Should be `shipping_resolved_at + 24h` |
| `settlement_deadline` field on auctions | auction.go | Should be derived from `shipping_resolved_at` |
| `BNRStrikeChecker` for auction bids | bnr_restriction.go | Should be `CommerceRestrictionChecker` |
| `AuctionViewerCapabilities.canBid` | auction_viewer_capabilities.go | Missing primary address dimension |

### No Duplicate Authority Found

| Authority | Finding |
|-----------|---------|
| ShippingQuote for Case B | Sufficient — no duplicate |
| ShippingCoverage for Case A | Sufficient — no duplicate |
| Primary address model | Sufficient — no duplicate |
| Product identity | Stable — no duplicate |

---

## 29. P0 Findings

### P0-1: State Machine Cannot Return to DRAFT

- **Finding:** `StatusExpiredBNR: {}` is terminal. `StatusWaitingSettlement` has no transition to `StatusDraft`.
- **File:** `auction.go:95,101`
- **Symbol:** `transitionAllowed[StatusWaitingSettlement]`, `transitionAllowed[StatusExpiredBNR]`
- **Current behavior:** Settlement failure → `expired_bnr` (terminal)
- **Business truth:** Settlement failure → `draft`
- **Why it conflicts:** State machine structurally prevents the canonical lifecycle
- **Root cause:** `expired_bnr` was designed as a terminal state for BNR-only
- **Required:** Add `StatusDraft` to `transitionAllowed[StatusWaitingSettlement]` + add `ReturnToDraft()` method

### P0-2: No Shipping Resolution Gate

- **Finding:** No `shipping_resolved_at` field exists. `/claim` bundles everything atomically.
- **File:** `auction_handler.go` ClaimAuction, `order_creation_service.go`
- **Symbol:** No shipping resolution concept
- **Current behavior:** Address + pricing + order creation in single atomic operation
- **Business truth:** Shipping must be resolved before order creation
- **Why it conflicts:** Cannot enforce 24h payment deadline from resolution time
- **Root cause:** MVP design bundled claim into single step
- **Required:** Add `shipping_resolved_at` to auctions, decompose `/claim` into resolution + order creation

### P0-3: No Commerce Restriction System

- **Finding:** `buyer_bnr_strikes` is buyer-only, auction-only, wrong ladder. No seller restriction.
- **File:** `bnr_restriction.go`, `role_checker_db.go`
- **Symbol:** `BNRStrikeChecker`, `HasActiveSellerCapability`
- **Current behavior:** 14d/90d/permanent for buyers only
- **Business truth:** 7/15/30d cross-commerce for both buyer and seller
- **Why it conflicts:** Seller cannot be restricted; buyer restriction doesn't apply to For Sale
- **Root cause:** BNR was auction-specific, not cross-commerce
- **Required:** New `commerce_violations` + `commerce_restrictions` tables + enforcement gates

### P0-4: Payment Expiry Wrong for Auctions

- **Finding:** `calculatePaymentExpiry` uses method-based timing (15min–6hr)
- **File:** `order_creation_service.go:33-44`
- **Symbol:** `calculatePaymentExpiry`
- **Current behavior:** 30 minutes for default payment method
- **Business truth:** `shipping_resolved_at + 24h`
- **Why it conflicts:** Buyers get 30 minutes instead of 24 hours
- **Root cause:** For Sale payment logic reused for auctions
- **Required:** Auction-specific payment expiry derived from `shipping_resolved_at`

### P0-5: PlaceBid Missing Primary Address Check

- **Finding:** Backend PlaceBid does not validate primary address exists
- **File:** `auction_service.go:690-789`
- **Symbol:** `PlaceBidInput`, `PlaceBid`
- **Current behavior:** No address validation
- **Business truth:** Bid requires primary address
- **Why it conflicts:** Winner may have no address → cannot determine shipping destination
- **Root cause:** Address was not a bid prerequisite in original design
- **Required:** Add primary address check to PlaceBid + mobile bid button disable

---

## 30. P1 Findings

### P1-1: Seller Deadline Not Implemented

- **Finding:** No `seller_shipping_deadline` field or worker exists
- **Business truth:** Seller has 24h from auction_end to provide private quote (Case A only)
- **Impact:** Seller default cannot be enforced
- **Required:** Deadline worker that differentiates Case A from Case B

### P1-2: Case A vs Case B Distinction Missing

- **Finding:** No mechanism to determine if private quote is required vs optional
- **Business truth:** Case A (outside coverage) = required; Case B (seller override) = optional
- **Impact:** Seller cannot be defaulted for not providing quote in Case B
- **Required:** Coverage check at auction end against winner's primary address

### P1-3: Race Condition — Normal Shipping vs Private Quote

- **Finding:** No guard prevents concurrent normal shipping selection and private quote creation
- **Business truth:** First valid resolution wins
- **Impact:** Two valid resolutions could coexist
- **Required:** `shipping_resolved_at IS NULL` guard in resolution path

### P1-4: Claim Endpoint Must Be Decomposed

- **Finding:** `/claim` bundles address + pricing + order creation atomically
- **Business truth:** Separate shipping resolution step required
- **Impact:** No way to resolve shipping before payment deadline starts
- **Required:** New endpoint(s) for shipping resolution, separate from order creation

### P1-5: `expired_bnr` Semantic Residue

- **Finding:** All three failure types (seller default, buyer shipping failure, buyer BNR) conflated into one state named "BNR"
- **Business truth:** Three distinct failure types with different violation and restriction consequences
- **Impact:** Incorrect violation attribution
- **Required:** Remove `expired_bnr`, use DRAFT return + violation record

---

## 31. P2 Findings

### P2-1: BNRDecayWorker Obsolete

- **Finding:** `BNRDecayWorker` implements strike decay
- **Business truth:** No automatic decay
- **Impact:** Stale code, no business effect if removed

### P2-2: Admin Reset Machinery

- **Finding:** `BNRAdminResetter` + `admin_reset` column
- **Business truth:** No normal admin reset
- **Impact:** Pending owner decision

### P2-3: Mobile `expired_bnr` Status Handling

- **Finding:** Mobile parses all statuses including `expired_bnr`
- **Impact:** Stale after removal

### P2-4: `settlement_deadline` Display

- **Finding:** Mobile + admin display `settlement_deadline`
- **Impact:** Stale after removal

### P2-5: `BNRAuctionRestrictedError`

- **Finding:** Auction-specific error type
- **Impact:** Must be replaced by cross-commerce `CommerceRestrictedError`

---

## 32. Remaining Owner Decisions

### OD-1: Private Quote Acceptance Window

**PROPOSED:** `DefaultShippingQuoteExpiryHours = 24` (24h from quote creation)
**STATUS:** Technically provided by existing expiry mechanism. Needs owner confirmation.

### OD-2: Restriction Stacking Formula

**Question:** When user has restriction with N days remaining and gets new violation:
- A) Replace: new restriction = violation_date + new_duration
- B) Extend: new restriction = max(current_expiry, violation_date + new_duration)
- C) Stack: new restriction = current_expiry + new_duration

**STATUS:** Business truth says "cumulative" and "stack" but exact formula unresolved.

### OD-3: Admin Manual Violation Reset

**Question:** Should admin have exceptional ability to reset violations?
- A) No reset — violations are immutable history
- B) Reset with audit trail + elevated capability
- C) Reset only for specific violation types

**STATUS:** Existing `BNRAdminResetter` provides some precedent. Owner decision required.

### OD-4: Winner Address Snapshot Timing

**Question:** When exactly should winner's primary address be resolved?
- A) At auction end (deterministic, but address may change)
- B) At first settlement action (winner initiates claim)
- C) At shipping resolution (latest possible)

**Business truth says:** "Primary address is prerequisite for bidding" → address exists at bid time. But winner address should be resolved deterministically. Recommended: at auction end or first settlement action.

---

## 33. Exact Implementation Boundary (After P0 Corrections)

### New Files Required

| File | Purpose |
|------|---------|
| `commerce_violation/entity/commerce_violation.go` | Violation entity |
| `commerce_violation/repository/commerce_violation_repository.go` | Violation repo |
| `commerce_restriction/entity/commerce_restriction.go` | Restriction entity |
| `commerce_restriction/application/commerce_restriction_checker.go` | Cross-commerce restriction check |
| `worker/seller_shipping_default_worker.go` | Seller deadline enforcement |
| `worker/buyer_shipping_failure_worker.go` | Buyer shipping deadline enforcement |

### Modified Files Required

| File | Change |
|------|--------|
| `auction/entity/auction.go` | Add `StatusDraft` to `transitionAllowed[StatusWaitingSettlement]`; add `ReturnToDraft()` method; add `ShippingResolvedAt` field; remove `SettlementDeadline` usage for payment |
| `auction/application/auction_service.go` | Add primary address check to `PlaceBid`; add `ResolveShipping()` method |
| `auction/delivery/http/auction_handler.go` | Add shipping resolution endpoint; modify `/claim` |
| `order/application/order_creation_service.go` | Auction payment expiry = `shipping_resolved_at + 24h` |
| `identity/auth/role_checker_db.go` | Add commerce restriction check to `HasActiveSellerCapability` |
| `worker/auction_end_worker.go` | Resolve winner address at auction end; trigger Case A/B detection |
| `worker/auction_settlement_worker.go` | **DELETE** — replaced by specific workers |
| `shared/auction_viewer_capabilities.go` | Add primary address to `canBid` computation |
| Migrations | Add `shipping_resolved_at`, `commerce_violations`, `commerce_restrictions`; drop `expired_bnr`, `settlement_deadline`, `buyer_bnr_strikes` |

### Deleted Files Required

| File | Reason |
|------|--------|
| `worker/auction_settlement_worker.go` | Replaced by specific workers |
| `worker/bnr_strike_handler.go` | Replaced by commerce violation handler |
| `worker/bnr_decay_worker.go` | No decay in new model |
| `worker/bnr_admin_reset.go` | Pending owner decision |

---

## 34. Exact Cleanup Boundary

| Artifact | Action |
|----------|--------|
| `expired_bnr` enum value | DROP from `auction_status_enum` |
| `settlement_deadline` column | DROP from `auctions` |
| `buyer_bnr_strikes` table | DROP (after data migration to `commerce_violations`) |
| `AuctionSettlementWorker` | DELETE file + bootstrap wiring |
| `BNRStrikeHandler` | DELETE file + bootstrap wiring |
| `BNRDecayWorker` | DELETE file + bootstrap wiring |
| `BNRAdminResetter` | DELETE file + bootstrap wiring |
| `BNRStrikeChecker` | REPLACE with `CommerceRestrictionChecker` |
| `BNRAuctionRestrictedError` | REPLACE with `CommerceRestrictedError` |
| Mobile `expired_bnr` handling | UPDATE |
| Mobile `settlementDeadline` field | REMOVE after `settlement_deadline` column dropped |
| Admin types | UPDATE for new statuses |

---

## 35. Required Regression Proof

### State Machine

- [ ] `waiting_settlement → draft` transition works atomically
- [ ] `draft → scheduled → active` cycle works after DRAFT return
- [ ] `uniq_active_auction_per_product` satisfied after DRAFT return
- [ ] `expired_bnr` removed from state machine
- [ ] `ended` remains terminal

### Shipping Resolution

- [ ] `shipping_resolved_at` set exactly once
- [ ] Concurrent normal + private resolution: first wins
- [ ] Order creation blocked when `shipping_resolved_at IS NULL`
- [ ] Normal shipping selection blocked when active private quote exists
- [ ] Private quote acceptance blocked when normal shipping already selected

### Payment

- [ ] Auction payment expires at `shipping_resolved_at + 24h`
- [ ] For Sale payment expiry unchanged
- [ ] BNR detected only after buyer was actually able to pay

### Restriction

- [ ] Buyer restriction blocks For Sale + Auction
- [ ] Seller restriction blocks For Sale + Auction + relist
- [ ] Cumulative violation count correct
- [ ] Restriction expiry correct per stacking formula (after owner decision)
- [ ] No admin reset (unless owner decides otherwise)

### Attempt Isolation

- [ ] `current_winner_id` cleared on DRAFT return
- [ ] `current_bid` cleared on DRAFT return
- [ ] `anti_snipe_extension_total` reset on DRAFT return
- [ ] Bid history preserved but not active
- [ ] New attempt starts fresh

---

## 36. Final Verdict

# **NOT READY — TECHNICAL CORRECTION REQUIRED**

### Blockers Summary

| # | Blocker | Severity | Effort |
|---|---------|----------|--------|
| 1 | State machine cannot return to DRAFT | P0 | Low (1 line + 1 method) |
| 2 | No shipping resolution gate | P0 | High (new field + endpoint decomposition) |
| 3 | No commerce restriction system | P0 | High (new tables + services) |
| 4 | Payment expiry wrong for auctions | P0 | Medium (conditional logic) |
| 5 | PlaceBid missing primary address | P0 | Low (1 query + 1 check) |
| 6 | Seller deadline not implemented | P1 | Medium (new worker) |
| 7 | Case A/B distinction missing | P1 | Medium (coverage check at auction end) |
| 8 | Race condition unresolved | P1 | Low (guard condition) |

### After All Corrections

The canonical auction settlement flow becomes deterministic:

```
BID (with primary address)
→ AUCTION END (winner + primary address resolved)
→ SHIPPING PATH (Case A: required private quote / Case B: optional / Normal: buyer selects)
→ SHIPPING RESOLVED (shipping_resolved_at set once)
→ 24H PAYMENT
→ SUCCESS (→ ended) or FAILURE (→ draft + violation)
```

Every failure path returns to DRAFT with correct violation attribution and appropriate restriction. No duplicate authority, no hidden state, no legacy residue.

**Estimated total implementation scope:** ~8 new files, ~15 modified files, ~5 deleted files, ~3 new migrations.

---

*End of Audit*
