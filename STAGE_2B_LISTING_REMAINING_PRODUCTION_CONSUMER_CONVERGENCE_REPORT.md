# STAGE 2B — LISTING → FOR-SALE REMAINING PRODUCTION CONSUMER CONVERGENCE REPORT

**Tanggal:** 2026-08-27  
**Scope:** 7 production files (remaining Listing→ForSale consumers)  
**Mode:** Production consumers only; test files TIDAK disentuh.

---

## 1. VERDICT

**PARTIAL**

**Alasan:** 7 target file berhasil di-repoint dari Listing authority lama ke
`catalog/for_sale`. Semua executable references Listing (`Listing`, `ListingStatus`,
`ListingVisibility`, `fixedPriceSaleDetailProvider`, `RoutePaths.createListing`,
`ShareReference.fixedPriceSale`) sudah bersih. Namun setelah perbaikan, analyzer
mengungkap **2 baseline blocker terpisah** di `RoutePaths` (route constant
`listings` dan `sellerListings` tidak ada) yang BUKAN hasil edit Stage 2B dan
BERADA DI LUAR scope 7 file. 7 file tidak memiliki error dari perubahan sendiri.

---

## 2. 7 FILE TARGET

| No | File Path | Status |
|---|---|---|
| 1 | `lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart` | ✅ Modified |
| 2 | `lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart` | ✅ Modified |
| 3 | `lib/shared/object/object_reference_bridge.dart` | ✅ Modified |
| 4 | `lib/domains/commerce/catalog/shared/attachment_truth_resolver.dart` | ✅ Modified |
| 5 | `lib/domains/commerce/catalog/shared/live_status_provider.dart` | ✅ Modified |
| 6 | `lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart` | ✅ Modified |
| 7 | `lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart` | ✅ Modified |

---

## 3. PERUBAHAN SETIAP FILE (old → new)

### File 1: checkout_order_summary_section.dart
- (part of `checkout_screen_impl.dart`; tidak punya import sendiri)
- `fixedPriceSaleDetailProvider(fixedPriceSaleId)` → `forSaleDetailProvider(fixedPriceSaleId)`
- `final Listing listing;` → `final ForSale listing;`
- `state.widget.fixedPriceSaleId` (field widget) → **TIDAK diubah** (bukan field entity ForSale)

### File 2: checkout_screen_logic.dart
- (part of `checkout_screen_impl.dart`; tidak punya import sendiri)
- `fixedPriceSaleDetailProvider(state.widget.fixedPriceSaleId)` → `forSaleDetailProvider(...)` (2 occurrences)
- `state.widget.fixedPriceSaleId` (field widget) → **TIDAK diubah**

### File 3: object_reference_bridge.dart
- `ShareReference.fixedPriceSale(fixedPriceSaleId: objRef.id, ...)` → `ShareReference.forSale(forSaleId: objRef.id, ...)` (2 occurrences)

### File 4: attachment_truth_resolver.dart
- Import sudah benar (`catalog/for_sale/...`) — tidak diubah
- `factory ListingAttachmentStatusData.fromListing(Listing? listing)` → `fromListing(ForSale? listing)`
- `listing.status == ListingStatus.sold` → `ForSaleStatus.sold`
- `listing.status == ListingStatus.withdrawn` → `ForSaleStatus.withdrawn`
- `listing.visibility == ListingVisibility.private` → `ForSaleVisibility.private`
- Enum/class `ListingAttachmentStatus` (boundary lokal) → **TIDAK diubah**

### File 5: live_status_provider.dart
- Import sudah benar (`catalog/for_sale/...`) — tidak diubah
- `factory ListingLiveStatus.fromLive(Listing listing, ...)` → `fromLive(ForSale listing, ...)`
- `static ListingAvailabilityStatus fromListing(Listing listing)` → `fromListing(ForSale listing)`
- `listing.status == ListingStatus.sold` → `ForSaleStatus.sold`
- `listing.status == ListingStatus.withdrawn` → `ForSaleStatus.withdrawn`
- Enum/class `ListingLiveStatus`, `ListingAvailabilityStatus` (boundary lokal) → **TIDAK diubah**

### File 6: order_list_screen.dart
- `Navigator.pushNamed(context, core.RoutePaths.createListing)` → `core.RoutePaths.createForSale`
- `'Tambah Listing'` (UI string) → **TIDAK diubah**

### File 7: seller_dashboard_screen.dart
- `Navigator.pushNamed(context, RoutePaths.createListing)` → `RoutePaths.createForSale`
- `_navigateToCreateListing` (method name), `'Buat Listing'` / `'Listing Saya'` / `'Listing tidak terlihat?'` (UI string) → **TIDAK diubah**

---

## 4. REFERENCE YANG SENGAJA TIDAK DIUBAH + ALASAN

| File | Reference | Alasan |
|---|---|---|
| File 1, 2 | `state.widget.fixedPriceSaleId` | Field widget `CheckoutScreen.fixedPriceSaleId` (String), bukan field entity `ForSale`. Bukan authority Listing. |
| File 1, 2, 3, 4, 5 | `listing` (local variable) | Nama variabel lokal; bukan symbol authority. |
| File 4 | `ListingAttachmentStatus` (enum + class) | Boundary domain lokal milik `attachment_truth_resolver`; bukan `Listing` class authority. |
| File 4 | `fromListing` (method name) | Nama method internal; param type sudah diubah ke `ForSale`. |
| File 5 | `ListingLiveStatus`, `ListingAvailabilityStatus` (enum + class) | Boundary domain lokal milik `live_status_provider`; bukan `Listing` class authority. |
| File 5 | `fromListing` (method name) | Nama method internal; param type sudah diubah ke `ForSale`. |
| File 6 | `'Tambah Listing'` | UI string literal. |
| File 7 | `_navigateToCreateListing`, `'Buat Listing'`, `'Listing Saya'`, `'Listing tidak terlihat?'` | UI string / method name internal. |
| Semua | Komentar dokumentasi "Listing" | Bukan executable code. |

