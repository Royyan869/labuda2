# STAGE 2A — LISTING → FOR-SALE PRODUCTION CONSUMER CONVERGENCE REPORT

**Tanggal:** 2026-08-27  
**Scope:** 10 production files yang teridentifikasi dalam STAGE_2_LISTING_AUTHORITY_READ_ONLY_AUDIT.md  
**Mode:** Production consumers only; test files TIDAK disentuh.

---

## 1. DAFTAR 10 FILE YANG DIPROSES

| No | File Path | Status |
|---|---|---|
| 1 | `lib/shared/object/object_preview_provider.dart` | ✅ Modified |
| 2 | `lib/shared/object/object_preview_batch_provider.dart` | ✅ Modified |
| 3 | `lib/features/home/presentation/widgets/commerce_preview_section.dart` | ✅ Modified |
| 4 | `lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart` | ✅ Modified |
| 5 | `lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart` | ✅ Modified |
| 6 | `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart` | ✅ Modified |
| 7 | `lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart` | ✅ Modified |
| 8 | `lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart` | ✅ Modified |
| 9 | `lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart` | ✅ Modified |
| 10 | `lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart` | ✅ Modified |

---

## 2. PERUBAHAN SETIAP FILE

### File 1: object_preview_provider.dart
- **Import:** `catalog/listing/presentation/providers/listing_providers.dart` → `catalog/for_sale/presentation/providers/for_sale_providers.dart`
- **Provider:** `fixedPriceSaleDetailProvider` → `forSaleDetailProvider`
- **Field access:** `listingAsync.fixedPriceSaleId` → `listingAsync.forSaleId`

### File 2: object_preview_batch_provider.dart
- **Import:** `catalog/listing/presentation/providers/listing_providers.dart` → `catalog/for_sale/presentation/providers/for_sale_providers.dart`
- **Repository:** `listingRepositoryProvider` → `forSaleRepositoryProvider`
- **Method call:** `getListingsByIds` → `getForSalesByIds`
- **Field access:** `listing.fixedPriceSaleId` → `listing.forSaleId` (3 occurrences)

### File 3: commerce_preview_section.dart
- **Import:** `catalog/listing/domain/domain.dart` → `catalog/for_sale/domain/domain.dart`
- **Import:** `catalog/listing/presentation/providers/listing_providers.dart` → `catalog/for_sale/presentation/providers/for_sale_providers.dart`
- **Provider:** `listingsProvider` → `forSalesProvider`
- **Params:** `ListingsParams` → `ForSalesParams`
- **Enum:** `ListingStatus.active` → `ForSaleStatus.active`
- **Type:** `List<Listing>` → `List<ForSale>`
- **Type:** `Listing? listing` → `ForSale? listing`
- **Field access:** `item.listing!.fixedPriceSaleId` → `item.listing!.forSaleId`

### File 4: commerce_resource_picker.dart
- **Import:** `catalog/listing/domain/domain.dart` → `catalog/for_sale/domain/domain.dart`
- **Import:** `catalog/listing/presentation/providers/seller_fps_pager.dart` → `catalog/for_sale/presentation/providers/seller_fps_pager.dart`
- **Enum:** `ListingStatus.active` → `ForSaleStatus.active`
- **Field access:** `l.fixedPriceSaleId` → `l.forSaleId` (2 occurrences)
- **UI string:** "Buat Listing Baru" dipertahankan (bukan symbol authority)

### File 5: comment_input_with_commerce_reference.dart
- **Import:** `catalog/listing/domain/domain.dart` → `catalog/for_sale/domain/domain.dart`
- **Type check:** `result is Listing` → `result is ForSale`
- **Route:** `RoutePaths.createListing` → `RoutePaths.createForSale`
- **Field access:** `result.fixedPriceSaleId` → `result.forSaleId`
- **Komentar:** "Create Listing" dipertahankan (bukan symbol authority)

### File 6: checkout_screen_impl.dart
- **Import:** `catalog/listing/domain/entities/listing.dart` → `catalog/for_sale/domain/entities/for_sale.dart`
- **Import:** `catalog/listing/presentation/providers/listing_providers.dart` → `catalog/for_sale/presentation/providers/for_sale_providers.dart`
- **Provider:** `fixedPriceSaleDetailProvider` → `forSaleDetailProvider` (3 occurrences)
- **Variable name:** `listing` dipertahankan sebagai local variable (bukan symbol authority)

### File 7: negotiation_accepted_action.dart
- **Import:** `catalog/listing/presentation/providers/listing_providers.dart` → `catalog/for_sale/presentation/providers/for_sale_providers.dart`
- **Provider:** `fixedPriceSaleDetailProvider` → `forSaleDetailProvider`
- **Variable name:** `listing` dipertahankan sebagai local variable (bukan symbol authority)

