# HANDOFF — ADDRESS: SATU BUKU ALAMAT PER AKUN

**Status:** OPEN — canonical design implemented and converged
**Scope:** identity/address (backend) + profile address (mobile) + seller/checkout/product/order consumers

---

## DOMAIN / SCOPE

Domain `address`. Menjawab satu pertanyaan: *alamat itu milik siapa dan untuk apa?*

## BUSINESS TRUTH

1. Alamat dimiliki **AKUN**. Satu buku alamat untuk seluruh akun (0..N alamat).
2. Jika akun punya ≥1 alamat aktif, **tepat satu alamat adalah alamat utama**.
   Jika akun punya 0 alamat, tidak ada alamat utama.
3. Alamat pertama otomatis menjadi alamat utama.
4. Menambahkan alamat baru **tidak mengubah** alamat utama yang sudah ada.
5. Alamat utama hanya berubah saat user **secara eksplisit** memilih
   "Jadikan alamat utama".
6. Menghapus alamat utama: sistem otomatis memilih **alamat aktif tertua**
   (`created_at ASC, id ASC`) sebagai alamat utama baru. Menghapus alamat
   terakhir: akun tidak punya alamat utama.
7. **Alamat utama adalah satu-satunya default address akun**: default tujuan
   kirim dan default origin untuk **semua produk**.
8. Label/nickname adalah label pengenalan user (Rumah, Kantor, ...). Bukan
   peran bisnis, bukan authority, tidak dipakai untuk branching.
9. **Tidak ada** `shipping address`, `sender address`, `address purpose`,
   `address tag`, atau `farm_address_id` (alamat per produk).

Model:

```text
User
 └── Address Book
      ├── Address A
      ├── Address B
      └── Primary Address ← satu-satunya default address

All Products
 └── use User Primary Address
```

## CANONICAL AUTHORITY

| Concern | Authority |
|---|---|
| Buku alamat | `addresses` (Postgres), milik `user_id` |
| Primary | `is_primary`, **satu per akun**, dijaga `idx_addresses_user_active_primary_unique` |
| Soft delete / visibility | `is_available_for_checkout` |
| Public origin line | `ResolvePublicOrigin` → `entity.BuildPublicOriginSummary` (derived, **bukan salinan**) — primary → oldest fallback |
| Form alamat di mobile | `AddressFormDialog` (satu-satunya) |
| Wire contract | tanpa `tags`/`tag_labels`/`?tag=`; label = `nickname` |

## FORBIDDEN / KILLED (jangan dihidupkan lagi)

- Kolom `addresses.purpose`.
- `addresses.tags text[]` + constraint/index-nya.
- `AddressTag`, `TagShipping`, `TagSender`, `HasTag`, `NormalizeTags`,
  `IsValidTag`, `TagStrings`, `TagsFrom`, `InvalidTagsError`.
- DTO `tags`/`tag_labels`, query `?tag=`, filter by-tag repository/service.
- `GetByUserIDFiltered`, `GetPrimaryByTag`.
- Seller onboarding `sender_address` gate (`hasSenderAddress`).
- `for_sale`/`order`/`auction` sender-tag/shipping-tag lookups.
- `products.farm_address_id` + FK; `listings.farm_address_id`.
- Mobile `AddressTag` + extensions, `hasTag`, `tagValues`, DTO `tags`.
- `getAddressesByTag`, `watchAddressesByTag`, `watchAddresses` (polling 30s).
- `senderAddressIdProvider`, `primaryShippingAddressProvider`,
  `addressesStreamProvider`, duplicate `primaryAddressProvider`/`addressCountProvider`.
- `AddressFormDialog.presetTags` dan tag selector.
- `ShippingSetup.farmAddressId` (dead).

## MIGRATION

- `000001_canonical_schema.up.sql` — baseline sudah **tanpa** `purpose`,
  `idx_addresses_purpose`, `addresses_purpose_check`, `products.farm_address_id`,
  `listings.farm_address_id`.
- `000119_address_tags_over_purpose.up.sql` — **DIHAPUS** (desain tag ditolak).
- `000124_address_single_book_convergence.up.sql` — konvergensi idempotent untuk
  DB lama: drop `tags`/constraint/index, `purpose`+residue, `farm_address_id`.