---

## 5. ANALYZER RESULT

**Command:**
```bash
dart analyze \
  lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart \
  lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart \
  lib/shared/object/object_reference_bridge.dart \
  lib/domains/commerce/catalog/shared/attachment_truth_resolver.dart \
  lib/domains/commerce/catalog/shared/live_status_provider.dart \
  lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart \
  lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart
```

**Output (relevant):**
```
error - order_list_screen.dart:531:63 - The getter 'listings' isn't defined for 'RoutePaths'
error - seller_dashboard_screen.dart:1833:45 - The getter 'sellerListings' isn't defined for 'RoutePaths'
info  - seller_dashboard_screen.dart (5x) unnecessary_underscores (pre-existing)
```

**Analisis:**
- **0 error** yang berasal dari perubahan Stage 2B (semua executable `Listing`/`createListing`/`ShareReference.fixedPriceSale` sudah bersih).
- 2 error di atas adalah **baseline blocker terpisah** (lihat §7).
- 5 `info` (`unnecessary_underscores`) adalah warning pre-existing, bukan regression.

---

## 6. RESIDUE GREP RESULT

### Import `catalog/listing/**` pada 7 target:
```
(patt: commerce/catalog/listing) → 0 matches
```

### Executable symbol Listing authority pada 7 target:
```
\bListing\b (as type)           → 0 (hanya local var `listing`)
ListingStatus                   → 0
ListingVisibility               → 0
fixedPriceSaleDetailProvider    → 0
RoutePaths.createListing        → 0
ShareReference.fixedPriceSale   → 0
```

### Sisa yang dipertahankan (bukan authority, acceptable):
- Local variable `listing` (di semua file)
- Enum/class lokal `ListingAttachmentStatus`, `ListingLiveStatus`, `ListingAvailabilityStatus` (boundary domain lokal)
- Method name `fromListing` (internal)
- Komentar dokumentasi "Listing"
- UI string "Listing"/"Buat Listing"/"Tambah Listing"/"Listing Saya"

---

## 7. BASELINE BLOCKER TERPISAH

Dua error berikut muncul SETELAH perbaikan Stage 2B karena file kini terkompilasi
lebih dalam (sebelumnya gagal lebih awal di `createListing`):

1. **`order_list_screen.dart:531`** — `RoutePaths.listings` tidak terdefinisi.
   - `RoutePaths` hanya punya `forSales` (browse) — `listings`→`forSales` adalah mapping selanjutnya.
   - **Bukan** hasil edit Stage 2B (line 531 tidak disentuh; hanya line 528 yang diubah).

2. **`seller_dashboard_screen.dart:1833`** — `RoutePaths.sellerListings` tidak terdefinisi.
   - `RoutePaths` hanya punya `sellerForSales` — `sellerListings`→`sellerForSales` adalah mapping selanjutnya.
   - **Bukan** hasil edit Stage 2B (line 1833 tidak disentuh).

**Tindakan:** Sesuai hard boundary ("jangan melebar sendiri"), dua blocker ini
**TIDAK diperbaiki** pada Stage 2B. Direkomendasikan sebagai stage lanjutan
(Stage 2C atau route-convergence terpisah): ganti `RoutePaths.listings`→`forSales`
dan `RoutePaths.sellerListings`→`sellerForSales` di file terkait.

---

## 8. TEST RESULT

**Tidak dijalankan.** Stage 2B hanya menyentuh 7 production files; 51 test consumer
Stage 2 tetap blocked (di luar scope, tidak disentuh). Test yang relevan tidak
dapat dikompilasi karena dependency listing lain (termasuk baseline blocker §7).

---

## 9. EXACT FILES CHANGED (git status)

```
M apps/mobile/lib/domains/commerce/transaction/checkout/presentation/widgets/checkout_order_summary_section.dart
M apps/mobile/lib/domains/commerce/transaction/checkout/presentation/screens/checkout_screen_logic.dart
M apps/mobile/lib/shared/object/object_reference_bridge.dart
M apps/mobile/lib/domains/commerce/catalog/shared/attachment_truth_resolver.dart
M apps/mobile/lib/domains/commerce/catalog/shared/live_status_provider.dart
M apps/mobile/lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart
M apps/mobile/lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart
```

**Konfirmasi:**
- Tepat 7 production files berubah (sesuai target).
- Tidak ada file production lain yang diubah.
- Tidak ada test file yang diubah.
- Tidak ada business logic / UI behavior yang diubah.
- Tidak ada global rename `fixedPriceSale`→`forSale`.
- Tidak ada shim/alias `Listing` dibuat.

---

## 10. KESIMPULAN

Stage 2B **COMPLETE untuk scope 7 file**: semua executable Listing authority
pada 7 target sudah di-migrate ke `catalog/for_sale`. Verdict **PARTIAL** karena
2 baseline blocker `RoutePaths` terungkap pasca-perbaikan dan berada di luar scope.
Rekomendasi: tangani `listings`→`forSales` dan `sellerListings`→`sellerForSales`
pada stage terpisah sebelum melanjutkan ke 51 test consumer.
