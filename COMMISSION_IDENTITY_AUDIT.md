# LABUDA — COMMISSION IDENTITY AUDIT

READ-ONLY AUDIT — ZERO IMPLEMENTATION.
No production code, tests, migrations, schema, ledger, refund, accounting, pricing, payment, or escrow behavior was modified.
This document establishes the FACTUAL identity of COMMISSION in the current codebase.

---

# 1. COMMISSION DEFINITION

## What `order.CommissionAmount` is

**`order.CommissionAmount` is a seller-side product commission snapshot.** It is computed at pricing-token generation time (i.e., before order creation) as:

```
CommissionAmount = floor( netSubtotal × commissionPercent / 100 )
netSubtotal      = Subtotal − DiscountAmount
```

- **Formula source:** `internal/pricing/token/application/pricing_token_service.go`
  - `calculateCommission(subtotal, percent)` at `:679-682`: `subtotal.Int64() * percent.IntPart() / 100`.
  - Called at `:384` (`commissionAmount := calculateCommission(netSubtotal, commissionPercent)`) and `:949` (auction claim path) with `netSubtotal = subtotal − discount`.
- **Rate source:** `s.configService.GetListingCommission(ctx, tx)` (`:379`, fixed-price) / `GetAuctionCommission(ctx, tx)` (`:1243`, auction claim). Rate is a platform config, not a per-listing field.
- **Product commission only?** YES — it is computed from the product net subtotal only.
- **Includes shipping commission?** NO. Shipping has no commission component anywhere: `refund_math.go:20` ("Shipping has NO commission and NO coin component") and `refund_policy.go:8` ("Shipping has NO commission component").
- **Includes any other fee?** NO. It is strictly product commission. The buyer payment-method fee is a separate field (`ServiceFeeAmount`) and a separate concept.
- **Stored as snapshot?** YES. `order.go:60` — "Commission amount snapshot". The order entity's pricing block is documented "immutable after creation" (`order.go:41-52`).
- **Can it change after order creation?** NO. The order copies it verbatim from the pricing snapshot at creation (`order_creation_service.go:936` for auctions, `:1623` for sale-surface). Nothing in the codebase mutates `orders.commission_amount` after creation (grep confirms only reads/inserts).

## Escrow formula at pricing time

```
escrowBase      = Subtotal + ShippingTotal + CommissionAmount
escrowAmount    = escrowBase − DiscountAmount
TotalBeforeCoinsAmount = escrowAmount  (canonical buyer base)
TotalPayableAmount     = escrowAmount + ServiceFeeAmount (payment fee added later by CorePaymentHandler)
```
Source: `pricing_token_service.go:404-415`.

## Key independent facts

- `order.TotalBeforeCoinsAmount` (canonical buyer base) = `Subtotal + Shipping − Discount + Commission`.
- `order.EscrowAmount` (escrow row) is created at gateway settlement from `CalculateGrossEscrowFromSnapshot(order)` = `Subtotal + ShippingTotal + CommissionAmount` (`canonical_finalization_service.go:129`), **after** the buyer payment fee has already been carved out of GATEWAY_CLEARING into PLATFORM_REVENUE (`:123-127`).
- Therefore the **escrow holds: Subtotal + Shipping + Commission** (the full gross snapshot), and the buyer payment fee never enters the escrow.

---

# 2. COMMISSION AUTHORITIES

Every independent representation of commission found:

