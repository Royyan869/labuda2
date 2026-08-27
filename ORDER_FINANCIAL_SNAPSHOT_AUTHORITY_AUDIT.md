# ORDER FINANCIAL SNAPSHOT AUTHORITY AUDIT

Evidence-only audit. No production code, tests, migrations, ledger, refund, payment, or accounting behavior modified. Nothing deleted. No fix implemented.

```
VERDICT: ORDER_SNAPSHOT_AUTHORITY_CONTRADICTION_FOUND
```

STOP RULE TRIGGERED: A contradiction remains in the canonical order financial snapshot. Per instructions, this pass stops at establishing the ONE truth — no fix, no cleanup, no behavior change.

---

## 1. CRITICAL QUESTIONS — DIRECT ANSWERS

| # | Question | Answer | Proof |
|---|---|---|---|
| A | Does production Order persistence preserve the complete financial snapshot? | **NO.** `CreateOrderTx` persists `subtotal`, `shipping_total`, `commission_percent`, `commission_amount`, `service_fee_amount`, `total_payable_amount`, `coins_used`, `coin_discount_amount=0`, `total_before_coins_amount`; it **drops `discount_amount` and `escrow_amount`** and the `Order` entity has **no fields** for them. | `backend/internal/commerce/order/infrastructure/repository/order_repository.go:56-121`; `backend/internal/commerce/order/entity/order.go:17-184` |
| B | Is `orders.discount_amount` persisted for real orders? | **NO.** Column defaults to `0` (migration `000001_canonical_schema.up.sql:1104`). No production `INSERT`/`UPDATE` writes it. Only hand-written test SQL writes it. | `order_repository.go:56-121`; `canonical_pricing_snapshot_persistence_test.go:42-71` (bypass) |
| C | Is `orders.coins_used` persisted for real orders? | **NO (always 0).** `NewOrderFromSource` hardcodes `CoinsUsed: 0` (`order.go:1104`); the INSERT writes it (`order_repository.go:94`) but no production path ever sets a nonzero value. `coins_refund_handler.go:130` explicitly: "Does NOT use order.coins_used snapshot (can be stale / **not persisted to orders**)". | `order.go:1103-1104`; `order_repository.go:94`; `worker/coins_refund_handler.go:130,288` |
| D | Is `orders.escrow_amount` persisted for real orders? | **NO.** Column defaults to `0` (migration:1099). No production write. | `order_repository.go:56-121` (column absent); grep of all `UPDATE orders SET` (none write escrow_amount in production) |
| E | Is `total_before_coins_amount` canonical & immutable? | **YES, functionally, but accidentally coupled.** Value = `TotalPayableAmount` at creation (`order.go:1103`) = token `EscrowAmount` = `(P-D)+S`. `UpdatePaymentSelectionTx` rewrites `total_payable_amount` (fee-inclusive) but not `total_before_coins_amount` — so it stays `(P-D)+S` and is *de facto* immutable. The coupling is accidental (it is `total_payable` *at creation*, not an independently-computed base). | `order.go:1103`; `order_repository.go:932-954`; `serverboot/payment_method_default_killed_test.go:183-190` |
| F | ONE canonical EscrowGross formula? | See §4. The **locked business truth is `EscrowGross = (P−D)+S`** (pricing token + canonical snapshot test + `ListPaymentMethods` + CreatePayment base). The **runtime escrow creation uses `P+S+C`** (`CalculateGrossEscrowFromSnapshot`) and release uses `P+S+C`. These are irreconcilable — the two authoritative sources disagree. | `pricing_token_service.go:404-415`; `pricing_helper.go:30-32`; `canonical_finalization_service.go:117`; `order_payment_service.go:266-270` |
| G | CommissionAmount identity? | **Product-only commission on discounted product value**: `C = floor((P−D)·rate/100)`. It is seller-side, never part of buyer refund, and has no shipping component. | `pricing_token_service.go:381-384, 680-683`; `refund_policy.go:30-33` (C seller-side); `refund_math.go:5-22` |
| H | CommissionAmount=13600 vs CommissionDelta=3600? | Not a contradiction. `CommissionAmount` is the **identity** (total product commission on PD). `CommissionDelta` is the **proportional reversal** for one refund event: `floor(C·cumProductAfter/PD) − floor(C·cumProductBefore/PD)`. Both use the same C, same PD; the delta is a subset of the identity. | `refund_math.go:83-91`; `verifier.go:681-684, 740-746` |
| I | Canonical decomposition `EscrowGross − EconomicRefund`? | `EconomicRefund = CashRefund + CoinDelta = Rpd + Rs` (accounting identity, `refund_math.go:111`). So `EscrowGross − EconomicRefund = (P−D)+S − (Rpd+Rs)` = seller remainder. But at runtime EscrowGross is `P+S+C` and refund math uses `pd = order.Subtotal` (undiscounted) — the components don't line up. See §6. | `refund_math.go:96-118`; `refund_gateway.go:326-329` |
| J | Funding source of partial-release components? | GATEWAY_CLEARING funds everything (settlement inflow `RecordGatewayPaymentSettlement`; release/partial-release drain it). PLATFORM_REVENUE receives commission + buyer fee. SELLER_PAYABLE receives seller net. BUYER_REFUNDABLE receives refund reversals. Coins are a **separate subsystem** (`coins_transactions`), never ledger accounts. | `finance_service.go:323-398, 187-234, 570-761, 794-866` |
| K | Rejected-model resurrection vectors? | See §10 (KILL LIST) — every formula that re-introduces escrow-gross/undiscounted-denominator/full-gross commission/gateway-clearing debit of unfunded escrow. | §10 |

