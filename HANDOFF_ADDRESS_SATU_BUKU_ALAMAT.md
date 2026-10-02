# HANDOFF — ADDRESS: SATU BUKU ALAMAT PER AKUN

**Status:** CLOSED — CANONICAL DESIGN LOCKED
**Scope:** identity/address (backend) + profile address (mobile) + checkout/seller consumers

---

## DOMAIN / SCOPE

Domain `address`. Menjawab satu pertanyaan: *alamat itu milik siapa dan untuk apa?*

## BUSINESS TRUTH (Owner, sesi ini)

1. Alamat dimiliki **AKUN**, bukan peran. Satu buku alamat untuk seluruh akun.
2. Satu alamat boleh memegang **lebih dari satu peran** (mis. rumah = tujuan kirim DAN asal kirim).
3. Satu halaman di Settings, **tanpa tab**. Peran ditampilkan sebagai **tag pada kartu**.
4. **Satu `isPrimary` per akun** — bukan primary per tab/per purpose.
5. Checkout yang belum punya alamat shipping: **empty-state + CTA yang langsung membuka form**.
6. Pendaftaran seller memakai **form alamat yang sama** (dengan tag sender terkunci), bukan form kedua.
7. Promotion scope menyusul — `tags` disiapkan sebagai tempat menempel scope, **bukan scope itu sendiri**.

## CANONICAL AUTHORITY

| Concern | Authority |
|---|---|
| Buku alamat | `addresses` (Postgres), milik `user_id` |
| Peran sebuah alamat | `addresses.tags text[]` — subset non-kosong dari `{'shipping','sender'}` |
| Primary | `is_primary`, **satu per akun**, dijaga `idx_addresses_user_active_primary_unique` (migrasi 000017) |
| Soft delete / visibility checkout | `is_available_for_checkout` |
| Public origin line | `ResolvePublicOrigin` → `entity.BuildPublicOriginSummary` (derived, **bukan salinan**) |
| Form alamat di mobile | `AddressFormDialog` (satu-satunya) |
| Wire contract | `tags: string[]` / `tag_labels: string[]`; query `?tag=` |

## FORBIDDEN / KILLED (jangan dihidupkan lagi)

- Kolom/purpose tunggal `addresses.purpose` — satu alamat dipaksa satu peran.
- **Tab Shipping / Sender** di Settings, dan `TabController` penyertainya.
- **Primary per purpose** (klaim di komentar entity lama; DB tidak pernah mengizinkannya —
  `UnsetAllPrimary` selalu global). Membaca primary per tag tetap valid, tetapi hanya
  MENEMBUS ke satu primary akun, tidak pernah membuat primervelit kedua.
- **`AddEditAddressDialog`** — form kedua dengan aturannya sendiri (ada/tidak autofill,
  cek seller, validasi min-1 yang berbeda). Dihapus total beserta 3 file part-nya.
- **`AddressEmptyState` / `AddressEmptyStateWidget` / `address_list_screen/*`** —
  duplikat empty-state. Sekarang satu `EmptyState` + satu `ShippingAddressEmptyState`.
- **`models/user_address.dart`** dan **`helpers/address_migration_helper.dart`** —
  sisa era Firestore single-address, hanya dipakai satu sama lain.
- Migrasi label lama (`_migrateLabelToPurpose`): `farm`/`warehouse` → sender.
  Data production = 0, tidak ada alasan mempertahankan parser warisan.
- `?purpose=` pada endpoint address. Sekarang `?tag=`.

## IMPLEMENTATION

**Migration**
- `000119_address_tags_over_purpose.up.sql` — tambah `tags text[]` (di-seed dari `purpose`),
  `CHECK (cardinality(tags) > 0)`, `CHECK (tags <@ ARRAY['shipping','sender'])`,
  index `idx_addresses_tags`, **drop kolom `purpose`**. Tidak ada kolom kompatibilitas.
- `000119_...down.sql` — rollback eksplisit: `purpose = tags[1]` (kehilangan peran ganda memang
  konsekuensi rollback, bukan sesuatu yang dipertahankan).

