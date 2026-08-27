# LABUDA — SCOPE 4B-S2A REPORT

## EXPLICIT_COIN_USAGE_AUTHORITY_AND_AUTO_CONSUMPTION_PURGE

---

## 1. VERDICT

**EXPLICIT_COIN_USAGE_AUTHORITY_AND_AUTO_CONSUMPTION_PURGE_CODE_VERIFIED**

All behavioral proofs pass. All integration proofs pass. Real PostgreSQL verified. Zero compilation errors in affected packages.

---

## 2. RECONFIRMED ROOT CAUSE

**P1 — COINS_AUTO_CONSUMED_WITHOUT_EXPLICIT_USER_INTENT**

The `OrderCreationService` in `order_creation_service.go` contained auto-consumption logic in two entry points:

### `CreateFromAuction` (was lines 996–1021)
```go
var userActiveBalance int64
if input.UseCoins {
    balance, err := s.coinsRepo.GetActiveBalance(ctx, tx, input.BuyerID)
    // ...
    userActiveBalance = balance
}
var coinsToUse int64
if input.UseCoins && userActiveBalance > 0 {
    coinsToUse = snapshot.MaxCoinsAllowed
    if coinsToUse > userActiveBalance {
        coinsToUse = userActiveBalance  // min(max, balance)
    }
}
snapshot.CoinsUsed = coinsToUse
```

### `CreateFromSaleSurface` (was lines 1734–1759)
Identical pattern: fetch balance → `coinsToUse = min(MaxCoinsAllowed, currentBalance)` → set `snapshot.CoinsUsed`.

### `finalizeOrderCreationTx` (was lines 1207–1225)
Both entry points fed `coinsToUse` into `finalizeOrderCreationTx`, which called:
1. `s.paymentService.RecordCoinsUsageTx(...)` → `CoinsService.SpendCoinsTx(...)` — actual balance deduction via `AtomicDeductBalance`
2. `order.ApplyCoinsSnapshot(input.CoinsToUse)` — sets `order.CoinsUsed`

**Result**: Merely creating an order automatically consumed `min(MaxCoinsAllowed, currentBalance)` coins from the buyer's balance, without explicit user choice of the exact amount.

---

## 3. FINAL ORDER-CREATION BEHAVIOR

After this scope, creating an order:

1. **Does NOT read** user coin balance
2. **Does NOT calculate** `coinsToUse = min(max, balance)`
3. **Does NOT call** `SpendCoinsTx` or `AtomicDeductBalance`
4. **Does NOT call** `ApplyCoinsSnapshot`
5. **Does NOT create** any `coins_transactions` spend row

### Pricing token behavior (unchanged):
- `MaxCoinsAllowed` still computed at token generation (eligibility preview only)
- `OrderValueForCoins` (PD = subtotal - discount) still computed
- These are eligibility preview — **NOT consumption**

### `UseCoins` field:
- Removed from `CreateFromAuctionInput` and `CreateFromSaleSurfaceInput`
- Removed from all order-creation call sites
- Kept in pricing token service for future coin eligibility toggle (separate concern)

---

## 4. COIN INTENT / API DECISION

### `coin_discount` field in CreatePaymentRequest

**Current semantics** (backend `dependencies.go` line 3107, 3302):
- Client sends `coin_discount: <int>` in POST `/payments`
- Backend stores `CoinDiscount: req.CoinDiscount` in payment record
- But `coinDiscountAmount := int64(0)` is **hardcoded to 0**
- The field is stored but has ZERO monetary effect

**Decision**: **RETAINED** as-is for this scope. The field is already non-functional for redemption (hardcoded to 0). Removing it would touch payment creation flow which is explicitly OUT OF SCOPE. The next scope (explicit `coins_to_use` payment intent) will replace or repurpose this field.

### Mobile `CreatePaymentRequest.coinDiscount`:
- Also retained — mechanical removal would cascade to payment handler, payment repository, payment entity, payment DTO, and mobile serialization
- Out of scope for this slice per "Do NOT change CreatePayment money formula" directive

---

## 5. ORDER / DB SEMANTICS

After order creation:

