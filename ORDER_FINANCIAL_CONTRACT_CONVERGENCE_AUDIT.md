# ORDER_FINANCIAL_CONTRACT_CONVERGENCE_AUDIT

Read-only authority/convergence audit. No production code, tests, migrations, schema, ledger, refund, payment, pricing, or accounting behavior modified. Nothing deleted. No fix implemented.

```
VERDICT: ORDER_SNAPSHOT_AUTHORITY_CONTRADICTION_FOUND
```

STOP: the authority map is not convergent. Per the instructions, this pass stops at establishing the ONE truth; no implementation is proposed.

---

## 1. CANONICAL FINANCIAL CONTRACT (established from locked evidence)

The canonical order financial contract, proven by the pricing-token producer, the payment-intent/coin-settlement integration tests (which encode actual runtime cash flow), the `loadOrderPricingTokenSnapshot` guard, and the refund-policy/refund-math contracts:

| Symbol | Canonical definition | Locked by |
|---|---|---|
| P (Subtotal) | `quantity × unit_price` | `pricing_token_service.go:339`; `NewOrderFromSource` |
| D (DiscountAmount) | validated discount at checkout | `pricing_token_service.go:351-376`; `discount_service.go` |
| PD (ProductSubtotalAfterDiscount) | `P − D` | `pricing_token_service.go:385` (`discountedProduct`), :424 (`orderValueForCoins`); `refund_math.go` (pd) |
| S (ShippingTotal) | coverage/quote cost | `pricing_token_service.go:265-335` |
| C (CommissionAmount) | `floor(PD × rate / 100)` — product-only, seller-side | `pricing_token_service.go:381-384, 680-683`; `refund_policy.go:30-33` |
| K (CoinsUsed) | buyer-chosen coins capped by `MaxCoinsAllowedForDiscountedProduct(PD)` | `token:425-426`; `dependencies.go:3285-3293` |
| TotalBeforeCoinsAmount | **`(P−D)+S`** (buyer funding base before coins and fee) | `payment_intent_verification_integration_test.go:42-43, 615, 655, 676, 712`; `dependencies.go:3128-3131` guard |
| TotalPayableAmount | `(P−D)+S − K + F` (F = method fee on cash) | `dependencies.go:3295-3306`; test:631-633 |
| EscrowAmount (contract) | **`(P−D)+S`** — equals the buyer cash base before coins/fee; the amount actually funded into GATEWAY_CLEARING is `(P−D)+S−K+F` | `payment_intent_verification_integration_test.go:529` (token EscrowAmount=110000 = base); `dependencies.go:3295-3306` |

### The one numeric truth (from the passing runtime fixtures)

`P=100000, D=10000, PD=90000, S=20000, C=4500, F=4000, K=18000`:
- `(P−D)+S = 110000` = `TotalBeforeCoins` = token `EscrowAmount` = `ListPaymentMethods.baseAmount` = `CreatePayment` base = verified by `loadOrderPricingTokenSnapshot` (`OrderValueForCoins + ShippingTotal = 90000+20000`).
- cash = `110000 − 18000 = 92000`; gross to gateway = `92000 + 4000 = 96000`; zero-K gross = `114000`.
- `P+S+C = 124500` — **nowhere in the runtime cash flow**.

**The contract is `EscrowAmount = (P−D)+S`, not `P+S+C`, and not `P+S+C−D`.**

---

## 2. FIELD AUTHORITY MATRIX

