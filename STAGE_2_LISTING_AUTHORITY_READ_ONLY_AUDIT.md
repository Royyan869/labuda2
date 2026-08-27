# STAGE 2 — LISTING AUTHORITY READ-ONLY AUDIT

**Mode:** READ-ONLY. Tidak ada perubahan production code, test, schema, atau rename.
**Tanggal:** 2026-08-27
**Scope:** `catalog/listing` + dependency langsung untuk menentukan Listing/For-Sale authority.

---

## 1. VERDICT

**AUDIT COMPLETE — STALE CONSUMERS ONLY**

Canonical Listing/For-Sale authority **masih hidup**, bukan hilang. Authority telah
dipindahkan secara resmi ke `apps/mobile/lib/domains/commerce/catalog/for_sale/**`
dengan entity `ForSale`, enum `ForSaleStatus`/`ForSaleVisibility`, provider
`forSaleDetailProvider`/`forSaleRepositoryProvider`, dan `SellerFPSPagerController`
(`sellerFPSPagerProvider`).

Directory `apps/mobile/lib/domains/commerce/catalog/listing/**` **terbukti absen dari
filesystem**. 10 production file + 51 test file masih mengimpor path
`commerce/catalog/listing/**` (dan simbol `Listing`, `ListingStatus`,
`listingRepositoryProvider`, `listingsProvider`, dll.) yang tidak ada lagi di manapun
di `lib/`. Semua blocker compile berasal dari **stale consumer yang belum dimigrasi**
ke `for_sale` (klasifikasi **C**), bukan dari hilangnya implementation authority
(klasifikasi **D**).

---

## 2. FACTUAL FILESYSTEM MAP

**Bukti keberadaan `catalog/listing`:**
```
Test-Path 'apps/mobile/lib/domains/commerce/catalog/listing'  →  False
glob 'apps/mobile/lib/domains/commerce/catalog/listing/**/*'  →  No files found
glob 'apps/mobile/lib/domains/commerce/**/listing*.dart'      →  No files found
glob 'apps/mobile/lib/domains/commerce/catalog/for_sale/**/listing*' → No files found
```
→ **Directory `listing/` dan semua file `listing_*.dart` benar-benar tidak ada.**

**Bukti `for_sale/` (canonical authority) ADA dan lengkap:**
```
apps/mobile/lib/domains/commerce/catalog/for_sale/
├── for_sale.dart                          (barrel: domain/data/presentation)
├── domain/domain.dart                     (export entity + repository)
├── domain/entities/for_sale.dart          (ForSale, ForSaleStatus, ForSaleVisibility)
├── domain/repositories/for_sale_repository.dart
├── data/{data.dart, dto/, mappers/, remote/, repositories/}
└── presentation/
    ├── presentation.dart
    ├── create_for_sale_route_contract.dart
    ├── providers/for_sale_controller.dart
    ├── providers/for_sale_providers.dart   (forSaleDetailProvider, forSaleRepositoryProvider)
    ├── providers/seller_fps_pager.dart     (sellerFPSPagerProvider — pengganti listing)
    ├── screens/{create,edit,for_sale_detail,for_sale_list,my_for_sales}_screen.dart
    └── widgets/{for_sale_card,for_sale_media_handler,for_sale_picker_bottom_sheet}.dart
```

**Simbol `Listing`/`ListingStatus`/`ListingVisibility` tidak ada di manapun di `lib/`:**
```
Select-String 'class Listing\b|enum ListingStatus\b|enum ListingVisibility\b|
listingRepositoryProvider|listingsProvider\b|class ListingsParams\b|
fixedPriceSaleDetailProvider\b'  in apps/mobile/lib/**/*.dart
→ 0 definisi; hanya penyebutan di file yang import path listing/** yang sudah hilang.
```

---

## 3. MISSING FILE/SYMBOL INVENTORY

Semua item berikut **terbukti hilang** dari filesystem (bukan asumsi grep):

