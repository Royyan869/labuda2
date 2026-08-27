# SCOPE 4A — CANONICAL COMMERCE MONEY CONTRACT IMPLEMENTATION IMPACT AUDIT

**Date:** 2026-08-08
**Mode:** AUDIT ONLY — NO CODE CHANGES
**Git branch:** main
**Git status:** Working tree dirty (pre-existing uncommitted changes to mobile app)

---

## 1. VERDICT

**`CANONICAL_COMMERCE_MONEY_CONTRACT_IMPLEMENTATION_IMPACT_AUDIT_COMPLETE`**

Sufficient information exists to begin safe implementation slices. Three P1 defects confirmed. Target canonical equations are derivable from locked business truth. No new business ambiguities that block implementation were found.

---

## 2. RE-VERIFIED CURRENT DEFECTS

### P1-1: SELLER_PLATFORM_FEE_INCIDENCE_VIOLATION — CONFIRMED

**Proof location:** `backend\internal\serverboot\dependencies.go:3289`
```go
escrowAmount := order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)
```

Also at `backend\internal\commerce\order\application\order_creation_service.go:1141`:
```
EscrowAmount = subtotal + shipping + commission - discounts
```

And at `backend\internal\finance\application\pricing_helper.go:30`:
```go
func CalculateGrossEscrowFromSnapshot(order *entity.Order) money.Money {
    return order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)
}
```

**Root cause:** Commission is designed as a seller-side deduction (correct formula at `pricing_token_service.go:382`: `netSubtotal = subtotal - discountAmount; commissionAmount = calculateCommission(netSubtotal, commissionPercent)`) but the escrow construction adds commission ON TOP of the seller's price. The escrow then becomes the amount sent to Midtrans as the buyer's gross. This means the buyer pays: `subtotal + shipping + commission + payment_fee`.

**Impact:** Buyer is surcharged the seller's platform fee. The payment amount to Midtrans is inflated by `commission_amount`. If a seller charges Rp100,000 with 5% commission, the buyer pays Rp105,000 + shipping + payment_fee instead of Rp100,000 + shipping + payment_fee.

---

### P1-2: DISCOUNT_PAYMENT_CONTINUITY_VIOLATION — CONFIRMED

**Proof location:**

At `pricing_token_service.go:382-406`, discount IS correctly calculated:
```go
netSubtotal := subtotal.Sub(discountAmount)
commissionAmount := calculateCommission(netSubtotal, commissionPercent)
escrowBase := subtotal.Add(shippingTotal).Add(commissionAmount)
escrowAmount := escrowBase.Sub(discountAmount)  // CORRECT: discount subtracted
```

But at `dependencies.go:3289`, at payment time:
```go
escrowAmount := order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)
// DISCOUNT NOT SUBTRACTED
```

Additionally, `NewOrderFromSource` (order.go:1064) does NOT accept a `discountAmount` parameter, so `order.discount_amount` defaults to 0 at persistence (`order_repository.go` INSERT hardcodes `discount_amount = 0`).

**Root cause:** The pricing token correctly computes escrow after discount, but this value is not persisted on the order. CreatePayment re-derives escrow from order fields without discount. Two critical breaks:
1. `NewOrderFromSource` does not carry discount_amount → order.discount_amount = 0
2. `CreatePayment` does not read `order.discount_amount` (nor is there a stored escrow_amount from the pricing token)

**Impact:** Seller discount is visible in pricing preview but silently lost when the buyer actually pays. The buyer pays the full pre-discount amount to Midtrans.

---

### P1-3: COIN_REDEMPTION_VALUE_LOST — CONFIRMED

**Proof location:**

At `dependencies.go:3302`:
```go
coinDiscountAmount := int64(0)  // HARDCODED ZERO
```

At `order_repository.go` INSERT:
```sql
coin_discount_amount = 0, total_before_coins_amount = order.TotalPayableAmount
```

At `payment_repository.go` CREATE:
```sql
coin_discount_amount = 0
```

Coins ARE spent from balance at `order_creation_service.go:1224`:
```go
s.paymentService.RecordCoinsUsageTx(ctx, tx, order.ID, order.BuyerID, input.CoinsToUse, input.OrderValueForCoins, snapshot.CommissionAmount.Int64())
```

This calls `coinsService.SpendCoinsTx` which atomically deducts balance and creates spend transaction.