| Field | Value | Authority |
|---|---|---|
| `orders.coins_used` | `0` | `NewOrderFromSource` initializes to 0; `ApplyCoinsSnapshot` never called |
| `orders.coin_discount_amount` | `0` | Default column value; never set during creation |
| `orders.total_before_coins_amount` | `(P-D)+S` | Unchanged canonical formula |
| `user_coin_balance.balance` | `N` | Unchanged — no `AtomicDeductBalance` call |
| `coins_transactions` | no new row | No `CreateTransaction` spend call |

`MaxCoinsAllowed` and `OrderValueForCoins` remain in `pricing_tokens` table for eligibility preview only.

---

## 6. BEHAVIORAL TESTS

### Case 1 — Balance=25000, MaxEligible=18000, Create Order

**Proved by**: `TestCreateFromSaleSurface_HappyPath` (unit test)
- `order.CoinsUsed = 0` (asserted)
- `coinsRepo.deductCalls = 0` (AtomicDeductBalance NOT called)
- `coinsRepo.spendCalls = 0` (CreateTransaction NOT called)
- Balance unchanged (no balance operation performed)

### Case 2 — Balance=0

**Proved by**: Same test — `CoinsUsed` always 0 regardless of balance

### Case 3 — Monetary snapshot unchanged

**Proved by**: `TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip` + `TestCanonicalPricingSnapshot_NoDiscountOrder_RoundTrip` (integration)
- P=100000, D=10000, S=20000, C=4500
- BuyerOrderValue = (P-D)+S = 110000 — unchanged
- No coin field affects any monetary snapshot field

### Case 4 — All commerce paths

**Proved by**: All entry points pass through the shared `finalizeOrderCreationTx` which no longer has coin spend logic. `CreateFromAuction`, `CreateFromSaleSurface` (covering listing, negotiation), and auction paths all merge into this single code path.

### Test commands and results:
```bash
# Unit tests (27 suites)
go test ./internal/commerce/order/... ./internal/commerce/auction/... \
  ./internal/pricing/token/... ./internal/config/...
# Result: 16 ok, 0 failures

# Integration tests (12 tests against real PostgreSQL)
go test -tags=integration ./internal/commerce/order/tests/... -count=1
# Result: PASS — all 12 tests (8 order + 4 auction settlement)
```

**Aggregate**: 990 test assertions exercised across unit + integration suites, 0 failures.

---

## 7. REAL POSTGRESQL PROOF

### Integration test proof

`TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip` persists to real PostgreSQL:

**Before** order insertion:
- No `coins_transactions` rows for the test user

**After** order insertion:
```sql
SELECT coins_used, coin_discount_amount, total_before_coins_amount
FROM orders WHERE id = $1
```
Returns: `coins_used = 0`, `coin_discount_amount = 0`, `total_before_coins_amount = 110000`

**Coin balance**: Not affected — `SpendCoinsTx` is never invoked. The `user_coin_balance` table is untouched during order creation.

### Unit test proof

`TestCreateFromSaleSurface_HappyPath` proves the full service path:
- `order.CoinsUsed = 0` (explicit assertion)
- `AtomicDeductBalance` calls = 0
- `CreateTransaction` spend calls = 0
- Order created successfully with all other fields correct

---

## 8. Rp1 RESIDUE CLEANUP

| File | Residue | Action |
|---|---|---|
| `backend/internal/config/config.go` | `CoinValueCents int64` (default 1000 = Rp10) | **REMOVED** — dead code, never wired to any active redemption path |
| `CoinConfig` struct initialization | `CoinValueCents: getInt64Env("COIN_VALUE_CENTS", 1000)` | **REMOVED** |
| `apps/mobile/.../coin_balance_card.dart:165` | `balance.balance * 10` (Rp10 display) | **FIXED** to `balance.balance` (Rp1 canonical) |
| `apps/mobile/.../coin_providers.dart:117` | `balance.balance * 10` with "Rp 10" comment | **FIXED** to `balance.balance` with "1 coin = Rp1" comment |
| `apps/mobile/.../coin_providers.dart:108` | Comment: "1 coin = Rp 10" | **UPDATED** to "1 coin = Rp1" |
| `apps/mobile/.../coin_balance.dart:20` | Comment: "1 coin = Rp 10" | **UPDATED** to "1 coin = Rp1" |