---

## 2. CANONICAL AUTHORITY MAP

The **ONE canonical financial snapshot authority for the order is the pricing token** (pricing domain), generated once by `PricingTokenService` and persisted in `pricing_tokens`. The **order row is a projection of the token** and must carry the identical snapshot. The **finance ledger** is the authority for money *movement*, not for the order's pricing identity.

| Authority | Scope | Files |
|---|---|---|
| `PricingTokenService` (generate) | Computes P, D, C, PD, S, EscrowGross=(P−D)+S, TotalPayable, OrderValueForCoins, MaxCoins | `pricing/token/application/pricing_token_service.go:211-516` (fixed), `846-1065` (negotiation), `1099-1378` (auction) |
| `pricing_tokens` table | Persists the complete snapshot incl. `escrow_amount`, `discount_amount`, `commission_amount` | `migrations/000001_canonical_schema.up.sql:1250-1290`; `token/infrastructure/repository/pricing_token_repository_impl.go:54-109` |
| `PricingToken` entity (hydrate) | Round-trips all 14 audited fields | `token/entity/pricing_token.go:26-94`; repo `GetByToken/GetByTokenForUpdate:119-231` |
| `orders` table | **Should** be the immutable order snapshot (all 14 fields) — currently incomplete (§1A) | `migrations/000001_canonical_schema.up.sql:1086-1143` |
| `escrows` table | Wallet-domain obligation record; `amount` = what was actually funded | `core/wallet/entity/escrow.go:56-67` |
| Finance ledger | Money movement truth (GATEWAY_CLEARING, SELLER_PAYABLE, PLATFORM_REVENUE, BUYER_REFUNDABLE) | `finance/application/finance_service.go` |
| `refund_math.go` | Canonical proportional refund math (pure) | `finance/refund/application/refund_math.go:35-146` |

---

## 3. FIELD-BY-FIELD FORMULA TABLE

| # | Field | Canonical formula | Canonical source | Persisted? | Immutable? | Writers | Readers |
|---|---|---|---|---|---|---|---|
| 1 | Subtotal (P) | `quantity × unit_price` | token:339 | **YES** (`orders.subtotal`) | Yes | CreateOrderTx:88 | verifier, refund, release, DTO |
| 2 | DiscountAmount (D) | validated by `ApplyDiscountAtCheckout` | token:351-376 | **NO** (orders stays 0) | (n/a) | none (production) | verifier:672-676, refund_math via pd |
| 3 | PD = Subtotal − Discount | `P − D` | token:385 (`discountedProduct`) | Derived (not stored) | — | — | coins cap:425; refund_math; verifier:676 |
| 4 | ShippingTotal (S) | coverage/quote cost | token:265-335 | **YES** | Yes | CreateOrderTx:89 | refund, release, DTO |
| 5 | CommissionAmount (C) | `floor(PD·rate/100)` | token:381-384, 680-683 | **YES** | Yes | CreateOrderTx:91 | release:268, refund:328, verifier |
| 6 | CommissionRate | platform config `GetListingCommission`/`GetAuctionCommission` | token:379, 948, 1243 | **YES** | Yes | CreateOrderTx:90 | DTO |
| 7 | CoinsUsed (K) | chosen by buyer at payment, capped by `MaxCoinsAllowed` | token:425-426; dependencies.go:3285-3293 | **NO (always 0 on orders)** | (n/a) | none (production) | refund_gateway:329 (reads 0), completion service (reads 0) |
| 8 | TotalBeforeCoins | `(P−D)+S` | token:404-415; order.go:1103 | **YES** (= total_payable at creation) | Yes (de facto) | CreateOrderTx:96 only | CreatePayment:3269, ListPaymentMethods:3590 |
| 9 | TotalPayable | `(P−D)+S + F` (F=0 at creation; later real fee) | token:414-415; `UpdatePaymentSelectionTx` | **YES** | Mutable (fees applied at payment) | CreateOrderTx:93; UpdatePaymentSelectionTx:942 | DTO, payment |
| 10 | EscrowAmount / EscrowGross | **Contract: `(P−D)+S`**; **Runtime: `P+S+C`** | token:404-408 vs pricing_helper:30-32 | **NO (orders stays 0)**; escrows.amount=P+S+C | (n/a) | CreateEscrowFromGatewaySettlement | verifier:640, escrow checker, recon |
| 11 | CommissionDelta | `floor(C·cumA/PD) − floor(C·cumB/PD)` | refund_math.go:83-91 | Refund row (implied), ledger | — | gateway ack:395 | verifier:681-684 |
| 12 | EconomicRefund | `Rpd + Rs` (= CashRefund + CoinDelta) | refund_math.go:96-118 | Refund row fields | — | refund policy, ack | verifier |
| 13 | RemainingObligation | `EscrowGross − EconomicRefund` (contract) | §6 | derived | — | — | verifier (indirect) |
| 14 | CashRefund | `Rpd + Rs − CoinDelta` | refund_math.go:98; refund_gateway.go:5-10 | `refunds.final_refund_amount` | — | ack:403 | verifier |