**Backend**
- `entity.Address.Tags []AddressTag`, `NormalizeTags`, `HasTag`, `TagStrings`, `TagsFrom`,
  `InvalidTagsError` (menggantikan `InvalidPurposeError` / `AddressPurpose*`).
- Repository: kolom `tags`, filter `$2 = ANY(tags)`, `GetPrimaryByUserIDFiltered` →
  **`GetPrimaryByTag`**, `CountByUserID` = `COUNT(*)` (distinct) + per-tag via `unnest(tags)`.
- Service: input `Tags []string`, validasi lewat `NormalizeTags`.
- Handler: DTO `tags`/`tag_labels`, query `?tag=`, create **dan update** menerima `tags`
  (validasi ulang + tolak daftar kosong).
- Konsumen commerce dipindahkan: `for_sale_service`, `order_creation_service`,
  `auction_service`, `seller_onboarding_service` — semua kini `HasTag` / `GetPrimaryByTag`.

**Mobile**
- `AddressPurpose` → **`AddressTag`**; `AddressEntity.purpose` → **`List<AddressTag> tags`**,
  `hasTag()`, `tagValues`, `isAvailableForCheckout = hasTag(shipping)`.
- DTO `tags` / `tag_labels` (`.g.dart` diregenerasi lewat `build_runner`).
- Repository interface: `getAddressesByTag`, `watchAddressesByTag`,
  `getPrimaryAddress({AddressTag? tag})`, `countAddresses({AddressTag? tag})`.
- `AddressListScreen`: tanpa `TabController`/`TabBar`, satu `ListView`, kartu memuat
  **chip tag peran**, `canDelete` dihitung per akun (min 1, max 10).
- `AddressFormDialog`: pemilih tag multi (`ChoiceChip`) — **`lockedTags` DIBUANG**;
  yang tersisa hanya `presetTags` (pra-pilih peran saat create).
  - Pemilih tag hanya tampil saat akun punya **≥2 alamat**; di bawah itu form
    menampilkan "Applies to shipping & sender" (alamat tunggal = segalanya)
  - Checkout CTA → `presetTags = [shipping]`
  - Seller wizard → `presetTags = [sender]`
  - Simpan: count <2 → client mengirim `[shipping, sender]`, backend reconciler
    memaksa lagi di server (satu aturan, dua lapis)
- `ShippingAddressEmptyState.onAdd` → **langsung membuka form**, lalu reload daftar;
  tombol `Kelola` tetap untuk mengelola.

## CLEANUP (dihapus total)

- `backend`: `AddressPurpose`, `InvalidPurposeError`, `GetPrimaryByUserIDFiltered`,
  `purposeLabel`, kolom `purpose`.
- `apps/mobile/lib`:
  - `presentation/widgets/add_edit_address_dialog.dart` + folder `add_edit_address_dialog/`
    (`address_dialog_header`, `address_dialog_actions`, `address_form_fields`)
  - `presentation/widgets/address_empty_state_widget.dart`
  - `helpers/address_migration_helper.dart`, `models/user_address.dart`
  - export `add_edit_address_dialog.dart` di `profile_feature.dart`
- Fixtures & test kontrak di 9 file test mobile dan 12 file test Go ikut dikonvergenkan.

## POSITIVE PROOF

```
backend : go vet ./...                       → hanya 1 error PRA-ADA di luar scope (lihat bawah)
backend : go test ./internal/identity/address/...                    → ALL PASS
backend : go test ./internal/commerce/{forsale,order,shared,seller,auction}/... → ALL PASS
mobile  : dart analyze lib                    → 0 error, 0 warning (24 info pra-ada)
mobile  : dart analyze test                   → 0 error
mobile  : flutter test (profile + seller screens + 3 checkout + 2 auction)
                                              → +169 ALL PASS
```

Test perilaku baru:
- `TestAddress_CarriesShippingAndSenderTagsTogether` — satu baris dua tag, hitungan
  `Total=1` sementara dua bucket per-tag keduanya `1`.
