# FINAL PRE-IMPLEMENTATION FORENSIC AUDIT

## WINNER DESTINATION + SHIPPING RESOLUTION + FAILURE ROLLBACK

**Audit Date:** September 2, 2026
**Mode:** AUDIT ONLY — NO IMPLEMENTATION
**Authority:** Current filesystem is the ONLY implementation truth

---

## VERDICT

# **NOT_READY — TECHNICAL CORRECTION REQUIRED**

Architecture is sound. No authority ambiguity remains. But 4 P0 structural gaps prevent deterministic implementation without assumption.

---

## P0 BLOCKERS

### P0-1: PlaceBid Has No Primary Address Gate

**Evidence:**
- `PlaceBidInput{AuctionID, BidderID, Amount, IdempotencyKey}` — `auction_service.go:690`
- `PlaceBid` service method: checks account active, BNR strike, idempotency, auction lock, seller capability, entity validation — **zero address references**
- `PlaceBid` entity method `auction.go:556-590`: checks status, end time, self-bid, minimum amount — **zero address references**
- `EvaluateAuctionViewerCapabilities` `auction_viewer_capabilities.go`: `canBid = canChat && isActive` — **zero address references**
- Mobile `PlaceBidDto{amount, idempotencyKey}` — `auction_dto.dart`

**Contradiction:** Business truth requires "bidder must have primary address before bidding." Backend accepts bids from bidders with no primary address.

**Impact:** Winner may have no address → destination cannot be determined → Case A/B detection impossible → settlement flow blocked.

### P0-2: State Machine Prevents DRAFT Return

**Evidence:**
- `transitionAllowed[StatusExpiredBNR] = {}` — `auction.go:101` — terminal, zero outgoing transitions
- `transitionAllowed[StatusWaitingSettlement] = {StatusEnded, StatusExpiredBNR, StatusCancelled}` — `auction.go:100` — NO `StatusDraft`
- `TransitionToExpiredBNR()` — `auction.go:412-419` — sets status to `expired_bnr`, no further transitions possible
- `TransitionToWaitingSettlement()` — `auction.go:397-407` — sets `settlement_deadline = now + 24h`

**Contradiction:** All three settlement failures must return auction to DRAFT. State machine structurally prevents this.

### P0-3: No Shipping Resolution Authority

**Evidence:**
- `shipping_resolved_at` — zero references in entire codebase (confirmed by code_search)
- `/claim` endpoint — `auction_handler.go ClaimAuction` — bundles validate winner + generate pricing token + validate token + create order + settle auction — all in one atomic transaction
- No separate "shipping resolved" step exists
- `CreateFromAuction` — `order_creation_service.go:685` — creates order with `calculatePaymentExpiry(snapshot.PaymentMethod, time.Now())` — method-based, not resolution-based

**Contradiction:** Business truth requires `shipping_resolved_at` as immutable marker before order creation. No such concept exists.

### P0-4: Payment Expiry Wrong for Auctions

**Evidence:**
- `calculatePaymentExpiry("default", time.Now())` → `createdAt.Add(30 * time.Minute)` — `order_creation_service.go:33-44`
- `CreateFromAuction` line: `calculatePaymentExpiry(snapshot.PaymentMethod, time.Now())` — `order_creation_service.go`
- `PaymentMethodDefault = "default"` — all auction orders use this

**Contradiction:** Business truth requires `payment_deadline = shipping_resolved_at + 24h`. Current: 30 minutes.

---

## P1 FINDINGS

### P1-1: No Commerce Restriction System

**Evidence:**
- `buyer_bnr_strikes` table: `buyer_id, auction_id, struck_at, decayed_at, admin_reset` — buyer-only, auction-only — `migration 000001:575`
- `BNRStrikeChecker` evaluate: 1→warning, 2→14d, 3→90d, 4+→permanent — `bnr_restriction.go:53-95`
- `HasActiveSellerCapability`: checks account status + seller profile + subscription — `role_checker_db.go:53-107` — **zero restriction check**
- No `commerce_violations` table exists
- No `commerce_restrictions` table exists
- No seller restriction enforcement anywhere

### P1-2: Seller Default Worker Does Not Exist