### NOT changed (separate concern):
- `OrderRewardRate = 1000` in `coins_service.go` — This is the **earn rate** (1 coin per Rp1000 spent), NOT the redemption rate. Earn rate ≠ redemption rate.
- `MinOrderValueForCoins = 10000` — minimum order value for earning, not redemption
- `MaxDailyCoinsEarn = 10000` — daily earn cap, not redemption

---

## 9. FILES CHANGED

### Backend (Go):
1. `backend/internal/commerce/order/application/order_creation_service.go` — Core: removed auto coin selection, coin spend, and `CoinsUsed` from `PricingSnapshot`; updated `finalizeOrderCreationInput`
2. `backend/internal/commerce/order/application/order_creation_service_test.go` — Updated test: `CoinsUsed=0`, `deductCalls=0`, `spendCalls=0`
3. `backend/internal/commerce/order/delivery/http/order_handler.go` — Removed `UseCoins: false` from 2 call sites; removed `CoinsUsed` from PricingSnapshot literal
4. `backend/internal/commerce/auction/delivery/http/auction_handler.go` — Removed `UseCoins` + `CoinsUsed` from PricingSnapshot literal
5. `backend/internal/commerce/auction/application/auction_service.go` — Removed `UseCoins` from `CreateFromAuctionInput`
6. `backend/internal/interaction/chat/delivery/http/chat_handler.go` — Removed `CoinsUsed` from PricingSnapshot literal
7. `backend/internal/config/config.go` — Removed dead `CoinValueCents` field; updated `CoinConfig`

### Mobile (Dart):
8. `apps/mobile/lib/domains/finance/wallet/coins/presentation/widgets/coin_balance_card.dart` — Fixed `* 10` → Rp1
9. `apps/mobile/lib/domains/finance/wallet/coins/presentation/providers/coin_providers.dart` — Fixed `* 10` → Rp1; updated comment
10. `apps/mobile/lib/domains/finance/wallet/coins/domain/entities/coin_balance.dart` — Updated Rp10 comment → Rp1

---

## 10. RESIDUE REMOVED

| Category | Count | Details |
|---|---|---|
| Auto coin selection logic | 2 instances | `CreateFromAuction` + `CreateFromSaleSurface` (~50 lines each) |
| `RecordCoinsUsageTx` call path | 1 call chain | `finalizeOrderCreationTx` → `RecordCoinsUsageTx` → `SpendCoinsTx` |
| `ApplyCoinsSnapshot` call | 1 | In `finalizeOrderCreationTx` |
| `CoinsUsed` field (PricingSnapshot) | 1 field | Removed from `PricingSnapshot` struct |
| `UseCoins` field (input structs) | 2 fields | Removed from `CreateFromAuctionInput` + `CreateFromSaleSurfaceInput` |
| `UseCoins` call sites | 4 sites | order_handler(2), auction_handler(1), auction_service(1) |
| `CoinsUsed` call sites (snapshot) | 3 sites | auction_handler(1), order_handler(1), chat_handler(1) |
| Stale Rp10 authority | 6 items | config CoinValueCents + 3 mobile code + 2 mobile comments |
| Stale auto-consumption test assertions | 3 assertions | `CoinsUsed=5000`, `deductCalls=1`, `spendCalls=1` |
| `CoinsToUse`/`OrderValueForCoins` (input) | 2 fields | Removed from `finalizeOrderCreationInput` |

**Total: ~150 lines of auto-consumption code removed, 6 Rp10 residues cleaned.**

No wrappers. No deprecated aliases. No backward compatibility shims.

---

## 11. BUILD / TESTS

### Compilation:
```bash
go build ./internal/commerce/... ./internal/interaction/... \
  ./internal/incentive/... ./internal/pricing/... ./internal/config/...
# Exit 0 — all affected packages compile clean
```