---

## 4. ESCROW IDENTITY ANALYSIS (Question F — every formula found)

| Formula | Where | Status |
|---|---|---|
| `EscrowGross = (P−D)+S` | `pricing_token_service.go:404-408` (`escrowBase := subtotal+shipping+commission; escrowAmount := escrowBase−D` → `P+S+C−D`) — note this equals `(P−D)+S` **only when C=0**; the code's arithmetic is actually `P+S+C−D`! | **CRITICAL FINDING**: the "contract" formula in the token code is `P+S+C−D`, NOT `(P−D)+S`. They differ by C. The canonical snapshot test asserts `(P−D)+S` (test:108-120) and forbids `P+S+C−D` (test:117-120) — **the test contradicts the producer code**. The token's `escrowAmount` at line 408 is `P+S+C−D` (e.g. 100000+20000+4500−10000=114500), which the test explicitly rejects. |
| `EscrowGross = P+S+C` | `finance/application/pricing_helper.go:30-32` (`CalculateGrossEscrowFromSnapshot`); used to create the real escrow: `canonical_finalization_service.go:117-126`; release gross: `order_payment_service.go:266-270`; partial dispute validation: `order_completion_service.go:1739-1744` | **Runtime authority** (money actually moves by this) |
| `EscrowGross = P+S` (buyer-visible "escrow_amount") | `dispute_handler.go:380` comment "Total escrow amount (subtotal + shipping)"; `serverboot/dependencies.go:3643` `"escrow_amount": baseAmount.Int64()` where baseAmount = TotalBeforeCoins = `(P−D)+S` | Third representation |
| `order_created` outbox escrow | `order_creation_service.go:1722-1742`: `escrowAmount := Subtotal+Shipping+Commission` (P+S+C) | Fourth representation (outbox payload) |

**Resolution (F):** The *locked business contract* (test + payment-base consumption) is `(P−D)+S`. The *runtime money path* (escrow creation + release) is `P+S+C`. The producer code computes `P+S+C−D` in `escrowAmount`. **Three mutually incompatible formulas exist in the codebase.** The ONE canonical formula per the contract is `(P−D)+S`; everything else is a CONFLICTING_AUTHORITY (see §9).

---

## 5. COMMISSION IDENTITY ANALYSIS (Question G)

- **Identity**: `C = floor((P−D) × rate / 100)` — product-only commission on the *discounted* product value.
  - Source: `pricing_token_service.go:381-384` (`netSubtotal := subtotal.Sub(discountAmount); commissionAmount := calculateCommission(netSubtotal, commissionPercent)`).
  - `calculateCommission`: `subtotal.Int64() * percent.IntPart() / 100` (line 680-683).
- **Not** shipping commission, not product+shipping, not a fee.
- **Seller-side**: `refund_policy.go:30-33` ("CommissionAmount int64 // C (seller-side, NOT buyer refund)"), `refund_math.go:5-22` ("C is seller-side and never in buyer CashRefund").
- **Buyer fee F is separate** (`paymentmethodentity.CalculateFee`, PASS_18V), never part of C.
- **Verifier confirms**: "NOT a commission identity — the identity is order.CommissionAmount" (`verifier.go:740`), and uses `verifierProportionalCommissionPD` with the same `pd = Subtotal−Discount`.

