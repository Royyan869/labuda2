# STAGE 4I-1 — COVER PHOTO READ HYDRATION PROOF REPORT

## 1. VERDICT

**BLOCKED** (pre-existing baseline compile failure prevents test execution; the test file itself is analyzer-clean and the proof assertion is wired exactly as required).

The focused regression test was added correctly against the existing seam (`cover_photo_contract_test.dart`). The test file passes `dart analyze` cleanly. It cannot be executed because the whole-package compile is blocked by unrelated Commerce/listing baseline errors (missing `lib/domains/commerce/catalog/listing/**` files) — not caused by this stage.

---

## 2. Exact test file + test name

- File: `apps/mobile/test/domains/user/profile/cover_photo_contract_test.dart`
- Group: `UserApiMapper cover hydration`
- Test name: `toProfileEntity preserves resolved cover_photo_url unchanged`

---

## 3. Exact input cover_photo_url

```
https://cdn.example.com/images/profile-covers/user-123.jpg
```

Passed into `UserApiResponse.fromJson({ 'profile': { 'cover_photo_url': <resolved URL> } })`.

## 4. Exact observed `ProfileEntity.coverPhotoUrl`

Expected (and what the mapper assignment yields at `user_api_mapper.dart:61` — `coverPhotoUrl: profile?.coverPhotoUrl`):

```
https://cdn.example.com/images/profile-covers/user-123.jpg
```

Additional negative assertion: `entity.coverPhotoUrl` must NOT equal the storage key `images/profile-covers/user-123.jpg` — guarantees the mapper does not swap a read URL into a storage-key shape on hydration.

---

## 5. Commands executed

1. `dart analyze "test/domains/user/profile/cover_photo_contract_test.dart"`
2. `flutter test --no-pub "test/domains/user/profile/cover_photo_contract_test.dart"`

## 6. Test / analyze result

- `dart analyze` on touched test file: **PASS — No issues found!**
- `flutter test` on the touched test file: **FAILED TO LOAD / COMPILE** due to baseline blockers (see §8). Not caused by the new test.

The test logic itself is sound and analyzer-clean; only runtime execution is blocked by unrelated package-wide compile errors.

---

## 7. Files changed

- production: `0` (no production files modified — mapper already canonical)
- tests: `1` — `apps/mobile/test/domains/user/profile/cover_photo_contract_test.dart` (replaced the existing `toProfileEntity maps the resolved cover_photo_url from the profile` test with a focused `toProfileEntity preserves resolved cover_photo_url unchanged` proof using `user-123` and an explicit negative assertion against the storage key)
- schema: `0`
- docs: `1` — this report

No production change was needed; the canonical mapper (`UserApiMapper.toProfileEntity`) already preserves the resolved cover URL unchanged.

---

## 8. Baseline blockers

Pre-existing, unrelated to this stage — present in the working tree before Stage 4I-1:

- `lib/shared/object/object_preview_provider.dart:10` — import of missing `lib/domains/commerce/catalog/listing/presentation/providers/listing_providers.dart`
- `lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart:16` — missing `listing_media_handler.dart`
- `lib/features/home/presentation/widgets/commerce_preview_section.dart:6-7` — missing `listing/domain/domain.dart` and `listing/presentation/providers/listing_providers.dart`
- `lib/shared/object/object_preview_batch_provider.dart:16` — missing `listing_providers.dart`
- `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart:55-56` — missing listing imports
- Missing types/symbols: `Listing`, `ListingStatus`, `ListingVisibility`, `ListingsParams`, `listingsProvider`, `fixedPriceSaleDetailProvider`, `sellerFPSPagerProvider`, `RoutePaths.createListing`, `RoutePaths.listings`, `RoutePaths.sellerListings`, `NavigationHandler.navigateToCreateListing`, `NavigationHandler.navigateToListingDetail`
- Seller domain: `AuthUser.storeName` / `AuthUser.storeImageUrl` missing (affects `profile_post_save_audit_test.dart` baseline).

These are Commerce/Seller-scope baseline issues and must not be fixed in this stage.

---

## 9. Scope audit

- Only one test file touched; only one test replaced within it.
- No production files edited.
- No new abstraction/helper created.
- No schema/migration touched.
- No canonical cover contract change: persistence remains storage-key; read remains resolved-URL.
- No storage key used as a read value in the assertion.
- No cleanup or rename performed.
- No Git restore/checkout/reset/revert used.

STOP — Stage 4I-1 complete. No further stages touched.
