# Auction Winner Shipping & Settlement — Technical Design (Corrected v3)

**Pass:** Technical Design (design only — no implementation)  
**Date:** 2026-09-02  
**Supersedes:** All previous versions  
**Status:** `NOT READY — OWNER DECISIONS REQUIRED`  
**Scope:** Canonical technical architecture for auction winner shipping, settlement, payment, restriction.

---

## 1. Executive Summary

### 1.1 Key Corrections (v2 → v3)

| Issue | v2 Error | v3 Correction |
|-------|----------|---------------|
| Seller deadline field timing | "Set when `requires_private_quote` determinable" (ambiguous) | **Set at auction end** as `auction_end + 24h`. Updated when buyer address submitted if coverage changes |
| Buyer shipping failure | Listed as OWNER DECISION REQUIRED in lifecycle table and register | **LOCKED** per correction document §3. Violation is buyer transaction violation |
| Restriction overlap | "Default assumption: 15d from violation #2" | **Removed default**. Owner Decision Required — no assumption |
| `claim` endpoint semantics | Retained as step 3 in normal path | **Renamed** to `settle` to separate from shipping resolution. `claim` marked for cleanup |
| Case B field model | Proposed `shipping_coverage_available` + `seller_requests_private_quote` fields | **Not added as fields**. `requires_private_quote` boolean remains sole stored indicator. Case B is a seller action that triggers private quote flow, tracked by quote existence |
| Case B timing | "Seller creates quote → requires_private_quote set to true" (reversed) | **Coverage check at address submission determines path first.** Case B is a seller override that requires its own determination mechanism |

### 1.2 Architecture Summary

1. Extended `waiting_settlement` with fields (Option A)
2. Seller deadline = `auction_end + 24h` — exists from auction end
3. Two shipping paths: normal (buyer selects setup) and private (seller creates quote, buyer accepts)
4. `requires_private_quote` is set when coverage check is performed (after buyer address submission)
5. Payment = `shipping_resolved_at + 24h` for auction winners; For Sale unchanged
6. Unified violation table replacing `buyer_bnr_strikes`
7. `expired_settlement` replaces `expired_bnr`

---

## 2. Source Documents & Authority

| Document | Authority |
|----------|-----------|
| REPORT_AUCTION_WINNER_SHIPPING_DESIGN.md | Forensic audit — evidence |
| REPORT_AUCTION_WINNER_SHIPPING_DESIGN_CORRECTED.md | **Supersedes** conflicting statements |
| REPORT_AUCTION_WINNER_SHIPPING_TECHNICAL_DESIGN.md (v1, v2) | **Superseded** |
| Owner decisions in reports | **LOCKED** |
| Current filesystem | Implementation truth |
| This document | **Canonical architecture** |

---

## 3. Locked Business Truth

### 3.1 Seller Shipping Deadline

Seller has **24 hours from auction end** to create private shipping quote, when private quote is required.

```
seller_shipping_deadline = auction_end + 24h
```

### 3.2 Buyer Shipping Selection Deadline

Buyer has **24 hours from auction end** to select Shipping Setup (normal path).

If buyer fails to select within 24h: **Buyer Transaction Violation** (LOCKED).

### 3.3 Buyer Payment Deadline

```
payment_deadline = shipping_resolved_at + 24h
```

### 3.4 Transaction Violation / Restriction

- One authority, buyer + seller, cross-commerce
- Ladder: 1st→7d, 2nd→15d, 3rd+→30d
- Cumulative, no automatic reset, no trust score, no permanent ban
- Not full account ban

### 3.5 Private Quote Paths

- **Case A**: Destination not covered by selected Shipping Setup
- **Case B**: Destination covered but seller wants to provide special/private price

### 3.6 Chat

- No automatic chat creation
- Lazy create for private quote flow

### 3.7 No Backward Compatibility

---

## 4. Current Implementation Audit

### 4.1 Auction State Machine

States: `draft → scheduled → active → waiting_settlement → ended | expired_bnr | cancelled`

### 4.2 Settlement Deadline Field