| Field | Canonical authority | Created at | Persisted where (production) | Actually persisted? | Immutable? | Classification |
|---|---|---|---|---|---|---|
| Subtotal | `PricingTokenService` | token gen | `pricing_tokens.subtotal`, `orders.subtotal` | ✅ | Yes | CANONICAL_AUTHORITY |
| DiscountAmount | `DiscountService.ApplyDiscountAtCheckout` | token gen | `pricing_tokens.discount_amount` | ✅ token / ❌ **orders** (0) | Yes | CANONICAL_AUTHORITY (token) + **CONFLICTING_AUTHORITY (orders column dead)** |
| PD (P−D) | derived | token gen | derived (not stored; `pricing_tokens.order_value_for_coins`) | ✅ (as order_value_for_coins) | — | DERIVED_VALUE (canonical formula) |
| ShippingTotal | `PricingTokenService` | token gen | `pricing_tokens.shipping_total`, `orders.shipping_total` | ✅ | Yes | CANONICAL_AUTHORITY |
| CommissionAmount | `calculateCommission(PD, rate)` | token gen | `pricing_tokens.commission_amount`, `orders.commission_amount` | ✅ | Yes | CANONICAL_AUTHORITY |
| CoinsUsed | buyer intent + coins domain cap | payment creation | `payments.coins_to_use`, `coin_reservations`, `coins_transactions`; **NOT orders** | ❌ orders always 0 | — | CANONICAL_AUTHORITY (coins domain) + **orders.coins_used = LEGACY/ZOMBIE/INCORRECT** |
| TotalBeforeCoinsAmount | `(P−D)+S` | order creation | `orders.total_before_coins_amount` | ✅ | Yes (de facto; guard prevents restamp) | CANONICAL_AUTHORITY (as persisted) |
| TotalPayableAmount | `(P−D)+S−K+F` | payment creation (fee applied later) | `orders.total_payable_amount` (initially `(P−D)+S`, later fee-inclusive) | ✅ | Mutable (fee stamp) | CANONICAL_AUTHORITY (mutable by design) |
| EscrowAmount | **`(P−D)+S`** (contract) | token gen | `pricing_tokens.escrow_amount`; **`orders.escrow_amount` never written (0)**; `escrows.amount` = `P+S+C` (conflicting) | ❌ orders | — | CONFLICTING_AUTHORITY (see §4) |

---

## 3. FORMULA MATRIX

| # | Formula | Symbol | Where | Classification |
|---|---|---|---|---|
| 1 | `P = qty × unit_price` | Subtotal | `pricing_token_service.go:339` | CANONICAL |
| 2 | `D` from discount service | Discount | `pricing_token_service.go:351-376` | CANONICAL |
| 3 | `PD = P − D` | ProductSubtotalAfterDiscount | `pricing_token_service.go:385,424`; `refund_math.go` | CANONICAL |
| 4 | `C = floor(PD × rate / 100)` | CommissionAmount | `pricing_token_service.go:381-384, 680-683` | CANONICAL |
| 5 | `TotalBeforeCoins = (P−D)+S` | buyer base | fixture test:529; guard dependencies.go:3128-3131 | CANONICAL |
| 6 | `Cash = (P−D)+S − K`; `Gross = Cash + F` | payment cash | `dependencies.go:3295-3306`; test:631-633 | CANONICAL |
| 7 | `EscrowAmount(token) = escrowBase − D` where `escrowBase = P+S+C` → **P+S+C−D** | token escrow | `pricing_token_service.go:404-408` | **CONFLICTING_AUTHORITY** (differs from (P−D)+S by C) |
| 8 | `EscrowGross = P+S+C` | escrow creation/release | `pricing_helper.go:30-32`; `canonical_finalization_service.go:117`; `order_payment_service.go:266-270`; `buildOrderPayload:1722-1742` | **CONFLICTING_AUTHORITY** |
| 9 | `escrow_amount = baseAmount = (P−D)+S` (API) | ListPaymentMethods | `dependencies.go:3643` | CANONICAL (matches contract) |
| 10 | `escrow = subtotal + shipping` (comment/DTO) | dispute handler | `dispute_handler.go:380` | CONFLICTING (undiscounted) |
| 11 | `CashRefund = Rpd + Rs − CoinDelta`; `CoinDelta = floor(K·cumA/PD) − floor(K·cumB/PD)`; `CommissionDelta = floor(C·cumA/PD) − floor(C·cumB/PD)`; `SellerComponent = Rpd+Rs−CommissionDelta` | refund math | `refund_math.go:83-118`; `refund_gateway.go:1-10` | CANONICAL (pure math) |
| 12 | `pd = order.Subtotal` (undiscounted) | gateway ack / refund init | `refund_gateway.go:326`; `order_payment_service.go:165` | **CONFLICTING_AUTHORITY** (drops D) |
| 13 | `orderGross = order.EscrowAmount` (0) | verifier | `verifier.go:640` | **CONFLICTING_AUTHORITY / INCORRECT** |
| 14 | `pd = Subtotal − orders.discount_amount` (0 → P) | verifier | `verifier.go:676` | **INCORRECT** (reads dead column) |
| 15 | `ProductGross = PD+S` (with PD from snapshot) | refund policy | `refund_policy.go:35` | CANONICAL (given correct PD input) |
| 16 | `LegacyGross = P+S+C` | refund policy (unused in money path) | `refund_policy.go:36` | LEGACY/ZOMBIE |

