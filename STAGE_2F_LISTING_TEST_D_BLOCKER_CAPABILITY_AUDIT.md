# STAGE 2F LISTING TEST D-BLOCKER CAPABILITY AUDIT

**VERDICT:** PARTIAL (all 15 D‑files remain, each blocked by missing canonical ForSale capability)

## Exact 15 D‑files audited (from `STAGE_2E3_LISTING_TEST_CONVERGENCE_REPORT.md`)
1. `test/features/home/presentation/screens/home_screen_promoted_card_rendering_test.dart`
2. `test/features/home/presentation/screens/home_screen_lifecycle_test.dart`
3. `test/features/home/presentation/screens/home_screen_feed_rendering_test.dart`
4. `test/features/home/presentation/screens/home_screen_cross_boundary_pipeline_test.dart`
5. `test/features/home/presentation/root_wiring/feed_root_wiring_test.dart`
6. `test/features/home/presentation/providers/feed_promoted_impression_authority_test.dart`
7. `test/features/home/presentation/providers/feed_promoted_click_destination_authority_test.dart`
8. `test/core/router/router_lifetime_preservation_test.dart`
9. `test/features/explore/explore_promotion_injection_test.dart`
10. `test/domains/user/identity/authentication/auth_portal_protected_provider_blocking_test.dart`
11. `test/domains/social/content/create_content_sale_picker_widget_test.dart`
12. `test/domains/commerce/catalog/shared/listing_media_handler_validation_test.dart`
13. `test/domains/commerce/catalog/auction/presentation/screens/create_auction_screen_media_test.dart`
14. `test/domains/chat/shipping_quote_contract_test.dart`
15. `test/domains/chat/chat_detail_composer_authority_test.dart`

## Classification table
| File | Obsolete dependency | Intended behavior | Classification (A/B/C/D) | Evidence |
|------|--------------------|------------------|------------------------|----------|
| home_screen_promoted_card_rendering_test.dart | `ListingRepository` (and `Listing` entity) | Verify promoted listing card rendering in HomeFeed pipeline | D | Implements `_FakeListingRepository implements ListingRepository` and calls `listingRepositoryProvider.overrideWithValue(_FakeListingRepository())` (lines ~277‑284) |
| home_screen_lifecycle_test.dart | `ListingRepository` | Verify feed lifecycle handling for listings | D | `_FakeListingRepository implements ListingRepository` (lines ~40‑42) and provider override (line 79) |
| home_screen_feed_rendering_test.dart | `ListingRepository` | Verify feed rendering of promoted listings | D | `_FakeListingRepository implements ListingRepository` (lines 25‑27) and override (line 149) |
| home_screen_cross_boundary_pipeline_test.dart | `ListingRepository` | Test cross‑boundary pipeline involving listings | D | `_FakeListingRepository implements ListingRepository` (lines 255‑258) and override (line 361) |
| feed_root_wiring_test.dart | `listingsProvider` & `ListingRepository` | Validate root‑wiring of feed, ensuring listing provider integration | D | `listingsProvider.overrideWith((ref, params) async { … })` (lines 759‑762) and `_FakeListingRepository implements ListingRepository` (lines 562‑564) |
| feed_promoted_impression_authority_test.dart | `ListingRepository` | Test impression tracking for promoted listings | D | `_FakeListingRepository implements ListingRepository` (lines 280‑283) and provider override (line 461) |
| feed_promoted_click_destination_authority_test.dart | `ListingRepository` | Test click‑destination routing for promoted listings | D | `_FakeListingRepository implements ListingRepository` (lines 226‑229) and provider overrides (lines 445‑463) |
| router_lifetime_preservation_test.dart | `fixedPriceSaleDetailProvider`, old listing screens (`create_listing_screen.dart`, `listing_detail_screen.dart`) | Ensure router state preservation across listing screens | D | Imports old listing domain and screens (lines 12‑15); overrides `fixedPriceSaleDetailProvider` (line 804) |
| explore_promotion_injection_test.dart | `ListingRepository` (old contract) | Verify promotion injection for listings in Explore flow | D | `_FakeListingRepository implements ListingRepository` with methods like `getListings`, `getFixedPriceSaleById` (lines 28‑41) |
| auth_portal_protected_provider_blocking_test.dart | `listingsProvider` & `Listing` | Ensure auth‑portal blocks listing fetches behind auth | D | `listingsProvider.overrideWith((ref, params) async { … return const <Listing>[]; })` (lines 285‑287) |
| create_content_sale_picker_widget_test.dart | `ListingRepository` | Test content‑sale picker UI for listings | D | `_FakeListingRepository implements ListingRepository` (lines 154‑160) and provider override (line 392) |
| listing_media_handler_validation_test.dart | `ListingMediaHandler` | Validate size‑validation logic of listing media handler | D | Instantiates `ListingMediaHandler()` and calls `mediaSizeValidationMessage` / `isVideoFile` (lines 16‑31) |
| create_auction_screen_media_test.dart | `ListingMediaHandler` | Test media handling in auction creation flow (listing version) | D | Uses `ListingMediaHandler()` and its static `isVideoFile` (lines 199‑215) |
| shipping_quote_contract_test.dart | Missing helper `buildForSaleShippingQuoteRequest` (old listing dto) | Verify shipping‑quote request/response fields for ForSale | D | Calls `buildForSaleShippingQuoteRequest` (line 17) which is not defined in canonical ForSale DTOs |
| chat_detail_composer_authority_test.dart | `ListingRepository`, `ListingController` | Test chat composer behavior with listing detail fetching | D | Implements `_NoOpListingRepository implements ListingRepository` and `_FakeLookupListingController extends ListingController` (lines 120‑124, 262‑267) |