Producer: `TransitionToWaitingSettlement()` sets `settlement_deadline = now + 24h`.  
Consumer: `AuctionSettlementWorker` checks `settlement_deadline <= NOW()`.  
Verdict: **OBSOLETE**. Replace with `buyer_shipping_deadline`. Drop.

### 4.3 Buyer Address — Destination Authority

Address provided at **claim time** (`AddressID` in `ClaimAuctionWithTokenRequest`), NOT at auction end.

### 4.4 Current BNR System

Divergent: ladder (14d/90d/permanent), scope (buyer-only), decay (180d), permanent ban. All must be replaced.

### 4.5 Order Creation

No shipping gate. Payment expiry uses method-based (15min/1hr/6hr/30min) instead of 24h from `shipping_resolved_at`.

---

## 5. Architecture Options

### Option A: Extend `waiting_settlement` with Fields

4 columns on auctions. Minimal migration. `waiting_settlement` remains correct semantic container.

### Option B: New Auction States

More transitions, more mobile complexity. Not justified.

### Option C: Dedicated Settlement Entity

Cross-entity complexity. Not justified.

### Verdict: Option A

---

## 6. Canonical Architecture

### 6.1 Core Principle

**One auction entity, one settlement phase, field-driven sub-phases.**

### 6.2 Seller Deadline — Exists From Auction End

The seller's clock starts at auction end, not when the buyer provides address.

```
seller_shipping_deadline = auction_end + 24h
```

This field is set at auction end, unconditionally for `waiting_settlement` auctions. It does NOT wait for address submission.

### 6.3 `requires_private_quote` — Stored Boolean Indicator

`requires_private_quote` (boolean, NOT NULL, default false) is the **sole stored indicator** for shipping mode.

**When is it set?**
- Initially `false` at auction end
- Updated to `true` when buyer submits address and coverage check reveals no covered Shipping Setup (Case A)

**Case B — Seller Requests Special Price:**
- Seller creates a private quote even though coverage exists
- The quote creation action itself (via `CreateShippingQuote` with `source_type="auction"`) implicitly overrides the normal path
- `requires_private_quote` is NOT set to `true` at quote creation time — the system checks for active quote existence instead
- **Lifecycle invariant:** Once `shipping_resolved_at` is set (via normal selection or quote acceptance), the resolution is final and immutable

**No additional fields.** `requires_private_quote` correctly represents the final shipping mode. Case A is determined by coverage check. Case B is determined by seller action (quote creation).

### 6.4 Coverage Check Timing

Coverage check happens when buyer submits address. At that point:
- If no Shipping Setup covers the address → `requires_private_quote = true`
- If a Shipping Setup covers the address → `requires_private_quote = false`

**Case B override:** Seller can create a private quote even when `requires_private_quote = false`. The system allows quote creation for any `waiting_settlement` auction where the seller is the owner.

---

## 7. Canonical Lifecycle

### 7.1 Lifecycle Table

| Phase | Actor | Deadline | Success | Failure | Violation |
|-------|-------|----------|---------|---------|-----------|
| Auction ended | System | — | Winner confirmed, `buyer_shipping_deadline` = auction_end + 24h, `seller_shipping_deadline` = auction_end + 24h | — | — |
| Buyer submits address | Buyer | Part of shipping selection (same deadline) | Coverage check performed | Address not submitted by deadline | Buyer shipping selection failure (LOCKED) |
| Normal shipping selection | Buyer | +24h from auction end | Buyer selects Shipping Setup, `shipping_resolved_at` set | Timeout | Buyer transaction violation (LOCKED) |
| Seller private quote input | Seller | `seller_shipping_deadline = auction_end + 24h` | Quote created | Quote not provided | Seller shipping default (LOCKED) |
| Private quote acceptance | Buyer | `PRIVATE_QUOTE_ACCEPTANCE_WINDOW` | `shipping_resolved_at` set | Buyer doesn't accept | **OWNER DECISION REQUIRED** |
| Payment | Buyer | +24h from shipping resolved | Paid | Timeout | BNR (LOCKED) |

