# LABUDA — COMMISSION AUTHORITY CONVERGENCE AUDIT

READ-ONLY AUDIT — ZERO IMPLEMENTATION.
No production code, tests, migrations, schema, ledger, refund, accounting, pricing, payment, or escrow behavior was modified.
The ONLY artifact created is this audit report.

---

# 1. VERDICT

## COMMISSION_AUTHORITY_CONTRADICTION_FOUND

There is **exactly one canonical commission identity** (`order.CommissionAmount`, product-only, immutable snapshot), and the **refund allocation math is internally consistent and canonical** (`refund_math.go`, denominator PD = Subtotal − Discount). However, **one live verifier reimplements proportional commission with a different denominator**, producing a genuinely conflicting formula:

- **Canonical refund math** (`refund_math.go`): denominator = `PD = Subtotal − Discount` (discounted, shipping-exclusive).
- **Verifier** (`verifier.go:722-727`): denominator = `orderGross = order.EscrowAmount` = `Subtotal + ShippingTotal + CommissionAmount` (undiscounted, shipping- AND commission-inclusive).

These two produce **different `CommissionDelta` values for the same refund** whenever Shipping > 0 or Discount > 0 — which is the exact shape of the 13600-vs-3600 discrepancy. The verifier's formula is a **CONFLICTING_AUTHORITY** and must be the primary removal target.

Beyond that single contradiction, a small set of duplicate/derived/legacy representations exist but are not formula conflicts (documented in §7, §10, §11).

---

# 2. CANONICAL COMMISSION IDENTITY

**`order.CommissionAmount`** (immutable order snapshot, copied verbatim from the pricing token at order creation).

- Entity: `internal/commerce/order/entity/order.go:60` — `CommissionAmount money.Money` "Commission amount snapshot".
- Immutable doctrine: `order.go:41-52` — pricing snapshot fields "NEVER recalculated", "immutable after creation".
- Copied at order creation: `order_creation_service.go:936` (auction claim) and `:1623` (sale surface) — `snapshot.CommissionAmount` → order.
- No mutation path exists (grep of `orders.commission_amount` shows only insert/read).

**Source of the identity: pricing-token computation.**
- `internal/pricing/token/application/pricing_token_service.go:379-384` (fixed-price):
  - `commissionPercent := s.configService.GetListingCommission(ctx, tx)`
  - `netSubtotal := subtotal.Sub(discountAmount)`
  - `commissionAmount := calculateCommission(netSubtotal, commissionPercent)`
- `:1243-1248` (auction claim): `GetAuctionCommission`, same `netSubtotal` formula.
- `:679-682` `calculateCommission(subtotal, percent)`: `subtotal.Int64() * percent.IntPart() / 100` — **floor division**.
- Persisted to token: `pricing_token_repository_impl.go:83,161,207`; entity field `pricing_token.go:51`.
- Serialized in snapshot DTO: `pricing_token_service.go:156,504`; handler `pricing_token_handler.go:382`.

---

# 3. CANONICAL COMMISSION FORMULA

```
CommissionAmount = floor(PD × commissionRate / 100)
PD               = Subtotal − DiscountAmount
commissionRate   = platform_configs listing/auction commission rate
```

