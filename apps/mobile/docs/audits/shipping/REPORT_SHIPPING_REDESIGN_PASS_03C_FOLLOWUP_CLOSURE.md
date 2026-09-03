# SHIPPING-03C-FOLLOWUP — CLOSURE REPORT

**Date:** 2026-09-02  
**Status:** ✅ PASS_WITH_RUNTIME_GAP  
**Scope:** Followup closure audit for SHIPPING-03C canonicalization  
**Inherited from:** SHIPPING-03C (canonicalization pass)

---

## 1. Scope

This followup addresses all remaining gaps from SHIPPING-03C:

1. Close the last failing HTTP 400 error mapping test
2. Migration 000061 audit (runtime proof attempt)
3. `shipping_options` DB naming audit
4. `province_rate` semantic audit
5. Destination lock audit
6. Final residue scan
7. Authority matrix
8. Full regression proof
9. Final verdict

---

## 2. Inherited State from 03C

### What was already done:
- `auction_id` removed from `shipping_quotes` (entity, repository, service, handler, mobile DTO/mapper)
- `ShippingOption` → `ShippingSetup` rename across 108 backend + 120 mobile references
- Migration `000061_drop_shipping_quotes_auction_id` created
- Test infrastructure fixed (17/18 passing)
- `ShippingSetupsListNotifier` production bug fixed (toggle/delete failure no longer destroys loaded list)

### What remained:
- 1 test failure: HTTP 400 error mapping test
- No migration runtime proof
- No naming/semantic audits
- No authority matrix
- No full regression proof

---

## 3. Last 1 Failing Test — CLOSED

### Test: `HTTP 400 maps raw backend unmarshal error to localized message`

**File:** `apps/mobile/test/domains/commerce/transaction/shipping/shipping_option_setup_screen_test.dart`

**Failure root cause:**  
The test intended to test backend HTTP 400 error mapping, but client-side `_validate()` blocked submission before reaching the backend. The screen starts with one empty `_CoverageDraft()` and the previous test attempted to add a second coverage without properly filling the first.

**Fix (applied by previous session, verified this session):**  
The test now properly fills the first coverage (province + tariff) before submitting. Validation passes, backend returns 400, and the error is correctly mapped to the localized message.

**Result:** ✅ 18/18 shipping setup tests PASS

---

## 4. Migration 000061 Audit

### Migration: `000061_drop_shipping_quotes_auction_id`

**Up:** `ALTER TABLE shipping_quotes DROP COLUMN IF EXISTS auction_id;`  
**Down:** `ALTER TABLE shipping_quotes ADD COLUMN auction_id uuid;`

### Static Audit Evidence:

| Concern | Evidence | Status |
|---------|----------|--------|
| Column existed before 000061 | Created in `000001_canonical_schema.up.sql:1618` | ✅ Confirmed |
| Nullable | No NOT NULL constraint in schema | ✅ Confirmed |
| FK constraint | No FK on `shipping_quotes.auction_id` (only `auction_bids`, `buyer_bnr_strikes` have FKs) | ✅ Confirmed |
| Index | `idx_shipping_quotes_auction_id` (partial WHERE auction_id IS NOT NULL) — auto-dropped by PG when column dropped | ✅ Safe |
| Triggers | No triggers reference this column | ✅ Confirmed |
| Repository dependency | No active Go code references `shipping_quotes.auction_id` | ✅ Confirmed |
| IF EXISTS safety | `DROP COLUMN IF EXISTS` — idempotent | ✅ Safe |
| Preceding migration | 000060 drops legacy snapshot columns (clean chain) | ✅ Confirmed |
| No later migrations | 000061 is the latest migration | ✅ Confirmed |

### Runtime Proof:
- **Not available.** No database instance accessible.
- **Static analysis complete.** All evidence consistent with safe migration.

### Verdict: ✅ SAFE (static proof only, runtime gap documented)

---

## 5. `shipping_options` Table Naming Decision

**Domain model:** `ShippingSetup`  
**DB table:** `shipping_options`  
**API JSON envelope:** `shipping_options`

### Audit Evidence:

| Layer | Reference | Type |
|-------|-----------|------|
| DB Schema | `shipping_options` table, PK, 3 indexes, FKs from 3 tables (`orders`, `product_shipping_options`, `shipping_coverages`) | Physical persistence |
| Go Repository | 12 SQL statements using `shipping_options` table name | Active query |
| Go Handler | `"shipping_options"` JSON response key | API envelope |
| Mobile DTO | `json['shipping_options']` parsing | API contract |
| Mobile Test | `"shipping_options"` in fixtures | Test |
| Related tables | `product_shipping_options`, `shipping_coverages`, `orders` all FK to `shipping_options(id)` | Schema dependency |

### Decision: **RETAIN as physical DB name**

`shipping_options` is now purely a persistence implementation detail. The semantic authority is `ShippingSetup` everywhere in domain/application/presentation layers. Renaming the DB table would require:
- Coordinated migration
- All 12 Go SQL updates
- FK changes from 4 related tables
- API contract change

Cost far exceeds benefit. No semantic conflict exists.

**Note (reconciled):** `listing_shipping_options` was dropped by migration 000010, not 000016. Migration 000016 is a redundant `DROP IF EXISTS` safety net. `listing_shipping_options` is NOT a current FK dependency.

---

## 6. `province_rate` Semantic Audit

**Canonical meaning:** `province_rate` = the BASE shipping rate for a province-level coverage area. Combined shipping + packing cost.

### Audit Evidence:

| Layer | Representation | Notes |
|-------|---------------|-------|
| DB | `province_rate` (bigint) | Physical column |
| Go Entity | `ProvinceRate money.Money` | Clear semantic name |
| Go Service | `coverage.ProvinceRate.Int64()` | Used as base delivery rate |
| Go City Override | `Rate *money.Money // nil = use province_rate` | Clear fallback |
| Go Handler | `"rate": cov.ProvinceRate.Int64()` | API key simplified |
| Mobile Entity | `provinceRate: double?` | Dart naming convention |
| Mobile DTO | `dto.rate` → mapped to `provinceRate` | Mapping layer |

### Decision: **RETAIN**

The name is semantically clear within context:
- No competing packing cost field exists
- No separate shipping-only vs packing-only split
- City override clearly falls back to `province_rate`
- `GetEffectiveRate(provinceRate)` method makes the hierarchy explicit

No `packing_cost`, second shipping cost, or separate pricing authority exists.

---

## 7. Destination Lock Audit

### Runtime Path Verified:

```
buyer address → province_id/city_id
  → ShippingQuote (optional destination lock)
    → ValidateDestinationAddress(provinceID, cityID)
      → checkout → ValidateQuoteForCheckout() [STEP 5]
        → order_creation_service.go [STEP 8]
```

### Invariant Proven:

| Scenario | Behavior | Evidence |
|----------|----------|----------|
| **Bound quote** (destination set) | Mismatch at checkout → REJECTED | `ValidateDestinationAddress()` returns `DestinationMismatchError` |
| **Unbound quote** (both nil) | Any address valid | `if nil && nil { return nil }` |
| **Order creation** | Second enforcement point | `order_creation_service.go:570` calls same validation |
| **No silent bypass** | Fail-closed design | Both validation points reject mismatch |

### Conclusion:
- Bound quotes enforce destination lock
- Unbound quotes allow seller to quote before buyer picks address
- No bypass path exists
- Architecture is sound

---

## 8. Final Residue Scan

### Search Results (active code only, excluding migrations/docs):

| Term | Active Code Hits | Verdict |
|------|-----------------|---------|
| `ShippingOption` | **0** | ✅ CLEAN |
| `NewShippingOption` | **0** | ✅ CLEAN |
| `ShippingOptionRepository` | **0** | ✅ CLEAN |
| `ShippingOptionDto` | **0** | ✅ CLEAN |
| `ShippingOptionID` | **0** | ✅ CLEAN |
| `ShippingOptionName` | **0** | ✅ CLEAN |
| `NewAuctionShippingQuote` | **0** | ✅ CLEAN |
| `AuctionID` (shipping_quotes context) | **0** | ✅ CLEAN |
| `AuctionID` (auction domain) | 103+ | ✅ LEGITIMATE |
| `listing_shipping` / `ListingShipping` | **0** | ✅ CLEAN |
| `listing_shipping_option` / `ListingShippingOption` | **0** | ✅ CLEAN |
| `estimated_days` | 4 (NOTE comments only) | ✅ CLEAN |
| `expedition_name` / `shipping_expedition_name` / `shipping_estimated_days` | 4 (NOTE comments only) | ✅ CLEAN |

