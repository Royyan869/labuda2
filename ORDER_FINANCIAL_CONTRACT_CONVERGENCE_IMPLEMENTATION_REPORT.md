# ORDER_FINANCIAL_CONTRACT_CONVERGENCE_IMPLEMENTATION_REPORT

```
VERDICT: ORDER_FINANCIAL_CONTRACT_PROOF_INCOMPLETE
```

STOP RULE 5 triggered (a test whose business expectation contradicts the locked canonical contract) and Phase E ledger funding analysis surfaced a genuine accounting gap requiring an owner decision. Production convergence (Phases B–D) is implemented and builds clean; the canonical payment-intent integration test passes against it. One tracked integration test encodes the rejected contract and cannot pass without a decision.

---

## 1. PHASE A — STALE LAYER DELETION (COMPLETE)

**DELETED (authorized by owner decision):**
- `backend/cmd/verify_partial_refund_semantics/main.go` (tracked; 4-arg orderGross-denominator tool) — now `D` in git status.
- `backend/internal/finance/refund/application/admin_refund_real_db_proof_integration_test.go` (untracked)
- `backend/internal/finance/refund/application/canonical_discount_snapshot_real_db_integration_test.go` (untracked)
- `backend/internal/finance/refund/application/refund_seller_approval_dispatch_test.go` (untracked)
- `backend/internal/finance/refund/application/refund_service_real_db_proof_integration_test.go` (untracked)
- `backend/internal/integration/payment/infrastructure/repository/refund_real_db_proof_integration_test.go` (untracked)
- `backend/internal/worker/coins_refund_full_real_db_proof_integration_test.go` (untracked)
- `backend/internal/worker/coins_refund_path_a_replay_real_db_proof_integration_test.go` (untracked)
- `backend/migrations/000044_one_successful_payment_per_order.up.sql` + `.down.sql` (untracked)
- `backend/migrations/000045_order_coin_refund_authority.up.sql` + `.down.sql` (untracked)

**ERROR MADE AND CORRECTED:** I also deleted `backend/internal/serverboot/payment_coin_settlement_integration_test.go`, believing it untracked stale residue. It is **tracked** and provides the shared `paymentSettlementHarness` used by two other tracked integration tests. I restored it from a backup copy (`D:\Temp\opencode\labuda-blueprint-push\...`), and the serverboot integration package compiles again. Its presence is now clean in git.

**PRESERVED (unrelated, untouched):** audit markdown files, mobile, social/content, worker content-mention/alert tests, migrations 000042/000043, `escrow_ledger_atomicity_real_db_proof_integration_test.go`, `escrow_repository_real_db_proof_integration_test.go`, `ledger_repository_real_db_proof_integration_test.go`.

---

## 2. PHASES B–D — PRODUCTION CONVERGENCE (COMPLETE, BUILDS CLEAN)

### Phase B — canonical snapshot consumption
| File | Change |
|---|---|
| `backend/internal/commerce/order/application/order_payment_service.go` | `InitiateGatewayRefundForOrder`: `pd := order.TotalBeforeCoinsAmount - order.ShippingTotal` (fallback `Subtotal` when base absent); `kVal` resolved from coins domain via new `CoinsSpendReader` interface (`coins_transactions`), never `order.CoinsUsed`. Added `SetCoinsSpendReader` + `coinsSpendForOrder`. |
| `backend/internal/finance/refund/application/refund_service.go` | Refund cap: `order.TotalBeforeCoinsAmount` (canonical PD+S) with legacy fallback. Added `CoinsSpendReader` interface + `SetCoinsSpendReader` + `coinsSpendForOrder`. |
| `backend/internal/finance/refund/application/refund_gateway.go` | Ack pipeline: `pd := order.TotalBeforeCoinsAmount - order.ShippingTotal` (fallback Subtotal); `kVal := s.coinsSpendForOrder(...)` (coins domain). |
| `backend/internal/serverboot/dependencies.go` | `coinsRepository := coinsRepo.NewCoinsRepository()` constructed once (before refund wiring); wired into `refundService.SetCoinsSpendReader` and `orderService.PaymentService().SetCoinsSpendReader`; later coins-service wiring reuses the instance. |

### Phase C — escrow/release gross = PD+S
| File | Change |
|---|---|
| `backend/internal/integration/payment/application/canonical_finalization_service.go` | Escrow creation: `escrowAmount := order.TotalBeforeCoinsAmount` (was `CalculateGrossEscrowFromSnapshot` = P+S+C). |
| `backend/internal/commerce/order/application/order_payment_service.go` | `ReleaseGatewayEscrowToSeller`: `gross := order.TotalBeforeCoinsAmount`; `sellerNet := gross - commission`; rejects `commission > gross`. |
| `backend/internal/commerce/order/application/order_completion_service.go` | Partial-dispute validation: `escrowAmount := order.TotalBeforeCoinsAmount`; `calculatedTotal := itemPrice + shippingFee` (removed commission from the equality). Removed unused `financeApp` import. |
| `backend/internal/commerce/order/application/order_creation_service.go` | `buildOrderPayload`: `escrow_amount := order.TotalBeforeCoinsAmount` (was P+S+C). |

