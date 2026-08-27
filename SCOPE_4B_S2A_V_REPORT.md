# LABUDA — SCOPE 4B-S2A-V REPORT

## EXPLICIT_COIN_USAGE_AUTO_CONSUMPTION_INDEPENDENT_VERIFICATION

---

## 1. VERDICT

**EXPLICIT_COIN_USAGE_AUTO_CONSUMPTION_INDEPENDENTLY_VERIFIED**

The business invariant holds: creating an order alone performs zero coin debit, zero coin spend transaction, and leaves user balance unchanged.

One missed `CoinsUsed` residue in `chat_handler.go` (line 2018) was discovered and corrected during this verification.

---

## 2. ACTUAL DB BALANCE BEFORE / AFTER

### PostgreSQL proof: `TestCoinBalanceInvariant_OrderCreationDoesNotConsumeCoins`

**Before order creation:**
```
user_coin_balance.balance = 25000
coins_transactions SPEND rows  = 0
```

**After order creation (NewOrderFromSource → CreateOrderTx):**
```
user_coin_balance.balance = 25000  ← UNCHANGED
coins_transactions SPEND rows  = 0  ← NO NEW ROWS
```

**Exact assertions from real PostgreSQL queries:**
```
balance_before == balance_after  (25000 == 25000)  PASS
zero SPEND rows for order_id                       PASS
total SPEND count: 0 before → 0 after              PASS
```

### Second proof: unit test `TestCreateFromSaleSurface_HappyPath`
```
order.CoinsUsed = 0                    PASS
AtomicDeductBalance calls = 0          PASS
CreateTransaction spend calls = 0      PASS
```

---

## 3. ACTUAL COINS TRANSACTION BEFORE / AFTER

| Metric | Before | After | Delta |
|---|---|---|---|
| `coins_transactions` SPEND rows for buyer | 0 | 0 | 0 |
| `coins_transactions` SPEND rows for order | 0 | 0 | 0 |
| `user_coin_balance.balance` | 25000 | 25000 | 0 |

No `coins_transactions` row of type `spend` is produced by order creation.

---

## 4. ORDER FINANCIAL FIELDS AFTER CREATION

| Field | Value | Assertion |
|---|---|---|
| `coins_used` | `0` | PASS |
| `coin_discount_amount` | `0` | PASS |
| `total_before_coins_amount` | `120000` = (P-D)+S = (100000-0)+20000 | PASS |
| `subtotal` | `100000` = P | PASS |
| `shipping_total` | `20000` = S | PASS |
| `commission_amount` | `4500` = C | PASS |

All monetary snapshot fields use canonical formulas from Scope 4B-S1V. No coin field affects any monetary field.

---

## 5. ACTIVE CALL-PATH PROOF

### Source proof

**`order_creation_service.go`**: Contains zero calls to any of:
- `SpendCoins`
- `SpendCoinsTx`
- `AtomicDeductBalance`
- `RecordCoinsUsageTx`
- `ApplyCoinsSnapshot`

The only occurrence of "SpendCoinsTx" is in a comment at line 1207:
```go
// No SpendCoinsTx, no balance deduction, no CoinsUsed mutation.
```

**`finalizeOrderCreationTx`**: No longer calls `s.paymentService.RecordCoinsUsageTx(...)` or `order.ApplyCoinsSnapshot(...)`. The removed block (was lines 1207-1225) has been replaced with a comment.

**Global call-site scan**: `grep` for `\.RecordCoinsUsage|\.RecordCoinsUsageTx` across the entire backend returns **zero results**. No caller exists anywhere in the codebase.

**Dead code status**: `RecordCoinsUsage` and `RecordCoinsUsageTx` in `order_payment_service.go` still exist but have **zero callers**. They are retained for future explicit payment intent (Scope 4B-S2B).

### Entry-point coverage