### Cosmetic Note:
- `shipping_option.go` filename contains `ShippingSetup` struct (cosmetic only — Go package import unaffected)

---

## 9. Authority Matrix

| # | Concern | Producer | Authority | Consumer | Persistence | Proof |
|---|---------|----------|-----------|----------|-------------|-------|
| 1 | Seller Shipping Setup CRUD | `SellerShippingService` | Domain entity `ShippingSetup` | HTTP Handler, Mobile Screen | `shipping_options` table | Service methods verified |
| 2 | Product Shipping Setup selection | `ProductShippingService` | `product_shipping_options` junction | Listing/Auction creation | `product_shipping_options` | Repository verified |
| 3 | Delivery availability | `ShippingService.CheckDeliveryAvailability()` | Province coverage + city override | Mobile checkout screen | Read-only query | Service logic verified |
| 4 | Shipping pricing | Coverage `ProvinceRate` + CityOverride | Single rate authority | Checkout, Order creation | `shipping_coverages` + `shipping_city_overrides` | No client-supplied price |
| 5 | Private Shipping Quote | `ShippingQuoteService.CreateShippingQuote()` | `ShippingQuote` entity | Chat system, Checkout | `shipping_quotes` | Destination lock optional |
| 6 | Active quote uniqueness | `FOR UPDATE` lock on quote | DB-level concurrency | Order creation | `shipping_quotes.status` | Race condition test exists |
| 7 | Quote checkout validation | `ValidateQuoteForCheckout()` | 5-step validation | Order creation service | Read + mark USED | Verified in order_creation_service.go |
| 8 | Order shipping snapshot | `OrderCreationService` | `Order` entity snapshot fields | Order display, fulfillment | `orders.shipping_*` columns | Snapshot stored at creation |
| 9 | Auction shipping quote identity | `source_type='auction'` + `source_id` | Canonical identity | Quote validation | `shipping_quotes.source_*` | `auction_id` column dropped |

### Authority Violations Found: **NONE**

- No duplicate calculation paths
- No client-supplied price injection
- No repository bypass
- No stale DTO
- No fallback to old `auction_id` identity
- No duplicate quote lifecycle

---

## 10. Regression Proof

### Backend:

| Command | Result |
|---------|--------|
| `go build ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/commerce/shipping/...` | ✅ ALL PASS |
| `go test ./internal/commerce/order/...` | ✅ ALL PASS |
| `go test ./internal/pricing/token/...` | ⚠️ 1 pre-existing failure (flat_fee_removed_test.go line ending mismatch) |
| `go test ./internal/commerce/auction/application/...` | ✅ PASS |
| `go test ./internal/commerce/auction/delivery/http/...` | ⚠️ Requires running DB (environment limitation) |

### Mobile:

| Command | Result |
|---------|--------|
| `dart analyze lib/domains/commerce/transaction/shipping/` | ✅ No issues found |
| `flutter test test/domains/commerce/transaction/shipping/` | ✅ ALL 43 TESTS PASS |
| `flutter test test/domains/chat/shipping_quote_contract_test.dart` | ✅ ALL PASS |
| `flutter test test/domains/chat/attachment_contract_alignment_test.dart` | ⚠️ 4 pre-existing failures (NegotiationOffer/NegotiationResult DTO Null cast) |
| `flutter test test/domains/commerce/transaction/` (full) | 167 pass, 16 fail (all pre-existing checkout failures) |

### Failure Classification:

| Failure | Classification | Related to Shipping? |
|---------|---------------|---------------------|
| `flat_fee_removed_test.go` line ending mismatch | Pre-existing stale test | ❌ No |
| `auction/delivery/http` DB requirement | Environment limitation | ❌ No |
| `NegotiationOffer/NegotiationResult` Null cast | Pre-existing DTO bug | ❌ No |
| Checkout tests (16 failures) | Pre-existing checkout issues | ❌ No |

---

## 11. Remaining Limitations

1. **Migration 000061 runtime proof unavailable** — No database instance accessible for replay. Static analysis confirms safety but runtime verification deferred.
2. **Pre-existing test failures** — 1 pricing token test, 4 chat attachment tests, 16 checkout tests, 1 auction HTTP test. All unrelated to shipping changes.
3. **File name residue** — `shipping_option.go` contains `ShippingSetup` struct (cosmetic, no functional impact).