**Root cause:** Coins balance is debited but the Rupiah value (coinsUsed × Rp10) is never applied as a reduction to the Midtrans gross amount. The buyer loses coins with zero financial benefit.

**Impact:** Buyer loses loyalty coins permanently — balance decremented, no payment reduction, no Midtrans adjustment.

---

## 3. TARGET CANONICAL EQUATIONS

Based on locked business truth:

```
P  = product subtotal (seller's listing price)
D  = seller-funded discount
PD = P - D (discounted product value)
S  = shipping cost
C  = commission = platformRate × PD  (seller-side only)
K  = Labuda-funded coin redemption Rupiah = usedCoins × Rp10
F  = buyer payment gateway fee
```

### Buyer-side equations:

```
BuyerOrderValueBeforeCoins = PD + S         (commission NOT added)
K ≤ 20% × PD                                (max coin redemption)
BuyerCashBeforeGatewayFee = PD + S - K       (what buyer actually pays in cash)
BuyerMidtransGross = BuyerCashBeforeGatewayFee + F
```

### Seller-side equations:

```
SellerProductNet = PD - C
SellerShippingEntitlement = S                (shipping proceeds belong to seller)
SellerTotalEntitlement = SellerProductNet + SellerShippingEntitlement
                                                = (P - D) - C + S
```

### Labuda economics:

```
LabudaCommissionRevenue = C
LabudaCoinsSubsidy = K                        (funded by Labuda, not seller)
LabudaPaymentFeeRevenue = F (subject to MDR settlement)
LabudaNet = C - K + (F - actual_MDR)
```

### Compatibility with current domain:

The current `escrowAmount` formula adds commission to buyer payable. The target equation removes commission from buyer's path. Commission becomes a post-settlement deduction from seller proceeds.

Current: `escrowAmount = P + S + C - D` → buyer pays this
Target: `buyerMidtransGross = (P - D) + S - K + F`

The escrow concept changes: escrow should hold the SELLER's pending entitlement, not the buyer's total payment.

---

## 4. AUTHORITY MAP

### Current flow (defective):

```
PRODUCER                             PERSISTENCE                  CONSUMER
────────────────────────────────────────────────────────────────────────────
Seller sets price                    listings.price_per_unit      PricingTokenService
                                                               
PricingTokenService                  pricing_tokens:              OrderCreationService
- subtotal = Q × unit_price          subtotal, shipping_total,   (snapshot → order)
- commission = (P-D) × rate%         commission_amount,
- escrow = P+S+C-D (correct!)        escrow_amount (correct),
                                     discount_amount,
                                     max_coins_allowed

OrderCreationService                 orders:                      CreatePayment
- NewOrderFromSource(snapshot)       subtotal (=P, no discount),  (derives escrow)
- discount_amount DROPPED            shipping_total,
- escrow_amount NOT STORED           commission_amount,
                                     discount_amount=0,           ← BUG
                                     coin_discount_amount=0       ← BUG

CreatePayment                        payments:                    Midtrans Snap
- escrow = Sub+S+Comm (NO disc)      gross_amount,                (gross = actual charge)
- gross = escrow + fee               net_amount,
- coinDiscountAmount = 0             coin_discount_amount=0       ← BUG

CanonicalFinalization                ledger_entries:              WalletService
- CreateEscrow(Sub+S+Comm)           GATEWAY_CLEARING,            (holds escrow)
                                     BANK_SETTLEMENT,
                                     PLATFORM_REVENUE
```

### Target flow (to be implemented):

```
PRODUCER                             PERSISTENCE                  CONSUMER
────────────────────────────────────────────────────────────────────────────
PricingTokenService                  pricing_tokens:              OrderCreationService
- same formulas +                    ALL financial components
- buyerCashBeforeFee                 stored atomically

OrderCreationService                 orders:                      CreatePayment
- ALL snapshot values persisted      canonical financial fields   (reads order, not recalcs)

CreatePayment                        payments:                    Midtrans Snap
- buyerGross = PD+S-K+F              gross = buyer gross          (actual charge)
- coinDiscountAmount = K             net = buyer cash

FinanceService                       ledger_entries:              
- Commission carved from SELLER       PLATFORM_REVENUE (C)         
  proceeds post-settlement            SELLER_PAYABLE (PD-C+S)     
- Coins subsidy as Labuda expense     COINS_SUBSIDY (K)           
```

---

## 5. SCHEMA DECISION MATRIX

### orders table fields:

| Field | Classification | Decision | Notes |
|-------|---------------|----------|-------|
| `subtotal` | MISNAMED | REDEFINE to `product_subtotal` = P (original price before discount) | Currently ambiguous whether it includes discount |
| `unit_price` | CANONICAL | KEEP | Per-unit price at order time |
| `shipping_total` | CANONICAL | KEEP | Shipping cost snapshot |
| `commission_percent` | CANONICAL | KEEP | e.g. 5 for 5% |
| `commission_amount` | CANONICAL | KEEP | Commission in Rupiah, but redefine as seller-side deduction |
| `discount_amount` | UNWRITTEN (always 0) | REDEFINE as seller-funded discount D in Rupiah | Currently defaults to 0 even when discount exists |
| `discount_code` | CANONICAL | KEEP | Discount code reference |
| `discount_type` | CANONICAL | KEEP | percentage, flat_amount, free_shipping |
| `discount_value` | CANONICAL | KEEP | Discount parameter value |
| `escrow_amount` | UNWRITTEN (always 0) | REDEFINE as `seller_entitlement` = (P-D) - C + S | Currently 0, recalculated elsewhere |
| `refunded_amount` | CANONICAL | KEEP | Total refunded to buyer |
| `service_fee_amount` | DERIVED | KEEP (already populated by UpdatePaymentSelectionTx) | Buyer payment gateway fee |
| `total_payable_amount` | DUPLICATE AUTHORITY | REDEFINE as buyer gross = PD+S-K+F | Currently overwritten twice (creation + payment) |
| `total_before_coins_amount` | DERIVED | KEEP as PD+S (buyer value before coins) | Currently hardcoded = total_payable at creation |
| `coins_used` | CANONICAL | KEEP | Count of coins used, for display + proportional refund |
| `coin_discount_amount` | UNWRITTEN (always 0) | REDEFINE as K = coins_used × 10 | Currently always 0 |
| `coins_refunded_at` | CANONICAL | KEEP | Timestamp of coin restoration |

### payments table fields:

| Field | Classification | Decision | Notes |
|-------|---------------|----------|-------|
| `gross_amount` | CANONICAL | KEEP | Buyer Midtrans gross = PD+S-K+F |
| `net_amount` | DERIVED | KEEP | = gross_amount - coin_discount_amount |
| `coin_discount` | CANONICAL | KEEP | Count of coins applied to this payment |
| `coin_discount_amount` | UNWRITTEN (always 0) | REDEFINE as K in Rupiah | Currently always 0 |
| `service_fee_amount` | CANONICAL | KEEP | Buyer payment gateway fee F |

### pricing_tokens table fields:

| Field | Classification | Decision | Notes |
|-------|---------------|----------|-------|
| `subtotal` | MISNAMED | REDEFINE or add `discounted_subtotal` field | Currently = P (undiscounted) |
| `discount_amount` | CANONICAL | KEEP | Already correctly calculated |
| `escrow_amount` | MISNAMED | REDEFINE as `seller_entitlement` | Currently = P+S+C-D |
| `service_fee_amount` | CANONICAL | KEEP (always 0 at token time) | Unknown until payment method chosen |
| `total_payable_amount` | CANONICAL | KEEP | = escrow + service_fee at token time |
| `max_coins_allowed` | CANONICAL | KEEP | Already correctly capped at 20%PD |
| `order_value_for_coins` | CANONICAL | KEEP | = PD+S for coins service |
| `coins_used` | DERIVED | KEEP | Set at order creation |

### New fields needed (migration):
- `orders.product_subtotal` (rename from `orders.subtotal` or add as net-new)
- `orders.seller_entitlement` (formerly escrow_amount)
- Ensure `orders.discount_amount`, `orders.coin_discount_amount` are actually written

---

## 6. LEDGER IMPACT MAP

Current ledger accounts:
- `ESCROW` — unused/legacy
- `SELLER_PAYABLE` — seller's net entitlement
- `BUYER_REFUNDABLE` — unused/legacy
- `PLATFORM_REVENUE` — commission + payment fee revenue
- `GATEWAY_CLEARING` — gross payment from buyer
- `BANK_SETTLEMENT` — contra to gateway clearing
- `WITHDRAWAL_PENDING`, `PLATFORM_BANK`, `VAT_LIABILITY`

