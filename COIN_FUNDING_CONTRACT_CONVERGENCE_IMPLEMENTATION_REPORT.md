# COIN_FUNDING_CONTRACT_CONVERGENCE_IMPLEMENTATION_REPORT

```
VERDICT: ORDER_FINANCIAL_CONTRACT_CONVERGED
```

The locked platform-funded coin contract is fully implemented, proven at runtime, and the rejected model is dead. All phases completed.

---

## 1. EXACT FINAL CONTRACT (implemented and proven)

| Symbol | Value | Source |
|---|---|---|
| P | product subtotal | `pricing_token_service.go` |
| D | seller-funded discount | `pricing_token_service.go` |
| PD | `P − D` | `pricing_token_service.go` (discountedProduct) |
| S | shipping | order.shipping_total |
| C | `floor(PD × rate / 100)` product-only | `calculateCommission` |
| K | coins redeemed (authoritative in coins domain) | `payments.coins_to_use`, `coins_transactions`, `user_coin_balance`, `coin_reservations` |
| BuyerBase / EscrowAmount | `PD + S` = `total_before_coins_amount` | `NewOrderFromSource`, `FinalizeOrderPayment` |
| Buyer cash | `(PD+S) − K + F` | `CreatePayment` |
| GATEWAY_CLEARING funding | settlement `(PD+S)−K+F`, fee sweep `−F`, platform K funding `+K` → `PD+S` | `RecordGatewayPaymentSettlement`, `RecordBuyerPaymentFeeRevenue`, `RecordCoinFunding` |
| Seller entitlement | `PD+S` (K never reduces it) | `ReleaseGatewayEscrowToSeller` |
| K funding source | `PLATFORM_BANK` (platform's own bank holdings) | `RecordCoinFunding` |
| Refund | `CashRefund = Rpd+Rs−CoinDelta`; funding reversed by `CoinDelta` | `refund_math.go`, `RecordCoinFundingReversal` |
| Rejected | `P+S+C`, `P+S+C−D`, orderGross denominator, `orders.coins_used/discount_amount/escrow_amount` as authority | all removed/dead |

## 2. AUTHORITY MAP

| Authority | Canonical source | Status |
|---|---|---|
| Pricing snapshot | `pricing_tokens` table + `PricingTokenService` | CANONICAL (producer converged to `(P−D)+S`) |
| Escrow row | `escrows.amount = total_before_coins_amount = PD+S` | CANONICAL |
| Coin authority | `user_coin_balance`, `coin_reservations`, `coins_transactions` | CANONICAL |
| K platform funding | `PLATFORM_BANK` (reserve float seeded) | CANONICAL (new ledger method) |
| Ledger | `FinanceService.Record*` (Σ=0 per tx) | CANONICAL |
| `orders.discount_amount` / `escrow_amount` / `coins_used` | dead columns, never authority | LEGACY/ZOMBIE (readers removed) |

## 3. FUNDING AUTHORITY DECISION

`PLATFORM_BANK` is the existing canonical source for K: it represents the platform's own external bank holdings (established by withdrawal-payout semantics: `WITHDRAWAL_COMMITTED → PLATFORM_BANK`). Funding a buyer benefit is the same economic direction (platform money funding a real obligation), so no account's semantic meaning changed. A reserve float (matching BANK_SETTLEMENT's pattern) was seeded so the `balance >= 0` CHECK holds across the funding lifecycle.

## 4. COIN LIFECYCLE (implemented)

RESERVE (payment creation) → CONSUME (settlement) → order_spend → settlement. Failure → RELEASE. Refund → proportional restoration.

- `CoinsService.ConsumeAndSpendForOrder` (`coins_service.go`): atomically consumes reservation (idempotent), writes `order_spend` (idempotent via UNIQUE), deducts balance (exactly once). Never writes a finance ledger entry.
- Wired in `CanonicalFinalizationService.FinalizeOrderPayment` when `payment.CoinsToUse > 0`.

## 5. LEDGER LIFECYCLE (implemented)

| Step | GATEWAY_CLEARING | PLATFORM_BANK | SELLER_PAYABLE | PLATFORM_REVENUE | Σ |
|---|---|---|---|---|---|
| Settlement | +((PD+S)−K+F) | | | | 0 |
| Fee sweep | −F | | | +F | 0 |
| **Coin funding** | **+K** | **−K** | | | 0 |
| Release | −(PD+S) | | +(PD+S)−C | +C | 0 |
| **Final** | **0** | **−K** | **(PD+S)−C** | **F+C** | 0 |

`GATEWAY_CLEARING` is never negative. K never becomes revenue.

## 6. REFUND LIFECYCLE (implemented)

- `RecordCoinFundingReversal` (`finance_service.go`): `DR GATEWAY_CLEARING −CoinDelta / CR PLATFORM_BANK +CoinDelta`, idempotent key `coin_funding_reversal_<refund_id>`. Called in `HandleGatewayRefundAck` after `RecordRefundReversal`.
- Canonical remainder: `(PD+S) − (cumProductRefundAfter + cumShippingRefundAfter)` — fully cash-backed after funding reversal.
- Full refund: clearing → 0, PLATFORM_BANK restored.

## 7. FILES CHANGED (this mission)

```
FILES CHANGED:
  backend/internal/finance/application/finance_service.go        (RecordCoinFunding, RecordCoinFundingReversal)
  backend/internal/finance/application/system_account_bootstrap.go (PLATFORM_BANK reserve float)
  backend/internal/finance/application/pricing_helper.go          DELETED (CalculateGrossEscrowFromSnapshot)
  backend/internal/finance/refund/application/refund_gateway.go    (FinanceReverser + funding reversal + canonical remainder)
  backend/internal/finance/refund/application/refund_gateway_webhook_spy_test.go (spy method)
  backend/internal/finance/refund/entity/refund_policy.go          DELETED LegacyGross()
  backend/internal/finance/refund/entity/refund_policy_test.go     converged to canonical ProductGross()/CashRefund
  backend/internal/finance/verifier/verifier.go                   (PLATFORM_BANK opening balance)
  backend/internal/incentive/coins/application/coins_service.go    (ConsumeAndSpendForOrder)
  backend/internal/integration/payment/application/canonical_finalization_service.go (CoinSpendConsumer + consume + funding)
  backend/internal/integration/payment/application/payment_webhook.go (ON CONFLICT DO NOTHING idempotent dedup)
  backend/internal/pricing/token/application/pricing_token_service.go (3 producer sites: escrow = (P-D)+S)
  backend/internal/serverboot/dependencies.go                      (coins wiring; refund/order K readers)
  backend/internal/serverboot/payment_coin_settlement_integration_test.go (canonical assertions + ledger proof + harness)
  backend/pkg/db/errors.go                                         (retryable commit-rollback safety net)
  + prior-mission converged files (order_payment_service.go, order_completion_service.go, order_creation_service.go,
    order_repository.go, escrow_integrity_checker.go, refund_service.go, projection_worker.go, recon/*, verifier.go)

DATABASE CHANGED:
  NONE (no schema/migration changed; PLATFORM_BANK float is a bootstrap seed, applied at runtime)

MIGRATIONS CHANGED:
  NONE
```

## 8. DELETED ARTIFACTS

- `backend/internal/finance/application/pricing_helper.go` — `CalculateGrossEscrowFromSnapshot` (P+S+C)
- `LegacyGross()` in `refund_policy.go` (P+S+C)
- `P+S+C−D` escrow formulas in all 3 pricing-token producers
- Rejected assertions in `payment_coin_settlement_integration_test.go` (escrow=124500, dead-column coin authority)

## 9. PROTECTED LIST

- `refund_math.go` (10-arg PD-denominator), `refund_math_test.go`
- `refund_policy.go` `ProductGross` = PD+S
- `FinanceService.Record*` (Σ=0), `ledgerRepo.CreateTransaction` idempotency
- Wallet escrow state machine, `escrows.amount`
- `payments.coins_to_use`, `coins_transactions`, `user_coin_balance`, `coin_reservations`
- `coins_refund_handler.go` (coins-domain restoration; `order.coins_used` NOT authority)
- Verifier PD-denominator convergence

## 10. TESTS RUN + EXIT STATUS

| Command | Exit |
|---|---|
| `go build ./...` | 0 |
| `go vet` (pricing, refund, verifier, coins, payment-app, order-app, wallet, pkg/db) | 0 |
| `go test` pricing/... refund/... verifier coins/... payment-app/... order-app wallet pkg/db | 0 |
| `go test -tags integration -run TestPaymentCoinSettlement_LedgerFundingProof` | 0 |
| `go test -tags integration -run TestPaymentCoinSettlement_KPositive_PersistsSpendSnapshotAndLedger` | 0 |
| `go test -tags integration -run TestPaymentCoinSettlement_KZero...\|...SellerEntitlement...` | 0 |
| `go test -tags integration -run ...AvailableBalanceContinuity...\|...ConsumedReplay...` | 0 |
| `go test -tags integration -run ...ConcurrentDuplicateWebhook_IsRecordedOnce...` | 0 |
| `go test -tags integration -run ...RollsBackWhenSpendInsertFails\|...ReleasedReservation...` | 0 |

Pre-existing unrelated failures (untouched): `internal/worker` content-mention/alert tests, `finance/application` withdrawal idempotency test, `refund/infrastructure/repository` refund-history contract test, `serverboot` chat projection test.

## 11. RUNTIME ACCOUNTING PROOF (K>0, fixture: BuyerBase=110000, K=10000, F=4000, C=4500)

Proven by `TestPaymentCoinSettlement_LedgerFundingProof` (passes):
- GATEWAY_CLEARING after settlement+fee+funding = **110000** = BuyerBase ✓
- PLATFORM_BANK debited exactly **K=10000** ✓
- Release drains GATEWAY_CLEARING to baseline (never negative) ✓
- SELLER_PAYABLE = **105500** = BuyerBase − C (K never reduces seller entitlement) ✓
- PLATFORM_REVENUE = **8500** = F(4000) + C(4500) (K never becomes revenue) ✓
- Every ledger transaction balanced (Σ=0, enforced by repository panic) ✓

## 12. RUNTIME COIN PROOF

Proven by `TestPaymentCoinSettlement_KPositive_PersistsSpendSnapshotAndLedger` and `..._ConsumedReplay...`:
- Reservation consumed exactly once (replay idempotent) ✓
- `order_spend` transaction written exactly once (reference `order_spend`/order_id) ✓
- Coin balance deducted exactly once (20000 → 10000) ✓
- Escrow = 110000 (PD+S, not 124500) ✓

## 13. RESIDUE STATUS

CLEAN. Semantic search confirms:
- No `P+S+C` / `P+S+C−D` producer remains (token producers emit `(P−D)+S`)
- No `CalculateGrossEscrowFromSnapshot`, no `LegacyGross`
- No orderGross commission denominator
- No `orders.discount_amount` / `orders.escrow_amount` / `orders.coins_used` authority reads
- `orders.coins_used` remains a display-only DTO field (not financial authority)
- Coin spend uses the canonical `order_spend`/order_id convention (matching the refund worker)

## 14. REMAINING BLOCKERS

NONE for the canonical contract. Pre-existing unrelated test failures (worker content-mention, withdrawal idempotency, refund-history contract, chat projection) are outside this mission and untouched per the worktree rule.

## 15. FINAL CLEANUP STATUS

COMPLETE. One financial truth → one authority (pricing token + coins domain + PLATFORM_BANK funding) → all producers/consumers converged → accounting conserves money (Σ=0, clearing never negative) → rejected model destroyed → residue proven clean → regression proven.
