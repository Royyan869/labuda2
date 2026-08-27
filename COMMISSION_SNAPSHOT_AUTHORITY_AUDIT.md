# COMMISSION_SNAPSHOT_AUTHORITY_AUDIT

Canonical commission snapshot lifecycle audit: pricing token → order creation/persistence → order hydration → payment → refund → release → reconciliation/verifier consumers.

Scope: `orders.discount_amount`, `orders.commission_amount`, `orders.total_before_coins_amount`, `orders.escrow_amount` — produced, persisted, hydrated, consumed.

Files changed: none except this report.

---

## 1. Verdict

```
COMMISSION_SNAPSHOT_AUTHORITY_CONTRADICTION_FOUND
```

---

## 2. Executive summary

The pricing token (pricing domain) is the true canonical authority and is persisted correctly with all four fields. The **order persistence layer drops `discount_amount` and `escrow_amount` outright** (they stay at their DB defaults `0`), while `commission_amount` is correctly persisted and `total_before_coins_amount` is persisted as `total_payable_amount` (which equals `(P-D)+S`, so that column is coincidentally correct for no-fee orders). The `Order` entity has **no `DiscountAmount` or `EscrowAmount` field**, so those columns can never be hydrated — yet the verifier, escrow integrity checker, projection worker, and refund gateway all read them as canonical financial truth, and the migration CHECK constraint `refunded_amount <= escrow_amount` (with `escrow_amount = 0`) actively blocks real discounted-order refunds at the DB level. The "canonical snapshot persistence" tests bypass the production INSERT with hand-written SQL, masking the gap.

### Answering the three explicit questions

1. **Can real production orders persist `discount_amount = 0` despite a non-zero discount?** — **YES.** `CreateOrderTx` (order_repository.go:57–121) never writes `discount_amount`; the column has `DEFAULT 0 NOT NULL` (migration line 1104). A discounted order created via `CreateFromSaleSurface`/`CreateFromAuction` persists `discount_amount = 0` while `discount_code`/`discount_type`/`discount_value` are silently dropped too (not in the INSERT column list).
2. **Is `escrow_amount` actually persisted?** — **NO.** Same INSERT omits `escrow_amount` (line 1099 `DEFAULT 0 NOT NULL`), so every production order row has `escrow_amount = 0`.
3. **Does `total_before_coins_amount` match `(P-D)+S`?** — **YES, by construction, only for the no-fee-at-creation path.** The INSERT writes `order.TotalPayableAmount.Int64()` (order_repository.go:96), and `TotalPayableAmount` is set to `escrowAmount = (P-D)+S` at token creation (pricing_token_service.go:404–415, order.go:1103). Note this is coincidental: the column name says "before coins" but the value written is the *total payable* snapshot; `UpdatePaymentSelectionTx` later overwrites `total_payable_amount` with the fee-inclusive gross (order_repository.go:939–944) **without** touching `total_before_coins_amount`, so the "canonical buyer base" stays `(P-D)+S` while `total_payable_amount` becomes `(P-D)+S+fee`. The payment handler validates `order.TotalBeforeCoinsAmount == token.OrderValueForCoins + token.ShippingTotal` (dependencies.go:3128–3131), which holds.

---

## 3. Canonical business truth (locked, as stated in code)

- **Pricing authority = pricing token.** `pricing_token_service.go:1–18` declares "NO PRICING CALCULATION SHALL HAPPEN OUTSIDE THIS SERVICE"; values computed once at token generation, never recomputed at order creation.
- **Formulas (locked):**
  - `D = discount_amount` (validated via `discountService.ApplyDiscountAtCheckout`; pricing_token_service.go:351–376).
  - `C = commission = floor((P − D) × rate%)` — `calculateCommission(netSubtotal, commissionPercent)`, pricing_token_service.go:381–384, 680–683.
  - `PD = P − D` (`discountedProduct`, pricing_token_service.go:385, 424).
  - `EscrowAmount = (P − D) + S` (escrowBase − D; pricing_token_service.go:404–408).
  - `TotalPayableAmount = EscrowAmount + ServiceFee` (service fee = 0 at token time; line 414–415).
  - `TotalBeforeCoinsAmount = (P−D)+S` — asserted by canonical_pricing_snapshot_persistence_test.go:108–120.