Current settlement flow (`canonical_finalization_service.go`):
1. DR GATEWAY_CLEARING / CR BANK_SETTLEMENT (gross amount funding)
2. Service fee carved: DR GATEWAY_CLEARING / CR PLATFORM_REVENUE (F)
3. Create escrow: `escrowAmount = Sub+S+Comm` — this includes commission!
4. On release: escrow released to SELLER_PAYABLE = Sub+S+Comm (seller gets commission back!)

Target settlement entries needed:
1. DR GATEWAY_CLEARING / CR BANK_SETTLEMENT (buyer gross = PD+S-K+F)
2. DR BANK_SETTLEMENT / CR SELLER_ENTITLEMENT_LIABILITY (PD-C+S — seller's pending proceeds)
3. DR BANK_SETTLEMENT / CR PLATFORM_REVENUE (C — commission)
4. DR PLATFORM_REVENUE / CR BANK_SETTLEMENT (K — coins subsidy, contra-entry)
5. DR BANK_SETTLEMENT / CR PLATFORM_REVENUE (F — payment fee revenue)
6. On release: DR SELLER_ENTITLEMENT_LIABILITY / CR SELLER_PAYABLE

New account needed: `SELLER_ENTITLEMENT_LIABILITY` (or reuse escrow accounts).

Coins subsidy K must be recorded as a Labuda expense (reduction of platform revenue) to maintain seller entitlement integrity.

---

## 7. REFUND / RESTORATION IMPACT

### Coins restoration audit:

Coins ARE restored on cancellation/expiry/refund via `RefundCoinsInternal`:
- `backend\internal\incentive\coins\application\coins_service.go:616`
- INSERT-FIRST idempotency pattern using UNIQUE constraint on (user_id, reference_type, reference_id)
- Atomic balance add back
- Caller: `order_completion_service.go:1099` (deferred to `CoinsRefundRequiredHandler` worker)

**Coins restoration paths verified:**
1. Order cancellation: via `CoinsRefundRequiredHandler` worker (async, event-driven)
2. Order expiry: same handler
3. Payment failure/deny/cancel/expire: same handler
4. Full refund: via `RefundCoinsInternal`
5. Partial refund: proportional coin restoration based on `coinsUsed` ratio

**Idempotency:** INSERT-FIRST with UNIQUE constraint protects against double-refund.

**Current gap:** Coins are debited at order creation time, before payment. If payment fails:
- Coins ARE restored (async via worker)
- There is a time window where buyer's balance is reduced without benefit
- The worker-based restoration is eventual consistency, not atomic with payment

**Impact of target change on coins:** When K (coin discount) is properly applied to Midtrans gross, the refund of coins needs careful proportional math. Currently the full coins are restored on full refund (which is correct since the buyer never got the discount). When the discount IS applied, partial refund must proportionally reverse the coin discount.

### Refund component impact:

Current refund math (`refund_math.go`):
- Proportional commission/seller split based on `requested_amount / escrow_amount`
- Escrow = Sub+S+Comm — refunds proportionally return commission to buyer (wrong per target: commission shouldn't be in escrow)

Target refund math:
- Buyer refund = proportional to buyer's payment (PD+S-K+F)
- Seller proceeds adjustment = proportional to seller entitlement (PD-C+S)
- Commission reversal = proportional to C
- Coins restoration = proportional to K
- Payment fee: need business decision on whether F is refundable

**Business ambiguity — Payment gateway fee refundability:**
> Is the buyer payment gateway fee F refundable on full/partial refund? Midtrans typically does NOT refund MDR fees. Need owner decision: does Labuda absorb the MDR loss on refunds, or does the buyer lose the payment fee?

---

## 8. CLIENT / API IMPACT

### Client-tainted financial fields:

The Flutter client's `CreatePaymentRequest` submits:
```json
{
  "order_id": "<uuid>",
  "payment_method_code": "qris",
  "coin_discount": 500,
  "price_snapshot_id": "<uuid>"
}
```

**Risk assessment:**
- `coin_discount`: Client submits a coin COUNT. Backend validates against balance, 20% cap, and commission safety. However, the backend currently stores this value but sets `coin_discount_amount = 0` (no Rupiah effect). **When fixed, the backend MUST validate `coin_discount` against the order's `max_coins_allowed` and current balance — not trust the client value.**

- Monetary amounts (gross, net, fee, discount_amount, commission) are ALL server-derived. **No P1 client authority violation exists.**

- The mobile app correctly follows "backend is single source of truth" with minimal client-side math (only display estimates labeled "~").

### Required client changes (when backend is fixed):

1. **New display fields for buyer:**
   - "Harga Produk" (P)
   - "Diskon Penjual" (D) — seller discount
   - "Biaya Pengiriman" (S)
   - "Koin Digunakan" with Rupiah equivalent (K)
   - "Biaya Layanan Pembayaran" (F)
   - "Total Pembayaran" (PD+S-K+F)

2. **New display fields for seller:**
   - "Harga Produk Setelah Diskon" (PD)
   - "Biaya Platform" (C) — clearly labeled as seller's cost
   - "Pendapatan Bersih" (PD-C+S)

3. **Remove/relabel:**
   - "Biaya Layanan" at checkout should NOT be confused with commission
   - Escrow display should represent seller's pending entitlement, not buyer's gross payment

---

## 9. MOBILE PRESENTATION IMPACT

Current buyer checkout display (`checkout_order_summary_section.dart`):
- ✅ "Subtotal" — shows P
- ✅ "Biaya Pengiriman" — shows S
- ✅ "Biaya Layanan Pembayaran" — shows F (only if > 0)
- ✅ "Diskon Koin" — shows K (if > 0)
- ✅ "Diskon" — shows D (if > 0)
- ✅ "Total" — shows total payable

Current seller dashboard (`seller_earnings_screen.dart`):
- "Pendapatan Bersih" currently delegates to backend — correct
- But "Dana Tertahan Sementara" shows gross escrow INCLUDING commission — misleading

**Impact of target fix on UI:**
- Buyer total will DECREASE (commission removed): good UX
- Seller dashboard must clearly separate: gross sale → commission → net
- Coin discount must show actual Rupiah value, not zero

---

## 10. TESTS REQUIRED — FUTURE PROOF MATRIX

| # | Proof | Category | Current State |
|---|-------|----------|---------------|
| 1 | Seller price Rp100.000 = buyer product price Rp100.000 | BEHAVIORAL | NEED NEW |
| 2 | 5% commission does NOT raise buyer payable | BEHAVIORAL | NEED NEW |
| 3 | Seller receives PD-C | BEHAVIORAL | NEED NEW |
| 4 | Seller discount survives through Midtrans | REAL POSTGRES | NEED NEW |
| 5 | 1 coin = Rp10 applied to payment | BEHAVIORAL | NEED NEW |
| 6 | Max coins = 20% × PD, excluding shipping | UNIT | PARTIAL (coins service has 20% check, but based on orderValueForCoins which includes shipping in current formula) |
| 7 | Coins subsidy does not reduce seller entitlement | BEHAVIORAL | NEED NEW |
| 8 | Payment fee added only to buyer | BEHAVIORAL | EXISTS (PASS_18V fee calculation) |
| 9 | Monetary request fields backend-authoritative | SOURCE-SCAN | EXISTS (payment boundary hardening tests) |
| 10 | Cancellation/failure coins restoration | REAL POSTGRES | NEED NEW |
| 11 | Full refund math | BEHAVIORAL | PARTIAL (existing refund math tests need update for new escrow formula) |
| 12 | Partial refund math | BEHAVIORAL | PARTIAL |
| 13 | Ledger balance (DR=CR) | REAL POSTGRES | EXISTS (total_money_invariant tests) |
| 14 | Replay/idempotency | REAL POSTGRES | EXISTS (idempotency key tests) |
| 15 | Postgres constraints/persistence | REAL POSTGRES | NEED NEW for new columns |
| 16 | Mobile amount rendering | UNIT (Flutter) | NEED NEW for new display fields |

Current test inventory:
- `order_creation_service_test.go` — behavioral, tests happy path
- `order_create_contract_test.go` — source-scan, validates request binding
- `flat_fee_removed_test.go` — behavioral, verifies fee=0 at pricing preview
- `payment_method_default_killed_test.go` — source-scan, verifies method validation
- `payment_payability_guard_test.go` — source-scan, verifies payability checks
- `pricing_token_identity_test.go` — unit, validates token fields
- `refund_seller_decision_test.go` — unit, validates seller decision
- Various integration tests in `*_integration_test.go` — REAL POSTGRES

---

## 11. BUSINESS AMBIGUITIES (new, not answered in prompt)

### BA-1: Payment gateway fee refundability
**Question:** When a full refund occurs, does the buyer get back the payment gateway fee F? Midtrans typically does NOT refund MDR. Should Labuda:
- (a) Absorb the MDR loss and refund F to buyer from platform funds?
- (b) Not refund F (buyer loses the fee on refund)?

### BA-2: Shipping entitlement on partial refund
**Question:** On partial refund where the item is returned, does shipping S get refunded to buyer? Current shipping rules are out of scope per the prompt, but shipping interacts with the money contract on refund.

Neither ambiguity blocks the implementation of the core equations (buyer gross without commission, discount continuity, coins value). They affect only the refund refinement phase.

---

## 12. FILES INSPECTED (relevant only)

```
backend\migrations\000001_canonical_schema.up.sql          (schema authority)
backend\internal\serverboot\dependencies.go:3099-3438      (CreatePayment handler)
backend\internal\pricing\token\application\pricing_token_service.go  (pricing formulas)
backend\internal\commerce\order\application\order_creation_service.go (order creation)
backend\internal\commerce\order\entity\order.go            (order model)
backend\internal\commerce\order\infrastructure\repository\order_repository.go (order persistence)
backend\internal\commerce\order\application\order_payment_service.go (coins usage)
backend\internal\incentive\coins\application\coins_service.go (coins spend/refund)
backend\internal\commerce\order\application\order_completion_service.go (cancel/expire)
backend\internal\integration\payment\application\canonical_finalization_service.go (settlement)
backend\internal\finance\application\pricing_helper.go     (escrow calc helper)
backend\internal\finance\application\finance_service.go    (ledger posting)
backend\internal\finance\application\refund\refund_math.go (refund proportional calc)
backend\internal\commerce\paymentmethod\entity\method.go   (fee calculation)
backend\internal\pricing\discount\entity\discount.go       (discount entity)
backend\cmd\core_server\routes_core.go                     (route registration)
apps\mobile\lib\domains\commerce\transaction\checkout\...  (mobile checkout)
apps\mobile\lib\domains\finance\transaction\payment\...    (mobile payment)
apps\mobile\lib\domains\finance\wallet\coins\...           (mobile coins)
apps\mobile\lib\domains\user\preference\seller\...         (mobile seller)
```

---

## 13. FILES CHANGED

**NONE** (audit only)

---

## 14. GIT STATUS

Working tree dirty with pre-existing uncommitted mobile changes. Main branch. Git is NOT used as product authority — current filesystem is sole implementation truth.

---

## 15. RECOMMENDED FIRST IMPLEMENTATION SLICE

**Scope 4A-S1: Fix escrow semantics — remove commission from buyer payable**

This is the smallest, safest, highest-impact slice.

### What it does:
1. Change `CreatePayment` escrow derivation from `Sub+S+Comm` to `Sub+S-Discount`
2. Store `pricing_token.escrow_amount` (which already has correct formula) as `orders.escrow_amount` at creation
3. Use `order.escrow_amount` as single source of truth in CreatePayment instead of recalculating
4. Update `CalculateGrossEscrowFromSnapshot` (pricing_helper.go) to match

### What it does NOT touch:
- Coins value (still zero until next slice)
- Commission accounting (seller still gets full escrow until next slice)
- Ledger posting changes
- Mobile UI changes
- Refund logic

### Files touched (~5 files):
- `order.go` — add discountAmount to NewOrderFromSource
- `order_repository.go` — write discount_amount, escrow_amount on INSERT
- `order_creation_service.go` — pass discountAmount, escrowAmount to NewOrderFromSource
- `dependencies.go` — use order.escrow_amount for escrow derivation
- `pricing_helper.go` — match formula

### Why this is safe:
- Pricing token already computes correct escrow: `subtotal + shipping + commission - discount`
- The discount is already validated and calculated at token time
- This just persists what's already computed and stops recomputing it wrong

### Test scope: ~8-12 new tests
- Verify `order.escrow_amount = P + S + C - D` from pricing token
- Verify `CreatePayment` uses order.escrow_amount, not recalculation
- Verify discount survives from pricing → order → payment
- Regression: all existing commerce tests

---

## APPENDIX A: Current formula locations (hardcoded escrow = Sub+S+Comm)

| File | Line | Context |
|------|------|---------|
| `dependencies.go` | 3289 | CreatePayment: `escrowAmount := order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)` |
| `dependencies.go` | 3493 | ListPaymentMethods: `escrowAmount := order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)` |
| `pricing_helper.go` | 30 | `CalculateGrossEscrowFromSnapshot`: `order.Subtotal.Add(order.ShippingTotal).Add(order.CommissionAmount)` |
| `order_creation_service.go` | 1808 | buildOrderPayload: `escrowAmount := order.Subtotal.Int64() + order.ShippingTotal.Int64() + order.CommissionAmount.Int64()` |
| `pricing_token_service.go` | 403 | GenerateForFixedPriceSale: `escrowBase := subtotal.Add(shippingTotal).Add(commissionAmount)` then `escrowAmount := escrowBase.Sub(discountAmount)` — CORRECT formula, but NOT persisted to order |

## APPENDIX B: Target formula — single source of truth design

The pricing token's `escrow_amount` field is the CORRECT computed value. It should be:
1. Computed once at token generation (already happening)
2. Persisted to `orders.escrow_amount` at order creation (NOT happening — defaults to 0)
3. Used as authority in CreatePayment (NOT happening — recalculated wrong)
4. Used as authority in escrow creation at settlement (NOT happening — recalculated wrong)

The fix is to make the pricing token's escrow_amount flow through all three consumers unchanged, rather than being recalculated at each step with different formulas.

---

## APPENDIX C: Coin rate discrepancy (additional finding)

The locked business truth states: **"1 coin = Rp10"** and **"Max redemption: 20% × product price setelah seller discount"**.

Current implementation:
- **Earn rate**: 1 coin per Rp1.000 of final paid amount (`OrderRewardRate = 1000`, `coins_service.go:46`). This means buyer earns 1 coin per Rp1.000 spent.
- **Spend cap**: `maxCoins = orderValueForCoins / 5` (20% of order value as coin COUNT, `pricing_token_service.go:424`). This is 1 coin : 1 Rupiah ratio in the cap formula.
- **Actual spend value**: `coinDiscountAmount = 0` hardcoded at payment time — coins have zero Rupiah effect regardless of count.
- **Config**: `CoinValueCents = 1000` (Rp10) defined in `config.go:144` but **dead — never referenced**.

### Example of discrepancy

Product PD = Rp100.000, buyer has 5.000 coins:

| Aspect | Locked Truth (Rp10/coin) | Current Code |
|--------|--------------------------|--------------|
| Max coins allowed | 2.000 coins (20% × 100.000 / 10) | 20.000 coins (100.000 / 5) |
| Rupiah value if used | Rp20.000 | Rp0 (coinDiscountAmount=0) |

This means:
1. The 20% cap formula is 10× too permissive (allows 10× more coins than business rule)
2. Even if capped correctly, the Rupiah value is never applied
3. The earn rate (1 coin / Rp1.000 spent) and spend value (Rp10/coin) are asymmetric — this may be intentional (earn != spend value) or a defect

**Recommendation:** Scope 4A-S3 (coins slice) must reconcile the rate and cap to `1 coin = Rp10` as locked by business truth.

---

## APPENDIX D: Refund coin semantics gap

Current refund behavior (`refund_gateway.go:783-799`):
- **Full refund**: coins are restored (event `coins.refund_required` → `CoinsRefundRequiredHandler`)
- **Partial refund**: coins are NOT restored ("partial refunds intentionally skip coins refund")

Since coins currently have zero Rupiah value on payment (P1-3), this gap is masked. Once coins have real monetary value:
- Partial refund must proportionally restore coins (e.g., 50% refund → 50% coin restoration)
- Full refund must restore 100% of coins
- The current `money_partially_refunded` event handler that skips coins will become a P1 defect

---

## APPENDIX E: Discount completeness verification

Current discount handling verified:
- **Seller-created discounts**: ACTIVE. Three types: `percentage`, `flat_amount`, `free_shipping`. Created via `POST /discounts`. Validated during checkout via `ApplyDiscountAtCheckout`.
- **Commission basis**: Correctly calculated on `netSubtotal = subtotal - discountAmount`.
- **Discount persistence on order**: BROKEN. `NewOrderFromSource` does not accept discount params. `CreateOrderTx` INSERT hardcodes `discount_amount = 0`. The discount is stored on `pricing_tokens` and in `discount_usages` but NOT on the `orders` row.
- **Admin/platform discount**: DOES NOT EXIST. Code actively forbids nil-seller discounts. `ValidateDiscount` rejects `discount.SellerID == nil`.
- **Negotiation discounts**: DISABLED by design (`req.DiscountCode = nil` forced). Private agreements between buyer and seller.