**Consumers using the identity correctly**: `OrderPaymentService.ReleaseGatewayEscrowToSeller` (order_payment_service.go:268), `refund_gateway.go:328` (cVal), `verifier.go:681-682`, `refund_math.go`.
**Consumers using an outdated denominator**: `refund_gateway.go:326` (`pd := order.Subtotal` — **undiscounted**), `order_payment_service.go:165` (`pd := order.Subtotal`), `refund_service.go:189` (escrowAmount = P+S, undiscounted), `verifier.go:676` (reads orders.discount_amount which is 0 → PD=P).

---

## 6. REFUND IDENTITY ANALYSIS (Questions H, I)

**H — CommissionAmount vs CommissionDelta** (numeric example 13600 vs 3600):
- `CommissionAmount` = 13600 is the **identity**: total commission on PD = `floor(PD·rate/100)`.
- `CommissionDelta` = 3600 is the **event reversal**: `floor(C·cumProductAfter/PD) − floor(C·cumProductBefore/PD)` (`refund_math.go:83-91`). For a product refund `Rpd = 36000/PD·13600 ≈ 3600` when the cumulative product refund moves by that amount.
- They are related by the proportional formula; neither is wrong. The verifier recomputes the delta independently from `(previouslyRefunded, C, PD)` (`verifier.go:681-684`) and checks the ledger's commission entry against it.
- **The defect is not H — it is that PD is derived wrong upstream**: `refund_gateway.go:326` uses `pd := order.Subtotal` (undiscounted) while `refund_math.go` and the verifier expect `PD = Subtotal − Discount`. For a discounted order the gateway ack uses the wrong denominator; the verifier (reading `orders.discount_amount = 0`) uses PD = P too — so they happen to "agree" on the wrong base.

**I — `EscrowGross − EconomicRefund` decomposition (focused refund case)**:
- Contract: `EscrowGross = (P−D)+S`; `EconomicRefund = CashRefund + CoinDelta = Rpd + Rs` (`refund_math.go:96-118`, invariant `CashRefund+CoinDelta == Rpd+Rs` at line 111).
- So `EscrowGross − EconomicRefund = (P−D)+S − (Rpd+Rs)` = the seller's remaining obligation (product remainder + shipping remainder, minus commission not yet released).
- Runtime mismatch: escrow row was created at `P+S+C` (canonical_finalization_service.go:117), refund math runs on `pd = Subtotal` (undiscounted), and `kVal = order.CoinsUsed = 0` (never persisted). The three snapshots disagree, so the *actual* decomposition does not equal the contract decomposition.

---

## 7. FUNDING-SOURCE / LEDGER INVARIANT (Question J)

| Ledger account | Settlement | Release | Partial release | Refund reversal (before release) | Refund reversal (after release) |
|---|---|---|---|---|---|
| GATEWAY_CLEARING | +gross (inflow) `finance_service.go:365-367` | −gross `:220-224` | −remainder `:842-846` | −refund `:679-682` | (untouched) |
| SELLER_PAYABLE[seller] | — | +sellerNet `:222` | +sellerNet `:844` | — | −sellerComponent `:651` |
| PLATFORM_REVENUE | (+fee via RecordBuyerPaymentFeeRevenue `:457-460`) | +commission `:223` | +commission `:845` | — | −commissionComponent `:652` |
| BUYER_REFUNDABLE[buyer] | — | — | — | +refund `:680` | +refund `:650` |
| BANK_SETTLEMENT | −gross `:367` | — | — | — | — |

- Σ entries = 0 in every transaction (invariant enforced by `ledger_transactions_balanced` migration:2460).
- Coins: **never** in the ledger. Restored via `coins_transactions` earn rows keyed by outbox event id (`coins_refund_handler.go:196-252`), `coin_delta` from the ack (`refund_gateway.go:451-457`).
- **Funding-source defect**: escrow is created at `P+S+C` (canonical_finalization_service.go:117) but the settlement inflow only funds `payment.GrossAmount = (P−D)+S−K+F` (dependencies.go:3295-3306). For a discounted order, `P+S+C > (P−D)+S−K+F` whenever `C+D+K > F` — **GATEWAY_CLEARING is debited at release for components that were never funded** (the discount is never funded, C is funded only if buyer paid it, K coins reduce cash). This is the "gateway-clearing debit of unfunded escrow components" resurrection vector (Q K).

---

## 8. PERSISTENCE PROOF

**What `CreateOrderTx` actually writes** (`order_repository.go:56-121`):

| Column | Value |
|---|---|
| subtotal | `order.Subtotal` ✅ |
| shipping_total | `order.ShippingTotal` ✅ |
| commission_percent | `order.CommissionPercent` ✅ |
| commission_amount | `order.CommissionAmount` ✅ |
| service_fee_amount | `order.ServiceFeeAmount` ✅ |
| total_payable_amount | `order.TotalPayableAmount` ✅ |
| coins_used | `order.CoinsUsed` (= 0 always) ⚠️ |
| coin_discount_amount | hardcoded `0` ⚠️ |
| total_before_coins_amount | `order.TotalPayableAmount` (= (P−D)+S at creation) ✅ (by construction) |
| discount_amount | **ABSENT** ❌ |
| escrow_amount | **ABSENT** ❌ |