### Phase D — remaining consumers
| File | Change |
|---|---|
| `backend/internal/core/wallet/application/escrow_integrity_checker.go` | `getHoldingOrders`/`getTotalOrderEscrow` read `total_before_coins_amount` (was dead `escrow_amount`); docs updated. |
| `backend/internal/finance/verifier/verifier.go` | `Order` struct: replaced `DiscountAmount`/`EscrowAmount` with `TotalBeforeCoinsAmount`. `orderGross := TotalBeforeCoinsAmount`; `pd := TotalBeforeCoinsAmount - ShippingTotal` (fallback Subtotal). `loadOrders` reads `total_before_coins_amount`. Fixtures updated. |
| `backend/internal/worker/projection_worker.go` | 3 queries: `o.total_before_coins_amount` instead of `o.escrow_amount` (incremental x2 + rebuild). |
| `backend/internal/integration/payment/application/recon/audit/resolver.go` | `fetchOrder` reads `total_before_coins_amount` (was dead `escrow_amount`). |
| `backend/internal/integration/payment/application/recon/types.go` | `OrderRow.GrossAmount` documented as `total_before_coins_amount`. |
| `backend/internal/integration/payment/application/recon/classifier.go` | D15 comment updated to `total_before_coins_amount`. |
| `backend/internal/commerce/order/infrastructure/repository/order_repository.go` | **Hydration fix:** `GetForUpdate` now selects/scans/hydrates `total_before_coins_amount` (was missing — caused escrow creation to read 0). |

### Tests run (all pass except the blocked tracked test)
- `go build ./...` → exit 0
- `go vet ./internal/finance/refund/application/ ./internal/finance/verifier/ ./internal/commerce/order/application/ ./internal/integration/payment/application/... ./internal/core/wallet/application/` → exit 0
- `go test ./internal/finance/refund/application/ ./internal/finance/verifier/ ./internal/core/wallet/application/ ./internal/commerce/order/application/ ./internal/integration/payment/...` → all ok
- `go test -tags integration -run TestCreatePayment_BasicFlowAndPreviewAuthority ./internal/serverboot/` → **ok** (canonical buyer base `(P−D)+S` verified end-to-end)
- `go test -tags integration ./internal/serverboot/` → compiles (harness restored)

---

## 3. STOP CONDITION A — TRACKED TEST ENCODES THE REJECTED CONTRACT

`backend/internal/serverboot/payment_coin_settlement_integration_test.go` (tracked, provides the shared settlement harness) contains assertions that directly contradict the locked canonical contract and now fail against the converged production:

| Line | Assertion | Conflict |
|---|---|---|
| 513 | `CalculateGrossEscrowFromSnapshot(orderBefore) == 124500` | Asserts P+S+C escrow (rejected) |
| 525-526 | `orderSnap.CoinsUsed == 10000`, `orderSnap.CoinDiscountAmount == 10000` | Asserts dead `orders.coins_used`/`coin_discount_amount` authority (rejected; production writes 0) |
| 546, 580 | `escrowAmount == 124500` | Asserts P+S+C escrow row (now PD+S=110000) |
| 620-621, 812-813 | `orderSnap.CoinsUsed == 15000/10000` | Same dead-column authority |
| 953-954 | `CalculateGrossEscrowFromSnapshot == 124500` | Same P+S+C |
| 959 | `zeroEscrow == 124500` | Same P+S+C escrow row |

The test's *valid* coverage (payment.CoinsToUse, payment.CoinDiscountAmount, reservation consumed, spend rows) must be preserved; only the rejected assertions above conflict.

**Owner decision required:** authorize updating the tracked test's rejected assertions to the canonical contract (`escrow == TotalBeforeCoinsAmount == 110000`, drop the `orders.coins_used`/`orders.coin_discount_amount` authority assertions), OR direct an alternative (e.g., remove the test, or revert the Phase C escrow change — the latter would resurrect P+S+C, which the mission forbids).

## 4. STOP CONDITION B — LEDGER FUNDING GAP WHEN K > 0 (PHASE E)

Traced the full ledger funding for a coin-redemption order (canonical fixture: PD=90000, S=20000, C=4500, K=18000, F=4000):

| Step | GATEWAY_CLEARING delta | Balance |
|---|---|---|
| Settlement (`RecordGatewayPaymentSettlement`, gross = `(PD+S)−K+F` = 96000) | +96000 | 96000 |
| Fee sweep (`RecordBuyerPaymentFeeRevenue`, F=4000) | −4000 | **92000** |
| Release (`RecordOrderRelease`, gross = PD+S = 110000) | −110000 | **−18000** |