**Evidence:**
- `AuctionSettlementWorker` — `auction_settlement_worker.go` — only detects expired `settlement_deadline` and transitions to `expired_bnr`
- No worker differentiates Case A (seller obligation) from Case B (optional) from buyer failure
- No seller deadline field exists on auction entity
- No Case A detection logic exists

### P1-3: Race Condition — Normal Shipping vs Private Quote

**Evidence:**
- `/claim` requires `ShippingSetupID` — `ClaimAuctionRequest` — `auction_handler.go`
- `ShippingQuote` can be created via `CreateShippingQuote` — `shipping_quote_service.go:154`
- **No guard** prevents buyer from submitting claim with normal ShippingSetupID while seller creates private quote
- **No guard** prevents seller from creating quote while buyer submits normal claim
- `validateShippingQuoteForOrder` — `order_creation_service.go` — only validates quote exists and is ACTIVE, does NOT check if normal shipping was already resolved

### P1-4: Buyer Shipping-Selection Failure Worker Does Not Exist

**Evidence:**
- No buyer shipping deadline field exists
- No worker detects buyer failure to select shipping
- `AuctionSettlementWorker` treats all settlement timeout uniformly as "BNR"

---

## VERIFIED CLEAN

These concerns are deterministic and require no additional authority:

### Primary Address Model
- `GetPrimaryByUserID` — `address_repository_impl.go:317` — deterministic query
- DB unique constraint on primary — `address_primary_invariant_integration_test.go:358`
- `PrimaryAddressAlreadyExistsError` — `address.go:154` — enforces single primary
- `GetPrimaryAddressForCheckout` — `address_service.go:442` — returns nil if no primary

### ShippingQuote Sufficiency for Case B
- `ShippingQuote` entity — `shipping_quote.go` — has `SourceType`, `SourceID`, `SellerID`, `BuyerID`, `ProductID`, `ChatID`
- `validateAuctionForQuote` — `shipping_quote_service.go` — validates auction is `waiting_settlement` + seller ownership + winner match
- Quote lifecycle: `ACTIVE → USED | EXPIRED` — deterministic
- Supersession: `SupersedeCurrentQuotes` — one active quote per canonical context — deterministic
- Quote expiry: `DefaultShippingQuoteExpiryHours = 24` — deterministic acceptance window
- `IsBuyerUsableAt(now)` — `shipping_quote.go` — `IsCurrent() && ExpiresAt != nil && UsedAt == nil && !IsExpiredAt(now)` — deterministic

### Coverage Detection Authority
- `GetByOptionAndProvince(shippingSetupID, provinceCode)` — `shipping_coverage_repository_impl.go:167` — deterministic province-level coverage check
- `CheckDeliveryAvailabilityForProduct(productID, provinceCode, cityCode)` — `shipping_service.go:164` — returns available options
- `ensureShippingCoverage` — `auction_service.go:488` — checks at least one coverage exists per product at schedule time

### Bid History Isolation
- `auction_bids` schema: `id, auction_id, bidder_id, amount, idempotency_key, created_at` — `migration 000001:468`
- All bid queries scoped by `auction_id`
- Active-attempt state stored on `auctions` entity (`CurrentBid`, `CurrentWinnerID`), NOT on bid table
- `ListByAuction` — `bidRepo` — display only, feeds no settlement logic

### Uniqueness Constraint Compatible
- `uniq_active_auction_per_product` — `migration 000001:2015` — `WHERE status IN (draft, scheduled, active, waiting_settlement)`
- DRAFT is included — only one non-terminal auction per product — compatible with DRAFT return

### ReleaseUnpaidOrder Pattern
- `ReleaseUnpaidOrder(orderID)` — `auction.go:462-472` — clears `OrderID`, idempotent, mismatch-safe
- `releaseAuctionOrderBinding` — `order_completion_service.go:2036-2054` — locks auction, clears binding, persists

### Order Expire Chain (existing)
- `Expire()` — `order_completion_service.go:1008` — locks order → mark expired → restore stock → release escrow (blocking) → coins refund event → shipping quote reactivation → update status → outbox event
- Escrow: `walletService.GetEscrowForOrder` → `InitiateGatewayRefundForOrder` → `RefundToBuyer` — atomic, blocking, idempotent
- Coins: `coins.refund_required` outbox event — idempotent handler
- Shipping quote: `ReactivateQuoteIfEligible` — 10-step validation — idempotent