---

## 4. ESCROW IDENTITY — WHICH MODEL IS CANONICAL (Question: "Determine the correct canonical escrow identity")

Evidence chain:

1. **Buyer cash flow (locked by runtime tests)**: base `(P−D)+S = 110000`; gross to gateway `(P−D)+S−K+F`. The buyer never pays C. `payment_intent_verification_integration_test.go:631-633, 696-697`.
2. **`loadOrderPricingTokenSnapshot` guard**: `TotalBeforeCoinsAmount == OrderValueForCoins + ShippingTotal` = `PD + S`. `dependencies.go:3128-3131`.
3. **Token fixture**: `EscrowAmount = 110000 = (P−D)+S` while `CommissionAmount = 4500` is separate. `payment_intent_verification_integration_test.go:519-547`.
4. **`ReleaseGatewayEscrowToSeller`**: `gross = P+S+C`; `sellerNet = P+S`; `RecordOrderRelease` requires `sellerNet + commission == gross`. `order_payment_service.go:266-270`; `finance_service.go:199-201`.
5. **Cash-flow consistency check**: gateway clearing inflow = `gross(cash) = (P−D)+S−K+F` (finance_service.go:323-398). Release outflow = `P+S+C`. For the fixture: inflow 96000 (with K=18000) or 114000 (K=0); outflow 124500. **The release debits `P+S+C` from GATEWAY_CLEARING but only `(P−D)+S−K+F` was ever funded.** At K=0: `114000 funded vs 124500 released` — the clearing account is debited **10,500 more than funded** (the discount 10,000 plus 500 = C−D? No: `124500−114000 = 10500 = C + D − F? = 4500+10000−4000 = 10500`). The unfunded portion equals `C + D − F` — commission and discount were never paid by the buyer.
6. **Economic truth**: `EscrowAmount = (P−D)+S` is the **buyer-funded cash** (before coins/fee). `C` is a **seller/platform allocation** that must be *carved out of* seller proceeds, not added to the buyer's obligation. The `P+S+C` model treats C as buyer-funded — economically impossible under the actual payment flow.

**Correct canonical escrow identity (proven, not implemented):**
```
EscrowGross (buyer-funded) = (P−D)+S          [= TotalBeforeCoins]
Cash to gateway           = (P−D)+S − K + F  [gross, payments.gross_amount]
Seller release            = (P−D)+S − CommissionDelta(proportional)   [out of the funded pool]
Platform revenue          = C (carved from seller release) + F
Refund (buyer)            = CashRefund = Rpd+Rs−CoinDelta  [≤ (P−D)+S−K]
```
`P+S+C` must be killed. `P+S+C−D` (token producer line 408) must be killed.

---

## 5. COMMISSION IDENTITY

- **Canonical**: `C = floor((P−D)·rate/100)` — commission on the discounted product value only. `pricing_token_service.go:381-384, 680-683`.
- **Buyer-funded or seller/platform allocation?** **Seller/platform allocation.** The buyer's cash flow (`(P−D)+S−K+F`) contains no C. The ledger realizes C from the seller side at release: `RecordOrderRelease` moves `commission` out of GATEWAY_CLEARING to PLATFORM_REVENUE and credits the seller only `sellerNet` (`finance_service.go:220-224`). The refund policy states C is "seller-side, NOT buyer refund" (`refund_policy.go:30-33`).
- **Product-only**: no shipping commission; shipping has no commission component (`refund_math.go:5-22`).
- **Misrepresentations to kill**: `ReleaseGatewayEscrowToSeller` gross = `P+S+C` (order_payment_service.go:266-270) and `CalculateGrossEscrowFromSnapshot` (pricing_helper.go:30-32) both treat C as part of the buyer-funded escrow gross.

---

## 6. PRODUCER / WRITER / READER MAP