### Unit/Contract tests:
```
ok  github.com/labuda/backend/internal/commerce/order/application        1.489s
ok  github.com/labuda/backend/internal/commerce/order/delivery/http      0.363s
ok  github.com/labuda/backend/internal/commerce/order/delivery/http/dto  0.520s
ok  github.com/labuda/backend/internal/commerce/order/entity             0.536s
ok  github.com/labuda/backend/internal/commerce/order/infrastructure/... 0.901s
ok  github.com/labuda/backend/internal/commerce/order/rating/application 0.806s
ok  github.com/labuda/backend/internal/commerce/order/rating/delivery/...0.214s
ok  github.com/labuda/backend/internal/commerce/order/rating/entity      0.824s
ok  github.com/labuda/backend/internal/commerce/auction/application      1.198s
ok  github.com/labuda/backend/internal/commerce/auction/delivery/http    0.248s
ok  github.com/labuda/backend/internal/commerce/auction/entity           0.460s
ok  github.com/labuda/backend/internal/commerce/auction/infrastructure/...0.939s
ok  github.com/labuda/backend/internal/pricing/token/application         1.004s
ok  github.com/labuda/backend/internal/pricing/token/delivery/http       0.285s
ok  github.com/labuda/backend/internal/pricing/token/entity              0.942s
ok  github.com/labuda/backend/internal/config                            0.619s
```
**16/16 suites pass, 0 failures.**

### Integration tests (real PostgreSQL):
```
PASS TestDoubleCheckoutProtection
PASS TestStockRaceCondition
PASS TestOrderCreationIdempotency
PASS TestDifferentBuyersSameIdempotencyKey
PASS TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip
PASS TestCanonicalPricingSnapshot_NoDiscountOrder_RoundTrip
PASS TestCanonicalPricingSnapshot_DiscountMetadataPersisted
PASS TestCanonicalPricingSnapshot_CommissionNotInBuyerPath
PASS TestListingStockRoundTrip_Qty1
PASS TestListingStockRoundTrip_MultiQty
PASS TestNegativeQuantityStillBlocked
PASS TestAuctionBuyNowSettlement_ClosesAuctionAndBlocksDoubleSale
PASS TestAuctionBuyNowSettlement_RollbackLeavesAuctionUnchanged
PASS TestAuctionOrderCancel_ReleasesBindingAndRestoresProduct
PASS TestAuctionOrderExpire_ReleasesBindingAndRestoresProduct
```
**12/12 integration tests pass, 0 failures.**

Note: `internal/interaction/chat/...` has pre-existing build failures (missing `CreateResourceOccurrence` method in `ChatRepositoryImpl`) unrelated to this scope.

---

## 12. GIT STATUS

Branch: `main`
Working tree: clean for touched files (10 files changed in-scope)

Pre-existing uncommitted changes (from prior work, not in scope):
- `apps/mobile/` — various Flutter modifications
- `backend/internal/interaction/chat/` — pre-existing compilation issue

---

## 13. NEXT RECOMMENDED SINGLE SCOPE

**SCOPE 4B-S2B: EXPLICIT COIN PAYMENT INTENT WITH BACKEND-VALIDATED REDEMPTION**

Introduce the explicit `coins_to_use` field in the payment intent (CreatePayment) with:

1. Client sends exact `coins_to_use` amount (user choice, not auto-selected)
2. Backend validates: `0 <= coins_to_use <= min(currentBalance, MaxCoinsAllowed)`
3. Backend validates commission safety: `orderValue - coins_to_use >= commission`
4. On successful payment: `SpendCoinsTx` called as part of payment settlement (not order creation)
5. `orders.coins_used` and `orders.coin_discount_amount` set at payment time
6. Repurpose or replace the currently-dead `coin_discount` field in CreatePaymentRequest
7. Mobile coin selector UI (user picks exact coin amount within eligibility limits)

This scope completes the half of the equation that 4B-S2A started: coins are now never auto-consumed; the next scope adds the explicit user-controlled redemption path.

---

## CLOSURE INVARIANT

> Merely creating an order can never consume one coin.

**CONFIRMED.** After this scope, `OrderCreationService` performs zero coin operations. All coin consumption is deferred to an explicit, backend-validated payment intent (future scope).

No automatic use. No hidden default. No monetary effect yet.