---

## FINAL AUTHORITY MATRIX

| Concern | Canonical Authority | Writer | Consumer | Evidence | Status |
| ------- | ------------------- | ------ | -------- | -------- | ------ |
| Primary address | `addresses` WHERE `user_id=$1 AND is_primary=true` | AddressService.SetPrimary | PlaceBid (MISSING), Claim, Checkout | `address_repository_impl.go:317` | ⚠️ Not enforced at bid |
| Winner destination | Winner's primary address at resolution time | Read at settlement | Shipping coverage check, quote creation | `GetPrimaryByUserID` | ⚠️ Not resolved at auction end |
| Case A detection | `winner_province NOT IN (product_shipping_coverage.province_codes)` | Derived at auction end | Seller default worker, seller notification | `GetByOptionAndProvince` + `GetPrimaryByUserID` | ❌ Not implemented |
| Case B quote | `ShippingQuote` WHERE `source_type='auction' AND status='ACTIVE' AND buyer_id=winner` | Seller via chat | Buyer acceptance, order creation | `shipping_quote_service.go:154` | ✅ Sufficient |
| Seller obligation | Case A detection result (derived) | Worker | Seller deadline enforcement | Derived from coverage + address | ❌ No worker |
| Buyer shipping deadline | `auction_end + 24h` | AuctionEndWorker | Buyer shipping failure worker | To be set at auction end | ❌ Not implemented |
| Shipping resolution | `shipping_resolved_at` on auctions | First valid resolution | Order creation gate | DOES NOT EXIST | ❌ Not implemented |
| Payment deadline | `shipping_resolved_at + 24h` | Order creation | Payment timeout worker | `calculatePaymentExpiry` (WRONG) | ❌ Wrong authority |
| Auction state | `auctions.status` | State machine | All consumers | `auction.go:95` | ⚠️ Missing DRAFT transition |
| Auction attempt | State transitions (draft→active→waiting_settlement→draft) | State machine | Bid isolation, winner calculation | State machine fields | ⚠️ DRAFT return blocked |
| Bid history | `auction_bids` rows | PlaceBid | Display only | `migration 000001:468` | ✅ Safe |
| Violation | `commerce_violations` (TO BE CREATED) | Settlement failure handlers | Restriction calculation | DOES NOT EXIST | ❌ Not implemented |
| Restriction | `commerce_restrictions` (TO BE CREATED) | Violation handlers | PlaceBid, CreateDraft, Schedule | DOES NOT EXIST | ❌ Not implemented |
| Order rollback | `Expire()` + `releaseAuctionOrderBinding` | OrderPaymentTimeoutWorker | Auction DRAFT return | `order_completion_service.go:1008` | ✅ Chain exists but no DRAFT return |

---

## DESTINATION VERDICT

**`DESTINATION_CAN_BE_DERIVED`**

Winner's primary address is deterministically derivable via `GetPrimaryByUserID(winnerID)`. No snapshot is needed IF the address is resolved at settlement time (not at bid time). The canonical source is the `addresses` table with `is_primary=true`. DB enforces single primary via unique constraint.

**Requirement:** Winner's primary address must be read at settlement entry (auction end or first settlement action) and used as the canonical destination for coverage check and shipping resolution.

---

## SHIPPING RESOLUTION VERDICT

### Marker vs Content

| Concern | Authority | Writer | Consumer | Mutable After Set? |
| ------- | --------- | ------ | -------- | ------------------ |
| Resolution mode (normal/quote) | `shipping_resolved_at` existence + `shipping_setup_id` or `shipping_quote_id` on order | First valid resolution action | Order creation, payment deadline | NO |
| Selected Shipping Setup | `orders.shipping_setup_id` | Order creation (from resolution) | Pricing, display | NO (snapshot) |
| Selected Shipping Quote | `orders.shipping_quote_id` | Order creation (from resolution) | Validation, reactivation | NO (snapshot) |
| Shipping amount | `orders.shipping_total` | Pricing snapshot at resolution | Payment | NO (snapshot) |
| Destination | `orders.shipping_destination_*` | Address snapshot at resolution | Display, dispute | NO (snapshot) |
| shipping_resolved_at | `auctions.shipping_resolved_at` | First valid resolution | Order creation gate | NO (immutable) |