- **Consumers' locked expectation:** verifier uses `PD = Subtotal − Discount` (verifier.go:676) and `orderGross = EscrowAmount` (verifier.go:640); escrow integrity checker expects `orders.escrow_amount == escrows.amount` (escrow_integrity_checker.go:48–49, 161–191); refund gateway computes `pd = order.Subtotal` (refund_gateway.go:326).

---

## 4. Producer / consumer registry (exact classification per symbol)

### A. PRICING TOKEN (pricing domain) — canonical

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| A1 | `PricingTokenService.GenerateForFixedPriceSale` (computes D, C, PD, Escrow, TotalPayable, OrderValueForCoins) | `backend/internal/pricing/token/application/pricing_token_service.go:211–516` (formulas: 337–415) | **CANONICAL_AUTHORITY** |
| A2 | `GenerateForNegotiation` (D forced 0, C on subtotal, escrow = P+S) | same file:846–1065 | CANONICAL_AUTHORITY (negotiation path) |
| A3 | `GenerateForAuction` (buy-now & bid-win; D allowed, C on P−D) | same file:1099–1378 | CANONICAL_AUTHORITY (auction path) |
| A4 | `calculateCommission` | same file:680–683 | CANONICAL_AUTHORITY (sole commission formula) |
| A5 | `coinsApp.MaxCoinsAllowedForDiscountedProduct(PD)` | same file:425 (helper in incentive/coins) | CANONICAL_DERIVED (coins cap, not in audit scope) |
| A6 | `PricingToken` entity fields `EscrowAmount`, `DiscountAmount`, `CommissionAmount` | `backend/internal/pricing/token/entity/pricing_token.go:47–62` | CANONICAL_AUTHORITY (struct) |
| A7 | `PricingTokenRepositoryImpl.CreateTx` — persists all four fields to `pricing_tokens` (incl. `escrow_amount`, `discount_amount`) | `backend/internal/pricing/token/infrastructure/repository/pricing_token_repository_impl.go:54–109` | CANONICAL_AUTHORITY (persistence) |
| A8 | `PricingTokenRepositoryImpl.GetByToken/GetByTokenForUpdate` — hydrates `EscrowAmount`, `DiscountAmount`, `CommissionAmount` | same file:119–231 (and 234+), SELECT:143–158 | CANONICAL_AUTHORITY (hydration) |
| A9 | `ValidateForOrderLocked` / `ValidateAndConsume` / `FinalizeOrderConsumption` | pricing_token_service.go:536–661 | CANONICAL_CONSUMER (order gate) |

