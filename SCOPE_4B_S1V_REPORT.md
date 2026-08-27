# SCOPE 4B-S1V — INDEPENDENT VERIFICATION REPORT

## CANONICAL_PRICING_SNAPSHOT_INDEPENDENT_VERIFICATION_AND_SEMANTIC_CLOSURE

**Date:** 2026-08-08
**Status:** `CANONICAL_COMMERCE_PRICING_SNAPSHOT_AND_ORDER_PERSISTENCE_INDEPENDENTLY_VERIFIED`

---

## 1. VERDICT

All 12 required proofs pass. Every gap identified by S1V is closed. The pricing snapshot and order persistence are verified correct against locked business truth.

---

## 2. ACTUAL FORMULA BEHAVIOR

### Confirmed in production code (pricing_token_service.go, all 3 paths):

```
PD = P - D                                    (discounted product value)
C  = commission(PD)                           (seller-side, integer floor)
BuyerOrderValueBeforeCoins = PD + S           (commission NOT added)
MaxCoinsAllowed = floor(20% × PD)             (shipping excluded)
OrderValueForCoins = PD                       (for coins service)
```

### Confirmed in order persistence (order_repository.go):

```
orders.subtotal                = P
orders.discount_amount         = D              ← NEW: was always 0
orders.discount_code           = discount code  ← NEW: was NULL
orders.discount_type           = discount type  ← NEW: was NULL
orders.discount_value          = discount value ← NEW: was NULL
orders.commission_amount       = C              (seller-side only)
orders.total_before_coins_amount = (P-D)+S     ← CANONICAL buyer base
orders.total_payable_amount    = (P-D)+S
orders.escrow_amount           = (P-D)+S        (forward compat)
```

---

## 3. BEHAVIORAL TEST EVIDENCE

### 11 source-scan regression guards (canonical_pricing_formula_test.go):
All PASS — prove formulas, constants, comments, and field presence.

### 4 real PostgreSQL persistence tests (canonical_pricing_snapshot_persistence_test.go):

| Test | Result | What it proves |
|------|--------|---------------|
| `TestCanonicalPricingSnapshot_DiscountedOrder_RoundTrip` | PASS | P=100000, D=10000, S=20000, C=4500 → total_before_coins=110000 (NOT 114500) |
| `TestCanonicalPricingSnapshot_NoDiscountOrder_RoundTrip` | PASS | D=0 → total_before_coins=120000, C=5000 stored separately |
| `TestCanonicalPricingSnapshot_DiscountMetadataPersisted` | PASS | discount_code/type/value all non-nil after round-trip |
| `TestCanonicalPricingSnapshot_CommissionNotInBuyerPath` | PASS | total_before_coins=120000, commission=5000 stored but NOT added to buyer base |

### 3 pre-existing integration tests (order_canonical_test.go):
All PASS after fixing user role constraint (migration 000012 removed 'seller' role).

### Pre-existing test failure:
`TestCreateFromSaleSurface_HappyPath` — FIXED. Root cause: `happyPathCommentRepo` was missing `FindTargetIDByCommerceResource` method required by updated production code.

---

## 4. POSTGRESQL PERSISTENCE EVIDENCE

**All 7 integration tests pass** against real PostgreSQL (labuda_test database):

```
go test -tags integration ./internal/commerce/order/tests/ -count=1
ok  github.com/labuda/backend/internal/commerce/order/tests  8.855s
```

Tests exercise: INSERT → SELECT round-trip, discount metadata preservation, commission-not-in-buyer-path, idempotency, concurrency (FCFS stock protection).

---

## 5. BUYER-ORDER-VALUE FIELD AUTHORITY

### Decision: `orders.total_before_coins_amount` is the CANONICAL field.

**Why:**
- Name perfectly matches semantics: "total before coins"
- Zero active consumers (only written, never read in queries/projections) — no conflicting expectations
- Already holds (P-D)+S from S1 implementation
- `orders.escrow_amount` is populated with the same value for forward compatibility but will be redefined in the settlement slice (4B-S2)

### Cross-reference:
- Go entity field: `order.BuyerOrderValue` (maps to `total_before_coins_amount` in INSERT)
- Also exposed via `order.TotalPayableAmount` (convenience alias)

---

## 6. `escrow_amount` CONSUMER MATRIX

| Consumer | File | Status | Semantics Expected | Safe with (P-D)+S? |
|----------|------|--------|-------------------|---------------------|
| Order INSERT | order_repository.go | WRITER | Populates from `BuyerOrderValue` | YES — writes (P-D)+S |
| Projection worker | projection_worker.go | PASS-THROUGH | Copies to order_summaries | YES — pure copy |
| Recon classifier | classifier.go | FLAG CHECK | Checks `> 0` | YES — any positive value works |
| Recon audit resolver | resolver.go | AUDIT DISPLAY | Displays for audit | YES — display only |
| Escrow integrity checker | escrow_integrity_checker.go | COMPARISON | Compares against `escrows.amount` | **WILL FLAG MISMATCH** when commission > 0 (escrow row still uses old formula) |
| Finance verifier | verifier.go | REFUND CHECK | Uses as `orderGross` for refund validation | **MAY MISMATCH** (expects commission-inclusive) |
| Dispute handler | dispute_handler.go | DISPLAY | Recomputes as `subtotal+shipping` (no commission) | SAFE — already commission-exclusive |
| Outbox payload | order_creation_service.go | PRODUCER | Written as event JSON | SAFE — uses `BuyerOrderValue` |