`GATEWAY_CLEARING` is debited `110000` against a funded `92000` — an over-debit of exactly **K=18000**. The buyer redeemed K coins, so `payment.gross_amount = (PD+S)−K+F`; the platform's coin cost is never booked to the ledger (`finance_service.go` has no coin entries; coins live only in `coins_transactions`).

This is **pre-existing** (the old code over-debited by `C+D+K` = even more) but the locked contract `EscrowAmount = PD+S` combined with "GATEWAY_CLEARING may only be debited by funds actually present" is unsatisfiable when K>0 **without** either:
1. `EscrowAmount = (PD+S)−K` (contradicts the locked `EscrowAmount = PD+S`), or
2. a coin-funding ledger entry crediting GATEWAY_CLEARING by K (inventing a ledger movement, forbidden unless implied by contract).

**Owner decision required:** (a) confirm the platform absorbs the coin redemption K as a business expense outside the ledger (making the over-debit an accepted cost, not a ledger bug — in which case Phase E passes with documentation), or (b) authorize a specific funding entry, or (c) direct the escrow formula change.

---

## 5. EXACT COMMANDS + EXIT CODES

| Command | Exit |
|---|---|
| `go build ./...` | 0 |
| `go vet ./internal/finance/refund/application/ ./internal/finance/verifier/ ./internal/commerce/order/application/ ./internal/integration/payment/application/... ./internal/core/wallet/application/` | 0 |
| `go test ./internal/finance/refund/application/ ./internal/finance/verifier/ ./internal/core/wallet/application/ ./internal/commerce/order/application/ ./internal/integration/payment/...` | 0 |
| `go test -tags integration -run TestCreatePayment_BasicFlowAndPreviewAuthority ./internal/serverboot/` | 0 |
| `go test -tags integration -run TestPaymentCoinSettlement_KPositive_PersistsSpendSnapshotAndLedger ./internal/serverboot/` | 1 (asserts rejected P+S+C + dead-column coins) |
| `go test -tags integration -run XXX ./internal/serverboot/` | 0 (compiles) |

## 6. RESIDUE STATUS

`RESIDUE_FOUND` — two items:
1. Tracked `payment_coin_settlement_integration_test.go` rejected assertions (STOP A).
2. `CalculateGrossEscrowFromSnapshot` still exists in `backend/internal/finance/application/pricing_helper.go` and is still referenced by the tracked test — the helper itself must be killed after the test decision.

## 7. FILES CHANGED (this mission)

```
FILES CHANGED:
  backend/internal/commerce/order/application/order_payment_service.go
  backend/internal/commerce/order/application/order_completion_service.go
  backend/internal/commerce/order/application/order_creation_service.go
  backend/internal/commerce/order/infrastructure/repository/order_repository.go
  backend/internal/core/wallet/application/escrow_integrity_checker.go
  backend/internal/finance/refund/application/refund_gateway.go
  backend/internal/finance/refund/application/refund_service.go
  backend/internal/finance/verifier/verifier.go
  backend/internal/integration/payment/application/canonical_finalization_service.go
  backend/internal/integration/payment/application/recon/audit/resolver.go
  backend/internal/integration/payment/application/recon/classifier.go
  backend/internal/integration/payment/application/recon/types.go
  backend/internal/serveboot/dependencies.go
  backend/internal/worker/projection_worker.go

DATABASE CHANGED:
  NONE

MIGRATIONS CHANGED:
  NONE
```

## 8. FINAL STOP STATEMENT

Production convergence (Phases B–D) is implemented, builds clean, and the canonical payment-intent integration test passes. Work stops because:

1. **STOP RULE 5**: the tracked `payment_coin_settlement_integration_test.go` asserts the rejected `P+S+C` escrow and the dead `orders.coins_used`/`coin_discount_amount` authority. It cannot pass against the canonical production without updating those assertions, and the file was not in the owner's authorized-change list.
2. **STOP RULE 3/7**: with K>0, `GATEWAY_CLEARING` is debited `PD+S` against a funded `(PD+S)−K`, an over-debit of K with no defined funding source under the locked contract. An owner decision is required on whether the platform absorbs coin redemption as a non-ledger cost or a specific funding entry is authorized.

Exact owner decisions required:
- **A**: Update the tracked `payment_coin_settlement_integration_test.go` rejected assertions to canonical values (escrow = TotalBeforeCoinsAmount, drop dead-column coins authority) — yes/no.
- **B**: Confirm the coin-redemption K shortfall in GATEWAY_CLEARING is the platform's accepted cost (no ledger entry), or authorize an explicit funding mechanism.

No production code was reverted, no rejected model was resurrected, and no accounting behavior was invented.