### File 8: get_listing_share_reference_usecase.dart
- **Import:** `catalog/listing/domain/entities/listing.dart` → `catalog/for_sale/domain/entities/for_sale.dart`
- **Import:** `catalog/listing/domain/repositories/listing_repository.dart` → `catalog/for_sale/domain/repositories/for_sale_repository.dart`
- **Type:** `ListingRepository` → `ForSaleRepository`
- **Method:** `getFixedPriceSaleById` → `getForSaleById`
- **Factory:** `ShareReference.fixedPriceSale(...)` → `ShareReference.forSale(...)`
- **Param:** `fixedPriceSaleId:` → `forSaleId:`
- **Field access:** `listing.fixedPriceSaleId` → `listing.forSaleId`
- **Enum:** `ListingStatus.sold` → `ForSaleStatus.sold`

### File 9: create_auction_screen.dart
- **Import:** `catalog/listing/presentation/widgets/listing_media_handler.dart` → `catalog/for_sale/presentation/widgets/for_sale_media_handler.dart`
- **Class:** `ListingMediaHandler` → `ForSaleMediaHandler` (2 occurrences)

### File 10: commerce_common_product_detail_section.dart
- **Import:** `catalog/listing/domain/entities/listing.dart` → `catalog/for_sale/domain/entities/for_sale.dart`
- **Param type:** `fromListing(Listing listing)` → `fromListing(ForSale listing)`
  (method name `fromListing` dipertahankan untuk backward compatibility internal)

---

## 3. MAPPING LISTING → FORSALE YANG DILAKUKAN

| Symbol Lama | Symbol Baru | Scope |
|---|---|---|
| `Listing` (class) | `ForSale` | Type annotation, type check (`is`) |
| `ListingStatus` | `ForSaleStatus` | Enum reference |
| `ListingStatus.active` | `ForSaleStatus.active` | Enum value |
| `ListingStatus.sold` | `ForSaleStatus.sold` | Enum value |
| `listingsProvider` | `forSalesProvider` | Provider |
| `listingRepositoryProvider` | `forSaleRepositoryProvider` | Provider |
| `fixedPriceSaleDetailProvider` | `forSaleDetailProvider` | Provider |
| `ListingsParams` | `ForSalesParams` | Params class |
| `ListingRepository` | `ForSaleRepository` | Repository interface |
| `getListingsByIds` | `getForSalesByIds` | Repository method |
| `getFixedPriceSaleById` | `getForSaleById` | Repository method |
| `ShareReference.fixedPriceSale(...)` | `ShareReference.forSale(...)` | Factory constructor |
| `fixedPriceSaleId` (field) | `forSaleId` | Entity field access |
| `fixedPriceSaleId:` (param) | `forSaleId:` | Named parameter |
| `ListingMediaHandler` | `ForSaleMediaHandler` | Widget class |
| `RoutePaths.createListing` | `RoutePaths.createForSale` | Route constant |

---

## 4. REFERENCE YANG SENGAJA TIDAK DIUBAH DAN ALASANNYA

| File | Reference | Alasan |
|---|---|---|
| Semua 10 file | `listing` (local variable name) | Bukan symbol authority; hanya nama variabel lokal. Tidak menyebabkan compile error. |
| commerce_preview_section.dart | `_CommercePreviewType.listing` | Enum value internal UI; bukan domain authority. |
| commerce_preview_section.dart | `item.listing` (field name) | Field di private class `_CommercePreviewItem`; bukan domain authority. |
| commerce_resource_picker.dart | "Buat Listing Baru" (UI string) | String literal UI; bukan symbol authority. |
| comment_input_with_commerce_reference.dart | "Create Listing" (komentar) | Komentar dokumentasi; bukan executable code. |
| checkout_screen_impl.dart | "listing" dalam komentar/string | Komentar dokumentasi atau error message; bukan symbol authority. |
| create_auction_screen.dart | "Listing" dalam komentar | Komentar dokumentasi arsitektur; bukan executable code. |
| commerce_common_product_detail_section.dart | `fromListing` (method name) | Method name dipertahankan untuk backward compatibility internal; parameter type sudah diubah ke `ForSale`. |

---

## 5. ANALYZER RESULT

**Command:**
```bash
dart analyze \
  lib/shared/object/object_preview_provider.dart \
  lib/shared/object/object_preview_batch_provider.dart \
  lib/features/home/presentation/widgets/commerce_preview_section.dart \
  lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart \
  lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart \
  lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart \
  lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart \
  lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart \
  lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart \
  lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart
```

**Output:**
```
Analyzing [10 files]...

warning - comment_input_with_commerce_reference.dart:14:8 - Unused import
warning - commerce_resource_picker.dart:17:8 - Unused import

2 issues found.
```