**No duplicate authority.** Each concern has exactly one writer. Auction holds the resolution marker; order holds the resolved content as immutable snapshots.

---

## SHIPPING RESOLUTION RACE PROOF

### First-Resolution-Wins Mechanism

**Business predicate:** `shipping_resolved_at IS NULL`

**Transaction boundary:**
```
BEGIN
  SELECT ... FOR UPDATE FROM auctions WHERE id = $1
  -- guard: shipping_resolved_at IS NULL
  UPDATE auctions SET shipping_resolved_at = NOW()
    WHERE id = $1 AND shipping_resolved_at IS NULL
  -- affected_rows = 1 → first resolution wins
  -- affected_rows = 0 → second resolution rejected
  -- create order with resolved shipping
COMMIT
```

### Race Simulations

**Race 1: Buyer normal selection + Seller private quote creation**
- T1: Buyer submits claim with ShippingSetupID
- T2: Seller creates ShippingQuote
- T3: Both attempt resolution
- **First to set `shipping_resolved_at` wins.** Second sees `shipping_resolved_at IS NULL` = false → rejected.

**Race 2: Buyer accepts quote + Buyer selects normal shipping**
- Same mechanism. `shipping_resolved_at IS NULL` guard prevents double resolution.

**Race 3: Resolution request + Worker deadline**
- Worker must also respect `shipping_resolved_at IS NULL` before recording violation.
- If resolution wins first → no violation.
- If worker wins first → violation recorded + DRAFT return → resolution path blocked by status check.

**Idempotency:** Same resolution request retried → `shipping_resolved_at` already set → second attempt is no-op.

---

## PRIVATE QUOTE ACCEPTANCE VERDICT

**`24H_FROM_QUOTE_CREATION_SUPPORTED`**

**Evidence:**
- `DefaultShippingQuoteExpiryHours = 24` — `shipping_quote_service.go:23`
- `expiresAt = time.Now().Add(time.Duration(expiryHours) * time.Hour)` — `shipping_quote_service.go:175`
- `IsExpiredAt(now)` — `!now.Before(*q.ExpiresAt)` — `shipping_quote.go`
- `IsBuyerUsableAt(now)` — `IsCurrent() && ExpiresAt != nil && UsedAt == nil && !IsExpiredAt(now)` — `shipping_quote.go`

**Scenario: Seller creates quote at T+23:59**
- `expiresAt = T+23:59 + 24h = T+47:59`
- Buyer has 24h from quote creation, not from auction end
- Seller deadline and buyer acceptance window are independent authorities

---

## SELLER SHIPPING DEFAULT ANALYSIS

### Current State

**No seller default mechanism exists.**

| Component | Finding |
|-----------|---------|
| Seller obligation detection | ❌ No Case A detection at auction end |
| Seller deadline field | ❌ No `seller_shipping_deadline` on auction |
| Seller default worker | ❌ Does not exist |
| Case A vs Case B distinction | ❌ No mechanism |
| Seller notification of obligation | ❌ Does not exist |

### Required Flow

```
AUCTION END
→ resolve winner primary address
→ for each product shipping setup:
    check coverage: GetByOptionAndProvince(setup_id, winner_province)
→ if NO coverage covers winner province:
    CASE A detected
    seller MUST provide quote
    seller_deadline = auction_end + 24h
    start seller default worker
→ if coverage covers winner province:
    CASE B possible (seller MAY provide optional quote)
    buyer may select normal shipping
    buyer_deadline = auction_end + 24h
```

### Seller Default Rule

Seller obligation is: **provide a ShippingQuote** before deadline.

NOT: **shipping resolved** before deadline.

If seller creates ANY ACTIVE quote for this auction/winner before deadline → obligation fulfilled. Buyer acceptance is a separate step.

---

## BUYER SHIPPING-SELECTION FAILURE ANALYSIS

### Current State

**No buyer shipping failure mechanism exists.**