**Hydration** (`order_repository.go:142-287`, `GetForUpdate:313+`): SELECT list has no `discount_amount`, no `escrow_amount`; `Order` entity has no fields for them. So even if a row had them, they could not be read into the entity.

**DB defaults**: `discount_amount bigint DEFAULT 0 NOT NULL` (migration:1104), `escrow_amount bigint DEFAULT 0 NOT NULL` (migration:1099), `coins_used bigint DEFAULT 0 NOT NULL` (migration:1140).

**DB constraint**: `orders_check: refunded_amount >= 0 AND refunded_amount <= escrow_amount` (migration:2473). With `escrow_amount = 0` on every real order, **any `refunded_amount > 0` violates the constraint** — refunds are physically impossible at the DB layer for real orders.

**Test fixtures that bypass production persistence**:
- `backend/internal/commerce/order/tests/canonical_pricing_snapshot_persistence_test.go:24-311` — hand-written `INSERT INTO orders` that **does** write `discount_amount`, `escrow_amount`, `total_before_coins_amount` (lines 42-71, 138-166, 199-223, 263-291). Bypasses `CreateOrderTx`. Also **asserts the contract `(P−D)+S` and forbids `P+S+C−D`** (117-120), contradicting the token producer formula.
- `backend/internal/finance/refund/application/canonical_discount_snapshot_real_db_integration_test.go` — `//go:build integration`; asserts `SELECT subtotal,discount_amount,coins_used` round-trips `preview.DiscountAmount` through `CreateFromSaleSurface` (line 215, 219) and `GetForUpdate().DiscountAmount` (228). **Does not compile**: references `InitiateAdminGatewayRefund`/`InitiateAdminGatewayRefundInput` (317) — no such symbols in the tree. `go test -tags integration` fails: `svc.InitiateAdminGatewayRefund undefined`, `undefined: InitiateAdminGatewayRefundInput`, `undefined: midtrans.RefundTarget`, `CountSuccessfulPaymentsForOrder`, `FindSuccessfulPaymentForOrder` (verified below).
- `backend/internal/finance/refund/application/admin_refund_real_db_proof_integration_test.go` — `//go:build integration`; `UPDATE orders SET subtotal=100000, shipping_total=25000, commission_amount=4500, escrow_amount=129500, total_payable_amount=129500` (line 47) hand-writes `escrow_amount`. Also does not compile (same missing symbols).
- `backend/internal/finance/refund/application/refund_service_real_db_proof_integration_test.go` — `//go:build integration`; does not compile (missing `midtrans.RefundTarget`, `CountSuccessfulPaymentsForOrder`, `FindSuccessfulPaymentForOrder`).
- `backend/internal/serde/various` tests (payment_reconciliation, coin settlement) write orders directly too.

**Commands run** (evidence — no DB touched):
- `go vet ./internal/finance/refund/application/` → exit 0 (production files only; integration-tagged tests excluded)
- `go test -run XXX -count=1 ./internal/finance/refund/application/` → `ok ... [no tests to run]` (integration tests excluded by build tag)
- `go test -tags integration -run XXX -count=1 ./internal/finance/refund/application/` → **exit 1 (build failed)** — proves the "proof" integration tests do not compile against the current tree.

---

## 9. CONFLICTING AUTHORITIES & INCORRECT CONSUMERS

### Conflicting authorities (each claims to be the truth)

| Symbol | File:lines | Claimed truth | Reality |
|---|---|---|---|
| `PricingTokenService.GenerateForFixedPriceSale` escrowAmount | `pricing_token_service.go:404-408` | Escrow = `P+S+C−D` | Contract says `(P−D)+S` |
| `CalculateGrossEscrowFromSnapshot` | `finance/application/pricing_helper.go:30-32` | Escrow = `P+S+C` ("display only" doc) | Used to create real escrow + release + partial validation |
| `buildOrderPayload` | `order_creation_service.go:1722-1742` | escrow = `P+S+C` in outbox | Outbox consumers see inflated escrow |
| `OrderRepository.CreateOrderTx` | `order_repository.go:56-121` | order snapshot complete | Drops D + Escrow |
| `escrow_integrity_checker` | `core/wallet/application/escrow_integrity_checker.go:26-34,157-266` | "orders.escrow_amount set at order creation from pricing token" | Never set → every holding order flagged |
| DB `orders_check` | migration:2473 | `refunded_amount <= escrow_amount` | escrow_amount=0 → blocks all real refunds |
| `serverboot ListPaymentMethods` | `dependencies.go:3643` | `escrow_amount = baseAmount = (P−D)+S` | Third formula |
| `dispute_handler.go:380` comment | governance/dispute/delivery/http/dispute_handler.go:380 | escrow = subtotal+shipping | Yet another formula |

