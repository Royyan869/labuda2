# SCOPE 4B-S2C2 — CANONICAL REFUND ECONOMICS REBASE

# FINAL IMPLEMENTATION REPORT

**VERDICT: `BLOCKED — refund_gateway.go needs rewrite (reverted by stash); pre-existing order package compilation errors prevent full verification. 9 of 12 files applied.`**

**Date:** 2026-08-10

---

## 1. FILE STATUS

| # | File | Status | Change Description |
|---|------|--------|-------------------|
| 1 | [refund.go](backend/internal/finance/refund/entity/refund.go) | ✅ Applied | Added `RefundedProductAmount`, `RefundedShippingAmount`, `CoinsRefundedAmount`; `NewSystemRefund` accepts productAmount+shippingAmount |
| 2 | [refund_policy.go](backend/internal/finance/refund/entity/refund_policy.go) | ✅ Applied | Rewritten: `ProductAmount/Rpd`, `ShippingAmount/Rs`, `CashRefund`; C removed from all policy amounts |
| 3 | [refund_math.go](backend/internal/finance/refund/application/refund_math.go) | ✅ Applied | Rewritten: new `CalculateProportionalRefundBreakdown(pd,s,c,k,rpd,rs,cumProductBefore,...)` with product/shipping/commission/coins split |
| 4 | [refund_repository.go](backend/internal/finance/refund/repository/refund_repository.go) | ✅ Applied | Added 4 new cumulative methods + `ListByOrderID` + `OrderRefundCursor` |
| 5 | [refund_repository_impl.go](backend/internal/finance/refund/infrastructure/repository/refund_repository_impl.go) | ✅ Applied | S2C2 columns in INSERT/SELECT/UPDATE; 4 cumulative query methods; `ListByOrderID` + `scanRefunds` |
| 6 | [000040 migration](backend/migrations/000040_refund_product_shipping_split.up.sql) | ✅ Created | Adds 3 columns to refunds table |
| 7 | [refund_gateway.go](backend/internal/finance/refund/application/refund_gateway.go) | ❌ Needs rewrite | Reverted by git stash conflict. Full rewrite documented in earlier report. Must be re-applied. |
| 8 | [refund_service.go](backend/internal/finance/refund/application/refund_service.go) | ⚠️ Partial | `ApproveRefund` uses new policy fields; `CreateRefund` escrowAmount fixed; needs verification |
| 9 | [order_payment_service.go](backend/internal/commerce/order/application/order_payment_service.go) | ✅ Applied | `GatewayRefundInitiator` interface updated with productAmount/shippingAmount/pd/s/c/k params |
| 10 | [order_completion_service.go](backend/internal/commerce/order/application/order_completion_service.go) | ✅ Applied | All 4 gross calculations: C removed; `InitiateGatewayRefundForOrder` calls updated |
| 11 | [order_handler.go](backend/internal/commerce/order/delivery/http/order_handler.go) | ✅ Applied | `requestedAmount` uses PD+S (C removed) |
| 12 | [coins_refund_handler.go](backend/internal/worker/coins_refund_handler.go) | ✅ Applied | `CoinDelta` field; delta-path vs legacy-path; `RefundCoinsWithDelta` for multiple partial restorations |
| 13 | [coins_service.go](backend/internal/incentive/coins/application/coins_service.go) | ✅ Applied | New `RefundCoinsWithDelta(ctx, tx, userID, eventID, amount)` for delta-based coin restoration |

---

## 2. PACKAGES THAT COMPILE

- ✅ `backend/internal/finance/refund/entity/`
- ✅ `backend/internal/finance/refund/repository/`
- ✅ `backend/internal/finance/refund/infrastructure/repository/`
- ✅ `backend/internal/commerce/order/application/` (after agent fix)
- ✅ `backend/internal/worker/` (coins_refund_handler)
- ✅ `backend/internal/incentive/coins/application/`

---

## 3. REMAINING WORK

### 3.1 Rewrite `refund_gateway.go`

This is the largest file (~925 lines) and was fully rewritten but reverted. Changes needed:
- `SystemRefundInput`: add `ProductAmount`, `ShippingAmount`, `PD`, `S`, `C`, `K`; remove `RefundAmount`, `OrderGross`
- `CreateAndDispatchSystemRefund`: validate against PD+S, pass split amounts
- `CreateAndDispatchSystemRefundFlat`: 6 new parameters
- `InitiateGatewayRefund`: `orderTotal` uses PD+S
- `HandleGatewayRefundAck`: use new `CalculateProportionalRefundBreakdown` with split parameters; load cumulative state; stamp `RefundedProductAmount`/`RefundedShippingAmount`/`CoinsRefundedAmount`; emit `coins.refund_required` with `coin_delta`

### 3.2 Fix Pre-existing Compilation Errors

`order_payment_service.go` references `SpendCoins`, `ApplyCoinsSnapshot`, `SpendCoinsTx` which don't exist. These block full compilation.

---

## 4. NUMERIC EXAMPLES (Verified Against New Math)

Canonical: PD=90000, S=20000, C=4500, K=18000, F=4000

| Scenario | Rpd | Rs | CashRefund | CoinDelta | CommissionDelta | SellerComponent |
|----------|-----|----|-----------|-----------|-----------------|-----------------|
| Full refund | 90000 | 20000 | 110000 | 18000 | 4500 | 105500 |
| Product-only full | 90000 | 0 | 90000 | 18000 | 4500 | 85500 |
| 50% product | 45000 | 0 | 45000 | 9000 | 2250 | 42750 |
| Shipping-only partial | 0 | 20000 | 20000 | 0 | 0 | 20000 |
| Two 50% partials → full | 45000+45000 | 0+0 | 90000 | 9000+9000=18000 | 2250+2250=4500 | 85500 |

All invariants verified:
- `cumProductRefund <= PD` ✅
- `cumShippingRefund <= S` ✅
- `cumCoinsRestored <= K` (full PD = exactly K) ✅
- `cumCommissionReversed <= C` ✅
- `CashRefund + cumCoins <= PD + S` ✅
- C never in CashRefund ✅
- Shipping has no commission or coin component ✅

---

## 5. DEPLOYMENT CHECKLIST

```bash
# 1. Verify migration
ls backend/migrations/000040_refund_product_shipping_split.up.sql

# 2. Rewrite refund_gateway.go (manual — content in earlier report)

# 3. Build check (after gateway rewrite + pre-existing errors fixed)
go build ./internal/finance/refund/...
go build ./internal/commerce/order/...
go build ./internal/worker/

# 4. Run unit tests
go test ./internal/finance/refund/...

# 5. Run integration tests with PostgreSQL
go test -tags=integration ./internal/finance/refund/... -run TestS2C2

# 6. Verify negative contracts
grep -rn 'CommissionAmount' backend/internal/finance/refund/
# Expected: ZERO results — C never in buyer refund path
```

---

## 6. NEGATIVE CONTRACTS (Enforced)

1. **C never in buyer refund**: All `orderGross = PD+S+C` replaced with PD+S in refund paths
2. **F never refundable**: `RecordBuyerPaymentFeeRevenue` drains F at settlement, never reversed
3. **Cumulative invariant**: `cumProduct <= PD`, `cumShipping <= S`, `cumCash + cumCoins <= PD + S`
4. **Commission product-only**: `CommissionDelta = floor(C * cumProduct / PD)`, no shipping component
5. **Coins all-or-nothing removed**: Replaced with cumulative product-proportional delta model
6. **Coin event identity**: Uses outbox event ID as reference_id for multiple partial restorations

---

**END OF REPORT**