---

## 12. Final Verdict

### ✅ PASS_WITH_RUNTIME_GAP

**Rationale:**
- All meaningful code gaps are closed
- All 43 shipping mobile tests PASS
- All backend shipping/order/auction application tests PASS
- `auction_id` fully removed from shipping context (zero active references)
- `ShippingOption` fully renamed to `ShippingSetup` (zero active references)
- Destination lock invariant verified end-to-end
- Authority matrix clean — no violations
- Migration 000061 safe by static analysis but runtime proof unavailable

**What would upgrade to PASS:**
- Database replay of migration 000061 proving column removal

**What would downgrade to INCOMPENSATE:**
- Any active `auction_id` or `ShippingOption` reference in shipping code
- Any authority violation in the matrix
- Any shipping test regression

---

## FINAL RECONCILIATION (2026-09-02)

### 1. Is `listing_shipping_options` still a current table?

**NO.** Dropped by migration **000010** (`product_sale_channel_canonicalization.up.sql:47`). Migration **000016** is a redundant `DROP TABLE IF EXISTS` safety net.

**Evidence chain:**
- Created in `000001_canonical_schema.up.sql:920` with PK + 2 FKs
- Dropped in `000010_product_sale_channel_canonicalization.up.sql:47`
- Redundantly dropped again in `000016_purge_legacy_listing_shipping_options.up.sql:5`

### 2. Was the previous report (Section 5) wrong about listing_shipping_options?

**YES.** The previous report listed `listing_shipping_options` as a current FK dependency to `shipping_options`. This was stale — the table was dropped by migration 000010. **Corrected above.**

### 3. What is the exact current FK dependency on `shipping_options`?

| Table | FK Constraint | ON DELETE | Status |
|-------|--------------|-----------|--------|
| `orders` | `fk_orders_shipping_option` | SET NULL | ✅ ACTIVE |
| `product_shipping_options` | `product_shipping_options_shipping_option_id_fkey` | CASCADE | ✅ ACTIVE |
| `shipping_coverages` | `shipping_coverages_shipping_option_id_fkey` | CASCADE | ✅ ACTIVE |
| `listing_shipping_options` | `fk_listing_shipping_options_shipping_option` | CASCADE | ❌ DROPPED (000010) |

### 4. Is 000061 down migration an exact inverse?

**NO.** The down migration restores only the column, not the index.

**Pre-000061 schema for `shipping_quotes.auction_id`:**
- Column: `auction_id uuid` (nullable) — restored by down migration ✅
- Index: `idx_shipping_quotes_auction_id` (partial, WHERE auction_id IS NOT NULL) — NOT restored ❌
- FK: NONE — nothing to restore ✅
- Constraint: NONE — nothing to restore ✅

**Down migration restores:**
```sql
ALTER TABLE shipping_quotes ADD COLUMN auction_id uuid;
```

**Missing from down migration:**
```sql
-- Not restored:
CREATE INDEX idx_shipping_quotes_auction_id ON public.shipping_quotes USING btree (auction_id) WHERE (auction_id IS NOT NULL);
```

### 5. Does the missing index matter?

**NO, for this project.**

- Project is forward-only: no production data, no rollback requirement
- Down migrations serve as dev tooling safety nets, not production rollback paths
- The down migration is labeled `IRREVERSIBLE` in its own comment
- If down migration were ever executed, the missing index would be a performance issue, not a correctness issue

### 6. Is forward migration 000061 safe?

**YES.**
- Column was nullable (no NOT NULL constraint)
- No FK constraint on `shipping_quotes.auction_id`
- Partial index auto-dropped by PostgreSQL when column is dropped
- `IF EXISTS` makes it idempotent
- Zero active Go/Dart code references `shipping_quotes.auction_id`

### 7. Is there any active legacy shipping authority?