| Component | Finding |
|-----------|---------|
| Buyer shipping deadline | ❌ No field on auction |
| Buyer shipping failure worker | ❌ Does not exist |
| Case A/B distinction | ❌ Cannot distinguish |
| Buyer violation recording | ❌ Does not exist |
| Auction DRAFT return on buyer failure | ❌ State machine blocks it |

### Case B Impact on Buyer Deadline

If seller creates Case B quote while buyer hasn't selected normal shipping:
- **Buyer deadline is NOT affected.** Buyer can still select normal shipping OR accept the private quote.
- Both paths remain valid until resolution.
- First valid resolution wins (see race proof above).

---

## BUYER BNR ROLLBACK — FULL FINANCIAL CHAIN

### Trace: BNR After Payment Failure

```
1. shipping_resolved_at set → order created → payment_expires_at = resolved + 24h
2. Buyer does not pay within 24h
3. OrderPaymentTimeoutWorker detects expired order
4. OrderService.Expire(orderID)
   a. Lock order FOR UPDATE
   b. order.MarkExpired() — pending → expired
   c. restoreForSaleStock / releaseAuctionOrderBinding
      → Auction.ReleaseUnpaidOrder(orderID) — clears OrderID
   d. walletService.GetEscrowForOrder → nil (no payment made)
      → "expiry_no_escrow_canonical_skip" — no gateway refund needed
   e. coins.refund_required event (if coins used)
   f. ReactivateQuoteIfEligible (if shipping quote used)
   g. Update order status → expired
   h. Emit order.expired outbox event
5. auction_bnr_detected event emitted (by new BNR detection worker)
6. Commerce violation recorded
7. Commerce restriction applied
8. Auction → DRAFT (via new ReturnToDraft)
```

### Financial Residue Check

| Concern | Status | Evidence |
|---------|--------|----------|
| Escrow | ✅ Clean | No escrow if buyer never paid; `GetEscrowForOrder` returns nil |
| Coins | ✅ Clean | `coins.refund_required` event → idempotent handler |
| Payment intent | ✅ Clean | Order status → expired terminalizes payment flow |
| Shipping quote | ✅ Clean | `ReactivateQuoteIfEligible` reactivates if eligible |
| Order | ✅ Clean | Status → expired (terminal) |
| Auction binding | ✅ Clean | `ReleaseUnpaidOrder` clears OrderID atomically |
| Ledger | ✅ Clean | No ledger entry if buyer never paid |

**The existing `Expire()` chain is financially sound for BNR rollback.** The only gap is the final step: auction must return to DRAFT (currently blocked by state machine).

---

## AUCTION DRAFT RETURN / ATTEMPT RESET — FIELD TABLE

| Field | Current Value | After DRAFT Return | Action | Why |
| ----- | ------------- | ------------------ | ------ | --- |
| `status` | `waiting_settlement` | `draft` | **SET** | Core transition |
| `current_winner_id` | Winner UUID | `NULL` | **CLEAR** | Old winner must not contaminate new attempt |
| `current_bid` | Winning amount | `NULL` | **CLEAR** | New attempt starts fresh |
| `order_id` | `nil` (at settlement failure) | `nil` | **KEEP** | No order exists at failure time |
| `shipping_resolved_at` | `nil` (at shipping failure) or set (at BNR) | `NULL` | **CLEAR** | New attempt re-resolves shipping |
| `settlement_deadline` | 24h from entry | `NULL` | **CLEAR** | Obsolete in new model |
| `start_at` | Original timing | Seller re-sets | **RESET** | New attempt needs new timing |
| `end_at` | Original timing | Seller re-sets | **RESET** | New attempt needs new timing |
| `anti_snipe_extension_total` | Cumulative | `0` | **RESET** | Clean slate |
| `product_id` | Product UUID | **UNCHANGED** | **KEEP** | Product identity preserved |
| `seller_id` | Seller UUID | **UNCHANGED** | **KEEP** | Ownership preserved |
| `start_price` | Original | **UNCHANGED** | **KEEP** | Seller may edit in DRAFT |
| `bid_increment` | Original | **UNCHANGED** | **KEEP** | Seller may edit in DRAFT |
| `buy_now_price` | Original | **UNCHANGED** | **KEEP** | Seller may edit in DRAFT |
| `created_at` | Creation time | **UNCHANGED** | **KEEP** | Historical |
| `updated_at` | Last update | `NOW()` | **SET** | Track DRAFT return |
| `product` (joined) | Canonical Product | **UNCHANGED** | **KEEP** | Product entity independent |