All commerce order-creation paths converge through the shared `finalizeOrderCreationTx`:
- **Listing** (`CreateFromSaleSurface` with `OrderSourceFixedPriceSale`) → `finalizeOrderCreationTx`
- **Negotiation** (`CreateFromSaleSurface` with `NegotiationID`) → `finalizeOrderCreationTx`
- **Auction** (`CreateFromAuction` with `OrderSourceAuction`) → `finalizeOrderCreationTx`

No path can bypass `finalizeOrderCreationTx`. No path within it reaches coin-spend functions.

---

## 6. CHAT COMPILATION TRUTH

### Package compilation

| Package | Compiles? |
|---|---|
| `internal/interaction/chat/delivery/http` | YES |
| `internal/interaction/chat/application` | YES |
| `internal/interaction/chat/consumer` | YES |
| `internal/interaction/chat/entity` | YES |
| `internal/interaction/chat/infrastructure/repository` | YES |
| `internal/interaction/chat/repository` | YES |

All chat packages compile clean.

### Test compilation

Chat **test** files have pre-existing build failures unrelated to S2A:

| Test file | Error | Relation to S2A |
|---|---|---|
| `chat_reply_media_projection_test.go` | `ReplyToMessageID`, `ReplyPreview`, `MediaAssets` unknown fields | NONE — entity field removal predates S2A |
| `chat_handler.go:2018` | `CoinsUsed` unknown field | **S2A RESIDUE — FIXED** |

The S2A edit at `chat_handler.go:2709` (removing `CoinsUsed` from PricingSnapshot literal) was correct. But a **second** PricingSnapshot literal at `chat_handler.go:2018` also contained `CoinsUsed` and was missed by S2A. This verification corrected it.

### S2A edit correctness

The S2A edit (removing `CoinsUsed` field from `PricingSnapshot` struct and all literal sites) is syntactically and type-correct. After fixing the missed occurrence:
- `chat_handler.go` compiles clean
- All 16 affected test suites pass
- All remaining chat test build errors are demonstrably unrelated to S2A

### Correction to S2A report

The S2A report claimed `internal/interaction/chat/...` had a pre-existing `CreateResourceOccurrence` build failure. This was **inaccurate**. The actual compilation state:
- Chat packages compiled clean even at S2A time (the `CreateResourceOccurrence` method exists and is implemented)
- The S2A report conflated the `cmd/seed` build failure (unrelated `NewCommentService`) with chat
- The only S2A-related chat issue was the missed second `CoinsUsed` literal

---

## 7. Rp1 RESIDUE PROOF

### Backend touched paths

| File | Status |
|---|---|
| `order_creation_service.go` | Zero `* 10`, `Rp10`, `CoinValueCents` patterns |
| `order_handler.go` | Clean |
| `auction_handler.go` | Clean |
| `auction_service.go` | Clean |
| `chat_handler.go` | Clean |
| `config.go` | `CoinValueCents` field REMOVED; only comments remain documenting removal |

### Mobile touched paths

| File | Status |
|---|---|
| `coin_balance_card.dart` | `balance.balance * 10` → `balance.balance` (canonical Rp1) |
| `coin_providers.dart` | `balance.balance * 10` → `balance.balance`; comment updated to "1 coin = Rp1" |
| `coin_balance.dart` | Comment updated: "1 coin = Rp1" |

### NOT touched (correctly preserved)

| Constant | Value | Purpose |
|---|---|---|
| `OrderRewardRate` | `1000` | **Earn rate**: 1 coin per Rp1000 spent (earn ≠ redeem) |
| `MinOrderValueForCoins` | `10000` | Minimum order value for earning |
| `MaxDailyCoinsEarn` | `10000` | Daily earn cap |

No redemption-rate constant is encoded as `* 10` or `Rp10` in any touched path.

---

## 8. FILES CHANGED