- `TestToAddressResponse_DualTaggedAddress` — wire membawa dua tag + dua label.
- `TestGetPrimaryFiltered_UsesCanonicalPrimaryRegardlessOfTag` — primary tetap primervelit
  akun walau dibaca lewat tag yang tidak dimilikinya.
- `TestCreateAddress_EmptyTagList` — gin `required` loloskan slice kosong; handler yang menolak.
- `TestListAddresses_DualTagFilterRejectedAsUnknown` — tag di luar vocabulary → 400.

## NEGATIVE PROOF / RESIDUE SEARCH

```
grep -rn "purpose|Purpose"  backend/internal/identity/address  → hanya 3 baris komentar
                                                                    FORBIDDEN DESIGN (disengaja)
grep -rn '"purpose"'        backend/internal                   → 0
grep -rn "AddressPurpose|forcedPurpose|initialPurpose|
          getAddressesByPurpose|loadAddressesByPurpose|
          AddEditAddressDialog"  apps/mobile/lib                → 0 aktif
                                                       (1 baris komentar: "…is gone")
grep -rn "purpose"          apps/mobile/lib/domains/user/profile
                            + checkout                          → 0 (sisa: "display purposes")
```

## KNOWN LIMITATIONS

- Migrasi `000119` sudah berjalan di DB dev `labuda` (terbukti: kolom `tags` terbaca,
  repair 2 baris user dev → `{shipping,sender}` + primary sukses). Klik end-to-end
  (checkout tampil, daftar seller, alamat ganda-tag) masih owner retest.
- `countAddresses` mobile memakai `/addresses/count`; kontraknya tetap
  `total` (distinct) + `shipping_count` + `sender_count` — per-tag boleh menjumlah > total
  saat ada alamat ganda-tag. Ini benar, bukan bug.
- **Baca (hukum fallback, semua count):** query `?tag=` yang miss → kembalikan
  semua alamat aktif (`GetByUserIDFiltered` / `GetPrimaryByTag` di repo impl;
  primary → primary tanpa tag → alamat tertua → nil). Tag = preferensi,
  bukan gerbang — checkout/sender mustahil kosong selama akun punya alamat.
- **Tulis (reconciler `AddressService`):** 0 alamat = bebas · 1 alamat = dipaksa
  `{shipping, sender}` + `is_primary` · ≥2 = tag milik user, tepi satu primary
  (promosi otomatis kalau hilang) + race-retry create serentak (23505).

## UNRESOLVED OWNER DECISIONS

Tidak ada. Empat keputusan sudah dijawab Owner di sesi ini:
satu buku per akun · satu halaman tanpa tab · form seller pre-isi · empty-state + CTA.

## OUT OF SCOPE (tercatat, TIDAK dikerjakan)

1. **P1 — test suite payment gagal.**
   `subscription/application`: `TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot`
   expects `PLATFORM_REVENUE=+107000` / `BANK_SETTLEMENT=-107000`, actual terbalik.
   **Bukan** akibat perubahan address (diff pada file itu hanya 4 baris fixture alamat;
   tidak ada file ledger/payment yang berubah di working tree). Milik domain payment.
2. **P1 — `go vet ./...` gagal di satu paket.**
   `discovery/search/delivery/http/search_commerce_seller_projection_test.go:25`
   `[]string` vs `[]ProductMedia` — akibat perubahan tak ter-commit sesi lain pada
   `search_projection_adapter.go`. Milik domain search/discovery.
3. **P2 — promotion scope** (`tags` sudah siap menerima scope, pengerjaan terpisah).
4. **P2 — naming `is_available_for_checkout`** sebenarnya flag soft-delete yang juga
   berlaku untuk alamat sender. Sudah ditangani dengan memisahkan jalur baca
   (`GetByUserIDForDisplay`), penggantian nama ditunda.

## NEXT SCOPE

1. Jalankan migrasi `000119` + runtime proof nyata (bikin alamat 2 tag → checkout → seller).
2. Promotion scope di atas `tags`.
3. Domain payment (temuan #1 di atas) — buka dengan trigger sah, bukan karena kebetulan ketemu.