### Producers (writers)
| Field | Producer | File:lines |
|---|---|---|
| P, S, C, D, Escrow(token), TotalPayable, OrderValueForCoins | `PricingTokenService.Generate*` | `pricing/token/application/pricing_token_service.go:211-516, 846-1065, 1099-1378` |
| pricing_tokens row | `PricingTokenRepositoryImpl.CreateTx` | `token/infrastructure/repository/pricing_token_repository_impl.go:54-116` |
| orders row (P, S, C, TotalPayable, TotalBeforeCoins, coins_used=0, coin_discount=0; **drops D, Escrow**) | `OrderRepository.CreateOrderTx` | `commerce/order/infrastructure/repository/order_repository.go:56-121` |
| orders.total_payable += fee | `UpdatePaymentSelectionTx` | `order_repository.go:932-954` |
| payments.gross_amount, coins_to_use | `CorePaymentHandler.CreatePayment` | `serverboot/dependencies.go:3322-3401` |
| escrows.amount = P+S+C | `CanonicalFinalizationService.FinalizeOrderPayment` | `integration/payment/application/canonical_finalization_service.go:117-126` |
| ledger (settlement/release/refund) | `FinanceService.Record*` | `finance/application/finance_service.go:187-234, 323-398, 570-761, 794-866` |

### Readers (consumers)
| Reader | Field(s) read | File:lines | Correct? |
|---|---|---|---|
| `loadOrderPricingTokenSnapshot` | TotalBeforeCoins vs token PD+S | `dependencies.go:3128-3131` | ✅ |
| `CreatePayment` | base = TotalBeforeCoins; gross | `dependencies.go:3269-3310` | ✅ |
| `ListPaymentMethods` | base = TotalBeforeCoins | `dependencies.go:3590-3646` | ✅ |
| `ReleaseGatewayEscrowToSeller` | P, S, C → gross = P+S+C | `order_payment_service.go:266-270` | ❌ (unfunded) |
| `CanonicalFinalizationService` | P, S, C → escrow = P+S+C | `canonical_finalization_service.go:117` | ❌ |
| `PartialRefundFromDispute` | P, S, C == escrow validation | `order_completion_service.go:1733-1745` | ❌ |
| `refund_gateway.go` ack | pd = Subtotal; kVal = CoinsUsed(0) | `refund_gateway.go:326-329` | ❌ |
| `order_payment_service.go` refund init | pd = Subtotal | `order_payment_service.go:165` | ❌ |
| `refund_service.go` CreateRefund | cap = P+S | `refund_service.go:189` | ❌ (undiscounted) |
| verifier | orders.discount_amount(0), escrow_amount(0) | `verifier.go:640, 672-684, 1024-1049` | ❌ |
| escrow_integrity_checker | orders.escrow_amount vs escrows.amount | `escrow_integrity_checker.go:157-191` | ❌ |
| projection worker | orders.escrow_amount → order_summaries | `worker/projection_worker.go:440, 518, 785` | ❌ (propagates 0) |
| recon classifier D15 | Order.GrossAmount > 0 | `recon/classifier.go:436-457` | ❌ (blind spot) |
| DTO/API | TotalBeforeCoins, TotalPayable, CommissionAmount | `order/entity/order.go:56-62`; `dto/decision.go:282-293`; `admin_order_handler.go:326-418` | ✅ (omits D/Escrow — acceptable) |
| pricing token API | escrow_amount = (P−D)+S | `pricing/token/delivery/http/pricing_token_handler.go:389` | ✅ (contract) |

---

## 7. CASH-FLOW RECONCILIATION (fixture numbers, P=100000, D=10000, S=20000, C=4500, F=4000, K=18000)

| Stage | Amount | Formula | Source |
|---|---|---|---|
| Token escrow (contract) | 110,000 | (P−D)+S | token fixture:529 |
| Token escrow (producer code) | 114,500 | P+S+C−D | `pricing_token_service.go:404-408` |
| ListPaymentMethods base | 110,000 | TotalBeforeCoins | test:619 |
| Cash (K=18000) | 92,000 | 110000−18000 | test:631 |
| Buyer fee F | 4,000 | CalculateFee(cash) | test:632 |
| Gateway gross (K=18000) | 96,000 | cash+F | test:633, 638-640 |
| Gateway gross (K=0) | 114,000 | 110000+4000 | test:696-697 |
| GATEWAY_CLEARING inflow | 96,000 / 114,000 | RecordGatewayPaymentSettlement | finance_service.go:365-367 |
| Escrow row created | **124,500** | P+S+C via CalculateGrossEscrowFromSnapshot | canonical_finalization_service.go:117-126 |
| Release debit from clearing | **124,500** | P+S+C | order_payment_service.go:266-270 |
| Seller net | 120,000 | P+S | order_payment_service.go:269 |
| Commission to platform | 4,500 | C | order_payment_service.go:268 |
| **GAP (K=0)** | **10,500** | release 124,500 − funded 114,000 = C+D−F = 4500+10000−4000 | arithmetic |