### 7.2 Address Is Part of Shipping Selection

Address submission is **not an independent business deadline**. It is part of the buyer's shipping selection action.

**Normal path:** Buyer submits address AND selects Shipping Setup (may be one or two API calls, but under the same `buyer_shipping_deadline`).

**Private path:** Buyer submits address → coverage check → seller creates quote → buyer accepts → shipping resolved.

**If buyer never provides address:** The `buyer_shipping_deadline` (24h from auction end) expires. This triggers buyer shipping selection failure — same deadline, same violation type. No new violation type invented.

### 7.3 Case B — Seller Overrides Normal Path

When destination IS covered but seller wants special pricing:

1. Buyer submits address → coverage check → `requires_private_quote = false`
2. Seller creates private quote via `POST /auctions/{id}/shipping-quote`
3. System allows quote creation for any `waiting_settlement` auction
4. `requires_private_quote` remains `false` (coverage exists) but private quote flow is active
5. Buyer accepts quote → `shipping_resolved_at` set

**Lifecycle invariant:** Once `shipping_resolved_at` is set, shipping resolution is final. No subsequent action can change the shipping price or method.

---

## 8. Deadline Model

### 8.1 Three Distinct Deadlines

| # | Deadline | Anchor | Duration | Applies When | Violation |
|---|----------|--------|----------|--------------|-----------|
| 1 | Buyer shipping selection | `auction.end_at` | 24h | Always | Buyer shipping selection failure (LOCKED) |
| 2 | Seller private quote input | `auction.end_at` | 24h | When private quote applicable | Seller shipping default (LOCKED) |
| 3 | Buyer payment | `shipping_resolved_at` | 24h | After shipping resolved | BNR (LOCKED) |

### 8.2 `settlement_deadline` — OBSOLETE

Current field: `auctions.settlement_deadline`

| Consumer | Action |
|----------|--------|
| `AuctionSettlementWorker` | REMOVE (decommission worker) |
| `GeneratePricingTokenForAuctionClaim` | UPDATE reference to `buyer_shipping_deadline` |
| Mobile | UPDATE countdown to `buyer_shipping_deadline` |

Drop field. Project from-zero, no production data.

### 8.3 Deadline Fields

| Field | Type | Set When | Authority |
|-------|------|----------|-----------|
| `buyer_shipping_deadline` | timestamptz | `EndAuctionInternal()` when winner exists | `auction_end + 24h` |
| `seller_shipping_deadline` | timestamptz | `EndAuctionInternal()` when winner exists | `auction_end + 24h` |
| `shipping_resolved_at` | timestamptz | Shipping resolution | Service |
| `requires_private_quote` | boolean, NOT NULL, default false | After coverage check (address submission) | Service |

**Important:** Both `buyer_shipping_deadline` and `seller_shipping_deadline` are set at auction end as `auction_end + 24h`. They do NOT change when buyer provides address.

---

## 9. Destination Authority

### 9.1 Not Known at Auction End

Winner's address is provided at claim/submission time.

### 9.2 Coverage Check

When buyer submits address:
1. System loads address, resolves province/city
2. Checks: does any ShippingSetup cover this address?
3. If covered: `requires_private_quote = false`
4. If not covered: `requires_private_quote = true`

### 9.3 Address Locking

Once submitted and path determined, address is locked for this settlement.

---

## 10. Shipping Resolution

### 10.1 Normal Path

```
1. Buyer submits address → coverage → requires_private_quote = false
2. Buyer selects valid Shipping Setup
3. Backend validates: status, caller is winner, setup covers address
4. shipping_resolved_at = NOW()
5. Outbox: auction.shipping_resolved
6. Order gate opened
```

### 10.2 Private Quote Path

```
1. Buyer submits address → coverage → requires_private_quote = true (Case A)
   OR seller creates quote directly (Case B)
2. Seller creates quote via chat
3. Buyer accepts quote
4. shipping_resolved_at = NOW()
5. Quote → USED
6. Outbox: auction.shipping_resolved
7. Order gate opened
```