- Product-only. Shipping carries no commission (locked truth #3; `refund_math.go:20`, `refund_policy.go:8`).
- Denominator basis for ALL proportional allocation: `PD = Subtotal − Discount` (locked truth #8).
- Rounding: integer floor division at computation (`pricing_token_service.go:681`) and at proportional allocation (`refund_math.go:141-146` `proportionalFloor`).
- Rate source is platform config, NOT a per-listing/per-order field.

---

# 4. CANONICAL REFUND COMMISSION FORMULA

```
CommissionDelta = floor(C × cumProductAfter / PD) − floor(C × cumProductBefore / PD)
SellerComponent = Rpd + Rs − CommissionDelta
CashRefund      = Rpd + Rs − CoinDelta        (buyer cash; commission excluded)
CoinDelta       = floor(K × cumProductAfter / PD) − floor(K × cumProductBefore / PD)
```

- Source: `internal/finance/refund/application/refund_math.go:1-146` (`CalculateProportionalRefundBreakdown`), documented at `:11-12` and duplicated at `refund_gateway.go:7-8`.
- Denominator: `PD` (parameter `pd`), validated `pd > 0` (`refund_math.go:39-40`).
- Canonical guards: `cumCoinsBefore` and `cumCommissionBefore` must match the floor-proportional derivation (`refund_math.go:63-72`); cumulative cash cap `cumCashAfter ≤ PD + S − K` (`:103-108`); accounting identity `cash + coins == rpd + rs` (`:110-114`).
- **`CommissionDelta` is a DERIVED allocation, NOT a second commission identity** (locked truth #7).

---

# 5. COMMISSION LIFECYCLE

| Stage | Commission value | Source (canonical?) | Posted? |
|---|---|---|---|
| Pricing | token.CommissionAmount = floor(PD × rate/100) | pricing_token_service.go:384,948,1248 (CANONICAL) | No |
| Order | order.CommissionAmount (copy) | order_creation_service.go:936,1623 (CANONICAL copy) | No |
| Escrow | escrow.Amount = Subtotal + Shipping + Commission | canonical_finalization_service.go:129 via `CalculateGrossEscrowFromSnapshot` (DERIVED display helper, `pricing_helper.go:30-31`) | No (escrow allocation) |
| Settlement | GATEWAY_CLEARING += gross | finance_service.go:323+ (RecordGatewayPaymentSettlement) | Yes (as part of gross) |
| Fee carve | PLATFORM_REVENUE += fee (NOT commission) | finance_service.go RecordBuyerPaymentFeeRevenue (separate concept) | Yes |
| Normal release | CR PLATFORM_REVENUE +commission; CR SELLER_PAYABLE +sellerNet; DR GATEWAY_CLEARING −gross | finance_service.go:187-234 (RecordOrderRelease) | Yes |
| Refund (before release) | NO commission ledger entry (stays in GATEWAY_CLEARING) | finance_service.go:678-683 (RecordRefundReversal before-release branch) | No |
| Refund (after release) | CR PLATFORM_REVENUE −CommissionComponent; CR SELLER_PAYABLE −SellerComponent | finance_service.go:624-676 (AfterRelease branch) — **currently disabled at ack gate** `refund_gateway.go:451-454` | Yes (dormant) |
| Partial release (remainder) | CR PLATFORM_REVENUE +remCommission; CR SELLER_PAYABLE +sellerNet | finance_service.go:794-865 (RecordPartialRefundRelease); remCommission computed `refund_gateway.go:495-501` | Yes |

---

# 6. AUTHORITY MAP

Every representation found, classified:

| # | Symbol / function | File:lines | Inputs | Formula | Denominator | Rounding | Class |
|---|---|---|---|---|---|---|---|
| 1 | `calculateCommission` | pricing_token_service.go:679-682 | subtotal, percent | `subtotal × pct / 100` | — | floor | **CANONICAL_AUTHORITY** (computation) |
| 2 | `token.CommissionAmount` | pricing_token.go:51; repo :83,207,324 | (stored) | — | — | — | **CANONICAL_AUTHORITY** (stored snapshot) |
| 3 | `order.CommissionAmount` | order.go:60; creation :936,1623 | copied from token | — | — | — | **CANONICAL_AUTHORITY** (immutable identity) |
| 4 | `CalculateGrossEscrowFromSnapshot` | finance/application/pricing_helper.go:30-31 | order snapshot | `Subtotal+Shipping+Commission` | — | — | **DERIVED_VALUE** (explicitly non-authoritative for finance, `:8-27`) |
| 5 | `ReleaseGatewayEscrowToSeller` gross/sellerNet/commission | order_payment_service.go:255-292 | order snapshot | sellerNet=Subtotal+Shipping; gross=sellerNet+Commission | — | — | **CONSUMER_OF_CANONICAL_VALUE** |
| 6 | `RecordOrderRelease` | finance_service.go:187-234 | gross, commission, sellerNet | entries DR GC −gross / CR SP +sellerNet / CR PR +commission | — | — | **CONSUMER_OF_CANONICAL_VALUE** (posting) |
| 7 | `RecordPartialRefundRelease` | finance_service.go:794-865 | remainder, sellerNet, commission | same shape as #6 | — | — | **CONSUMER_OF_CANONICAL_VALUE** (posting) |
| 8 | `CalculateProportionalRefundBreakdown` | refund_math.go:35-139 | PD,S,C,K,Rpd,Rs,cums | `CommissionDelta = floor(C×cumAfter/PD) − floor(C×cumBefore/PD)` | **PD** | floor | **CANONICAL_AUTHORITY** (refund allocation) |
| 9 | `proportionalFloor` | refund_math.go:141-146 | amount, numerator, denominator | `amount×num/den` | PD (caller) | floor | **CANONICAL_AUTHORITY** (primitive) |
| 10 | `CommissionDelta` field | refund_math.go:30,135; refund_gateway.go:464 | from #8 | derived per-event | PD | floor | **DERIVED_VALUE** (NOT an identity) |
| 11 | `verifierProportionalCommission` | verifier.go:722-727 | amount, orderCommission, orderGross | `amount × C / orderGross` | **orderGross = EscrowAmount** | floor | **CONFLICTING_AUTHORITY** |
| 12 | verifier usage `:668-671` | verifier.go:668-671 | previouslyRefunded, FinalRefundAmount, orderCommission, orderGross | expectedCommissionComponent = after − before | **orderGross = EscrowAmount** | floor | **CONFLICTING_AUTHORITY** |
| 13 | verifier `orderGross` binding | verifier.go:639 | order.EscrowAmount | = Subtotal+Shipping+Commission | — | — | **CONFLICTING_AUTHORITY** (bad denominator source) |
| 14 | `OrderSnapshot.LegacyGross` | refund_policy.go:36 | Subtotal,Shipping,Commission | `Subtotal+Shipping+Commission` | — | — | **LEGACY/ZOMBIE** (named Legacy; used only by tests) |
| 15 | `OrderSnapshot.ProductGross` | refund_policy.go:35 | Subtotal,Shipping | `Subtotal+Shipping` | — | — | **DERIVED_VALUE** (used by policy `CashRefund`) |
| 16 | verifier `Order` snapshot load | verifier.go:1025 | orders row | — | — | — | **CONSUMER_OF_CANONICAL_VALUE** (reads `commission_amount`, `escrow_amount` from DB) |
| 17 | `refund_policy_test.go` testOrder | refund_policy_test.go:14-18 | hardcoded | Subtotal 100000, Shipping 25000, C 6250 | — | — | **TEST_ONLY_REIMPLEMENTATION** (fixture, not a formula) |
| 18 | `refund_math_test.go` constants | refund_math_test.go:8-13 | hardcoded | PD 90000, S 20000, C 4500, K 18000 | PD | — | **TEST_ONLY_REIMPLEMENTATION** (canonical-math tests — consistent with #8) |
| 19 | `order_domain_test.go:388-391` | order_domain_test.go:388-391 | order fixture | `expectedCommission` + `Subtotal+Shipping+Commission` | — | — | **TEST_ONLY_REIMPLEMENTATION** (asserts canonical copy + escrow formula) |
| 20 | `canonical_discount_snapshot_real_db_integration_test.go:196` | (test) | preview snapshot | uses `preview.PricingSnapshot.CommissionAmount` | — | — | **CONSUMER_OF_CANONICAL_VALUE** |
| 21 | `payment_coin_settlement_integration_test.go:532,983-984` | (test) | `CalculateGrossEscrowFromSnapshot` | 124500 | — | — | **TEST_ONLY_REIMPLEMENTATION** (uses derived helper) |
| 22 | `admin_order_handler.go:174,407` / `order_query_service.go:92,325,531,636` / `decision.go:288,726` / `notification_worker_shared.go:82` | DTO layers | read order.CommissionAmount | passthrough | — | — | **CONSUMER_OF_CANONICAL_VALUE** (serialization) |
| 23 | `projection_worker.go:452,530` / `projection/repository.go:77,152,189` | projection read model | read order.CommissionAmount | passthrough | — | — | **CONSUMER_OF_CANONICAL_VALUE** (projection) |
| 24 | `refund_gateway.go:409` | refund_gateway.go:409 | `pd = order.Subtotal − order.DiscountAmount` | denominator for ack-time breakdown | PD (discounted) | — | **CANONICAL_AUTHORITY** (consistent with #8) |
| 25 | `refund_gateway.go:436,495-501` | refund_gateway.go:436,495-501 | `proportionalFloor(cumProduct, C, pd)` | remainder commission | PD | floor | **CANONICAL_AUTHORITY** (consistent) |

---

# 7. DUPLICATE / CONFLICTING AUTHORITIES

## CONFLICTING (must eventually die)
1. **`verifierProportionalCommission`** (`verifier.go:722-727`) — denominator `orderGross = EscrowAmount = Subtotal+Shipping+Commission`; conflicts with canonical PD.
2. **Verifier call sites `:668-671`** — same conflicting denominator, used to validate real ledger entries.
3. **Verifier `orderGross` binding at `:639`** — uses `EscrowAmount` (commission-inclusive) as the "gross" for commission proportionality; this is the root of the divergence.

## DUPLICATE (benign but should converge)
4. **Two release posting functions with identical shape** (`RecordOrderRelease` `:187-234` and `RecordPartialRefundRelease` `:794-865`) — same DR GC / CR SP+PR shape; duplicated pattern, not a conflict.
5. **`LegacyGross()`** (`refund_policy.go:36`) duplicates `CalculateGrossEscrowFromSnapshot` under a different name; test-only usage.

## Consistent duplicates (NOT conflicts — same PD denominator)
6. `refund_math.go:11-12` doc comment + `refund_gateway.go:7-8` doc comment — same canonical formula restated twice; consistent.

---

# 8. 13600 VS 3600

## Why 13600 exists
`order.CommissionAmount` = `floor(PD × rate/100)` computed at pricing on the discounted product basis. In the observed order: PD = 90000 (Subtotal, no discount), rate ≈ 15.11% → 13600. It is the **total product commission identity** for the order, held inside the escrow gross (118600 = 90000 + 15000 + 13600). Source: `pricing_token_service.go:384`; escrow composition `canonical_finalization_service.go:129`.

## Why 3600 exists
`CommissionDelta` for a **partial refund event** = `floor(C × cumProductAfter / PD) − floor(C × cumProductBefore / PD)`. For a first partial refund reversing ~26.5% of the product, `floor(13600 × ~24000 / 90000) ≈ 3600`. Source: `refund_math.go:84-86`.

## Same identity or different derived values?
**Different.** 13600 is the canonical *identity* (total obligation). 3600 is a *derived per-event allocation* (proportional reversal). They are related by the canonical formula (3600 ≤ 13600, and 3600 = 13600 only for a full product refund minus prior partials).

## Where each originates
- 13600: pricing-time computation → token → order snapshot (CANONICAL_AUTHORITY chain, §2).
- 3600: refund-ack computation via `CalculateProportionalRefundBreakdown` (CANONICAL_AUTHORITY chain, §4).

## Is either incorrectly treated as the canonical commission identity?
- 13600: correctly treated as the identity. ✓
- 3600: **correctly treated as a derived allocation** in `refund_math.go` and `refund_gateway.go`. ✗ **BUT the verifier treats its own proportional result as the expected commission component** (`verifier.go:668-671`) using a *different denominator*, so the verifier's "expected" value for the same refund would diverge from the canonical 3600. The 3600 observed in the focused runtime is consistent with the canonical math, not with the verifier's formula (verifier would compute `amount × 13600 / 118600`).

## Conclusion
The 13600/3600 discrepancy is **not a data bug** — it is the canonical identity vs a derived allocation. The only place a *conflicting* 3600-like value can be produced is the **verifier** (`verifier.go:722-727`), whose denominator is commission-inclusive and therefore provably different.

---

# 9. VERIFIER DIVERGENCE

**Confirmed from source:**

- `verifier.go:639` — `orderGross := order.EscrowAmount` (= `Subtotal + ShippingTotal + CommissionAmount`).
- `verifier.go:668-669` — `expectedCommissionBefore/After := verifierProportionalCommission(prevRefunded, order.CommissionAmount, orderGross)`.
- `verifier.go:722-727` — `return (amount * orderCommission) / orderGross`.

**Canonical refund math:**

- `refund_math.go:35-139` — denominator `pd` (validated `pd > 0`), and at ack time `refund_gateway.go:409` binds `pd = order.Subtotal − order.DiscountAmount` (discounted, shipping-exclusive).

**Which is canonical?** Per locked business truth #8 ("Refund proportional allocation uses denominator = PD = Subtotal − Discount"), and because the refund pipeline (`refund_math.go` + `refund_gateway.go`) is the actual money path while the verifier is an offline forensic check, **the canonical authority is `refund_math.go` (PD denominator)**. The verifier's `orderGross` denominator is the **conflicting authority**.

**Numerical consequence:** For the observed order (PD=90000, Shipping=15000, C=13600):
- Canonical full-product delta: `floor(13600 × 90000/90000) = 13600`.
- Verifier full-product delta: `floor(13600 × 90000/118600) = 10320`.
- Canonical 26.5%-product delta: ~3600. Verifier same-amount delta: `floor(13600 × 24000/118600) = 2752`.
→ The verifier would flag a valid canonical refund as a mismatch (`after_release_platform_mismatch` / `seller_mismatch` at `:704-709`) whenever Shipping > 0.

---

# 10. TEST / RESIDUE AUDIT

| Location | What it does | Classification |
|---|---|---|
| `refund_math_test.go:8-13` (testPD/S/C/K) | Uses canonical constants with PD denominator, matches production `CalculateProportionalRefundBreakdown` | **TEST_ONLY_REIMPLEMENTATION** — consistent, keep |
| `refund_policy_test.go:14-18` (testOrder) | Fixture with Subtotal 100000 / Shipping 25000 / C 6250 | **TEST_ONLY_REIMPLEMENTATION** — fixture, keep |
| `refund_policy_test.go:245-255` | Explicitly pins "CommissionAmount only" refund behavior | **TEST_ONLY_REIMPLEMENTATION** — documents policy |
| `order_domain_test.go:388-391` | Asserts order.CommissionAmount copy + `Subtotal+Shipping+Commission` escrow | **TEST_ONLY_REIMPLEMENTATION** — consistent, keep |
| `shipping_proof_test.go:280-281,318-319` | Fixture C=5500, pct=5 | **TEST_ONLY_REIMPLEMENTATION** — fixture, keep |
| `canonical_discount_snapshot_real_db_integration_test.go:196` | Reads canonical snapshot value | **CONSUMER_OF_CANONICAL_VALUE** |
| `payment_coin_settlement_integration_test.go:532,983-984` | Asserts `CalculateGrossEscrowFromSnapshot` = 124500 | **TEST_ONLY_REIMPLEMENTATION** — uses derived helper, keep |
| `verifier_test` fixtures (`verifier.go:1190,1211`) | Hardcoded orders with EscrowAmount incl. commission | **TEST_ONLY_REIMPLEMENTATION** — feeds the conflicting verifier formula; must be revisited when verifier converges |

**No test in the refund pipeline reimplements commission with a conflicting denominator** — the only conflicting formula is in the production verifier itself (`verifier.go:722-727`).

---

# 11. ITEMS THAT MUST EVENTUALLY BE REMOVED

1. **`verifierProportionalCommission`** (`verifier.go:722-727`) — conflicting denominator formula. Must be replaced by a call into the canonical `CalculateProportionalRefundBreakdown` (or a shared primitive with PD denominator), or removed.
2. **Verifier commission-expected computation** (`verifier.go:668-671`) — depends on #1; must be rebased onto the canonical formula.
3. **Verifier `orderGross := order.EscrowAmount` for commission proportionality** (`verifier.go:639`) — commission-inclusive denominator source; must be replaced by `PD = Subtotal − Discount`.
4. **`OrderSnapshot.LegacyGross()`** (`refund_policy.go:36`) — legacy-named duplicate of `CalculateGrossEscrowFromSnapshot`; test-only usage. Remove once tests converge.
5. **Duplicate doc-comment formula restatement** (`refund_gateway.go:7-8`) — duplicate of `refund_math.go:11-12`; keep one canonical doc location (minor).
6. **Test fixtures tied to the conflicting verifier formula** (`verifier.go:1190,1211` in-`Test` snapshot data) — must be updated to canonical denominators when #1-#3 are fixed.

---

# 12. ITEMS THAT MUST NOT BE REMOVED

1. **`order.CommissionAmount`** (immutable identity) — `order.go:60`.
2. **`pricing_token_service.calculateCommission`** + `netSubtotal` basis (PD) — `:679-682`, `:384`.
3. **`refund_math.CalculateProportionalRefundBreakdown`** (canonical refund allocation, PD denominator) — `refund_math.go:35-139`.
4. **`proportionalFloor`** (canonical floor primitive) — `refund_math.go:141-146`.
5. **`refund_gateway.go:409` PD binding** (`pd = Subtotal − Discount`) — canonical at ack time.
6. **`RecordOrderRelease` / `RecordPartialRefundRelease` ledger postings** — the accounting contract (explicitly out of scope per §H; do NOT touch).
7. **`RecordRefundReversal` before/after-release branches** — the refund-reversal accounting contract (out of scope; the after-release branch is currently disabled by the ack gate `refund_gateway.go:451-454` — that gate is a separate accounting question).
8. **DTO/projection/notification serialization passthroughs** — consumers of the canonical value.

---

# 13. OPEN QUESTIONS

1. **Verifier intent:** is `verifier.go` meant to validate the *ledger posting* (in which case its expected-commission must exactly match what `RecordRefundReversal` posts — i.e., the canonical `CommissionDelta` with PD denominator), or is it a *forensic* check with its own model? Current code does the latter, which is the conflict.
2. **`orderGross` semantics elsewhere in verifier:** `:739` uses `orderGrossByID[o.ID] = o.Subtotal + o.ShippingTotal` (shipping-inclusive, commission-EXCLUSIVE) for dispute-freeze checks — a **third** denominator variant. This is a separate concern (freeze vs gross) but shows the verifier is internally inconsistent about what "gross" means.
3. **The 3600-value provenance:** the focused runtime's exact fixture (90000/15000/13600/3600/25000/10000) was not found verbatim in committed tests I read; its arithmetic is consistent with the canonical formulas, but the specific commission rate (≈15.11%) is not evidenced by a committed fixture — it presumably comes from a platform_configs seed value or a live-db run. This is a data-level, not code-level, observation.
4. **Discount in the verifier:** the verifier's `Order` snapshot (loaded at `:1025`) does **not** appear to load `discount_amount`, so it cannot compute `PD = Subtotal − Discount` without a schema/query change. This is a prerequisite for converging the verifier onto the canonical denominator.

---

# 14. IMPLEMENTATION BOUNDARY

- **NO production code, tests, migrations, schema, ledger, refund, accounting, pricing, payment, or escrow behavior was or may be modified in this audit.**
- **NO implementation is proposed.** The authority map is complete (§6), the conflict is identified (§9), and the removal list is enumerated (§11). Any future implementation must:
  - converge the verifier onto the canonical PD denominator (or delete the verifier's commission reimplementation), and
  - NOT touch the ledger/accounting contract (per §H of the task: account funding, GATEWAY_CLEARING debit/credit, PLATFORM_REVENUE reversal, and escrow funding are separate questions).
- The canonical formula must remain: `CommissionAmount = floor(PD × rate / 100)` with `PD = Subtotal − Discount`, product-only, shipping commission-free.

---

# 15. FINAL CLEANUP STATUS

| Item | Status |
|---|---|
| Canonical commission identity established | `order.CommissionAmount` (immutable, product-only) |
| Canonical computation formula | `floor(PD × rate/100)`, PD = Subtotal − Discount |
| Canonical refund allocation | `refund_math.go` (PD denominator) |
| Conflicting authority found | `verifier.go:722-727` + `:668-671` + `:639` (EscrowAmount denominator) |
| Duplicate authorities | `LegacyGross()` (test-only); duplicate release-postings (benign) |
| Test-only reimplementations | consistent with canonical math (keep); verifier fixtures must follow convergence |
| Cleanup performed | **NONE** (read-only; list only) |
| Implementation authorized | **NO** |

---

*This audit is intentionally read-only. It establishes ONE canonical commission truth and enumerates every representation that must eventually be removed so future agents/developers cannot accidentally resurrect the rejected commission model.*