The gap `C+D−F` is real whenever commission+discount exceed the fee. **The release debits GATEWAY_CLEARING for components the buyer never funded.** This is the primary financial contradiction.

---

## 8. ESCROW FUNDING INVARIANT (canonical)

```
GATEWAY_CLEARING inflow  = (P−D)+S − K + F        (RecordGatewayPaymentSettlement)
GATEWAY_CLEARING outflow = (P−D)+S − K + F        (must equal inflow for Σ=0)
  → Release:   sellerNet + commission = (P−D)+S − K (+F carved to revenue)
  → Refund:    CashRefund = Rpd+Rs−CoinDelta ≤ (P−D)+S−K
  → Remainder: (P−D)+S − K − cumulativeRefund
Σ(sellerNet + commission + refund) = (P−D)+S − K + F
```
The current implementation violates it: `ReleaseGatewayEscrowToSeller` uses `gross = P+S+C`, so the ledger debits `P+S+C` from a pool funded with `(P−D)+S−K+F`. **The escrow row must carry `(P−D)+S`** (the buyer-funded obligation) — not `P+S+C`.

---

## 9. REFUND FUNDING INVARIANT (canonical)

| Component | Economic meaning | Funding account | Current implementation |
|---|---|---|---|
| CashRefund (`Rpd+Rs−CoinDelta`) | buyer's cash back | GATEWAY_CLEARING (before release); SELLER_PAYABLE+PLATFORM_REVENUE (after release) | ✅ `finance_service.go:679-683, 649-653` |
| CoinDelta | coins restored, not cash | coin subsystem (`coins_transactions` earn) | ✅ `coins_refund_handler.go:196-252`; ack emits delta `refund_gateway.go:451-457` |
| CommissionDelta | platform forfeits its carve on refunded product | PLATFORM_REVENUE reversal | ✅ `finance_service.go:652`; verifier:681-684 |
| SellerComponent (`Rpd+Rs−CommissionDelta`) | seller gives up economic value | SELLER_PAYABLE reversal | ✅ `finance_service.go:651` |
| RemainingObligation | `(P−D)+S − cumulativeRefund` | GATEWAY_CLEARING → SELLER_PAYABLE + PLATFORM_REVENUE | ✅ `RecordPartialRefundRelease` (`finance_service.go:842-846`) |
| **Denominator PD** | must be `P−D` | — | ❌ ack uses `pd = order.Subtotal` (`refund_gateway.go:326`); verifier reads dead column |

The refund math itself is canonical (`refund_math.go`); the funding *inputs* are corrupted upstream by the undiscounted `pd` and the never-persisted `K`.

---

## 10. CONFLICTING MODELS (each must be killed)

| # | Model | Where | Evidence it conflicts |
|---|---|---|---|
| M1 | `EscrowAmount = P+S+C` | `pricing_helper.go:30-32`; `canonical_finalization_service.go:117-126`; `order_payment_service.go:266-270`; `order_completion_service.go:1739-1744`; `buildOrderPayload:1722-1742` | unfunded clearing debit (gap §7) |
| M2 | `EscrowAmount = P+S+C−D` | `pricing_token_service.go:404-408` | differs from contract (P−D)+S by C; test:117-120 forbids it |
| M3 | `pd = Subtotal` (undiscounted) | `refund_gateway.go:326`; `order_payment_service.go:165` | drops D; verifier & math expect PD=P−D |
| M4 | `orders.escrow_amount` as authority | verifier:640; escrow checker; projection; recon | never written (0) |
| M5 | `orders.discount_amount` as authority | verifier:672-676 | never written (0) |
| M6 | `orders.coins_used` as authority | `refund_gateway.go:329`; completion service coin gates | never written (0) |
| M7 | DB `orders_check: refunded_amount <= escrow_amount` | migration:2473 | escrow_amount=0 blocks real refunds |
| M8 | `escrow = subtotal+shipping` (undiscounted) | `dispute_handler.go:380` | drops D |
| M9 | `LegacyGross = P+S+C` | `refund_policy.go:36` | rejected model (unused in money path) |
| M10 | hand-written test INSERTs that write D/Escrow | `canonical_pricing_snapshot_persistence_test.go:42-71`; `admin_refund_real_db_proof_integration_test.go:47` | bypass production persistence |
| M11 | non-compiling integration "proofs" | `canonical_discount_snapshot_real_db_integration_test.go`; `admin_refund_real_db_proof_integration_test.go`; `refund_service_real_db_proof_integration_test.go` | reference removed symbols (see §13) |