**Bid history (`auction_bids`):** PRESERVED. Historical audit data. All active-attempt queries use auction entity fields, not bid table.

---

## ATTEMPT ISOLATION — FINAL PROOF

**Verdict: `SAFE_WITHOUT_ATTEMPT_ID`**

### Exhaustive Consumer Audit

| Consumer | Reads From | Uses Active State? | Contaminated by Old Bids? | Evidence |
|----------|-----------|-------------------|--------------------------|----------|
| Winner calculation | `auction.CurrentWinnerID` | YES | NO — cleared on DRAFT return | `auction.go:580-581` |
| Highest bid | `auction.CurrentBid` | YES | NO — cleared on DRAFT return | `auction.go:579` |
| Bid validation | `entity.PlaceBid()` checks `CurrentBid` | YES | NO — uses entity field | `auction.go:572-576` |
| Anti-snipe | `auction.EndAt` + `AntiSnipeExtensionTotal` | YES | NO — reset on DRAFT return | `auction.go:596-611` |
| Bid history display | `auction_bids` rows | NO — read-only | YES — historical display is correct | `bidRepo.ListByAuction` |
| Idempotency | `GetByAuctionAndIdempotencyKey` scoped to bidder | Per-bidder | NO — same bidder+key = same bid | `bidRepo` |
| Notifications | Payload from auction entity | YES | NO — event payload from current state | `buildAuctionPayload` |
| Admin display | Auction entity + bid list | Mixed | NO — admin sees history separately | Handler response |
| Moderation | Auction status | YES | NO — status-based | Handler |
| Analytics | Event payloads | YES | NO — snapshot at event time | Outbox |
| Mobile DTO | API response from entity | YES | NO — response from current state | `auctionToResponse` |
| Chat projection | `EvaluateAuctionViewerCapabilities` | YES | NO — uses entity status | `auction_viewer_capabilities.go` |
| Tests | Various | Mixed | NO — test isolation per case | Test files |

### Proof

1. **Historical bids never determine current winner.** Winner = `auction.CurrentWinnerID`, set by `PlaceBid()`. Cleared on DRAFT return.

2. **Historical bids never determine current highest bid.** Highest = `auction.CurrentBid`, set by `PlaceBid()`. Cleared on DRAFT return.

3. **Old bids cannot affect anti-snipe.** Anti-snipe uses `auction.EndAt` and `auction.AntiSnipeExtensionTotal`. Both reset on DRAFT return.

4. **Idempotency keys cannot collide between attempts.** Keys are scoped by `(auction_id, bidder_id, idempotency_key)`. Same auction, same bidder, same key = same bid. New attempt requires new bids from new state.

5. **Current auction fields are sufficient as active-attempt authority.** `CurrentBid`, `CurrentWinnerID`, `Status`, `EndAt`, `AntiSnipeExtensionTotal` collectively define the active attempt.

6. **DRAFT return + new ACTIVE attempt is truly isolated.** All mutable attempt state is cleared. Historical data is preserved for audit but feeds no active logic.

---

## COMMERCE VIOLATION + RESTRICTION SYSTEM

### Current BNR System — Destructive Audit

| Component | File | Finding | Disposition |
|-----------|------|---------|-------------|
| `buyer_bnr_strikes` table | `migration 000001:575` | buyer-only, auction-only, has `admin_reset` + `decayed_at` | **REPLACE** with `commerce_violations` |
| `BNRStrikeChecker` | `bnr_restriction.go` | buyer-only, 14d/90d/permanent, decay-aware | **REPLACE** with `CommerceRestrictionChecker` |
| `BNRStrikeHandler` | `bnr_strike_handler.go` | Inserts `buyer_bnr_strikes` row on `auction_bnr_detected` | **REPLACE** with commerce violation handler |
| `BNRDecayWorker` | `bnr_decay_worker.go` | Decays strikes over time | **DELETE** — business truth says no decay |
| `BNRAdminResetter` | `bnr_admin_reset.go` | Sets `admin_reset = TRUE` | **OWNER DECISION** |
| `BNRAuctionRestrictedError` | `auction.go` | Auction-specific error with `ActiveStrikes`, `PermanentBan`, `RestrictionUntil` | **REPLACE** with `CommerceRestrictedError` |
| `auction_bnr_detected` event | `auction_settlement_worker.go` | Emitted on settlement timeout | **REPLACE** with specific violation events |