### B. ORDER CREATION (commerce order domain)

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| B1 | `order_handler.go buildPricingSnapshotFromToken` — maps token → order `PricingSnapshot` including `EscrowAmount`, `DiscountAmount`, `TotalPayableAmount` | `backend/internal/commerce/order/delivery/http/order_handler.go:1587–1631` | **CANONICAL_CONSUMER** (correct mapping) |
| B2 | `auction_handler.go buildClaimPricingSnapshot` (same mapping) | `backend/internal/commerce/auction/delivery/http/auction_handler.go:862–891+` | CANONICAL_CONSUMER |
| B3 | `chat_handler.go` `buildPricingSnapshotFromToken` (negotiation checkout) | `backend/internal/interaction/chat/delivery/http/chat_handler.go:685`, :1978 area | CANONICAL_CONSUMER |
| B4 | `CreateFromSaleSurface` / `CreateFromAuction` — `PricingSnapshot` required; all order fields taken from snapshot; **no D/Escrow written to Order** | `backend/internal/commerce/order/application/order_creation_service.go:1264–1719`, :688–1040 | **CANONICAL_CONSUMER (partial — drops D & Escrow)** |
| B5 | `finalizeOrderCreationTx` — integrity checks on snapshot (incl. `EscrowAmount + ServiceFee == TotalPayable`) then `CreateOrderTx` + outbox | same file:1151–1233 | CANONICAL_CONSUMER / integrity gate |
| B6 | `orderentity.NewOrderFromSource` — **sets `TotalBeforeCoinsAmount = totalPayableAmount`; never sets any Discount/Escrow field (fields don't exist on entity)** | `backend/internal/commerce/order/entity/order.go:1000–1128` (esp. 1103) | **DUPLICATE_AUTHORITY / CONFLICTING_AUTHORITY (silent loss of D & Escrow)** |
| B7 | `Order` struct — has `CommissionAmount`, `TotalPayableAmount`, `TotalBeforeCoinsAmount`, `CoinsUsed`; **NO `DiscountAmount`, NO `EscrowAmount` fields** | order.go:17–184 (esp. 56–62, 92) | **CONFLICTING_AUTHORITY (struct cannot carry canonical D/Escrow)** |
| B8 | `OrderRepository.CreateOrderTx` — INSERT **omits `discount_amount` and `escrow_amount`**; writes `coins_used`, `coin_discount_amount = 0`, `total_before_coins_amount = TotalPayableAmount` | `backend/internal/commerce/order/infrastructure/repository/order_repository.go:56–121` (columns 57–74, values 80–120) | **CONFLICTING_AUTHORITY (primary defect: drops D & Escrow; mislabels total_before_coins)** |
| B9 | `buildOrderPayload` — **recomputes `escrowAmount = Subtotal + Shipping + Commission`** for `order.created` outbox (different formula from token's `(P−D)+S`) | order_creation_service.go:1722–1742 | **CONFLICTING_AUTHORITY (outbox event payload escrow ≠ token escrow)** |

### C. ORDER HYDRATION

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| C1 | `OrderRepository.GetByID` SELECT — reads `total_before_coins_amount`, `commission_amount`, `total_payable_amount`; **never selects `discount_amount` or `escrow_amount`** | order_repository.go:142–287 (SELECT 180–215, entity 242–287) | **CONFLICTING_AUTHORITY (hydration gap)** |
| C2 | `OrderRepository.GetForUpdate` (same gap) | order_repository.go:313+ (SELECT at :355) | CONFLICTING_AUTHORITY |
| C3 | Order entity JSON exposes `total_before_coins_amount`, `commission_amount` — no discount/escrow | order.go:17–184 | CONFLICTING_AUTHORITY (same gap) |

### D. PAYMENT (PASS_18V / coins)

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| D1 | `CorePaymentHandler.CreatePayment` — `baseAmount := order.TotalBeforeCoinsAmount`; validates `TotalBeforeCoinsAmount == token.OrderValueForCoins + token.ShippingTotal`; computes `cashAmount = base − coins`; fee on cash; `gross = cash + fee` | `backend/internal/serverboot/dependencies.go:3269–3310`, :3128–3131 | **CANONICAL_CONSUMER (correct; consumes total_before_coins)** |
| D2 | `UpdatePaymentSelectionTx` — overwrites `service_fee_amount`, `total_payable_amount`; **does NOT touch `total_before_coins_amount`** (verified by payment_method_default_killed_test.go:183–190) | order_repository.go:932–954 | CANONICAL_DERIVED (fee-aware total payable; base preserved) |
| D3 | `PaymentSettlementService.SettlePaymentByID` / `SettlePayment` — locks order, blocks post-expiry, marks payment settled; no order money-column writes | `backend/internal/integration/payment/infrastructure/repository/payment_settlement_service.go:153–248` | CANONICAL_CONSUMER (state machine) |
| D4 | `CanonicalFinalizationService.FinalizeOrderPayment` — **`escrowAmount := financeApp.CalculateGrossEscrowFromSnapshot(order)` = P+S+C (NOT (P−D)+S)**; creates escrow row with that amount; marks paid | `backend/internal/integration/payment/application/canonical_finalization_service.go:117–130` | **CONFLICTING_AUTHORITY (escrow creation formula diverges from token; discount ignored)** |
| D5 | `financeApp.CalculateGrossEscrowFromSnapshot` = `Subtotal + ShippingTotal + CommissionAmount` (doc says "validation and display only", but used to create real escrow) | `backend/internal/finance/application/pricing_helper.go:30–32` | **DUPLICATE_AUTHORITY (used in money path despite "display only" doc)** |
| D6 | `CoreCoinHandler` / coins reservation (`CreateReservation`, `ReleaseReservation`, `ConsumeReservation`) — coins spend persisted in `coins_transactions` + `payments.coin_discount_amount`, **NOT on orders** | `backend/internal/serverboot/dependencies.go:3386–3397`, `:3903–3928`; `backend/internal/incentive/coins/infrastructure/repository/coins_repository_impl.go:478+` | CANONICAL_AUTHORITY (coins domain); orders.coins_used is display-only |
| D7 | `CoinsRefundRequiredHandler` — comment: "Does NOT use order.coins_used snapshot (can be stale / **not persisted to orders**)" | `backend/internal/worker/coins_refund_handler.go:130`, :288 | **LEGACY/ZOMBIE witness (orders.coins_used never written in prod path)** |

### E. RELEASE / COMPLETION (escrow movement)

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| E1 | `OrderService.MarkPaid` + `OrderPaymentService.ReleaseGatewayEscrowToSeller` — `commission := order.CommissionAmount`; `gross := sellerNet + commission`; releases via `WalletService.ReleaseGatewayEscrow(orderID, gross)` | `backend/internal/commerce/order/application/order_payment_service.go:257–290`; order_completion_service.go:486+ | **CANONICAL_CONSUMER (consumes order.commission_amount)** |
| E2 | `WalletService` escrow row = canonical money truth (`escrows.amount`) | `backend/internal/core/wallet/application/wallet_service.go:313+` | CANONICAL_AUTHORITY (wallet domain) |
| E3 | `OrderCompletionService` partial-dispute path: `escrowAmount := CalculateGrossEscrowFromSnapshot(order)` and validates `item+shipping+commission == escrow` | order_completion_service.go:1736–1744 | CONFLICTING_AUTHORITY (same P+S+C divergence) |
| E4 | `EscrowIntegrityChecker` — reads `orders.escrow_amount` from DB, compares to `escrows.amount`, sums holding orders; comment claims "set at order creation time from pricing token" | `backend/internal/core/wallet/application/escrow_integrity_checker.go:26–34, 85–124, 157–191, 201–266` | **CONFLICTING_AUTHORITY (compares always-0 orders.escrow_amount vs real escrows → guaranteed false-positive alert)** |

### F. REFUND

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| F1 | `RefundService.CreateRefund` — `escrowAmount := order.Subtotal + order.ShippingTotal` (excludes D, excludes C); rejects `requested > that` | `backend/internal/finance/refund/application/refund_service.go:188–195` | **CONFLICTING_AUTHORITY (uses P+S, not (P−D)+S; overstates available refund for discounted orders)** |
| F2 | `InitiateAdminGatewayRefund` — `policy.ProductAmount` from order snapshot; `RefundedProductAmount = rpd` | refund_service.go:400–446 | CANONICAL_DERIVED |
| F3 | `refund_gateway.go` gateway ack — **`pd := order.Subtotal.Int64()`** (ignores `order.DiscountAmount`), `kVal := order.CoinsUsed` | `backend/internal/finance/refund/application/refund_gateway.go:326–329` | **CONFLICTING_AUTHORITY (PD mis-derived; discount ignored; coins read from never-persisted field)** |
| F4 | `refund_math.go CalculateProportionalRefundBreakdown` — canonical math keyed on `pd, s, c, k`; expects `pd = P−D` (callers must supply correct PD) | refund_math.go:35–139 | CANONICAL_AUTHORITY (pure math) |
| F5 | DB constraint `orders_check`: `refunded_amount >= 0 AND refunded_amount <= escrow_amount` — with `escrow_amount = 0` on real orders, **any refunded_amount > 0 violates the constraint** | `backend/migrations/000001_canonical_schema.up.sql:2473` | **CONFLICTING_AUTHORITY (DB-level refund blocker)** |
| F6 | `order_completion_service` full-refund paths read `order.CoinsUsed > 0` to emit coins refund | order_completion_service.go:935, 1108–1114, 1311–1316, 1485–1490, 1778–1786 | CANONICAL_CONSUMER (of a field that is never persisted → no-op) |

### G. RECONCILIATION / VERIFIER / PROJECTION

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| G1 | `verifier.go` `Order` struct includes `DiscountAmount`, `EscrowAmount` | `backend/internal/finance/verifier/verifier.go:56–69` | CANONICAL_CONSUMER (intent) |
| G2 | `loadOrders` — `SELECT ... subtotal, discount_amount, shipping_total, commission_amount, escrow_amount, refunded_amount ...` | verifier.go:1024–1044 | **CANONICAL_CONSUMER of never-populated columns** |
| G3 | refund invariant: `orderGross := order.EscrowAmount`; `pd := Subtotal − DiscountAmount`; commission-proportional check | verifier.go:640–682, 735–746 | **CONFLICTING_AUTHORITY (checks against zeros)** |
| G4 | `ProjectionWorker` — copies `o.escrow_amount`, `o.refunded_amount` into `order_summaries` (rebuild and incremental) | `backend/internal/worker/projection_worker.go:440–453, 518–531, 766–815` | **CANONICAL_CONSUMER (of always-0 column)** |
| G5 | `projection/repository.go` `OrderSummary.EscrowAmount` + read paths | `backend/internal/projection/repository.go:84, 110–112, 137–140, 177–190, 361–362, 403, 440–441, 482, 687–688, 710` | CANONICAL_CONSUMER (of always-0 column) |
| G6 | `recon/classifier.go` — flags orders where `orders.escrow_amount > 0` but no escrow row / no payment | `backend/internal/integration/payment/application/recon/classifier.go:432–453` | CANONICAL_CONSUMER (inverted: real orders have 0 → misclassified) |
| G7 | `recon/audit/resolver.go` — reads `escrow_amount` | `backend/internal/integration/payment/application/recon/audit/resolver.go:197` | CANONICAL_CONSUMER (of always-0 column) |
| G8 | `notification_worker_shared.go` `EscrowAmount` in notification payload | `backend/internal/worker/notification_worker_shared.go:77` | CANONICAL_CONSUMER (payload always 0) |
| G9 | Order list/admin DTOs — do **not** expose `discount_amount`/`escrow_amount` (financial truth from Ledger) | order_query_service.go:58–112, 247–350; dto/decision.go:247–356 | CANONICAL_DERIVED (display omits them) |

### H. TESTS (do not change — cited as evidence only)

| # | Symbol | File:lines | Classification |
|---|---|---|---|
| H1 | `TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip` — **hand-written INSERT writes `discount_amount`/`escrow_amount`**, bypassing `CreateOrderTx`; asserts `(P−D)+S` | `backend/internal/commerce/order/tests/canonical_pricing_snapshot_persistence_test.go:24–121` | **DUPLICATE_AUTHORITY (masks the production INSERT gap)** |
| H2 | `TestCanonicalPricingSnapshot_*` (3 more) — same bypass | same file:123–311 | DUPLICATE_AUTHORITY |
| H3 | `canonical_discount_snapshot_real_db_integration_test.go` — real flow: asserts `SELECT subtotal,discount_amount,coins_used` round-trips `preview.DiscountAmount` **through `CreateFromSaleSurface`** (line 215, 219) and `GetForUpdate().DiscountAmount` (line 228) — **this test would FAIL against production `CreateOrderTx`**, proving the gap | `backend/internal/finance/refund/application/canonical_discount_snapshot_real_db_integration_test.go:127–232` (esp. 215–231), also :283–296 | **DUPLICATE_AUTHORITY / fails to detect production gap (only passes because test harness order differs — see §6)** |
| H4 | `admin_refund_real_db_proof_integration_test.go:47` — manual `UPDATE orders SET ... escrow_amount=129500 ...` (test-only writer) | same | LEGACY/ZOMBIE (test fixture writer) |
| H5 | `payment_method_default_killed_test.go:183–190` — guards that `UpdatePaymentSelectionTx` must NOT write `total_before_coins_amount` | `backend/internal/serverboot/payment_method_default_killed_test.go` | CANONICAL_CONSUMER (guard test) |

---

## 5. Call chains (end-to-end)

### Fixed-price discounted checkout (the money path)

```
POST /pricing-token (GenerateForFixedPriceSale)
  → D = ApplyDiscountAtCheckout (discount_service.go)
  → C = floor((P−D)·rate%)                       pricing_token_service.go:381–384
  → escrow = (P−D)+S; payable = escrow + 0       :404–415
  → INSERT pricing_tokens (escrow_amount, discount_amount, ...)   token_repo:54–109   ✅
POST /orders (order_handler)
  → ValidateForOrderLocked (FOR UPDATE)          pricing_token_service.go:536–579
  → buildPricingSnapshotFromToken → orderApp.PricingSnapshot (D, Escrow, C, payable)  order_handler:1587–1631 ✅
  → CreateFromSaleSurface
      → snapshot integrity checks                order_creation_service.go:1166–1181 ✅
      → NewOrderFromSource                       order.go:1000–1128
          → TotalBeforeCoinsAmount = payable     order.go:1103 (=(P−D)+S) ✅
          → (DiscountAmount / EscrowAmount: NO FIELDS) ❌
      → CreateOrderTx
          → INSERT omits discount_amount, escrow_amount ❌
          → total_before_coins_amount = TotalPayableAmount ✅ (=(P−D)+S)
          → commission_amount = C ✅
          → coins_used = 0, coin_discount_amount = 0 (always)
  → FinalizeOrderConsumption (discount usage + token used)  ✅
  → buildOrderPayload escrow = P+S+C (outbox) ❌ formula divergence
```

### Payment

```
POST /payments (CreatePayment)
  → base = order.TotalBeforeCoinsAmount (= (P−D)+S)  dependencies.go:3269 ✅
  → validate base == token.OrderValueForCoins + token.ShippingTotal  :3128–3131 ✅
  → cash = base − coins; gross = cash + fee         :3295–3306 ✅
  → UpdatePaymentSelectionTx (service_fee, total_payable only)   order_repository.go:939–944 ✅
```

### Webhook → escrow → refund

```
webhook → SettlePaymentByID (order lock, expiry guard) → FinalizeOrderPayment
  → escrow amount = P+S+C (CalculateGrossEscrowFromSnapshot)  canonical_finalization_service.go:117 ❌ (token said (P−D)+S)
  → escrows.amount = P+S+C (wallet row)  ✅ wallet is truth
  → orders.escrow_amount = 0 ❌
refund path
  → CreateRefund cap = P+S (refund_service.go:189) ❌
  → gateway ack pd = order.Subtotal (refund_gateway.go:326) ❌ (should be P−D)
  → verifier pd = Subtotal − orders.discount_amount (= P−0=P) ❌ (should be P−D)
  → verifier orderGross = orders.escrow_amount (= 0) ❌
  → CHECK refunded_amount <= escrow_amount(=0) → DB reject ❌
```

---

## 6. Key contradiction proofs

### P1: `orders.discount_amount` is never written by production code
- `CreateOrderTx` column list (order_repository.go:57–74) contains **no `discount_amount`**; only `discount_code` etc. are absent too. Schema default is `DEFAULT 0 NOT NULL` (migration:1104).
- `Order` entity has no `DiscountAmount` field (order.go:17–184) → impossible to write via repo.
- Only writers of `orders.discount_amount` in the entire backend are **tests** (canonical_pricing_snapshot_persistence_test.go:48, :61; canonical_discount_snapshot_real_db_integration_test.go:215 reads it; admin_refund test writes other columns). Verified by grep: production `INSERT`/`UPDATE orders` statements (order_repository.go:56–121, 939–944; projection_worker.go:440/518/785; coins_refund_handler.go:479) contain no `discount_amount` write.
- **Therefore: YES — every discounted production order persists `discount_amount = 0`.** The verifier's `order_invalid_pd` check (verifier.go:672–679) will then treat PD = P (discount invisible), and refund proportional math uses the wrong denominator.

### P2: `orders.escrow_amount` is never written by production code
- Same INSERT omits `escrow_amount` (column list order_repository.go:57–74; schema default migration:1099).
- No `UPDATE orders SET escrow_amount` in production code (grep shows only test fixtures: admin_refund_real_db_proof_integration_test.go:47 and hand-written test INSERTs).
- `EscrowIntegrityChecker` compares `orders.escrow_amount (=0)` vs `escrows.amount (>0)` → **every holding order is a mismatch** (escrow_integrity_checker.go:157–191).
- Recon classifier's "escrow_amount > 0 but no escrow/payment row" detector (classifier.go:432–453) never fires on real rows because the column is 0 — a monitoring blind spot in the opposite direction.
- `order_summaries.escrow_amount` = 0 for all real orders (projection_worker.go:785).

### P3: `total_before_coins_amount = (P−D)+S` holds, but by mislabeling
- Token: `TotalPayableAmount = escrowAmount.Add(serviceFeeAmount)`, `escrowAmount = (P−D)+S` (pricing_token_service.go:404–415).
- Order entity: `TotalBeforeCoinsAmount = totalPayableAmount` (order.go:1103).
- INSERT: `total_before_coins_amount = order.TotalPayableAmount.Int64()` (order_repository.go:96).
- After `UpdatePaymentSelectionTx` overwrites `total_payable_amount` with fee-inclusive gross (order_repository.go:939–944), `total_before_coins_amount` retains `(P−D)+S` — the base remains correct and stable. Payment uses it as `baseAmount` (dependencies.go:3269, 3295, 3590). **The value matches `(P−D)+S` exactly for the no-fee-at-creation path**, and remains the pre-fee/pre-coin base afterward. So this column is functionally correct — though semantically the INSERT comment "= total_payable when no coins" papers over the fee divergence.

### P4: The escrow formula divergence (token vs escrow-creation)
- Token: `EscrowAmount = (P−D)+S` (pricing_token_service.go:404–408).
- `FinalizeOrderPayment`: `escrowAmount := CalculateGrossEscrowFromSnapshot(order) = P+S+C` (canonical_finalization_service.go:117; pricing_helper.go:30–32).
- For a discounted order: token escrow = 110,000; created escrow = 100,000+20,000+4,500 = **124,500** → the wallet holds more than the buyer's discounted payable; the discount is silently paid by… the escrow pool. `buildOrderPayload` (order_creation_service.go:1724) even emits `P+S+C` in `order.created`.
- Refund paths then return up to `P+S` (refund_service.go:189) while the gateway cash paid was `(P−D)+S − coins`; combined with `orders.escrow_amount = 0`, the DB constraint `refunded_amount <= escrow_amount` (migration:2473) **blocks any real refund write for discounted orders** (and for the integrity-checker path).

### P5: `orders.coins_used` never persisted → refund coin math is hollow
- INSERT writes `coins_used = order.CoinsUsed` where `CoinsUsed` is hardcoded `0` in `NewOrderFromSource` (order.go:1104); no production path updates `orders.coins_used` (grep confirms; `UpdatePaymentSelectionTx` doesn't; payment settlement writes `payments.coins_to_use`/`coin_discount_amount` and `coins_transactions` instead).
- `coins_refund_handler.go:130`: "Does NOT use order.coins_used snapshot (can be stale / not persisted to orders)" — explicit admission.
- Yet `refund_gateway.go:329` `kVal := order.CoinsUsed` (= 0) and `order_completion_service` gates coins-refund outbox emissions on `order.CoinsUsed > 0` (never true) → **coins are never restored via the order-field path** (they are restored via `coins_transactions` lookups only in the legacy path, and via delta in the ack path).

---

## 7. Exact classification tally

| Classification | Producers/Consumers |
|---|---|
| CANONICAL_AUTHORITY | A1–A8, E2, F4, D6 |
| CANONICAL_DERIVED | A5, D2, G9 |
| CANONICAL_CONSUMER | A9, B1–B3, B4/B5 (partial), D1, D3, E1, F2, F6, G1–G2, G4–G8, H5 |
| DUPLICATE_AUTHORITY | B6, D5, H1–H3 |
| CONFLICTING_AUTHORITY | B6–B8, B9, C1–C3, D4, E3–E4, F1, F3, F5, G3 |
| LEGACY/ZOMBIE | D7, H4 |
| UNKNOWN | none |

---

## 8. Conclusion

- The **pricing token** lifecycle (A1–A9) is internally consistent and the true canonical authority.
- **Order persistence drops `discount_amount` and `escrow_amount`** (B8) and the entity cannot carry them (B7), so production orders persist 0 for both — answering "YES, real discounted orders persist `discount_amount = 0`" and "NO, `escrow_amount` is never actually persisted" with file-and-line proof.
- `total_before_coins_amount` does equal `(P−D)+S` at creation and stays stable, but the surrounding escrow creation (`P+S+C`) and refund caps (`P+S`) diverge from the locked `(P−D)+S` truth, and the DB CHECK constraint makes discounted-order refunds physically impossible.
- The "proof" tests (H1–H3) bypass the production INSERT, so they do not catch the gap; H3 would fail if run against the real repository path.
- Downstream consumers (verifier G1–G3, integrity checker E4, projection G4–G5, recon G6, refund F1/F3) read these always-0 columns as financial truth → systemic contradiction.

```
COMMISSION_SNAPSHOT_AUTHORITY_CONTRADICTION_FOUND
```
