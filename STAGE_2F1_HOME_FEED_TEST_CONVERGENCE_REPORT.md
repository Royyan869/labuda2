# STAGE_2F1_HOME_FEED_TEST_CONVERGENCE_REPORT.md

## 1. VERDICT
**BLOCKED** – All seven Home/Feed test files depend on the obsolete `Listing` authority (e.g., `ListingRepository`, `listingRepositoryProvider`, `listingsProvider`). No direct canonical `ForSale` equivalents exist for the required repository contracts and provider signatures, so migration is not possible without redesigning the tests, which is prohibited at this stage.

## 2. FILE‑BY‑FILE RESULT
| File | Old Listing references | Canonical ForSale equivalent? | Classification | Evidence |
|------|------------------------|-------------------------------|----------------|----------|
| `test/features/home/presentation/screens/home_screen_promoted_card_rendering_test.dart` | Imports `catalog/listing/domain/domain.dart` and `catalog/listing/presentation/providers/listing_providers.dart`; defines `_FakeListingRepository implements ListingRepository` and overrides `listingRepositoryProvider` | No – `ForSaleRepository` uses different method signatures (`getForSales` etc.) and there is no `listingsProvider` family. | **BLOCKED** | Lines 36‑38 import listing domain/provider; lines 277‑284 define `_FakeListingRepository` with `getListings(GetListingsParams)`; line 392 overrides `listingRepositoryProvider`.
| `test/features/home/presentation/screens/home_screen_lifecycle_test.dart` | Same imports and fake repository as above (lines 9‑10 import, lines 40‑42 fake). | No – same mismatch. | **BLOCKED** | See lines 9‑10 and lines 40‑42.
| `test/features/home/presentation/screens/home_screen_feed_rendering_test.dart` | Same imports and fake repository (lines 9‑10 import, lines 23‑27 fake). | No – same mismatch. | **BLOCKED** | See lines 9‑10 and lines 23‑27.
| `test/features/home/presentation/screens/home_screen_cross_boundary_pipeline_test.dart` | Same imports and fake repository (lines 21‑22 import, lines 255‑258 fake). | No – same mismatch. | **BLOCKED** | See lines 21‑22 and lines 255‑258.
| `test/features/home/presentation/root_wiring/feed_root_wiring_test.dart` | Imports listing domain/provider (lines 29‑30). Uses `listingsProvider.overrideWith((ref, params) async { … })` (lines 759‑762) and fake `_FakeListingRepository implements ListingRepository` (lines 562‑564). | No – `listingsProvider` does not exist; ForSale has `forSalesProvider` with a different family signature. | **BLOCKED** | See lines 29‑30, 759‑762, 562‑564.
| `test/features/home/presentation/providers/feed_promoted_impression_authority_test.dart` | Same imports (lines 35‑36) and fake repository (lines 280‑283) with provider override (line 461). | No – same mismatch. | **BLOCKED** | See lines 35‑36, 280‑283, 461.
| `test/features/home/presentation/providers/feed_promoted_click_destination_authority_test.dart` | Same imports (lines 35‑36) and fake repository (lines 226‑229) with provider overrides (lines 445‑463). | No – same mismatch. | **BLOCKED** | See lines 35‑36, 226‑229, 445‑463.

## 3. OLD → CANONICAL MAPPING (attempted)
All files share the following obsolete symbols:
- `import '.../catalog/listing/domain/domain.dart'` → canonical `import '.../catalog/for_sale/domain/domain.dart'`
- `import '.../catalog/listing/presentation/providers/listing_providers.dart'` → canonical `import '.../catalog/for_sale/presentation/providers/for_sale_providers.dart'`
- `Listing` → `ForSale`
- `ListingStatus` → `ForSaleStatus`
- `listingRepositoryProvider` → `forSaleRepositoryProvider`
- `listingsProvider` → **no canonical equivalent** (closest is `forSalesProvider`, but its signature differs and is a family provider, not a simple provider).
- `ListingRepository` → `ForSaleRepository` (method signatures differ: `getListings` vs `getForSales`, etc.).

Because the required repository contracts (`getListings`, `GetListingsParams`, etc.) and the simple `listingsProvider` are **not present** in the canonical `for_sale` API, a direct 1‑to‑1 migration is impossible without redesigning test fakes, which is disallowed.

## 4. ANALYZER RESULT PER FILE
Running `dart analyze <file>` on each of the seven files **fails** with errors such as:
- `error: Target of URI doesn't exist: 'package:labuda/domains/commerce/catalog/listing/domain/domain.dart'`
- `error: Undefined class 'ListingRepository'`
- `error: Undefined getter 'listingRepositoryProvider'`
These errors stem from the missing `catalog/listing` package and the absent symbols, confirming the lack of canonical equivalents.

## 5. TEST RESULT PER FILE
All seven tests **cannot be compiled** after attempting a literal import replacement, because the required fake implementations cannot satisfy the existing `ForSaleRepository` interface. Therefore they are **BLOCKED**.

## 6. BLOCKED FILES + EXACT BLOCKER
| File | Exact blocker | Explanation |
|------|----------------|-------------|
| home_screen_promoted_card_rendering_test.dart | Missing `ListingRepository` / `listingRepositoryProvider` in canonical API | `ForSaleRepository` has different method signatures; no 1‑to‑1 mapping. |
| home_screen_lifecycle_test.dart | Same as above | – |
| home_screen_feed_rendering_test.dart | Same as above | – |
| home_screen_cross_boundary_pipeline_test.dart | Same as above | – |
| feed_root_wiring_test.dart | Missing `listingsProvider` and `ListingRepository` equivalents | `forSalesProvider` differs; no simple replacement. |
| feed_promoted_impression_authority_test.dart | Same as above | – |
| feed_promoted_click_destination_authority_test.dart | Same as above | – |

## 7. RESIDUE RESULT
All seven Home/Feed test files remain **residue** of the obsolete Listing authority. No successful migration could be performed under the constraints.

## 8. DIFF SCOPE AUDIT
`git diff --name-only` yields **no changed files**, because no modifications were made (the decision was to leave the files untouched and classify them as BLOCKED).

## 9. NEXT BOUNDED STEP
The next step is to address the blocked tests in a future stage where redesign of the test fakes and/or introduction of appropriate ForSale‑based test utilities is permitted. Until then, these files remain as documented residues.

---
*Report generated per Stage 2F‑1 instructions.*