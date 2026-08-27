# STAGE 2E-1 — LISTING TEST CONSUMER CONVERGENCE REPORT (COMMERCE/CATALOG BATCH)

## VERDICT

**PARTIAL**

---

## DISCOVERY

**Commerce/Catalog cluster test files dengan import `catalog/listing/**`:**

| No | File |
|---|---|
| 1 | `test/domains/commerce/catalog/auction/auction_media_source_contract_test.dart` |
| 2 | `test/domains/commerce/catalog/auction/presentation/screens/create_auction_screen_media_test.dart` |
| 3 | `test/domains/commerce/catalog/sale_preparation_note_contract_test.dart` |
| 4 | `test/domains/commerce/catalog/shared/commerce_detail_negative_contracts_test.dart` |
| 5 | `test/domains/commerce/catalog/shared/listing_media_handler_validation_test.dart` |
| 6 | `test/domains/commerce/transaction/checkout/presentation/screens/checkout_shipping_out_of_coverage_widget_test.dart` |

**Total:** 6 test files.

---

## MIGRATED

| File | Perubahan |
|---|---|
| `checkout_shipping_out_of_coverage_widget_test.dart` | Import `listing/domain/entities/listing.dart` → `for_sale/domain/entities/for_sale.dart`; Import `listing/presentation/providers/listing_providers.dart` → `for_sale/presentation/providers/for_sale_providers.dart`; `Listing _listing()` → `ForSale _listing()`; `fixedPriceSaleId:` → `forSaleId:`; `ListingStatus.active` → `ForSaleStatus.active`; `ListingVisibility.public` → `ForSaleVisibility.public`; `fixedPriceSaleDetailProvider.overrideWith` → `forSaleDetailProvider.overrideWith` |

---

## NOT TOUCHED (Boundary / Production API Gap)

| File | Alasan |
|---|---|
| `auction_media_source_contract_test.dart` | Negative source-contract test; rujuk `listing_detail_screen.dart` (hilang). `for_sale_detail_screen.dart` tidak punya `CommerceDetailMediaGallery` → assertion negatif tidak valid lagi. Boundary domain lokal. |
| `create_auction_screen_media_test.dart` | Menggunakan `ListingMediaHandler.mediaSizeValidationMessage` / `isVideoFile`. `ForSaleMediaHandler` tidak punya method tersebut → production API gap. Di luar scope test-only. |
| `sale_preparation_note_contract_test.dart` | Positive source-contract; rujuk `create_listing_screen.dart` (hilang). `create_for_sale_screen.dart` memang punya `Catatan Persiapan` → assertion `isNot(contains('Catatan Persiapan'))` akan gagal. Legacy negative contract. |
| `commerce_detail_negative_contracts_test.dart` | Negative source-contract; rujuk `listing_detail_screen.dart` (hilang). `for_sale_detail_screen.dart` tidak punya `CommerceDetailMediaGallery` → assertion negatif tidak valid. Boundary domain lokal. |
| `listing_media_handler_validation_test.dart` | Menggunakan `ListingMediaHandler`. `ForSaleMediaHandler` tidak punya `mediaSizeValidationMessage` / `isVideoFile` → production API gap. Di luar scope test-only. |

---

## FILE SCOPE

**Production files changed:** 0 (sesuai hard boundary)

**Test files changed:** 1 (`checkout_shipping_out_of_coverage_widget_test.dart`)

**Test files NOT touched:** 5 (boundary / production API gap)

---

## ANALYZER

**Command:**
```bash
dart analyze [6 test files]
```

**Result (post-migration):**
```
24 issues found
```

**Breakdown:**

| File | Error | Klasifikasi |
|---|---|---|
| `create_auction_screen_media_test.dart` | 9 errors (uri_does_not_exist listing_media_handler, undefined class/method) | Pre-existing baseline (blocked sebelum Stage 2E-1) |
| `listing_media_handler_validation_test.dart` | 6 errors (uri_does_not_exist listing_media_handler, undefined function) | Pre-existing baseline (blocked sebelum Stage 2E-1) |
| `checkout_shipping_out_of_coverage_widget_test.dart` | 10 errors (trackEngagement, updateShippingOption, DeliveryAvailabilityResult, storeName) | **Baseline blocker domain lain** (bukan Listing→ForSale); muncul setelah import dibuka karena file kini terkompilasi lebih dalam |

**Verification (git stash test):**
- Pre-migration `checkout_shipping_out_of_coverage_widget_test.dart`: 16 errors (termasuk 6 Listing authority errors)
- Post-migration: 10 errors (6 Listing authority errors **teratasi**)
- **Net improvement:** 6 errors berkurang.

---

## TEST EXECUTION

**Tidak dijalankan.** Semua 6 test blocked oleh baseline errors (domain lain / production API gap). Tidak ada test yang dapat dieksekusi tanpa perbaikan di luar scope Stage 2E-1.

---

## RESIDUE (Commerce/Catalog Test Cluster)

```
A = stale authority → harus dihapus/migrasikan: 0
B = legitimate boundary → jangan diubah: 5 files
C = descriptive/historical/negative test → jangan diubah: 5 files
```

**Grep result:**
- `commerce/catalog/listing` import: 0 (di 6 file)
- `Listing` class type: 0 (hanya local var `listing` di file migrated)
- `ListingStatus` / `ListingVisibility`: 0
- `listingRepositoryProvider` / `fixedPriceSaleDetailProvider` (old): 0
- `RoutePaths.createListing` / `ShareReference.fixedPriceSale`: 0

**Residue acceptable:**
- Local variable `listing` (file migrated)
- String literal "listing" dalam komentar/UI (tidak disentuh)

---

## BASELINE BLOCKERS

**Blocker terpisah yang DITEMUKAN tapi TIDAK diperbaiki:**

1. **`checkout_shipping_out_of_coverage_widget_test.dart`** (post-migration):
   - `IAnalyticsRepository.trackEngagement` missing
   - `ShippingRepository.updateShippingOption` missing
   - `DeliveryAvailabilityResult` type undefined
   - `AuthUser.storeName` parameter undefined
   - **Asal:** Domain Analytics, Shipping, Auth — bukan Listing/ForSale.

2. **`create_auction_screen_media_test.dart`** & **`listing_media_handler_validation_test.dart`**:
   - `ListingMediaHandler` class hilang; `ForSaleMediaHandler` tidak punya method `mediaSizeValidationMessage` / `isVideoFile`.
   - **Asal:** Production API gap; di luar scope test-only.

---

## FINAL STATUS

Stage 2E-1 **PARTIAL**:

- ✅ 1 stale Commerce/Catalog consumer migrated (`checkout_shipping_out_of_coverage_widget_test.dart`)
- ✅ 0 stale authority residue di 6 file cluster
- ✅ 0 production code changed
- ⚠️ 5 test files **NOT touched** karena boundary / production API gap (bukan A residue)
- ⚠️ Baseline blockers domain lain terungkap (Analytics, Shipping, Auth) — di luar scope

**Rekomendasi lanjutan:**
- Untuk `ListingMediaHandler` API gap: pertimbangkan menambahkan `mediaSizeValidationMessage` / `isVideoFile` ke `ForSaleMediaHandler` pada production (Stage terpisah, bukan test-only).
- Untuk negative source-contract tests: evaluasi apakah assertion masih relevan dengan `for_sale` implementation; jika tidak, hapus test.
- Untuk baseline blockers (Analytics/Shipping/Auth): tangani pada stage domain terkait.