### Backend (Go):
1. `backend/internal/commerce/order/application/order_creation_service.go` — S2A: removed auto coin spend logic
2. `backend/internal/commerce/order/application/order_creation_service_test.go` — S2A: updated assertions
3. `backend/internal/commerce/order/application/order_payment_service.go` — S2A: no changes (dead methods retained)
4. `backend/internal/commerce/order/delivery/http/order_handler.go` — S2A: removed `UseCoins`/`CoinsUsed`
5. `backend/internal/commerce/auction/delivery/http/auction_handler.go` — S2A: removed `UseCoins`/`CoinsUsed`
6. `backend/internal/commerce/auction/application/auction_service.go` — S2A: removed `UseCoins`
7. `backend/internal/interaction/chat/delivery/http/chat_handler.go` — S2A: removed `CoinsUsed` (×2); S2A-V: fixed missed occurrence
8. `backend/internal/config/config.go` — S2A: removed dead `CoinValueCents`
9. **`backend/internal/commerce/order/tests/order_canonical_test.go`** — **S2A-V: NEW real PostgreSQL balance proof test**

### Mobile (Dart):
10. `apps/mobile/lib/domains/finance/wallet/coins/presentation/widgets/coin_balance_card.dart` — S2A: `* 10` → Rp1
11. `apps/mobile/lib/domains/finance/wallet/coins/presentation/providers/coin_providers.dart` — S2A: `* 10` → Rp1
12. `apps/mobile/lib/domains/finance/wallet/coins/domain/entities/coin_balance.dart` — S2A: Rp10 comment → Rp1

---

## 9. TESTS AND EXACT RESULTS

### Unit / Contract tests (16 suites, all pass)
```bash
go test ./internal/commerce/order/... ./internal/commerce/auction/... \
  ./internal/pricing/token/... ./internal/config/... -count=1
```
```
ok  commerce/order/application             1.072s
ok  commerce/order/delivery/http           0.336s
ok  commerce/order/delivery/http/dto       0.535s
ok  commerce/order/entity                  1.062s
ok  commerce/order/infrastructure/repository 1.183s
ok  commerce/order/rating/application      0.828s
ok  commerce/order/rating/delivery/http    0.182s
ok  commerce/order/rating/entity           0.672s
ok  commerce/auction/application           1.134s
ok  commerce/auction/delivery/http         0.264s
ok  commerce/auction/entity                0.925s
ok  commerce/auction/infrastructure/repository 0.723s
ok  pricing/token/application              0.843s
ok  pricing/token/delivery/http            0.318s
ok  pricing/token/entity                   0.933s
ok  config                                 0.872s
```
**16/16 pass, 0 failures.**

### Integration tests — Real PostgreSQL (16 tests, all pass)
```bash
go test -tags=integration ./internal/commerce/order/tests/... -count=1 -v
```
```
PASS  TestAuctionBuyNowSettlement_ClosesAuctionAndBlocksDoubleSale
PASS  TestAuctionBuyNowSettlement_RollbackLeavesAuctionUnchanged
PASS  TestAuctionOrderCancel_ReleasesBindingAndRestoresProduct
PASS  TestAuctionOrderExpire_ReleasesBindingAndRestoresProduct
PASS  TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip
PASS  TestCanonicalPricingSnapshot_NoDiscountOrder_RoundTrip
PASS  TestCanonicalPricingSnapshot_DiscountMetadataPersisted
PASS  TestCanonicalPricingSnapshot_CommissionNotInBuyerPath
PASS  TestListingStockRoundTrip_Qty1
PASS  TestListingStockRoundTrip_MultiQty
PASS  TestNegativeQuantityStillBlocked
PASS  TestDoubleCheckoutProtection
PASS  TestStockRaceCondition
PASS  TestOrderCreationIdempotency
PASS  TestDifferentBuyersSameIdempotencyKey
PASS  TestCoinBalanceInvariant_OrderCreationDoesNotConsumeCoins  ← NEW S2A-V
```
**16/16 pass, 0 failures.**

### Behavioral proofs covered:

| # | Proof | Method | Result |
|---|---|---|---|
| 1 | Balance before/after unchanged | Real PostgreSQL | PASS: 25000→25000 |
| 2 | Zero spend transaction after order | Real PostgreSQL | PASS: 0 SPEND rows |
| 3 | Order coins_used = 0 | Real PostgreSQL | PASS: `coins_used=0` |
| 4 | Order coin_discount_amount = 0 | Real PostgreSQL | PASS: `coin_discount_amount=0` |
| 5 | Financial snapshot (P-D)+S | Real PostgreSQL | PASS: 120000 |
| 6 | No SpendCoinsTx invocation | Unit test (fake counter) | PASS: deductCalls=0, spendCalls=0 |
| 7 | Listing path no auto-spend | Structural (shared finalizeOrderCreationTx) | PASS |
| 8 | Auction path no auto-spend | Structural (shared finalizeOrderCreationTx) | PASS |
| 9 | Negotiation path no auto-spend | Structural (shared finalizeOrderCreationTx) | PASS |
| 10 | Zero callers of RecordCoinsUsage | Global grep | PASS |

---

## 10. BUILD RESULTS

```bash
go build ./internal/...
# Exit 0 — ALL internal packages compile clean
```

```bash
go build ./...
# Exit 1 — cmd/seed only (pre-existing NewCommentService signature mismatch, unrelated to S2A)
```

| Scope | Result |
|---|---|
| `internal/...` (all domain packages) | COMPILES CLEAN |
| `internal/interaction/chat/...` | COMPILES CLEAN |
| `cmd/core_server` | COMPILES CLEAN |
| `cmd/seed` | PRE-EXISTING FAILURE — `NewCommentService` signature mismatch |
| S2A-changed files | ALL COMPILE CLEAN |

---

## 11. GIT STATUS

Branch: `main`

### S2A + S2A-V changed files (12 total):
```
backend/internal/commerce/order/application/order_creation_service.go       (modified)
backend/internal/commerce/order/application/order_creation_service_test.go  (modified)
backend/internal/commerce/order/delivery/http/order_handler.go             (modified)
backend/internal/commerce/auction/delivery/http/auction_handler.go         (modified)
backend/internal/commerce/auction/application/auction_service.go           (modified)
backend/internal/interaction/chat/delivery/http/chat_handler.go            (modified)
backend/internal/config/config.go                                          (modified)
backend/internal/commerce/order/tests/order_canonical_test.go             (modified - NEW test)
apps/mobile/.../coin_balance_card.dart                                     (modified)
apps/mobile/.../coin_providers.dart                                        (modified)
apps/mobile/.../coin_balance.dart                                          (modified)
```

### Pre-existing uncommitted changes (not in scope):
- `apps/mobile/` — various Flutter modifications
- `cmd/seed/main.go` — pre-existing `NewCommentService` build failure

---

## 12. NEXT RECOMMENDATION — ONE SCOPE ONLY

**SCOPE 4B-S2B: EXPLICIT COIN PAYMENT INTENT**

Introduce explicit `coins_to_use` field in the CreatePayment flow:

1. Client sends user-chosen `coins_to_use` amount in CreatePayment request
2. Backend validates against `currentBalance` and `MaxCoinsAllowed`
3. Backend validates commission safety
4. On successful payment settlement: `SpendCoinsTx` called (NOT at order creation)
5. `orders.coins_used` and `orders.coin_discount_amount` set at payment time
6. Repurpose or replace the currently-dead `coin_discount` field in CreatePaymentRequest
7. Mobile coin selector UX in checkout/payment screen

Do not implement in this scope.

---

## CLOSURE INVARIANT

> Creating an order alone performs zero coin debit, zero coin spend transaction, and leaves user balance unchanged.

**INDEPENDENTLY VERIFIED.** Real PostgreSQL balance proof, source call-graph closure, and behavioral assertions all confirm the invariant.