### Incorrect consumers (treat derived/zero as authority)

| Symbol | File:lines | What it reads | Why wrong |
|---|---|---|---|
| `verifier.checkRefundInvariants` `orderGross := order.EscrowAmount` | `verifier.go:640` | orders.escrow_amount (=0) | Refund caps checked against 0 |
| `verifier` `pd := order.Subtotal − order.DiscountAmount` | `verifier.go:676` | orders.discount_amount (=0) | PD = P, undiscounted |
| `verifier.loadOrders` | `verifier.go:1024-1049` | selects discount_amount, escrow_amount | Columns always 0 |
| `refund_gateway.go` ack `pd := order.Subtotal` | `refund_gateway.go:326` | order.Subtotal | Undiscounted denominator |
| `refund_gateway.go` ack `kVal := order.CoinsUsed` | `refund_gateway.go:329` | order.CoinsUsed (=0) | Coins never persisted |
| `OrderPaymentService.InitiateGatewayRefundForOrder` `pd := order.Subtotal` | `order_payment_service.go:165` | order.Subtotal | Undiscounted PD |
| `RefundService.CreateRefund` cap `Subtotal+Shipping` | `refund_service.go:189` | P+S | Overstates (should be (P−D)+S) and ignores D |
| `escrow_integrity_checker` | escrow_integrity_checker.go:157-191 | orders.escrow_amount | Mismatch alarm on every real order |
| `recon/classifier.detectD15` | `integration/payment/application/recon/classifier.go:436-457` | `Order.GrossAmount` (>0 check) | Never fires for real rows (0); blind spot |
| `OrderPaymentService.ReleaseGatewayEscrowToSeller` gross = P+S+C | `order_payment_service.go:266-270` | subtotal+shipping+commission | GATEWAY_CLEARING debited for unfunded D+C |
| `CanonicalFinalizationService` escrow = P+S+C | `canonical_finalization_service.go:117-126` | same | Same |
| `PartialRefundFromDispute` validation P+S+C == escrow | `order_completion_service.go:1733-1745` | CalculateGrossEscrowFromSnapshot | Same inflated formula; also validates against a 0 column |
| Projection worker + `order_summaries.escrow_amount` | `worker/projection_worker.go:440,518,785`; `projection/repository.go:84,110-112` | orders.escrow_amount (=0) | Propagates zero to read model |

---

## 10. LEGACY/ZOMBIE RESIDUES & KILL LIST

Everything below must eventually die (or be re-derived to the ONE truth). **Nothing here is deleted in this pass.**

### Kill list (rejected financial model residues)

| # | Symbol / file | Why it dies |
|---|---|---|
| 1 | `orders.discount_amount` (column + absent write) | Never persisted by production; consumers read 0 |
| 2 | `orders.escrow_amount` (column + absent write) | Never persisted; consumers read 0; constraint blocker |
| 3 | `orders.coins_used` snapshot on Order entity + INSERT | Never persisted; `coins_refund_handler.go:130` admits staleness |
| 4 | `orders.coin_discount_amount` column | Always written 0; coin truth is in `payments` + `coins_transactions` |
| 5 | `CalculateGrossEscrowFromSnapshot` (P+S+C) | Rejected formula used in money paths |
| 6 | `escrowAmount := escrowBase.Sub(discountAmount)` (P+S+C−D) in `pricing_token_service.go:408` | Contract is (P−D)+S; test forbids P+S+C−D |
| 7 | `buildOrderPayload` escrow = P+S+C | Outbox misrepresents order |
| 8 | `ReleaseGatewayEscrowToSeller` gross = P+S+C | Debits unfunded clearing |
| 9 | `CreateEscrowFromGatewaySettlement` amount = P+S+C (caller side) | Same |
| 10 | `PartialRefundFromDispute` P+S+C==escrow validation | Same |
| 11 | `refund_gateway.go:326` `pd := order.Subtotal` | Undiscounted denominator |
| 12 | `order_payment_service.go:165` `pd := order.Subtotal` | Undiscounted denominator |
| 13 | `refund_service.go:189` cap = P+S | Undiscounted cap |
| 14 | `refund_gateway.go:329` `kVal := order.CoinsUsed` | Reads never-persisted coins |
| 15 | `verifier` reads of `orders.discount_amount`/`escrow_amount` (loadOrders + invariants) | Must read real persisted snapshot (or ledger/escrow row) |
| 16 | `escrow_integrity_checker` per-order & global check vs orders.escrow_amount | Compares zero; should compare escrows.amount only |
| 17 | DB constraint `orders_check` (refunded ≤ escrow_amount) | Blocks refunds when escrow_amount=0 |
| 18 | `order_summaries.escrow_amount` projection + `OrderSummary.EscrowAmount` | Propagates zero |
| 19 | `recon detectD15` `Order.GrossAmount > 0` gate | Inverted blind spot |
| 20 | `canonical_pricing_snapshot_persistence_test.go` (hand-written INSERT) | Bypasses production persistence; also asserts (P−D)+S while producer computes P+S+C−D |
| 21 | `canonical_discount_snapshot_real_db_integration_test.go` | Does not compile (references removed symbols); asserts non-existent persistence |
| 22 | `admin_refund_real_db_proof_integration_test.go` (line 47 UPDATE) | Does not compile; hand-writes escrow_amount |
| 23 | `refund_service_real_db_proof_integration_test.go` | Does not compile (missing payment repo methods) |
| 24 | `refund_policy.go` `OrderSnapshot.LegacyGross()` (P+S+C) | Rejected formula (though currently unused in money path, keep-classified LEGACY_ZOMBIE) |
| 25 | `dispute_handler.go:380` "escrow (subtotal+shipping)" comment/DTO | Misleading formula |
| 26 | Any `pd := order.Subtotal` / `escrow := P+S+C` comment claiming to be canonical | Comment-level zombie |