- `000017_primary_address_invariant_hardening.up.sql` — invariant primary
  (canonical, dipertahankan).

## BACKEND

- `entity.Address` tanpa `Tags`; `NewAddress` tanpa `tags`.
- `AddressService`: `CreateAddress`/`UpdateAddress` tanpa tags; `reconcile`
  hanya menjaga primary (0→none, 1→primary, ≥2→tepat satu, promote oldest).
  `enforceSingleAddressRule` hanya menetapkan primary untuk alamat pertama.
- `GetPrimaryFiltered` = `GetPrimaryByUserID` (tanpa tag).
- Repository: kolom tanpa `tags`; `GetByUserIDFiltered`/`GetPrimaryByTag`/`oldestActiveAddress` dihapus.
- Handler DTO tanpa tags; `GET /addresses` & `GET /addresses/primary` tanpa `?tag=`.
- `ResolvePublicOrigin`: primary → oldest (tanpa chain tag/farm).
- Seller onboarding: butuh **primary address**, kode `primary_address`.
- `for_sale`: `EnsureSellerOriginValid` (butuh primary), kode `SELLER_ORIGIN_NOT_CONFIGURED`.
- Order origin snapshot: dari **primary address** penjual (`getSellerOriginSnapshot`).
- Auction winner default address: `GetPrimaryByUserID`.
- Product: `FarmAddressID` dihapus dari entity/repository/proyeksi.

## MOBILE

- `AddressEntity` tanpa tags; `nickname` = label bebas.
- `AddressResponseApi`/`CreateAddressRequestApi`/`UpdateAddressRequestApi` tanpa tags.
- `IAddressRepository`: `getAddressesByUserId`, `getPrimaryAddress`, `countAddresses` (tanpa tag/stream).
- Satu path state kanonik: `AddressNotifier`/`addressProvider` + `addressesListProvider`.
- **Settings**: `addressesFutureProvider(userId)` — one-shot read langsung,
  tanpa polling 30s (loading instan).
- Seller wizard: memakai **primary address akun** (bukan sender address),
  error business-facing (tanpa raw `Result.error`).
- Checkout & auction claim: `loadAddresses`/`getAddressesByUserId` (tanpa tag).
- Form alamat: label bebas untuk semua alamat; tidak ada role selector.

## CLEANUP

- `backend`: `AddressTag`/`Tag*`/`Purpose`/`farm_address_id`, migrasi `000119`.
- `apps/mobile/lib`: `sender_address_provider.dart`, `address_list_provider.dart`,
  tag methods/DTO/provider, `presetTags`, `ShippingSetup.farmAddressId`.
- Fixtures & test dikonvergenkan.

## POSITIVE PROOF

```
backend : go build ./...                      → bersih
backend : go vet ./...                         → bersih
backend : go test ./internal/identity/address/... ./internal/commerce/...  → PASS
mobile  : dart analyze lib                     → 0 error (info pre-existing)
mobile  : dart analyze test                    → 0 error
mobile  : flutter test (address + seller + checkout + auction + for_sale) → PASS
```

## NEGATIVE PROOF / RESIDUE SEARCH

```
backend : grep -rn "AddressTag|TagSender|TagShipping|farm_address_id|GetPrimaryByTag|GetByUserIDFiltered" internal → 0 (kecuali komentar FORBIDDEN)
mobile  : grep -rn "AddressTag|hasTag|tagValues|getAddressesByTag|watchAddresses|senderAddressIdProvider|primaryShippingAddressProvider|farmAddressId" lib → 0
```

## KNOWN LIMITATIONS

- `UPDATE /addresses/:id` tetap mewarisi tag pada DB lama lewat migration 000124
  (drop kolom), jadi update tidak lagi menyentuh peran.
- `ShippingSetup.farmAddressId` dihapus; shipping option tidak lagi menunjuk alamat.

## OUT OF SCOPE (tercatat, TIDAK dikerjakan)

1. Pre-existing test stale `negotiation_default_contract_test.dart` membaca
   `edit_for_sale_screen.dart` yang sudah dihapus oleh sesi lain (uncommitted).
2. Domain payment/ledger, search/discovery, promotion — tidak disentuh.

## NEXT SCOPE

1. Jalankan migrasi `000124` + runtime proof.
2. Promotion scope (bila ada) — lewat entitas baru, bukan tag alamat.