### 10.3 `shipping_resolved_at` Authority

Both paths lock auction row with `FOR UPDATE` before setting.

### 10.4 Immutable Once Set

Once `shipping_resolved_at != NULL`, shipping resolution is final. No subsequent action changes it.

---

## 11. Private Quote Lifecycle

### 11.1 Complete Flow

```
1. requires_private_quote = true (Case A) or seller creates quote (Case B)
2. seller_shipping_deadline = auction_end + 24h
3. Seller clicks "Give Shipping Quote" → lazy chat
4. Seller submits quote
5. Quote ACTIVE, destination locked
6. Buyer accepts
7. shipping_resolved_at = NOW()
8. Quote → USED
9. Order gate opened
```

### 11.2 Case B Timing

Seller can create a private quote at any time during `waiting_settlement`, even if `requires_private_quote = false`. The existing `validateAuctionForQuote()` validates auction status and seller ownership, not coverage.

### 11.3 Acceptance Deadline

**OWNER DECISION REQUIRED**

| Option | Duration | Impact |
|--------|----------|--------|
| A: No explicit | Quote expiry controls | Simple |
| B: Fixed window | New timer | Worker needed |
| C: Tied to seller deadline | Complex | Hard to implement |

**Placeholder:** `PRIVATE_QUOTE_ACCEPTANCE_WINDOW`

**Technical note:** Quote expiry (default 24h) is a technical lifecycle constraint, not a business decision.

---

## 12. Order Creation

### 12.1 Canonical Sequence

```
Auction Ends
    ↓
Winner Confirmed
    ↓
Buyer Submits Address → Coverage Check
    ↓
Shipping Resolution
    ↓
Pricing Token Generation
    ↓
Order Creation (PaymentExpiresAt = shipping_resolved_at + 24h)
    ↓
Payment
```

### 12.2 Single Path

1. `GeneratePricingTokenForAuctionClaim` — validates shipping resolved
2. Client sends token + payment method
3. `CreateOrderFromAuction` — explicit `paymentExpiresAt`

No alternate paths.

### 12.3 Gate

```go
if auction.ShippingResolvedAt == nil {
    return fmt.Errorf("shipping not resolved")
}
```

### 12.4 Payment Expiry Coexistence

Auction: `shipping_resolved_at + 24h`  
For Sale: `calculatePaymentExpiry(method, now)` — unchanged.

---

## 13. Payment Deadline

```
payment_deadline = shipping_resolved_at + 24h
```

Workers query `PaymentExpiresAt` directly. Unchanged.

---

## 14. Transaction Violation & Restriction

### 14.1 `commerce_violations`

```sql
CREATE TABLE commerce_violations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id        UUID NOT NULL,
    actor_type      TEXT NOT NULL CHECK (actor_type IN ('buyer', 'seller')),
    violation_type  TEXT NOT NULL CHECK (violation_type IN (
        'bnr',
        'shipping_selection_failure',
        'shipping_default'
    )),
    source_id       UUID NOT NULL,
    source_type     TEXT NOT NULL DEFAULT 'auction',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (source_id, source_type, violation_type)
);

CREATE INDEX idx_commerce_violations_actor ON commerce_violations (actor_id, created_at DESC);
```

Violations are immutable. No `admin_reset` (pending Owner Decision).

### 14.2 Ladder

1st→7d, 2nd→15d, 3+→30d. Cumulative.

### 14.3 Restriction Overlap

**OWNER DECISION REQUIRED**

When violation #2 occurs during active restriction from #1:

| Option | Result | Impact |
|--------|--------|--------|
| A: Latest violation resets clock | 15d from violation #2 | Simple formula |
| B: Stack restrictions | Day 7 + 15d = Day 22 | Complex calculation |

No default assumption. Owner must choose.

### 14.4 Enforcement Points

| Check | Actor | When |
|-------|-------|------|
| Place bid | Buyer | Before bid |
| Create order (For Sale) | Buyer | Before order |
| Auction settlement | Buyer | Before order |
| Create listing (For Sale) | Seller | Before listing |
| Create auction | Seller | Before auction |