## Details per classification
### D – canonical ForSale API does not provide required capability
All 15 files rely on obsolete symbols or contracts that have no 1:1 counterpart in the current `catalog/for_sale` implementation. The intended behaviours (feed rendering, promotion injection, router state preservation, media validation, shipping‑quote handling, chat composition) are still conceptually relevant, but the production API now uses different interfaces (`ForSaleRepository`, `forSalesProvider`, `ForSaleMediaHandler`, etc.) and different method signatures. Consequently the tests cannot compile or run against the canonical API.

**Missing capabilities (examples):**
- `ListingRepository` methods (`getListings`, `getFixedPriceSaleById`, `createListing`, …) vs `ForSaleRepository` (`getForSales`, `getForSaleById`, `createForSale`).
- Provider `listingsProvider` vs `forSalesProvider` (different family signatures).
- `ListingMediaHandler.mediaSizeValidationMessage` / `isVideoFile` – not present in `ForSaleMediaHandler`.
- Helper `buildForSaleShippingQuoteRequest` – absent from canonical shipping‑quote DTOs.
- `fixedPriceSaleDetailProvider` – renamed to `forSaleDetailProvider` (different name).
- `ListingController` – replaced by `ForSaleController` with different API.

### Recommendations per file
- **FUTURE IMPLEMENTATION:** For files where the behaviour remains relevant (feed tests, promotion injection, auth portal, chat composer, shipping quote) – create new tests using the canonical `ForSale` APIs once those capabilities are available or rewrite using existing `ForSale` contracts.
- **DELETE CANDIDATE:** For media‑handler tests (`listing_media_handler_validation_test.dart`, `create_auction_screen_media_test.dart`) – the old `ListingMediaHandler` no longer exists; the validation logic has moved to `ForSaleMediaHandler` with a different API, making these tests obsolete.
- **DELETE CANDIDATE:** `router_lifetime_preservation_test.dart` – old listing routes (`/listing/...`) and screens have been removed; the test targets a dead UI path.
- **FUTURE IMPLEMENTATION:** `shipping_quote_contract_test.dart` – the test’s intention (verify request/response fields) is still valid; replace missing `buildForSaleShippingQuoteRequest` with the appropriate DTO constructor when available.

## Final counts
- **A:** 0
- **B:** 0
- **C:** 0
- **D:** 15

## Files changed
- Production files: **0**
- Test files: **0** (no modifications performed in this audit)

---
*Report generated per Stage 2F instructions.*