---

## 11. PERSISTENCE PROOF — THE THREE DEAD COLUMNS

| Column | Production writer? | Production reader relying on it? | Conclusion |
|---|---|---|---|
| `orders.discount_amount` | none (`CreateOrderTx` omits; `order.go` has no field) | verifier:672-676; refund via pd | **Not a genuine required authority** — the discount truth lives in `pricing_tokens.discount_amount` + `pricing_tokens.order_value_for_coins` (PD). Either it must be written from the token or consumers must read the token. Current state = dead column + INCORRECT consumers. |
| `orders.coins_used` | none (always 0) | `refund_gateway.go:329`; completion service gates | **Not required on orders** — coin truth lives in `payments.coins_to_use` + `coin_reservations` + `coins_transactions`. The column is display-only residue; consumers must read coins domain. |
| `orders.escrow_amount` | none | verifier:640; escrow checker; projection; recon; DB constraint | **Not required as stored** — the funded obligation is `escrows.amount` (wallet domain) and the contract value is derivable as `(P−D)+S` from the snapshot. The column is a dead 0 that actively breaks refunds via the CHECK constraint. |

**Decision (evidence-based, not convenience)**: these three columns should be **replaced by the canonical snapshot representation** — the pricing token (`pricing_tokens` row, which already carries P, D, PD(order_value_for_coins), S, C, Escrow, TotalPayable) plus `escrows.amount` for funded truth. The orders row should persist only what is genuinely consumed and funded: P, S, C, `total_before_coins_amount = (P−D)+S`, `total_payable_amount`. Consumers (verifier, escrow checker, refund, recon) must be rewired to the token/escrow/ledger instead of the dead columns. **This is the authority map; the implementation pass will kill M4-M7 accordingly.**

---

## 12. KILL LIST

1. `CalculateGrossEscrowFromSnapshot` (P+S+C) — `finance/application/pricing_helper.go:8-32`
2. `escrowAmount := escrowBase.Sub(discountAmount)` (P+S+C−D) — `pricing_token_service.go:404-408`
3. `ReleaseGatewayEscrowToSeller` gross = P+S+C — `order_payment_service.go:266-270`
4. `FinalizeOrderPayment` escrow amount = P+S+C — `canonical_finalization_service.go:117-126`
5. `PartialRefundFromDispute` P+S+C==escrow validation — `order_completion_service.go:1733-1745`
6. `buildOrderPayload` escrow = P+S+C — `order_creation_service.go:1722-1742`
7. `refund_gateway.go:326` `pd := order.Subtotal` → must be PD
8. `order_payment_service.go:165` `pd := order.Subtotal` → must be PD
9. `refund_service.go:189` cap = P+S → must be (P−D)+S
10. `refund_gateway.go:329` `kVal := order.CoinsUsed` → coins domain
11. verifier reads of `orders.discount_amount`/`orders.escrow_amount` — `verifier.go:640, 672-684, 1024-1049`
12. escrow_integrity_checker per-order/global vs orders.escrow_amount — `escrow_integrity_checker.go:157-191, 201-266`
13. projection `orders.escrow_amount` → order_summaries — `worker/projection_worker.go:440, 518, 785`; `projection/repository.go:84, 110-112, 137-140`
14. recon D15 `Order.GrossAmount > 0` gate — `recon/classifier.go:436-457`
15. DB `orders_check` (refunded ≤ escrow_amount) — migration:2473
16. `orders.discount_amount`, `orders.escrow_amount`, `orders.coins_used` as order-snapshot authorities
17. `LegacyGross` P+S+C — `refund_policy.go:36`
18. `dispute_handler.go:380` escrow=subtotal+shipping comment/DTO
19. Hand-written INSERT tests bypassing `CreateOrderTx` — `canonical_pricing_snapshot_persistence_test.go:24-311`; `admin_refund_real_db_proof_integration_test.go:47`
20. Non-compiling integration "proof" tests referencing removed symbols — `canonical_discount_snapshot_real_db_integration_test.go`, `admin_refund_real_db_proof_integration_test.go`, `refund_service_real_db_proof_integration_test.go`