---

## 15. Worker Design

| Worker | Poll | Purpose | Status |
|--------|------|---------|--------|
| `AuctionEndWorker` | 30s | active → waiting_settlement | MODIFY: set deadlines |
| `SellerShippingDeadlineWorker` | 30s | Detect seller timeout | NEW |
| `BuyerShippingDeadlineWorker` | 30s | Detect buyer timeout | NEW |
| `PaymentExpiryWorker` | 1min | Expire payments | UNCHANGED |
| `OrderPaymentTimeoutWorker` | 2min | Expire orders + BNR | MODIFY |
| `AuctionSettlementWorker` | 5min | — | **DECOMMISSION** |
| `BNRDecayWorker` | 24h | — | **DECOMMISSION** |

### 15.1 SellerShippingDeadlineWorker

```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND requires_private_quote = true
  AND seller_shipping_deadline <= NOW()
  AND NOT EXISTS (
      SELECT 1 FROM shipping_quotes sq
      WHERE sq.source_type = 'auction'
        AND sq.source_id = auctions.id
        AND sq.seller_id = auctions.seller_id
  )
  AND order_id IS NULL
FOR UPDATE SKIP LOCKED
```

**Key:** Checks quote creation, NOT `shipping_resolved_at`. If seller created quote at T+23:59, seller did NOT default.

### 15.2 BuyerShippingDeadlineWorker

```sql
SELECT id FROM auctions
WHERE status = 'waiting_settlement'
  AND requires_private_quote = false
  AND shipping_resolved_at IS NULL
  AND buyer_shipping_deadline <= NOW()
  AND order_id IS NULL
FOR UPDATE SKIP LOCKED
```

### 15.3 Idempotency

All workers: fetch IDs (SKIP LOCKED) → process (own tx) → guard (re-check status) → constraint (violation UNIQUE).

---

## 16. Concurrency & Idempotency Matrix

| # | Race | Lock | Guard | Winner |
|---|------|------|-------|--------|
| 1 | Seller quote vs seller deadline | Auction FOR UPDATE | Quote existence | First commit |
| 2 | Buyer selection vs buyer deadline | Auction FOR UPDATE | shipping_resolved_at | First commit |
| 3 | Buyer acceptance vs acceptance deadline | Auction FOR UPDATE | shipping_resolved_at NULL | First commit |
| 4 | Payment vs payment expiry | Order FOR UPDATE | Order status | Payment if first |
| 5 | BNR vs payment | Order FOR UPDATE | Order status | Payment if first |
| 6 | Seller default vs buyer acceptance | Auction FOR UPDATE | Terminal status | Worker if first |
| 7 | Duplicate quote | Chat lock | Supersession | Latest |
| 8 | Duplicate order | Auction FOR UPDATE | pricing_token_id UNIQUE | First commit |

### 16.1 Termination Invariant

**One auction = one terminal settlement outcome.**

After terminal: no order, no violation recording, no payment, no shipping resolution.

**Mechanism:**
1. `FOR UPDATE` serializes mutations
2. Status check after lock
3. Terminal status prevents all transitions
4. Outbox atomic with state change

**Settlement outcome IS the authority.** Violations are side effects. Violation table does NOT determine terminality.

---

## 17. State Naming

### 17.1 `expired_bnr` → `expired_settlement`

Current `expired_bnr` is semantically misleading for seller default and buyer shipping failure.

**New enum:** `expired_settlement`

**Semantics:** "Auction settlement failed — terminal state."

Separate from violation type:
```
terminal state: expired_settlement
reason: shipping_selection_failure | seller_default | bnr
violation: recorded in commerce_violations
```

### 17.2 Migration

1. Add `expired_settlement` to enum
2. Rename existing `expired_bnr` rows
3. Remove `expired_bnr` from enum
4. Update all references

---

## 18. Event / Outbox