| # | Representation | Source (file:func/field) | Formula / value | Semantic | Lifecycle stage | Authority |
|---|---|---|---|---|---|---|
| A | Platform config rate | `pricing_token_service.go:379,1243` (`GetListingCommission`/`GetAuctionCommission`) | config percent | The rate | pricing | AUTHORITATIVE (input) |
| B | Pricing token `CommissionPercent`/`CommissionAmount` | `pricing_token.go:51`; `pricing_token_service.go:384,443-444,503-504` | `floor(netSubtotal × pct / 100)` | Snapshot of commission at quote time | pricing token | AUTHORITATIVE (snapshot source) |
| C | Order `CommissionAmount` | `order.go:60`; `order_creation_service.go:936,1623` | copied verbatim from token | Immutable order snapshot | order creation → life | AUTHORITATIVE (snapshot) |
| D | Escrow gross | `CalculateGrossEscrowFromSnapshot` (`finance/application/pricing_helper.go:30`) | `Subtotal + Shipping + Commission` | Gross held in escrow | payment→release | **DERIVED** (explicitly non-authoritative for finance, `pricing_helper.go:8-27`) |
| E | `ReleaseSummary` gross/commission/sellerNet | `order_payment_service.go:264-291` | `sellerNet = subtotal+shipping`; `gross = sellerNet+commission` | Normal release split | release | DERIVED (computed at release from order snapshot) |
| F | PLATFORM_REVENUE credit at release | `finance_service.go:187-234` (`RecordOrderRelease`) | `DR GATEWAY_CLEARING −gross; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | Commission **posted** at release | release | AUTHORITATIVE (ledger posting) |
| G | Partial release commission | `finance_service.go:794-865` (`RecordPartialRefundRelease`) | `DR GATEWAY_CLEARING −remainder; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | Commission on the un-refunded remainder | partial refund ack | AUTHORITATIVE (ledger posting) |
| H | `CommissionDelta` | `refund_math.go:11,84-86` (`CalculateProportionalRefundBreakdown`) | `floor(C × cumProductAfter / PD) − floor(C × cumProductBefore / PD)` | Product-proportional commission reversal for this refund | refund ack | AUTHORITATIVE for refund math |
| I | Commission reversal at ledger | `finance_service.go:570-759` (`RecordRefundReversal`) | Before-release: `DR BUYER_REFUNDABLE +refund; CR GATEWAY_CLEARING −refund` (NO commission entries). After-release: `DR BUYER_REFUNDABLE +refund; CR SELLER_PAYABLE −SellerComponent; CR PLATFORM_REVENUE −CommissionComponent` | Where commission is reversed (or not) | refund ack | AUTHORITATIVE (ledger posting) |
| J | Verifier proportional commission | `finance/verifier/verifier.go:722-727` (`verifierProportionalCommission`) | `(amount × orderCommission) / orderGross` | Forensic check (uses full gross incl. shipping as denominator) | verification (offline) | **DERIVED — DIFFERENT DENOMINATOR than refund_math (PD)** |
| K | Payment fee (NOT commission) | `CorePaymentHandler` (`serverboot/dependencies.go`) + `RecordBuyerPaymentFeeRevenue` | fee formula from `payment_methods` | Buyer-facing fee, carved into PLATFORM_REVENUE at settlement | payment settlement | AUTHORITATIVE (separate concept) |
| L | `pricing_helper.go` | same as D | — | — | — | **DERIVED / DISPLAY-ONLY** |

**Critical authority statement:** The order snapshot (C) and the pricing token (B) are the same value. The ledger postings (F, G, I) are where commission becomes money. The refund math (H) is a *proportional* commission reversal that is only posted (I) **after release** — **before release it is NOT posted at all.**

---

# 3. COMMISSION LIFECYCLE

Trace of commission across the full lifecycle, with economic vs accounting vs escrow state:

| Stage | Where commission exists economically | Accounting location | Account holding it | Posted? |
|---|---|---|---|---|
| Pricing | `CommissionAmount` in token | none | none | No (not money yet) |
| Order creation | `order.CommissionAmount` snapshot | none | none | No |
| Payment settlement | inside `payment.GrossAmount` (gross = cash + fee; commission not separately carved) | GATEWAY_CLEARING += gross (from BANK_SETTLEMENT) | GATEWAY_CLEARING (system) | Yes (as part of gross) |
| Fee carve-out | commission still embedded in GATEWAY_CLEARING balance | `DR PLATFORM_REVENUE +fee; CR GATEWAY_CLEARING −fee` (`RecordBuyerPaymentFeeRevenue`) | GATEWAY_CLEARING | Fee posted; commission still just in balance |
| Escrow create | `escrow.Amount = Subtotal + Shipping + Commission` | escrow row (no ledger) | **escrow (not a ledger account)** | No — escrow allocation only |
| Pre-release refund (before escrow release) | refund amount to buyer = `CashRefund` = Rpd + Rs − CoinDelta; **commission NOT refunded to buyer** | `DR BUYER_REFUNDABLE +CashRefund; CR GATEWAY_CLEARING −CashRefund` | BUYER_REFUNDABLE / GATEWAY_CLEARING | Commission stays in GATEWAY_CLEARING (un-posted to revenue) |
| Partial refund ack | economicRefund = `cumProduct + cumShipping + cumCommissionReversed`; remainder released | `DR GATEWAY_CLEARING −remainder; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | PLATFORM_REVENUE (commission part of remainder) | Yes — commission on remainder posted |
| Normal release (no refund) | commission = full `order.CommissionAmount` | `DR GATEWAY_CLEARING −gross; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | PLATFORM_REVENUE | Yes |
| Full refund (before release) | commission never posted to revenue; stays in GATEWAY_CLEARING | `DR BUYER_REFUNDABLE +refund; CR GATEWAY_CLEARING −refund` | GATEWAY_CLEARING | No — commission never became revenue |
| Post-release reversal (AfterRelease path) | commission reversal = `CommissionDelta` | `DR BUYER_REFUNDABLE +refund; CR SELLER_PAYABLE −SellerComponent; CR PLATFORM_REVENUE −CommissionComponent` | PLATFORM_REVENUE (decremented) | Yes — **but ack path disables this** (see below) |

**Key lifecycle facts:**

1. **Escrow gross includes commission, but commission is only "posted" when the remainder is released.** Before any release, commission exists only inside the GATEWAY_CLEARING balance and the escrow row — it is an allocation, not a revenue posting.
2. **Pre-release refund does NOT reverse commission** — there is no PLATFORM_REVENUE entry in the before-release branch of `RecordRefundReversal` (`finance_service.go:678-683`). This is correct only if commission was never posted to revenue before release (it wasn't — it's still in GATEWAY_CLEARING).
3. **After-release reversal reverses commission via `−CommissionComponent` against PLATFORM_REVENUE** (`finance_service.go:649-653`). But the gateway ack handler currently **disables after-release processing**: `refund_gateway.go:451-454` returns `"post-release refund acknowledgements are disabled"`. So in the current runtime, **post-release refunds do not reach the after-release ledger branch through the gateway ack path.**
4. **Partial release:** when a partial refund leaves a remainder, `RecordPartialRefundRelease` posts the remainder's commission to PLATFORM_REVENUE and the remainder's sellerNet to SELLER_PAYABLE (`finance_service.go:842-846`), using `remCommission = floor(remProduct × C / PD)` (`refund_gateway.go:495-501`).

---

# 4. 13600 vs 3600 ANALYSIS

## The discrepancy

```
Escrow:       Subtotal 90000 + Shipping 15000 + Commission 13600 = 118600
Refund math:  CommissionDelta = 3600   (for some refund amount)
```

## Facts established from source

1. **13600 is the escrow-embedded commission snapshot**: `order.CommissionAmount` = 13600. This is `floor(netSubtotal × pct/100)` computed at pricing.
2. **3600 is a `CommissionDelta`** — the *proportional commission reversal for one refund event*: `floor(C × cumProductAfter / PD) − floor(C × cumProductBefore / PD)` (`refund_math.go:84-86`).
3. They are **different concepts, not a contradiction**: 13600 is the total commission obligation on the order; 3600 is the commission reversed for a specific partial refund.
4. **Is 3600 a subset of 13600?** Almost certainly — `CommissionDelta ≤ floor(C × PD/PD) = C = 13600` because it's floor-scaled from cumulative product refunded ≤ PD. A full product refund (`cumProductAfter = PD`) would give `CommissionDelta = floor(C × PD/PD) − 0 = 13600` (minus any prior partials).
5. **Does 13600 contain shipping commission?** NO. It's product-only, computed on net subtotal. Shipping has no commission.
6. **Which value is "incorrectly sourced"?** Neither is wrong per se — they answer different questions. The **only place the two could be conflated** is the **verifier**, which uses a different denominator:
   - `refund_math.go` (canonical refund math): denominator = **PD** (product basis, discounted).
   - `verifier.go:722-727` (`verifierProportionalCommission`): denominator = **orderGross** = `Subtotal + ShippingTotal` (includes shipping, uses **undiscounted** Subtotal).
   These two proportional formulas **disagree** whenever Shipping > 0 or Discount > 0. This is a genuine **duplicate/divergent authority** in the verification layer.