## 13. PROTECTED LIST (must NOT be deleted)

1. `PricingTokenService.GenerateForFixedPriceSale/Negotiation/Auction` + `calculateCommission` — sole pricing authority
2. `pricing_tokens` table + `PricingToken` entity + repo round-trip — complete snapshot truth
3. `ValidateForOrderLocked` + `FinalizeOrderConsumption` — order gate + atomic token consumption
4. `MaxCoinsAllowedForDiscountedProduct` — canonical coins cap on PD
5. `orders.total_before_coins_amount` = `(P−D)+S` + `CreatePayment`/`ListPaymentMethods` baseAmount — canonical buyer base
6. `UpdatePaymentSelectionTx` not touching total_before_coins — immutability guard (payment_method_default_killed_test.go:183-190)
7. `refund_math.go` `CalculateProportionalRefundBreakdown` + `proportionalFloor` — canonical refund math
8. `refund_policy.go` `ResolveRefundPolicy` + `ProductGross` — canonical policy (given correct PD input)
9. `RecordGatewayPaymentSettlement` / `RecordBuyerPaymentFeeRevenue` / `RecordOrderRelease` / `RecordRefundReversal` / `RecordPartialRefundRelease` — canonical ledger movements
10. `WalletService.CreateEscrowFromGatewaySettlement` / Release / Refund / PartialRefund + `escrows.amount` — funded obligation truth
11. `payments.coins_to_use` / `coin_discount_amount` + `coin_reservations` + `coins_transactions` — canonical coins truth
12. `refunds.refunded_product_amount` / `refunded_shipping_amount` / `coins_refunded_amount` / `final_refund_amount` — event split stamping
13. `verifierProportionalCommissionPD` — correct expectation given correct PD
14. `SyncRefundSettlementFromGatewayAck` — ack status sync
15. `pricing/token/delivery/http/pricing_token_handler.go:389` escrow_amount = token value — contract surface

## 14. UNKNOWN / UNPROVEN ITEMS

| Item | Status |
|---|---|
| Exact seeded `listing_commission_percent` in a live deployed DB | Migration seeds 4 (migration:2531-2532); runtime value could differ per environment — **UNKNOWN** without DB access (DB not touched per constraints) |
| Whether `P+S+C` escrow creation ever actually produced a negative GATEWAY_CLEARING balance in production | Possible if funded gross < released gross (gap §7); requires ledger inspection — **UNPROVEN** here |
| Whether any historical order predates the `(P−D)+S` contract and carries `total_before_coins_amount = P+S+C` | **UNKNOWN** (no DB query allowed) |
| Whether the buyer-payment-fee (F) is ever included in `escrows.amount` | `CreateEscrowFromGatewaySettlement` uses P+S+C only (no F) — consistent; but reconciliation of F against clearing relies on `RecordBuyerPaymentFeeRevenue` — **UNPROVEN** end-to-end |
| Whether `orders.coins_used` was ever populated by an older release | Comment at `coins_refund_handler.go:130` says "can be stale / not persisted" — **UNKNOWN** historical |

## 15. EXACT COMMANDS + EXIT CODES

| Command | Exit | Purpose |
|---|---|---|
| `git status --short` (repo root) | 0 | Confirm audit-referenced files untracked; verifier.go modified |
| `go vet ./internal/finance/refund/application/` | 0 | Production package compiles (integration tag excluded) |
| `go test -run XXX -count=1 ./internal/finance/refund/application/` | 0 | No tests run (integration tag excluded) |
| `go test -tags integration -run XXX -count=1 ./internal/finance/refund/application/` | 1 | Build failed: `InitiateAdminGatewayRefund`/`InitiateAdminGatewayRefundInput` undefined; `midtrans.RefundTarget` undefined; `CountSuccessfulPaymentsForOrder`/`FindSuccessfulPaymentForOrder` undefined |
| `go list -f "{{.GoFiles}}" ./internal/finance/refund/application/` | 0 | Prod files: `refund_gateway.go refund_math.go refund_service.go` |