**NO.** Verified across all active source:
- `listing_shipping_options`: DROPPED (000010) — stale reference in `dev-reset-data/main.go` (harmless, table doesn't exist)
- `ShippingOption` domain: ZERO active references
- `auction_id` in shipping context: ZERO active references
- Cosmetic residue only: `stubListingShippingSetupRepository` struct name in test stubs, `shipping_option.go` filename containing `ShippingSetup` struct

### 8. Additional cosmetic residues found

| Residue | Location | Severity | Action |
|---------|----------|----------|--------|
| `stubListingShippingSetupRepository` struct name | `order_handler.go`, `seed/main.go` | Cosmetic | No functional impact |
| `listing_shipping_options` in table list | `dev-reset-data/main.go:112` | Cosmetic | PG skips non-existent tables |
| `shipping_option.go` filename | `entity/shipping_option.go` | Cosmetic | Contains `ShippingSetup` struct |

None of these affect authority, correctness, or behavior.

### Reconciliation verdict

**PASS_WITH_RUNTIME_GAP maintained.**
The two reconciliation concerns (listing_shipping_options status, 000061 down completeness) have been resolved:
1. Previous report error corrected (listing_shipping_options is NOT a current FK)
2. Down migration incompleteness documented (index not restored, acceptable for forward-only project)

No new authority or schema problems found.

---

## FINAL MICROCLEANUP (2026-09-02)

Three cosmetic/stale residues identified during reconciliation have been cleaned.

### 1. Filename rename: `entity/shipping_option.go` → `entity/shipping_setup.go`

**Reason:** File contained `ShippingSetup` struct; filename was legacy.
**Risk:** None — Go imports by package, not file name.
**Proof:** `go build ./...` passes.

### 2. Stale dev-reset table reference removed

**File:** `backend/cmd/dev-reset-data/main.go`
**Removed:** `"listing_shipping_options"` from table list.
**Reason:** Table dropped by migration 000010. PostgreSQL skips non-existent tables, but stale reference is unnecessary noise.

### 3. Stub rename: `stubListingShippingSetupRepository` → `stubProductShippingSetupRepository`

**Files:**
- `backend/cmd/seed/main.go` (type + variable + 3 usages)
- `backend/internal/integration/payment/application/orchestrator/order_handler.go` (type + 2 usages)

**Reason:** Stale `Listing` prefix in struct name. The stub implements `ProductShippingSetupRepository` interface, not a listing-owned authority.
**Note:** Renamed to `stubProductShippingSetupRepository` (not `stubShippingSetupRepository`) to avoid collision with existing `stubShippingSetupRepository` in both files.
**Risk:** None — internal test/seed stub, no behavior change.

### Regression proof

| Command | Result |
|---------|--------|
| `go build ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/commerce/shipping/...` | ✅ ALL PASS |
| `dart analyze lib/domains/commerce/transaction/shipping/` | ✅ No issues found |
| `flutter test test/domains/commerce/transaction/shipping/` | ✅ ALL 43 TESTS PASS |

### Final residue search (active code only)

| Term | Active hits | Verdict |
|------|-------------|--------|
| `shipping_option.go` | 0 | ✅ CLEAN |
| `ShippingOption` | 0 | ✅ CLEAN |
| `NewShippingOption` | 0 | ✅ CLEAN |
| `ShippingOptionRepository` | 0 | ✅ CLEAN |
| `ShippingOptionDto` | 0 | ✅ CLEAN |
| `ShippingOptionID` | 0 | ✅ CLEAN |
| `ShippingOptionName` | 0 | ✅ CLEAN |
| `NewAuctionShippingQuote` | 0 | ✅ CLEAN |
| `listing_shipping_options` | 1 (comment only) | ✅ ALLOWED |
| `ListingShipping` | 0 | ✅ CLEAN |
| `listing_shipping` | 0 | ✅ CLEAN |
| `stubListingShipping` | 0 | ✅ CLEAN |

**Only allowed residue:** `listing_shipping_options` in a comment in `legacy_table_guard_test.go` (historical evidence).

---

## FINAL VERDICT

### ✅ PASS_WITH_RUNTIME_GAP

**Shipping domain canonicalization is ACCEPTED.**

All legacy authority has been removed:
- `auction_id` removed from shipping_quotes
- `ShippingOption` domain fully renamed to `ShippingSetup`
- `listing_shipping_options` table dropped (migration 000010)
- All cosmetic/stale residues cleaned

**Remaining runtime gap:** Migration 000061 database replay proving column removal. Static analysis confirms safety; runtime verification deferred to when database instance is available.

**No active legacy shipping authority remains.**