### What Must Be Replaced Total

1. `buyer_bnr_strikes` → `commerce_violations` (immutable history) + `commerce_restrictions` (active state)
2. `BNRStrikeChecker` → `CommerceRestrictionChecker` (cross-commerce, correct ladder)
3. `BNRStrikeHandler` → `CommerceViolationHandler` (records violations for all three types)
4. `BNRDecayWorker` → DELETE (no decay)
5. `AuctionSettlementWorker` → Decomposed into specific failure workers

### What Is Obsolete

- `expired_bnr` status — replaced by DRAFT return + violation record
- `settlement_deadline` field — replaced by `shipping_resolved_at + 24h` for payment
- `TransitionToExpiredBNR()` — replaced by `ReturnToDraft()`
- `BNRAuctionRestrictedError` — replaced by cross-commerce error

### Violation History Immutability

`commerce_violations` rows are INSERT-ONLY. No UPDATE, no DELETE. Violations are historical facts. Restriction state is separate (`commerce_restrictions`).

### Enforcement Points

| Entry Point | Current Gate | Required Additional Gate |
|-------------|-------------|------------------------|
| PlaceBid | BNRStrikeChecker (buyer only) | CommerceRestrictionChecker (buyer + seller) |
| CreateDraft | Account active | Seller commerce restriction |
| Schedule | HasActiveSellerCapability | + Seller commerce restriction |
| CreateForSale | HasActiveSellerCapability | + Seller commerce restriction |
| Checkout (For Sale) | Account active + profile | Buyer commerce restriction |
| Checkout (Auction claim) | Winner check | Buyer commerce restriction |

---

## RESTRICTION STACKING

**`OWNER_DECISION_REQUIRED`**

Business truth says "cumulative" and "stack" but the exact mathematical formula is not locked:

- **Option A (Replace):** New restriction = `MAX(current_expiry, violation_date + new_duration)`
- **Option B (Extend):** New restriction = `current_expiry + new_duration`

This must be explicitly decided before implementation. Do not silently adopt either formula.

---

## ADMIN RESET

**`OWNER_DECISION_REQUIRED`**

Existing machinery:
- `admin_reset` column on `buyer_bnr_strikes` — `migration 000001:582`
- `BNRAdminResetter` — `bnr_admin_reset.go` — sets `admin_reset = TRUE`
- Query filter: `WHERE admin_reset = FALSE` — effectively removes strikes

Business truth: "Violation history should not have normal reset."

Question: Should admin have exceptional override capability?
- If NO → `BNRAdminResetter` and `admin_reset` column are obsolete
- If YES → needs separately audited authority/capability

---

## RESIDUE AUDIT

| Artifact | Location | Classification | Action |
|----------|----------|---------------|--------|
| `StatusExpiredBNR` | `auction.go:93` | OBSOLETE | Remove after DRAFT return |
| `TransitionToExpiredBNR` | `auction.go:412-419` | OBSOLETE | Remove after DRAFT return |
| `transitionAllowed[StatusExpiredBNR]` | `auction.go:101` | OBSOLETE | Remove |
| `AuctionSettlementWorker` | `auction_settlement_worker.go` | OBSOLETE | Replace with specific workers |
| `BNRStrikeHandler` | `bnr_strike_handler.go` | OBSOLETE | Replace with violation handler |
| `BNRDecayWorker` | `bnr_decay_worker.go` | OBSOLETE | DELETE |
| `BNRAdminResetter` | `bnr_admin_reset.go` | OWNER DECISION | Pending |
| `BNRStrikeChecker` | `bnr_restriction.go` | OBSOLETE | Replace with restriction checker |
| `BNRAuctionRestrictedError` | `auction.go` | OBSOLETE | Replace with cross-commerce error |
| `buyer_bnr_strikes` table | `migration 000001:575` | OBSOLETE | Migrate to `commerce_violations` |
| `settlement_deadline` column | `auctions` table | OBSOLETE | Drop after new model |
| `expired_bnr` enum value | `auction_status_enum` | OBSOLETE | Drop from enum |
| `settlement_deadline` on mobile DTO | `auction_dto.dart` | STALE | Remove after column drop |
| `expired_bnr` in mobile status handling | `auction_dto.dart` | STALE | Update |
| `BNRAuctionRestrictedError` in handler | `auction_handler.go` | STALE | Update |
| Test A6 `expired_bnr` capability test | `auction_viewer_capabilities_test.go` | STALE TEST | Update |
| `commerce_violation` report type | `report.go` | UNRELATED | Different domain (moderation) |