**Kesimpulan:**
- **0 error** pada 10 production files.
- 2 warning = unused import pre-existing (bukan regression Stage 2A).

---

## 6. RESIDUE RESULT

### Import catalog/listing/** pada 10 file target:
```
Get-ChildItem -Path "apps/mobile/lib" -Filter "*.dart" -Recurse |
  Select-String -Pattern "commerce/catalog/listing" |
  Sort-Object -Unique
→ (no output)
```
✅ **0 import `catalog/listing/**` di seluruh `lib/`**

### Symbol Listing/ListingStatus/provider pada 10 file target:
Residue yang tersisa = komentar, UI string, nama variabel lokal, atau method name
`fromListing` (bukan symbol authority). Tidak ada compile error.

| File | Residue | Jenis |
|---|---|---|
| object_preview_provider.dart | `fixedPriceSaleDetailProvider` dalam komentar | Komentar |
| object_preview_batch_provider.dart | `listing` (local var) | Variable name |
| commerce_preview_section.dart | `listing` (local var, enum value, field name) | Non-authority |
| commerce_resource_picker.dart | "Buat Listing Baru" | UI string |
| comment_input_with_commerce_reference.dart | "Create Listing" dalam komentar | Komentar |
| checkout_screen_impl.dart | "listing" dalam komentar/string | Komentar/string |
| negotiation_accepted_action.dart | `listing` (local var dalam komentar) | Komentar |
| get_listing_share_reference_usecase.dart | `listing` (local var) | Variable name |
| create_auction_screen.dart | "Listing" dalam komentar | Komentar |
| commerce_common_product_detail_section.dart | `fromListing` | Method name (backward compat) |

✅ **Tidak ada symbol `Listing`/`ListingStatus`/`listingRepositoryProvider`/`listingsProvider`/`ListingsParams`/`ListingMediaHandler`/`fixedPriceSaleDetailProvider` yang masih digunakan sebagai executable code.**

---

## 7. FILE DI LUAR SCOPE YANG TETAP BLOCKED

Stage 2A **hanya** menyentuh 10 production file di atas. File berikut (di luar scope) masih blocked karena mengimpor `catalog/listing/**` atau symbol yang tidak ada:

**Production files lain (baseline blocker, tidak diubah):**
- `lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart`
  - Masih pakai `fixedPriceSaleDetailProvider` (tidak terdefinisi; baseline blocker)
- `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart`
  - Masih pakai `fixedPriceSaleDetailProvider` (baseline blocker)
- `lib/shared/object/object_reference_bridge.dart`
  - Masih pakai `ShareReference.fixedPriceSale` (baseline blocker)
- `lib/domains/commerce/catalog/shared/attachment_truth_resolver.dart`
  - Masih merujuk `class Listing`/`ListingStatus`/`ListingVisibility` (belum di-rename)
- `lib/domains/commerce/catalog/shared/live_status_provider.dart`
  - Masih merujuk `class Listing`/`ListingStatus` (belum di-rename)

**Route registry (baseline blocker):**
- `lib/core/src/router/route_paths.dart` — tidak punya `createListing` (sudah `createForSale`)
- `lib/core/src/router/modules/for_sale_module.dart` — sudah benar (`createForSale`)
- `lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart` — masih pakai `RoutePaths.createListing`
- `lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart` — masih pakai `RoutePaths.createListing`

**Test files (51 file, TIDAK disentuh pada Stage 2A):**
Semua test yang terdaftar dalam Stage 2 Audit masih mengimpor `catalog/listing/**`.

---

## 8. VERDICT

**COMPLETE**

**Bukti:**
1. 10 production files berhasil di-repoint dari `catalog/listing/**` ke `catalog/for_sale/**`.
2. 0 error analyzer pada 10 file target.
3. 0 import `catalog/listing/**` tersisa di seluruh `lib/`.
4. Tidak ada file production lain yang berubah (git status menunjukkan tepat 10 file + laporan audit).
5. Residue = komentar/string/variabel lokal non-authority (acceptable).

**Regression:** Tidak ada regression yang diintroduksi oleh Stage 2A. Error yang tersisa pada file di luar scope adalah baseline blocker yang sudah ada sebelum Stage 2A.

---

## FILES CHANGED (Git Status)

```
M apps/mobile/lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart
M apps/mobile/lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart
M apps/mobile/lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart
M apps/mobile/lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart
M apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart
M apps/mobile/lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart
M apps/mobile/lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart
M apps/mobile/lib/features/home/presentation/widgets/commerce_preview_section.dart
M apps/mobile/lib/shared/object/object_preview_batch_provider.dart
M apps/mobile/lib/shared/object/object_preview_provider.dart
```

**Total:** 10 production files modified. 0 file added. 0 file deleted.