7. **The 118600/93600/25000/10000 relationship:** Given EscrowGross = 118600 and EconomicRefund = 93600:
   - If EconomicRefund = ProductRefund + ShippingRefund + CommissionReversed, and CommissionReversed = 3600 for a partial, the remaining obligation = EscrowGross − EconomicRefund = 25000. That implies the partial refunded Product + Shipping = 90000 of the 105000 (PD+S), leaving 15000 shipping + 10000... actually this specific combination only balances if the refund allocation (Rpd,Rs) and the cumulative math are consistent. **I did not find any production code that computes a `RemainingObligation` field** — it does not exist as a named field in the codebase (grep found none). It is an *inferred* quantity from `escrow.Amount − economicRefund` (`refund_gateway.go:493`), which the source uses for partial-release remainder.
   - **The 13600 vs 3600 discrepancy is consistent with:** CommissionDelta for a partial refund that reverses commission proportional to the product portion refunded. It is **NOT** a contradiction in the escrow math — escrow still holds the full 13600 until release/remainder; the refund only reverses the *proportional* share.

## Fixture vs production

- The observed values (118600 / 93600 / 25000 / 3600 / 10000 / 15000) match a **fixture/seed scenario** shape. I did not find a committed fixture with exactly these numbers in the files I read, but the arithmetic is internally consistent with the canonical formulas:
  - EscrowGross = 90000 + 15000 + 13600 = 118600 ✓ (`pricing_helper.go`).
  - A partial refund of product 90000 (full product) with `CommissionDelta = 3600` would require `floor(13600 × cumProductAfter/PD) = 3600`, which implies `cumProductAfter/PD ≈ 0.265` — **not** a full-product refund. A **full product refund** (PD=90000) would give CommissionDelta = 13600 (or 13600 − prior), **not** 3600.
  - So 3600 as a *single-event* delta is consistent with a ~26.5% product refund; as a *cumulative* reversal it would only equal 13600 at full product refund. **The 3600 value cannot be the commission reversal for a full refund** — it must correspond to a partial product refund, OR the fixture's `CommissionAmount` was 3600 and the escrow's 13600 came from a different source.

## Actual discrepancy candidates (to record, not resolve)

| Candidate | Evidence | Likelihood |
|---|---|---|
| (a) 13600 = full `order.CommissionAmount`; 3600 = proportional `CommissionDelta` for a partial refund | refund_math.go formula | HIGH — different concepts, no conflict |
| (b) The refund `pd` used in the denominator is **discounted** (`refund_gateway.go:409`: `pd = Subtotal − discount`), while the escrow `CommissionAmount` is also computed on discounted subtotal — so both use the same PD basis. Consistent. | refund_gateway.go:409, pricing_token_service.go:384 | HIGH |
| (c) **Verifier uses orderGross (Subtotal+Shipping) as denominator** — if the 3600 came from the verifier, it would use `3600 = (amount × 13600)/(90000+15000)` and be **inconsistent with the canonical refund_math denominator (PD)**. | verifier.go:722-727 | MEDIUM — this is the one real divergence |
| (d) The fixture's `CommissionAmount` was 3600 and the escrow's 13600 came from a different snapshot (e.g., token vs order mismatch) | no code path found that would cause this | LOW — no mutation path exists |