| Missing path / symbol | Direferensikan oleh | Ada pengganti di `for_sale`? |
|---|---|---|
| `catalog/listing/domain/domain.dart` | 10 prod + 22 test | `catalog/for_sale/domain/domain.dart` |
| `catalog/listing/domain/entities/listing.dart` (`class Listing`) | 6 prod + 6 test | `for_sale.dart` (`class ForSale`) |
| `catalog/listing/domain/repositories/listing_repository.dart` (`ListingRepository`) | 2 test | `for_sale_repository.dart` (`ForSaleRepository`) |
| `catalog/listing/presentation/providers/listing_providers.dart` (`listingsProvider`, `listingRepositoryProvider`) | 11 prod + 19 test | `for_sale_providers.dart` (`forSaleDetailProvider`, `forSaleRepositoryProvider`) |
| `catalog/listing/presentation/providers/listing_controller.dart` | 2 test | `for_sale_controller.dart` |
| `catalog/listing/presentation/providers/seller_fps_pager.dart` (`sellerFPSPagerProvider`) | 2 prod + 2 test | `seller_fps_pager.dart` (`sellerFPSPagerProvider`) |
| `catalog/listing/presentation/widgets/listing_media_handler.dart` | 2 prod + 2 test | `for_sale_media_handler.dart` (`ForSaleMediaHandler`) |
| `catalog/listing/presentation/widgets/listing_card.dart` | 1 test | `for_sale_card.dart` |
| `catalog/listing/presentation/create_listing_route_contract.dart` | 1 test | `create_for_sale_route_contract.dart` |
| `catalog/listing/presentation/screens/{create,edit,listing_detail,listing_list,my_listings}_screen.dart` | 2 test + 3 test (path-string) | `for_sale/*_screen.dart` |
| `catalog/listing/data/dto/shipping_quote_dto.dart` | 1 test | `for_sale/data/dto/shipping_quote_dto.dart` |
| Simbol `ListingStatus`, `ListingVisibility`, `ListingsParams` | prod + test | `ForSaleStatus`, `ForSaleVisibility`, `ForSalesParams` |

---

## 4. CALLER MAP

**Total: 65 referensi** (10 unique production files, 51 unique test files).

**Production callers (10 files) — semua import `catalog/listing/**`:**
1. `lib/shared/object/object_preview_provider.dart:10` → `listing_providers.dart`
2. `lib/shared/object/object_preview_batch_provider.dart:16` → `listing_providers.dart`
3. `lib/features/home/presentation/widgets/commerce_preview_section.dart:6,7` → `domain/domain.dart`, `listing_providers.dart`
4. `lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart:15,16` → `domain/domain.dart`, `seller_fps_pager.dart`
5. `lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart:13` → `domain/domain.dart`
6. `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart:55,56` → `listing.dart`, `listing_providers.dart`
7. `lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart:12` → `listing_providers.dart`
8. `lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart:2,3` → `listing.dart`, `listing_repository.dart`
9. `lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart:16` → `listing_media_handler.dart`
10. `lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart:6` → `listing.dart`

**Test callers (51 files)** — lihat §7 untuk pengelompokan cluster.

---

## 5. CANONICAL LISTING/FOR-SALE AUTHORITY

Berdasarkan source aktual `catalog/for_sale/`:

- **Canonical entity:** `ForSale` (`for_sale/domain/entities/for_sale.dart:102`)
- **Canonical ID field:** `forSaleId` (String) — bukan `listingId`
- **Canonical status enum:** `ForSaleStatus` (`draft, active, sold, withdrawn`)
- **Canonical visibility enum:** `ForSaleVisibility` (`private, public`)
- **Canonical repository:** `ForSaleRepository` + `ForSaleRepositoryImpl`
- **Canonical controller:** `ForSaleController` (`for_sale_controller.dart`)
- **Canonical providers:** `forSaleDetailProvider`, `forSaleRepositoryProvider`,
  `sellerFPSPagerProvider` (`seller_fps_pager.dart`)
- **Canonical route contract:** `create_for_sale_route_contract.dart`
- **Canonical media handler:** `ForSaleMediaHandler` (`for_sale_media_handler.dart`)

Field parity memadai untuk migrasi: `ForSale` memiliki `fixedPriceSaleId`, `title`,
`price`, `media`, `stock`, `status`, `visibility`, `formattedPrice`, `isAvailable`,
`isAvailableForCommerce` — ekuivalen 1:1 dengan ekspektasi `Listing` di caller lama.

→ **Authority tidak hilang; hanya consumer yang belum di-repoint ke `for_sale`.**

---

## 6. DEPENDENCY CHAINS

Chain 1 (Home feed):
`home_screen_*_test` → `commerce_preview_section.dart` → `listing_providers.dart`
(missing) + `domain/domain.dart` (missing). BLOCKED.

Chain 2 (Comment/Composer):
`comment_input_create_listing_gorouter_test` /
`create_content_sale_picker_widget_test` →
`comment_input_with_commerce_reference.dart` + `commerce_resource_picker.dart`
→ `domain/domain.dart` + `seller_fps_pager.dart` (missing). BLOCKED.
`commerce_resource_picker.dart` sendiri sudah memakai `sellerFPSPagerProvider`
(yang ADA di `for_sale`) — hanya import path-nya salah.

Chain 3 (Chat):
`chat_detail_composer_authority_test` / `chat_seller_quote_cta_test` /
`shipping_quote_contract_test` → import `listing.dart` / `listing_repository.dart` /
`listing_providers.dart` / `shipping_quote_dto.dart` (semua missing). BLOCKED.
Pengganti ada di `for_sale/domain/entities/for_sale.dart`, `for_sale_repository.dart`,
`for_sale_providers.dart`, `for_sale/data/dto/shipping_quote_dto.dart`.