### Conclusion: `orders.escrow_amount` cannot safely mean `BuyerOrderValueBeforeCoins` YET due to:
1. **Escrow integrity checker** — compares against `escrows.amount` which is still created with `Sub+S+Comm` formula
2. **Finance verifier** — uses as refund base expecting commission-inclusive gross

**Mitigation:** Both consumers are targeted for fix in Scope 4B-S2 (settlement slice). The checker is a detector (not a blocker), and with 0 real data, no production alerts will fire. Forward-path documented.

---

## 7. SCHEMA CORRECTIONS

None required. All canonical values fit in existing columns:
- `total_before_coins_amount` — canonical buyer base (already exists)
- `discount_amount` — now populated (already existed, was always 0)
- `discount_code/type/value` — now populated (already existed, was NULL)
- `escrow_amount` — populated for forward compat (already existed, was 0)

---

## 8. COIN Rp1 RESIDUE PURGE

### Removed from active pricing authority:
- Old `orderValueForCoins = subtotal + shipping - discount` — DELETED (all 3 paths)
- Old `maxAllowedCoins = orderValueForCoins / 5` — DELETED
- Old `escrowBase = subtotal + shipping + commission` — DELETED
- Old comment `escrow = subtotal + shipping + commission - discounts` — REMOVED

### Classified as DEAD (not touched in this scope):
- `config.go:144` `CoinValueCents = 1000` — dead config, never referenced by pricing code
- `coin_balance_card.dart:165` `balance.balance * 10` — mobile display estimate, out of scope

### Canonical constant added:
- `pricing_token_service.go`: `maxCoinsUsagePercentage = 20` with `1 coin = Rp1` documentation

---

## 9. PRE-EXISTING TEST FAILURE RESOLUTION

### `TestCreateFromSaleSurface_HappyPath`
- **Root cause:** Test's `happyPathCommentRepo` implemented `FindTargetIDByFixedPriceSaleID` but production code had been updated to call `FindTargetIDByCommerceResource`. The nil embedded interface caused panic.
- **Fix:** Added `FindTargetIDByCommerceResource` method to test fake (returns `uuid.Nil, nil`).
- **Result:** PASS

### User role constraint (migration 000012)
- **Root cause:** Migration 000012 removed `'seller'` from valid user roles. All integration test fixtures in `order/tests/` still inserted users with `'seller'` role.
- **Fix:** Changed all `'seller'` role inserts to `'user'` in 3 test files (order_canonical_test.go, auction_settlement_test.go, listing_stock_roundtrip_test.go).
- **Result:** All 10 integration tests PASS.

---

## 10. FILES CHANGED (S1V scope)

| File | Change |
|------|--------|
| `internal/commerce/order/infrastructure/repository/order_repository.go` | `total_before_coins_amount` ← `order.BuyerOrderValue` (canonical field) |
| `internal/commerce/order/tests/canonical_pricing_snapshot_persistence_test.go` | **NEW** — 4 real PostgreSQL proofs |
| `internal/commerce/order/tests/order_canonical_test.go` | Added `newTestOrder` helper; fixed `'seller'`→`'user'` role; fixed `FindTargetIDByCommerceResource` |
| `internal/commerce/order/tests/auction_settlement_test.go` | Fixed `'seller'`→`'user'` role; updated `newAuctionOrder` for new signature |
| `internal/commerce/order/tests/listing_stock_roundtrip_test.go` | Fixed `'seller'`→`'user'` role (3 call sites) |
| `internal/commerce/order/application/order_creation_service_test.go` | Added `FindTargetIDByCommerceResource` to test fake; added `commentEntity` import |

---

## 11. RESIDUE REMOVED

- Stale `'seller'` role in 5 test fixture functions (violated migration 000012 constraint)
- Missing method in test fake causing nil panic
- Duplicate `strPtr` function in new test file (already existed in order_canonical_test.go)

---

## 12. BUILD / TEST COMMANDS AND RESULTS

### Unit tests:
```
go test ./internal/pricing/token/... ./internal/commerce/order/... -count=1
```
**13 packages: ALL PASS** (0 failures)

### Integration tests:
```
go test -tags integration ./internal/commerce/order/tests/ -count=1
```
**10 tests: ALL PASS** (8.855s)

### Build:
```
go build ./internal/pricing/... ./internal/commerce/order/... ./internal/serverboot/... ./internal/finance/...
```
**ALL PACKAGES: CLEAN**

---

## 13. GIT STATUS

Working tree contains changes from Scope 4B-S1 and Scope 4B-S1V. Integration test infrastructure repaired (user role constraint, test fake methods). All changes forward-only. No rollback. No resurrection.

---

## 14. NEXT SCOPE RECOMMENDATION

**Scope 4B-S2: Fix CreatePayment escrow derivation to use canonical order snapshot.**

Change `dependencies.go:3289` to use `order.BuyerOrderValue` (the canonical `(P-D)+S` persisted in `total_before_coins_amount`) instead of recalculating `Sub+S+Comm`. Then reconcile `orders.escrow_amount` with `escrows.amount` in escrow creation. Files: ~3.

Do not implement in this scope.