**Conclusion on 13600 vs 3600:** The values represent **two different things** (total snapshot vs per-refund proportional delta) and are **not inherently contradictory**. The only genuine cross-representation inconsistency is the **verifier's use of a different denominator (orderGross incl. shipping + undiscounted subtotal) than the canonical refund math (PD, discounted)**. That is a **DUPLICATE-AUTHORITY divergence**, not a data bug.

---

# 5. FUNDING / ACCOUNTING IDENTITY

## Who funds commission

| Movement | Ledger entries | Funded by |
|---|---|---|
| Payment settlement | `DR GATEWAY_CLEARING +gross; CR BANK_SETTLEMENT −gross` | BANK_SETTLEMENT (reserve) → GATEWAY_CLEARING |
| Buyer payment fee carve | `DR PLATFORM_REVENUE +fee; CR GATEWAY_CLEARING −fee` | GATEWAY_CLEARING → PLATFORM_REVENUE |
| Normal release | `DR GATEWAY_CLEARING −gross; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | GATEWAY_CLEARING → SELLER_PAYABLE + PLATFORM_REVENUE |
| Partial release (remainder) | `DR GATEWAY_CLEARING −remainder; CR SELLER_PAYABLE +sellerNet; CR PLATFORM_REVENUE +commission` | GATEWAY_CLEARING → SELLER_PAYABLE + PLATFORM_REVENUE |
| Pre-release refund | `DR BUYER_REFUNDABLE +CashRefund; CR GATEWAY_CLEARING −CashRefund` | GATEWAY_CLEARING → BUYER_REFUNDABLE |
| Post-release refund (disabled at ack) | `DR BUYER_REFUNDABLE +refund; CR SELLER_PAYABLE −SellerComponent; CR PLATFORM_REVENUE −CommissionComponent` | SELLER_PAYABLE + PLATFORM_REVENUE → BUYER_REFUNDABLE |

**Facts:**
- **Commission is funded from GATEWAY_CLEARING** (the money physically sits in the platform clearing account after settlement). It is **not** a separate escrow sub-account.
- **Commission is posted to PLATFORM_REVENUE only at release** (normal or partial-remainder). Before release, it is an **escrow allocation** inside GATEWAY_CLEARING + the escrow row — **not posted revenue**.
- **GATEWAY_CLEARING ≠ escrow gross.** GATEWAY_CLEARING holds all settled gross minus carved-out fees; escrow holds a per-order `Subtotal+Shipping+Commission` allocation. They are different concepts (a pooled system account vs a per-order allocation). The observed "gateway clearing before partial release = 15000" and "attempted partial release clearing debit = 25000" in the prompt are consistent with this: a partial release debits GATEWAY_CLEARING by the remainder (25000), while the prior refund debited it by the refunded portion.
- **Commission is never funded by PLATFORM_REVENUE as a source** — PLATFORM_REVENUE is the *destination* of commission. The only reversal against PLATFORM_REVENUE is the after-release refund (currently disabled via ack gate).

---

# 6. REFUND COMMISSION POLICY (current implementation)

| Path | What happens today |
|---|---|
| **Product commission refund** | Reversed proportionally: `CommissionDelta = floor(C × cumProductAfter/PD) − floor(C × cumProductBefore/PD)` (`refund_math.go`). Only ever posted in ledger **after release**; before release, no commission ledger entry (commission never became revenue). |
| **Shipping commission** | **None exists** — shipping has no commission component (`refund_math.go:20`, `refund_policy.go:8`). |
| **Commission reversal (pre-release)** | Not posted to ledger (stays in GATEWAY_CLEARING). The buyer refund excludes commission entirely (`CashRefund = Rpd + Rs − CoinDelta`). |
| **Commission reversal (post-release)** | Code exists (`RecordRefundReversal` AfterRelease branch, `−CommissionComponent` on PLATFORM_REVENUE) but the **gateway ack path returns "post-release refund acknowledgements are disabled"** (`refund_gateway.go:451-454`) — so it is not currently reached via webhook. |
| **Pre-release refund** | Buyer gets `CashRefund` (cash) + coins restored; escrow flips refunded/partial; commission stays in GATEWAY_CLEARING. |
| **Post-release refund** | Not active via gateway ack. The `AfterRelease` ledger branch is a dormant/partially-built path. |

**Policy truth (from `refund_policy.go` + `refund_math.go`):**
- Buyer refund = product + shipping (Rpd + Rs), minus coin restoration; **commission and payment fee excluded from buyer refund** ("C is seller-side and never in buyer CashRefund", `refund_math.go:22`).
- Commission reversal is **product-proportional only**; shipping carries no commission.
- `SellerComponent = Rpd + Rs − CommissionDelta` — the seller gives up the commission on the refunded product portion economically.

---

# 7. CANONICAL IDENTITY CANDIDATES

## A. Definitely canonical commission authority
1. **Pricing-time rate:** `platform_configs` listing/auction commission rate (`configService.GetListingCommission`/`GetAuctionCommission`).
2. **Pricing-token snapshot:** `pricing_tokens.commission_amount` + `commission_percent` (computed `floor(netSubtotal × pct / 100)`).
3. **Order snapshot:** `orders.commission_amount` — immutable copy of the token value. This is THE commission identity for every downstream financial calc (release, refund, verifier).
4. **Release posting:** `RecordOrderRelease` / `RecordPartialRefundRelease` — the only places commission is posted to PLATFORM_REVENUE.

## B. Merely derived
- `CalculateGrossEscrowFromSnapshot` (explicitly non-authoritative display/validation helper).
- `ReleaseSummary.gross/commission/sellerNet` (computed at release from the snapshot).
- `CommissionDelta` (derived proportional reversal per refund event).
- `SellerComponent` (derived).
- `economicRefund` / `RemainingObligation` (inferred cumulative quantities; `RemainingObligation` has **no named field** in source).

## C. Contradictory representations
- **Verifier vs refund math:** `verifierProportionalCommission` uses denominator `orderGross = Subtotal + ShippingTotal` (undiscounted, shipping-inclusive); canonical refund math uses denominator `PD = Subtotal − Discount` (discounted, shipping-exclusive). **These two disagree** whenever shipping > 0 or discount > 0. This is the only true formula-level contradiction found.

## D. Duplicate authorities
- **Two proportional-commission formulas** (refund_math vs verifier) — duplicate logic with different denominators.
- **`LegacyGross()` in refund_policy.go** (`Subtotal + Shipping + Commission`) vs `CalculateGrossEscrowFromSnapshot` (same formula) — two names for the same derived value (benign duplicate, both derived).
- **Commission posted in two release functions** (`RecordOrderRelease` and `RecordPartialRefundRelease`) — same shape, different idempotency keys; not a conflict but a duplicated pattern.

## E. Legacy / zombie / incorrect
- **`refund_policy.go:36` `LegacyGross()`** — named "Legacy", currently used only in tests (`refund_policy_test.go`). Appears to be a retained helper with a legacy name; no production reader found.
- **Post-release refund ledger branch** (`RecordRefundReversal` AfterRelease) — currently unreachable via the gateway ack gate (`refund_gateway.go:451-454`). Either a dormant future path or a half-built feature.
- **`money.partial_release` / `money.partial_refund` outbox events** — marked in the event registry as consumed-without-side-effect / audit-only; the partial release path's real money movement is the ledger `RecordPartialRefundRelease`, not these events.

---

# 8. REQUIRED CONSERVATION IDENTITIES

Verified against source:

| Identity | Status | Source |
|---|---|---|
| `EscrowGross = Subtotal + Shipping + Commission` | **VALID** (snapshot-derived; escrow row = this sum) | `pricing_helper.go:30-31`; `order_creation_service.go:1726`; `canonical_finalization_service.go:129` |
| `TotalBeforeCoinsAmount = Subtotal + Shipping − Discount + Commission` | **VALID** | `pricing_token_service.go:404-415` |
| `ReleaseGross = SellerNet + Commission`, `SellerNet = Subtotal + Shipping` | **VALID** | `order_payment_service.go:264-268`; enforced `finance_service.go:199-201` |
| `CashRefund + CoinDelta = Rpd + Rs` | **VALID** (explicit invariant) | `refund_math.go:16,111-114` |
| `SellerComponent + CoinDelta = Rpd + Rs` (economic) | **VALID** — seller gives up commission; `SellerComponent = Rpd + Rs − CommissionDelta` | `refund_math.go:12,118` |
| `cumCash + cumCoins = cumProduct + cumShipping` | **VALID** (explicit accounting identity) | `refund_math.go:17` |
| `EconomicRefund = ProductRefund + ShippingRefund + CommissionReversed` | **VALID as cumulative accounting** — used at `refund_gateway.go:482` | `refund_gateway.go:482-484` |
| `RemainingObligation = EscrowGross − EconomicRefund` | **VALID only as an inferred quantity** — no named field; source uses `escrow.Amount − economicRefund` at `refund_gateway.go:493`. **This is NOT a canonical identity in code.** | `refund_gateway.go:493` |
| `CommissionDelta = floor(C × cumProductAfter/PD) − floor(C × cumProductBefore/PD)` | **VALID** — canonical refund formula | `refund_math.go:11,84-86` |
| Verifier `proportionalCommission = amount × C / orderGross` | **VALID but uses a DIFFERENT denominator than the canonical PD** | `verifier.go:722-727` |
| `cumCashAfter ≤ PD + S − K` (gateway cap) | **VALID** (explicit cap) | `refund_math.go:18,103-108` |
| `MaxGatewayRefund = PD + S − K` | **VALID** | `refund_math.go:149-154` |

---

# 9. FINAL VERDICT

## COMMISSION_IDENTITY_CONTRADICTION_FOUND

The commission snapshot itself is **single-sourced and self-consistent** (token → order snapshot → release posting → refund delta all derive from the same immutable value). But **two formula-level authorities disagree**, and the observed 13600/3600 values are consistent with that divergence:

1. **Canonical refund math** uses denominator `PD = Subtotal − Discount` (product-only, discounted).
2. **Verifier** uses denominator `orderGross = Subtotal + ShippingTotal` (shipping-inclusive, undiscounted).

These produce **different CommissionDelta for the same refund** whenever shipping or discount is non-zero — which is the exact shape of the 13600 vs 3600 discrepancy (3600 ≠ floor(90000×13600/105000)). This is a **CONTRADICTION between two commission-reversal authorities**.

---

## CANONICAL_FACTS

- Commission = `floor(netSubtotal × rate / 100)`, netSubtotal = Subtotal − Discount, rate from platform config (listing or auction).
- Commission is product-only; shipping carries no commission; payment fee is a separate concept.
- `order.CommissionAmount` is an immutable snapshot copied from the pricing token at order creation.
- Escrow gross = Subtotal + Shipping + Commission (created at gateway settlement, after fee carve-out).
- Commission is posted to PLATFORM_REVENUE only at release (normal or partial-remainder).
- Pre-release refunds never touch PLATFORM_REVENUE — commission stays in GATEWAY_CLEARING.
- Buyer refund = Rpd + Rs − CoinDelta; commission is never in the buyer cash refund.
- Post-release refund ledger branch exists but is disabled at the gateway ack path.

## COMMISSION_AUTHORITIES

See §2 table. Canonical: platform-config rate → pricing token snapshot → order snapshot → `RecordOrderRelease`/`RecordPartialRefundRelease` (posting) → `refund_math.go` (refund delta). Divergent: `verifier.go` proportional commission. Derived: `pricing_helper.go`, `ReleaseSummary`, `economicRefund`, `RemainingObligation`.

## COMMISSION_LIFECYCLE

See §3. Economic allocation at escrow → posted revenue only at release → proportional reversal on refund (before-release: no ledger commission entry; after-release: disabled at ack).

## 13600_VS_3600_ANALYSIS

13600 = full order commission snapshot (product, on net subtotal). 3600 = per-event proportional `CommissionDelta`. They are different concepts and are NOT contradictory per se; the contradiction is that the **verifier** would compute a different delta (denominator orderGross) than the canonical refund math (denominator PD). A 3600 delta is consistent with a partial product refund under the canonical formula, NOT with a full product refund.

## ACCOUNTING_LOCATION

Commission lives in GATEWAY_CLEARING (unposted) until release; posted to PLATFORM_REVENUE at release; reversed from PLATFORM_REVENUE only in the (currently disabled) after-release path. Escrow is an allocation, not a ledger account. GATEWAY_CLEARING ≠ escrow gross.

## REFUND_TREATMENT

Buyer refund excludes commission. Commission reversal is product-proportional (`refund_math.go`). Pre-release: no ledger reversal (commission never posted). Post-release: reversal code exists but the gateway ack gate disables it. Shipping has no commission.

## CONTRADICTIONS

1. Verifier denominator (orderGross, shipping-inclusive, undiscounted) vs canonical refund denominator (PD, discounted) — **the only formula-level contradiction**.
2. Post-release refund path: ledger code exists but ack gate disables it — **dormant/partial**, not active.

## UNSUPPORTED_ASSUMPTIONS

- `RemainingObligation` has no named field in source — it is an inference from `escrow.Amount − economicRefund` (`refund_gateway.go:493`). The prompt's "remaining escrow obligation = 25000" is consistent with that inference but is **not** a code identity.
- The specific fixture values (118600/93600/25000/3600/10000/15000) were not found verbatim in committed fixtures I read; their arithmetic is consistent with the canonical formulas, and 3600 cannot be a full-refund commission reversal under the canonical formula.
- Whether the observed 13600 escrow commission and 3600 delta came from the same order snapshot is NOT proven by the code alone — if the fixture's `order.CommissionAmount` was actually 3600 and the escrow's 13600 came from a token/order mismatch, that would be a data-level (not code-level) inconsistency. No code path that would cause such a mismatch was found.

## CANDIDATE_CANONICAL_TRUTH

**The canonical commission identity is the immutable `orders.commission_amount` snapshot (product commission on discounted subtotal), posted to PLATFORM_REVENUE only at release, reversed product-proportionally per refund with denominator PD = Subtotal − Discount.** The verifier's orderGross-based proportional commission is the contradictory representation and should be the target of any future convergence.

## EVIDENCE

- `internal/pricing/token/application/pricing_token_service.go:378-405,679-682,943-962` (formula, escrow base)
- `internal/pricing/token/entity/pricing_token.go:51`
- `internal/commerce/order/entity/order.go:41-63` (immutable snapshot doctrine)
- `internal/commerce/order/application/order_creation_service.go:925-953,1623,1726` (snapshot copy, escrow calc)
- `internal/finance/application/pricing_helper.go:8-32` (derived helper, non-authoritative)
- `internal/finance/application/finance_service.go:187-234` (RecordOrderRelease), `:570-759` (RecordRefundReversal), `:794-865` (RecordPartialRefundRelease)
- `internal/integration/payment/application/canonical_finalization_service.go:100-138` (settlement funding, fee carve, escrow create)
- `internal/commerce/order/application/order_payment_service.go:255-292` (ReleaseGatewayEscrowToSeller)
- `internal/finance/refund/application/refund_math.go:1-146` (canonical proportional breakdown)
- `internal/finance/refund/entity/refund_policy.go:29-36` (OrderSnapshot, LegacyGross)
- `internal/finance/refund/application/refund_gateway.go:400-534` (ack handling, economicRefund, partial release, post-release disabled)
- `internal/finance/verifier/verifier.go:722-727` (divergent proportional commission)

## FILES_CHANGED: NONE
## PRODUCTION_FILES_CHANGED: NONE
## TEST_FILES_CHANGED: NONE
## DATABASE_CHANGED: NONE
## IMPLEMENTATION: NOT_AUTHORIZED

*This audit is intentionally read-only. It exists to establish the factual identity of commission before any future convergence that would remove conflicting implementations.*