Chain 4 (Checkout):
`checkout_screen_impl.dart` + `checkout_shipping_out_of_coverage_widget_test` →
`listing.dart` + `listing_providers.dart` (missing). BLOCKED.

Chain 5 (Catalog-shared, internal):
`get_listing_share_reference_usecase.dart` → `listing.dart` + `listing_repository.dart`
(missing). BLOCKED.
`catalog/shared/attachment_truth_resolver.dart` dan `live_status_provider.dart`
masih merujuk `class Listing` / `ListingStatus` / `ListingVisibility` secara internal
(bukan via import path lama) — file ini juga broken karena `Listing` tidak ada di
`lib/`. Ini bukti tambahan migration `for_sale` belum tuntas di layer `shared/`
(terkait, di luar scope path `listing/**` langsung).

---

## 7. CROSS-CLUSTER IMPACT

Tujuan: hanya dependency chain, tidak memperbaiki.

| Cluster | Test file terdampak | Root cause |
|---|---|---|
| **Comment** | `comment_input_create_listing_gorouter_test.dart`, `social/content/create_content_sale_picker_widget_test.dart` | import `domain/domain.dart`, `seller_fps_pager.dart`, `listing_controller.dart` (missing) |
| **Search/Explore** | `features/explore/explore_promotion_injection_test.dart` | import `listing.dart`, `listing_repository.dart`, `listing_providers.dart`, `listing_card.dart` (missing) |
| **Profile** | `user/profile/.../address_route_initial_tab_contract_test.dart`, `user/preference/seller/.../seller_shipping_sender_address_section_test.dart` | string-path ref `create_listing_screen.dart` (missing) |
| **Chat** | `chat_detail_composer_authority_test`, `chat_seller_quote_cta_test`, `shipping_quote_contract_test` | import `listing.*` (missing) |
| **Home/Feed** | 7 file `features/home/**` | import `domain/domain.dart`, `listing_providers.dart` (missing) |
| **Auth** | `auth_portal_protected_provider_blocking_test.dart` | import `domain/domain.dart`, `listing_providers.dart` |
| **Router** | `core/router/router_lifetime_preservation_test.dart` | import `domain/domain.dart`, `listing_providers.dart`, `create_listing_screen.dart`, `listing_detail_screen.dart` |
| **Checkout** | `checkout_shipping_out_of_coverage_widget_test.dart` | import `listing.dart`, `listing_providers.dart` |
| **Commerce-internal** | `auction_media_source_contract_test`, `create_auction_screen_media_test`, `sale_preparation_note_contract_test`, `commerce_detail_negative_contracts_test`, `listing_media_handler_validation_test` | import `listing_detail_screen.dart` / `listing_media_handler.dart` (missing) |

**Kesimpulan cross-cluster:** Semua cluster lain (Comment/Search/Profile/Chat/Auth/
Router/Checkout) gagal compile **semata-mata karena mereka mengimpor path
`catalog/listing/**` yang sudah dihapus**. Tidak ada dependency logic yang rusak di
cluster tersebut; mereka hanya menunggu repoint import ke `catalog/for_sale/**`.

---

## 8. CLASSIFICATION A/B/C/D/E

- **A — active canonical implementation:** `catalog/for_sale/**` (ForSale,
  ForSaleStatus, ForSaleVisibility, ForSaleRepository, ForSaleController,
  forSaleDetailProvider, forSaleRepositoryProvider, sellerFPSPagerProvider,
  ForSaleMediaHandler, create_for_sale_route_contract). Authority hidup.
- **B — legitimate domain-owned legacy vocabulary/boundary:** N/A di scope ini.
  (Catatan: `ListingLiveStatus`/`ListingAttachmentStatus` di `catalog/shared/` adalah
  boundary legacy yang belum di-rename, tapi mereka merujuk `Listing` yang sudah
  tidak ada → effectively broken, bukan boundary hidup.)
- **C — stale test/consumer:** 10 production files + 51 test files yang masih import
  `catalog/listing/**`. Inilah penyebab seluruh compile blocker.
- **D — genuinely missing production implementation:** **TIDAK TERBUKTI.** Authority
  ada di `for_sale`. Tidak ada implementation Listing yang hilang tanpa pengganti.
- **E — unrelated external blocker:** Tidak ditemukan dalam scope ini.

---

## 9. BASELINE BLOCKERS

**BLOCKED BASELINE (diakui, BUKAN berasal dari scope ini):**
- Whole-package compile gagal karena 65 impor ke `catalog/listing/**` yang absen.
- `catalog/shared/attachment_truth_resolver.dart` & `live_status_provider.dart`
  masih merujuk `class Listing`/`ListingStatus`/`ListingVisibility` yang undefined
  (migration `for_sale` belum tuntas di layer `shared/`). Ini blocker terkait tapi
  berada di luar path `listing/**` langsung.