## 16. KEY FILE PATHS / SYMBOLS / LINES

- `backend/internal/pricing/token/application/pricing_token_service.go:211-516, 381-415, 424-426, 680-683`
- `backend/internal/pricing/token/entity/pricing_token.go:26-94`
- `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go:54-116, 119-232`
- `backend/internal/commerce/order/application/order_creation_service.go:1151-1233, 1722-1742`
- `backend/internal/commerce/order/entity/order.go:17-184, 1000-1128`
- `backend/internal/commerce/order/infrastructure/repository/order_repository.go:29-139, 142-287, 932-954`
- `backend/internal/commerce/order/application/order_payment_service.go:146-184, 257-294`
- `backend/internal/commerce/order/application/order_completion_service.go:373-559, 835-992, 1240-1395, 1418-1560, 1699-1801`
- `backend/internal/integration/payment/application/canonical_finalization_service.go:61-140`
- `backend/internal/finance/application/pricing_helper.go:8-32`
- `backend/internal/finance/application/finance_service.go:187-234, 323-398, 426-475, 570-761, 794-866`
- `backend/internal/finance/refund/application/refund_math.go:35-146`
- `backend/internal/finance/refund/application/refund_gateway.go:77-142, 178-264, 266-476`
- `backend/internal/finance/refund/application/refund_service.go:124-240, 392-484, 494-519`
- `backend/internal/finance/refund/entity/refund_policy.go:14-61`
- `backend/internal/finance/verifier/verifier.go:56-69, 454-534, 592-726, 735-746, 1024-1050`
- `backend/internal/core/wallet/application/wallet_service.go:313-400, 464-517, 537-585, 610-680`
- `backend/internal/core/wallet/application/escrow_integrity_checker.go:26-34, 85-124, 157-191, 201-266`
- `backend/internal/core/wallet/entity/escrow.go:56-67`
- `backend/internal/worker/projection_worker.go:440-453, 518-531, 766-815`
- `backend/internal/worker/coins_refund_handler.go:119-130, 196-280`
- `backend/internal/projection/repository.go:84, 110-112, 137-140, 177-190`
- `backend/internal/integration/payment/application/recon/classifier.go:431-457`
- `backend/internal/serveboot/dependencies.go:3128-3131, 3269-3310, 3590-3646` (serverboot)
- `backend/internal/governance/dispute/delivery/http/dispute_handler.go:380`
- `backend/internal/commerce/order/delivery/http/dto/decision.go:247-356`
- `backend/internal/commerce/order/delivery/http/admin_order_handler.go:315-434`
- `backend/migrations/000001_canonical_schema.up.sql:1086-1143, 1250-1290, 2471-2481, 2531-2532`
- Runtime fixtures: `serverboot/payment_intent_verification_integration_test.go:42-44, 508-605, 607-727`; `serverboot/payment_coin_settlement_integration_test.go:275-332`
- Test fixtures (bypass/compile-fail): `commerce/order/tests/canonical_pricing_snapshot_persistence_test.go:24-311`; `finance/refund/application/canonical_discount_snapshot_real_db_integration_test.go:127-232, 283-344`; `admin_refund_real_db_proof_integration_test.go:36-101`; `refund_service_real_db_proof_integration_test.go:37-97`

## 17. FILES / DATABASE CHANGED

- **FILES_CHANGED: NONE** except this designated report `ORDER_FINANCIAL_CONTRACT_CONVERGENCE_AUDIT.md`.
- **DATABASE_CHANGED: NONE** (no DB process started; only `git`, `go vet`, `go test -run XXX`, `go list`; the `-tags integration` run failed at compile before any test could execute).

---

## 18. FINAL STOP STATEMENT

A contradiction remains in the canonical order financial snapshot: the buyer-funded contract is `EscrowAmount = (P−D)+S` (proven by runtime payment fixtures, the pricing-token guard, and the actual gateway cash flow), while escrow creation and release implement `P+S+C`, the token producer computes `P+S+C−D`, and multiple consumers read never-persisted `orders.discount_amount` / `orders.escrow_amount` / `orders.coins_used` as authority. The release path debits GATEWAY_CLEARING for `C+D−F` that the buyer never funded.

Per the STOP RULE, this audit stops here. The ONE truth is established above; the kill list (§12) and protected list (§13) are complete. No implementation is proposed and no code was changed.