---

## FINAL STATE MACHINE PROOF

### Canonical Lifecycle (After Corrections)

```
DRAFT
  ↓ (seller edits + schedule)
SCHEDULED
  ↓ (activation time or manual)
ACTIVE
  ↓ (end_at reached)
WAITING_SETTLEMENT
  ↓
RESOLVE WINNER ADDRESS
  ↓
DETECT SHIPPING PATH
  ├── CASE A (outside coverage)
  │     ↓ seller provides quote
  │     ↓ buyer accepts
  │     ↓ shipping_resolved_at set
  │
  ├── CASE B (seller optional quote)
  │     ↓ buyer accepts quote OR selects normal
  │     ↓ shipping_resolved_at set
  │
  └── NORMAL (buyer selects Shipping Setup)
        ↓ shipping_resolved_at set
  ↓
ORDER CREATED
  ↓ (payment_expires_at = shipping_resolved_at + 24h)
24H PAYMENT
  ├── SUCCESS → ENDED
  └── FAILURE → Expire() → DRAFT + BNR violation
```

### Failure Paths

```
WAITING_SETTLEMENT
   │
   ├── Seller Shipping Default (Case A, seller deadline expired)
   │     → seller violation
   │     → auction → DRAFT
   │     → seller restricted
   │     → seller CANNOT relist until restriction expires
   │
   ├── Buyer Shipping-Selection Failure (normal shipping available, buyer deadline expired)
   │     → buyer violation
   │     → auction → DRAFT
   │     → buyer restricted
   │     → seller may immediately relist
   │
   └── Buyer BNR (shipping resolved, payment window expired)
         → order expired (via Expire())
         → buyer violation
         → auction → DRAFT
         → buyer restricted
         → seller may immediately relist
```

### No Intermediate States

- ❌ `expired_bnr` — NOT a business state
- ❌ `expired_settlement` — NOT a business state
- ✅ All failures → DRAFT + violation record

---

## IMPLEMENTATION GATE

### Must Be Resolved Before Implementation

| # | Gap | Severity | Resolution |
|---|-----|----------|------------|
| 1 | PlaceBid primary address check | P0 | Add `GetPrimaryByUserID` check to PlaceBid |
| 2 | State machine DRAFT transition | P0 | Add `StatusDraft` to `transitionAllowed[StatusWaitingSettlement]` + `ReturnToDraft()` |
| 3 | `shipping_resolved_at` field + resolution endpoint | P0 | New field, new resolution endpoint with `shipping_resolved_at IS NULL` guard |
| 4 | Auction payment expiry | P0 | `payment_expires_at = shipping_resolved_at + 24h` instead of `calculatePaymentExpiry` |
| 5 | Commerce violation + restriction tables | P1 | `commerce_violations` + `commerce_restrictions` + checker |
| 6 | Seller default worker (Case A) | P1 | New worker with Case A detection + seller deadline |
| 7 | Buyer shipping failure worker | P1 | New worker with buyer deadline |
| 8 | Race condition guard | P1 | `shipping_resolved_at IS NULL` predicate in resolution path |

### Must Be Decided Before Implementation

| # | Decision | Options |
|---|----------|---------|
| 1 | Restriction stacking formula | Replace vs Extend |
| 2 | Admin violation reset | Yes (with capability) vs No |

### After All Corrections

No authority ambiguity. No duplicate authority. No hidden state. No assumption required during implementation.

---

*End of Final Pre-Implementation Audit*