### PROTECTED LIST (must NOT be deleted — canonical)

| # | Symbol / file | Why protected |
|---|---|---|
| 1 | `PricingTokenService.GenerateForFixedPriceSale/Negotiation/Auction` | Sole pricing authority (compute once) |
| 2 | `calculateCommission` (`pricing_token_service.go:680-683`) | Canonical C formula on PD |
| 3 | `pricing_tokens` table + `PricingToken` entity | Complete snapshot persistence (the ONE truth) |
| 4 | `PricingTokenRepositoryImpl` Create/Get/GetByTokenForUpdate | Snapshot round-trip |
| 5 | `ValidateForOrderLocked` + `FinalizeOrderConsumption` | Order gate + atomic token consumption + discount usage recording |
| 6 | `MaxCoinsAllowedForDiscountedProduct` | Canonical 20% coins cap on PD |
| 7 | `refund_math.go` `CalculateProportionalRefundBreakdown` + `proportionalFloor` | Canonical refund math (pure, correct) |
| 8 | `refund_policy.go` `ResolveRefundPolicy` + `ProductGross()` | Canonical policy resolution; ProductGross=PD+S |
| 9 | `RecordGatewayPaymentSettlement` | Funds GATEWAY_CLEARING with actual gross |
| 10 | `RecordBuyerPaymentFeeRevenue` | Realizes F at settlement |
| 11 | `RecordOrderRelease` | Drained release: GC→SP+PR, sellerNet+commission==gross |
| 12 | `RecordRefundReversal` (before/after release branches) | Canonical reversal double-entry |
| 13 | `RecordPartialRefundRelease` | Remainder release double-entry |
| 14 | `WalletService.CreateEscrowFromGatewaySettlement` / Release / Refund / PartialRefund | Escrow row state machine |
| 15 | `escrows.amount` (wallet table) | Actual funded obligation |
| 16 | `Order.TotalBeforeCoinsAmount` + `CreatePayment`/`ListPaymentMethods` baseAmount usage | Canonical buyer base (P−D)+S |
| 17 | `payment_method_default_killed_test.go` | Guard that total_before_coins is not overwritten by payment selection |
| 18 | `verifierProportionalCommissionPD` | Correct expectation derivation given correct PD |
| 19 | `Payments.coins_to_use` / `coin_discount_amount` + `coins_transactions` | Canonical coin spend/restore records |
| 20 | Migration CHECK constraints other than #17 (commission/escrow non-negative, subtotal = qty×price) | Data integrity guards |
| 21 | `CommissionAmount` on order + token | Canonical C identity (product-only, seller-side) |
| 22 | `Refund.RefundedProductAmount/RefundedShippingAmount/CoinsRefundedAmount/FinalRefundAmount` | Event-level split stamping |
| 23 | `SyncRefundSettlementFromGatewayAck` | Order status sync on ack |

---

## 11. EXACT FILE PATHS / SYMBOLS / LINE RANGES