| Event | Producer | Consumer | Idempotency |
|-------|----------|----------|-------------|
| `auction.waiting_settlement` | AuctionEndWorker | Notification | Status check |
| `auction.shipping_resolved` | ShippingService | Read model | shipping_resolved_at NULL |
| `auction.seller_default` | SellerShippingDeadlineWorker | Violation handler | UNIQUE constraint |
| `auction.buyer_shipping_violation` | BuyerShippingDeadlineWorker | Violation handler | UNIQUE constraint |
| `auction.bnr_detected` | OrderPaymentTimeoutWorker | Violation handler | UNIQUE constraint |
| `auction.settled` | SettlementService | Read model | order_id NULL |

All atomic with state change.

---

## 19. API Contract

### 19.1 Normal Path

```
1. POST /api/v1/auctions/{id}/submit-address
   Body: { address_id }
   → validates address, checks coverage, sets requires_private_quote

2. POST /api/v1/auctions/{id}/select-shipping
   Body: { shipping_setup_id }
   → validates setup covers address, sets shipping_resolved_at
   Precondition: requires_private_quote = false

3. POST /api/v1/auctions/{id}/settle
   Body: { discount_code?, use_coins? }
   → creates pricing token, creates order
   Precondition: shipping_resolved_at IS NOT NULL
```

### 19.2 Private Quote Path

```
1. POST /api/v1/auctions/{id}/submit-address
   Body: { address_id }
   → validates address, checks coverage, sets requires_private_quote

2. POST /api/v1/auctions/{id}/shipping-quote (seller)
   Body: { cost, note?, destination_* }
   → creates quote via chat
   Precondition: requires_private_quote = true OR seller is owner

3. POST /api/v1/auctions/{id}/shipping/accept-quote (buyer)
   Body: { shipping_quote_id }
   → validates quote, sets shipping_resolved_at
   Precondition: quote ACTIVE

4. POST /api/v1/auctions/{id}/settle
   Body: { discount_code?, use_coins? }
   → creates pricing token, creates order
   Precondition: shipping_resolved_at IS NOT NULL
```

### 19.3 `claim` Endpoint

The existing `POST /auctions/{id}/claim` is **renamed to `/settle`** to separate it from shipping resolution. The `claim` name is semantic residue from the old single-step flow. If backward compatibility is needed temporarily, `claim` can redirect to `settle`, but must not bypass the shipping resolution gate.

---

## 20. Mobile Contract

| Screen | Change |
|--------|--------|
| Winner screen | Address + shipping selection (not "claim") |
| Shipping selector | New screen for normal path |
| Quote acceptance | In chat |
| Countdowns | 3 timers: buyer shipping, seller quote, buyer payment |
| Status | `expired_settlement` replaces `expired_bnr` |

---

## 21. Database Schema

### 21.1 New Fields on `auctions`

| Field | Type | Purpose |
|-------|------|---------|
| `buyer_shipping_deadline` | timestamptz | = `auction_end + 24h` |
| `seller_shipping_deadline` | timestamptz | = `auction_end + 24h` |
| `requires_private_quote` | boolean, NOT NULL, default false | Set after coverage check |
| `shipping_resolved_at` | timestamptz | Set when resolved |

### 21.2 DROP

`settlement_deadline` column.

### 21.3 New Table

`commerce_violations` (see §14.1).

### 21.4 Enum

Add `expired_settlement`, remove `expired_bnr`.

### 21.5 Migration Order

1. Add 4 columns (additive)
2. Add `expired_settlement` to enum
3. Create `commerce_violations`
4. Rename `expired_bnr` rows
5. Remove `expired_bnr` from enum
6. Migrate `buyer_bnr_strikes` data
7. Drop `buyer_bnr_strikes`
8. Drop `settlement_deadline`

---

## 22. Migration Strategy

| Change | Risk |
|--------|------|
| Add 4 columns | Low |
| Add enum value | Low |
| Drop `settlement_deadline` | Low (from-zero) |
| Create `commerce_violations` | Low |
| Rename enum value | Medium (type recreation) |
| Drop `buyer_bnr_strikes` | Low (from-zero) |

---

## 23. Cleanup

