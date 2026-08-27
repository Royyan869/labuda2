# STAGE 2C — LISTING ROUTE CONVERGENCE REPORT

## VERDICT

**PASS**

---

## AUTHORITY

**RoutePaths definition** (`lib/core/src/router/route_paths.dart`):

| Canonical Constant | Path Value | Line |
|---|---|---|
| `RoutePaths.forSales` | `'/for-sale'` | 25 |
| `RouteNames.forSales` | `'forSales'` | 121 |
| `RoutePaths.sellerForSales` | `'/seller/for-sale'` | 81 |
| `RouteNames.sellerForSales` | `'sellerForSales'` | 166 |

**Consumer → Canonical Mapping:**

| Old Reference (missing) | Canonical Replacement (exists) |
|---|---|
| `RoutePaths.listings` | `RoutePaths.forSales` |
| `RoutePaths.sellerListings` | `RoutePaths.sellerForSales` |

**Bukti:** Kedua constant sudah didefinisikan di `route_paths.dart` (verified via `Select-String`).
Tidak perlu membuat route baru.

---

## FILES CHANGED

| File | Old Reference | New Reference |
|---|---|---|
| `lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart:531` | `core.RoutePaths.listings` | `core.RoutePaths.forSales` |
| `lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart:1833` | `RoutePaths.sellerListings` | `RoutePaths.sellerForSales` |

**Tepat 2 file berubah.** Tidak ada file production/test/doc lain yang diubah.

---

## DIFF SCOPE

Tepat 1 baris per file. Perubahan:
- order_list_screen.dart:531 — `pushReplacementNamed(context, core.RoutePaths.listings)` → `...forSales`
- seller_dashboard_screen.dart:1833 — `pushNamed(context, RoutePaths.sellerListings)` → `...sellerForSales`

Tidak ada perubahan di luar route destination. Tidak ada perubahan business logic.
Tidak ada perubahan UI text. Tidak ada rename `fixedPriceSale`.

---

## ANALYZER

**Command:**
```bash
dart analyze \
  lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart \
  lib/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart
```

**Result:**
```
Analyzing order_list_screen.dart, seller_dashboard_screen.dart...

6 issues found (6 info — unnecessary_underscores, pre-existing)
```

**0 error, 0 warning.** Semua error `RoutePaths.listings` (order_list_screen:531)
dan `RoutePaths.sellerListings` (seller_dashboard_screen:1833) dari Stage 2B
sudah teratasi.

---

## RESIDUE

```
RoutePaths.listings (production):    1 consumer (OUT OF SCOPE — see baseline)
RoutePaths.sellerListings (production): 0 consumers — CLEAN
```

**Detail:**
- `RoutePaths.sellerListings`: **0** production consumers. Fully converged.
- `RoutePaths.listings`: 1 remaining consumer — `home_screen.dart:334`
  (`context.push(RoutePaths.listings)`).
  **Ini BUKAN target Stage 2C** (hanya 2 consumer dari Stage 2B). 
  Baseline blocker terpisah, di luar hard boundary Stage 2C.

---

## BASELINE BLOCKERS

Berikut **pre-existing blockers** yang DITEMUKAN tapi TIDAK diperbaiki (di luar scope Stage 2C):

1. **`home_screen.dart:334`** — `RoutePaths.listings` tidak terdefinisi.
   - Consumer baru, bukan bagian dari 2 target Stage 2B.
   - Per boundary: "Ubah hanya dua consumer tersebut." Tidak diperbaiki.
   - Direction: `RoutePaths.listings` → `RoutePaths.forSales` (sama seperti order_list_screen).

**Tidak ada regression Stage 2C.** Kedua error dari Stage 2B (order_list_screen:531,
seller_dashboard_screen:1833) sudah teratasi.

---

## FINAL STATUS

Stage 2C **PASS** — kedua route reference target sudah di-migrate:
- `order_list_screen.dart:531` → `RoutePaths.forSales` ✅
- `seller_dashboard_screen.dart:1833` → `RoutePaths.sellerForSales` ✅
- 0 analyzer error pada 2 target
- 0 regression
- `home_screen.dart:334` (`RoutePaths.listings`) adalah baseline blocker terpisah
  di luar scope 2C (direction: `RoutePaths.forSales`).