Covered inline above; consolidated key paths:
- `backend/internal/pricing/token/application/pricing_token_service.go:211-516, 680-683, 404-415, 425`
- `backend/internal/pricing/token/entity/pricing_token.go:26-94`
- `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go:54-116, 119-232`
- `backend/internal/commerce/order/application/order_creation_service.go:1151-1233, 1264-1719, 1722-1742`
- `backend/internal/commerce/order/entity/order.go:17-184, 1000-1128`
- `backend/internal/commerce/order/infrastructure/repository/order_repository.go:29-139, 142-287, 313-488, 932-954`
- `backend/internal/commerce/order/application/order_payment_service.go:146-184, 257-294`
- `backend/internal/commerce/order/application/order_completion_service.go:373-559, 835-992, 1240-1395, 1418-1560, 1699-1801`
- `backend/internal/integration/payment/application/canonical_finalization_service.go:61-140`
- `backend/internal/integration/payment/infrastructure/repository/payment_settlement_service.go:153-248`
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
- `backend/internal/worker/coins_refund_handler.go:119-130, 196-280, 288`
- `backend/internal/projection/repository.go:84, 110-112, 137-140, 177-190, 361-362, 440-441, 687-688`
- `backend/internal/integration/payment/application/recon/classifier.go:431-457`
- `backend/internal/integration/payment/application/recon/audit/resolver.go:197`
- `backend/internal/serveboot/dependencies.go:3128-3131, 3269-3310, 3590-3646, 3903-3928` (serverboot)
- `backend/internal/governance/dispute/delivery/http/dispute_handler.go:380`
- `backend/internal/commerce/order/delivery/http/dto/decision.go:247-356`
- `backend/internal/commerce/order/delivery/http/admin_order_handler.go:315-434`
- `backend/migrations/000001_canonical_schema.up.sql:1086-1143, 1250-1290, 2471-2481`
- Test fixtures: `commerce/order/tests/canonical_pricing_snapshot_persistence_test.go:24-311`; `finance/refund/application/canonical_discount_snapshot_real_db_integration_test.go:127-232, 283-344`; `admin_refund_real_db_proof_integration_test.go:36-101`; `refund_service_real_db_proof_integration_test.go:37-97`; `serverboot/payment_method_default_killed_test.go:99-115, 183-190`

## 12. COMMANDS RUN + EXIT CODES

| Command | Exit | Note |
|---|---|---|
| `git status --short` (repo root) | 0 | Confirmed audit-referenced test files are untracked; verifier.go modified |
| `go vet ./internal/finance/refund/application/` (backend) | 0 | Production package compiles; integration tests excluded |
| `go test -run XXX -count=1 ./internal/finance/refund/application/` | 0 | "no tests to run" — integration tag excluded |
| `go test -tags integration -run XXX -count=1 ./internal/finance/refund/application/` | **1** | Build failed: `InitiateAdminGatewayRefund`/`InitiateAdminGatewayRefundInput` undefined; `midtrans.RefundTarget` undefined; `CountSuccessfulPaymentsForOrder`/`FindSuccessfulPaymentForOrder` undefined |
| `go list -f "{{.GoFiles}}" ./internal/finance/refund/application/` | 0 | Production files: `refund_gateway.go refund_math.go refund_service.go` |

## 13. TESTS OBSERVED (read-only)

See §8 (Persistence proof) — the three integration-tagged "proof" tests do not compile; the untagged canonical snapshot test bypasses production persistence and contradicts the producer formula.

## 14. DATABASE / FILES CHANGED

- **DATABASE TOUCHED: NONE** (no DB process started; commands were `go vet`/`go test -run XXX`/`go list` only; `go test -tags integration` failed at compile before any test could run).
- **FILES CHANGED: NONE** except this audit report `ORDER_FINANCIAL_SNAPSHOT_AUTHORITY_AUDIT.md`.

---

## 15. CONCLUSION (STOP RULE)

The contradiction is structural and present at every layer:

1. **Producer**: pricing token computes `EscrowAmount = P+S+C−D` (`pricing_token_service.go:408`), the contract test demands `(P−D)+S` and rejects `P+S+C−D` (`canonical_pricing_snapshot_persistence_test.go:117-120`), and the runtime money path uses `P+S+C` (`pricing_helper.go:30-32` → `canonical_finalization_service.go:117` → `order_payment_service.go:266-270`). **Three mutually exclusive formulas**.
2. **Persistence**: `orders` drops `discount_amount` and `escrow_amount`; `coins_used` is always 0; the DB CHECK `refunded_amount <= escrow_amount` makes real refunds impossible.
3. **Consumers**: verifier, escrow integrity checker, projection, recon, and refund gateway all read the zeroed columns as authority.
4. **Proof tests**: the integration-tagged "proofs" do not compile against the current tree; the untagged snapshot test bypasses `CreateOrderTx` with hand-written SQL.

Because a contradiction remains in the canonical order financial snapshot, per the STOP RULE this pass stops here. No fix, no cleanup, no behavior modification.