| Component | Action |
|-----------|--------|
| `buyer_bnr_strikes` | DROP |
| `settlement_deadline` | DROP |
| `BNRDecayWorker` | DECOMMISSION |
| `BNRAdminResetter` | REMOVE (pending Owner Decision) |
| `AuctionSettlementWorker` | DECOMMISSION |
| `BNRStrikeHandler` | DECOMMISSION |
| `BNRStrikeChecker` | REPLACE |
| `PermanentBan` field | REMOVE |
| Old BNR ladder | REPLACE with 7/15/30d |
| `expired_bnr` enum | RENAME to `expired_settlement` |
| `settlement_deadline` refs | UPDATE to `buyer_shipping_deadline` |
| `calculatePaymentExpiry` auction | BYPASS with explicit expiry |
| `claim` endpoint name | RENAME to `settle` |

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

### OWNER DECISION REQUIRED

| # | Decision | Options | Impact |
|---|----------|---------|--------|
| 1 | **Private quote acceptance deadline** | A: No explicit; B: Fixed window; C: Tied to seller deadline | Worker, lifecycle stall |
| 2 | **Terminal state after seller default** | A: `expired_settlement`; B: `cancelled`; C: New state | State machine, mobile |
| 3 | **Product relisting after seller default** | A: Yes; B: No; C: Admin action | Product lifecycle |
| 4 | **Winner benefit after seller default** | A: None; B: Priority; C: Platform policy | Feature scope |
| 5 | **Admin manual violation reset** | A: Allow; B: Disallow | Admin mechanism |
| 6 | **Restriction overlap semantics** | A: Latest resets clock; B: Stack | Restriction formula |

---

## 25. Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Seller deadline timing | **High** | Fixed: `auction_end + 24h` from auction end |
| Seller default vs buyer acceptance race | **High** | FOR UPDATE + quote existence |
| Payment vs BNR | **High** | FOR UPDATE + status check |
| `expired_bnr` migration | **Medium** | Clean rename, from-zero |
| For Sale regression | **Medium** | `calculatePaymentExpiry` unchanged |
| Private quote stall | **Medium** | Quote expiry natural bound; Owner Decision pending |
| `settlement_deadline` drop | **Medium** | All refs updated |

---

## 26. Evidence

**Backend:** `auction.go`, `auction_service.go`, `bnr_restriction.go`, `auction_handler.go`, `order_creation_service.go`, `order.go`, `shipping_quote_service.go`, `shipping_quote.go`

**Workers:** `auction_end_worker.go`, `auction_settlement_worker.go`, `bnr_strike_handler.go`, `bnr_decay_worker.go`, `bnr_admin_reset.go`, `payment_expiry_worker.go`, `order_payment_timeout_worker.go`

**Admin:** `admin_handler.go`, `dependencies.go`

**Database:** `000001_canonical_schema.up.sql`

**Reports:** Forensic audit, correction document, v1/v2 technical designs

---

## 27. Final Verdict

```
NOT READY — OWNER DECISIONS REQUIRED
```

### Corrections Made (v3)

1. Seller deadline timing clarified: set at auction end as `auction_end + 24h`, not conditional on address
2. Buyer shipping selection failure: **LOCKED** as buyer transaction violation — removed from OWNER DECISION REQUIRED
3. Restriction overlap: removed "default assumption" — OWNER DECISION REQUIRED without default
4. `claim` endpoint: renamed to `settle`; `claim` marked for cleanup
5. Case B model: `requires_private_quote` remains sole boolean; Case B is seller action tracked by quote existence
6. Coverage check timing: clarified in lifecycle — happens at address submission
7. Lifecycle invariant: once `shipping_resolved_at` set, resolution is final and immutable

### Architecture Verdict

**Option A (field extension) remains canonical.** Seller deadline = `auction_end + 24h`. `requires_private_quote` set after coverage check. `expired_settlement` replaces `expired_bnr`.

### Implementation Allowed: NO

6 Owner Decisions must be resolved.

### SOURCE CODE CHANGED: NO

This report is design only. No backend, mobile, migration, or test files were modified.