**Masalah yang BENAR-BENAR berasal dari scope ini:**
- 10 production file + 51 test file mengimpor `catalog/listing/**` (missing) →
  staleness murni (C), bukan hilangnya authority (D).

---

## 10. EXACT NEXT BOUNDED STAGE

**Satu next step:** Lakukan migrasi repoint (bukan implementasi baru) dari
`catalog/listing/**` → `catalog/for_sale/**` pada **10 production file** terlebih
dulu, lalu **51 test file**. Authority (`ForSale`) sudah ada dan parity field 1:1,
jadi ini murni pemindahan import + penyesuaian nama simbol (`Listing`→`ForSale`,
`ListingStatus`→`ForSaleStatus`, `listingRepositoryProvider`→`forSaleRepositoryProvider`,
`listingsProvider`→`forSaleDetailProvider`, `sellerFPSPagerProvider` tetap sama lintas
path, `ListingMediaHandler`→`ForSaleMediaHandler`, route contract path, screen path).

**Minimum file/symbol yang harus ditangani pada stage berikutnya (production dulu):**

| File (production) | Simbol yang harus di-repoint |
|---|---|
| `lib/shared/object/object_preview_provider.dart` | `listing_providers.dart` → `for_sale_providers.dart` |
| `lib/shared/object/object_preview_batch_provider.dart` | `listing_providers.dart` → `for_sale_providers.dart`; `listingRepositoryProvider`→`forSaleRepositoryProvider`; `getListingsByIds`→`getForSalesByIds`; `Listing`→`ForSale`; `ListingStatus`→`ForSaleStatus`; `listing.fixedPriceSaleId` tetap sama |
| `lib/features/home/presentation/widgets/commerce_preview_section.dart` | `domain/domain.dart`→`for_sale/domain/domain.dart`; `listing_providers.dart`→`for_sale_providers.dart`; `Listing`→`ForSale`; `ListingStatus`→`ForSaleStatus`; `listingsProvider`→`forSaleDetailProvider` (atau `forSalesProvider`); `ListingsParams`→`ForSalesParams`; `listing.fixedPriceSaleId` tetap |
| `lib/domains/social/comment/presentation/widgets/commerce_resource_picker.dart` | `domain/domain.dart`→`for_sale/domain/domain.dart`; `seller_fps_pager.dart`→`for_sale/.../seller_fps_pager.dart`; `ListingStatus`→`ForSaleStatus`; `l.fixedPriceSaleId` tetap |
| `lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart` | `domain/domain.dart`→`for_sale/domain/domain.dart`; `Listing`→`ForSale`; `RoutePaths.createListing` & `Listing` return type (handle `ForSale` dari route) |
| `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_impl.dart` | `listing.dart`→`for_sale/domain/entities/for_sale.dart`; `listing_providers.dart`→`for_sale_providers.dart`; `Listing`→`ForSale` |
| `lib/domains/commerce/negotiation/negotiation/presentation/widgets/negotiation_accepted_action.dart` | `listing_providers.dart`→`for_sale_providers.dart` |
| `lib/domains/commerce/catalog/usecases/get_listing_share_reference_usecase.dart` | `listing.dart`→`for_sale/.../for_sale.dart`; `listing_repository.dart`→`for_sale_repository.dart`; `ListingRepository`→`ForSaleRepository`; `getFixedPriceSaleById`→`getForSaleById`; `Listing`→`ForSale`; `ListingStatus`→`ForSaleStatus`; `listing.fixedPriceSaleId` tetap |
| `lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart` | `listing_media_handler.dart`→`for_sale/.../for_sale_media_handler.dart`; `ListingMediaHandler`→`ForSaleMediaHandler` |
| `lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart` | `listing.dart`→`for_sale/.../for_sale.dart`; `Listing`→`ForSale` |

**Plus perbaikan layer `shared/` (terkait, blocker baseline):** rename internal
`Listing`/`ListingStatus`/`ListingVisibility` → `ForSale*` di
`catalog/shared/attachment_truth_resolver.dart` & `live_status_provider.dart`
(gunakan `forSaleDetailProvider` yang sudah dipakai `attachment_truth_resolver.dart`).

**Test files (51):** repoint import path `catalog/listing/**` → `catalog/for_sale/**`
dan nama simbol sesuai produksi di atas.

**TIDAK PERLU:** implementasi baru, compatibility shim, rename lintas domain
`fixedPriceSale`, atau pull dari Git history.

---

## 11. FILES CHANGED

**NONE.** Audit read-only. Tidak ada file yang dimodifikasi, dihapus, dibuat,
atau di-rename.
