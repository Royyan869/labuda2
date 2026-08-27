# STAGE 2E-3 LISTING TEST CONSUMER CONVERGENCE REPORT

**VERDICT:** PASS WITH PRE-EXISTING WARNINGS

## Test files discovered (importing `catalog/listing/**`)
- `test/features/home/presentation/screens/home_screen_promoted_card_rendering_test.dart`
- `test/features/home/presentation/screens/home_screen_lifecycle_test.dart`
- `test/features/home/presentation/screens/home_screen_feed_rendering_test.dart`
- `test/features/home/presentation/screens/home_screen_cross_boundary_pipeline_test.dart`
- `test/features/home/presentation/root_wiring/feed_root_wiring_test.dart`
- `test/features/home/presentation/providers/feed_promoted_impression_authority_test.dart`
- `test/features/home/presentation/providers/feed_promoted_click_destination_authority_test.dart`
- `test/core/router/router_lifetime_preservation_test.dart`
- `test/features/explore/explore_promotion_injection_test.dart`
- `test/domains/user/identity/authentication/auth_portal_protected_provider_blocking_test.dart`
- `test/domains/social/content/create_content_sale_picker_widget_test.dart`
- `test/domains/social/comment/comment_input_create_listing_gorouter_test.dart` **(migrated)**
- `test/domains/commerce/catalog/shared/listing_media_handler_validation_test.dart`
- `test/domains/commerce/catalog/auction/presentation/screens/create_auction_screen_media_test.dart`
- `test/domains/chat/shipping_quote_contract_test.dart`
- `test/domains/chat/chat_detail_composer_authority_test.dart`

## Classification
- **A (migrated):** `comment_input_create_listing_gorouter_test.dart`
- **D (canonical ForSale API does not provide required capability):** all other 15 files (listed above) – each relies on obsolete `ListingRepository` contract, old providers (`listingsProvider`), `ListingMediaHandler` methods, or removed symbols such as `fixedPriceSaleDetailProvider`.

## Migration performed
- `test/domains/social/comment/comment_input_create_listing_gorouter_test.dart`
  - Updated imports from `catalog/listing/...` to `catalog/for_sale/...`
  - Renamed `Listing` → `ForSale`
  - Renamed constructor field `fixedPriceSaleId:` → `forSaleId:`
  - Renamed `ListingStatus.active` → `ForSaleStatus.active`
  - No other code changes.

## Files intentionally left untouched
- All 15 D‑category files remain unchanged; migration would require extensive API redesign (e.g., `ListingRepository` vs `ForSaleRepository`, missing `ListingMediaHandler` methods, missing `fixedPriceSaleDetailProvider`). Leaving them unchanged respects the rule *“Jangan memperbaiki blocker tersebut”.*

## Analyzer result (single file)
- Command: `dart analyze test/domains/social/comment/comment_input_create_listing_gorouter_test.dart`
- Output: 2 warnings (unused import at line 8, unused optional `key` at line 127) – both pre‑existing, unrelated to the Listing→ForSale migration.
- **Result:** PASS WITH PRE‑EXISTING WARNINGS.

## Test execution result
- Tests were not executed as part of this audit; only static analysis was performed.

## Baseline / external blockers
- No external blockers affecting the migrated file; warnings are internal to the test file.

## Summary counts
- Final residue A count: **0**
- Residue B/C/D count: **15** (all D)
- Production files changed: **0**
- Test files changed: **1**

---
*Report generated per Stage 2E‑3 instructions.*