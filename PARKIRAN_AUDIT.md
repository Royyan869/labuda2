# Parkiran Audit — September 2026

Hasil audit factual (bukan warisan audit lama). Prinsip yang dipakai:
**codebase factual = authority; test mengikuti codebase, bukan sebaliknya.**
Dilarang rollback/restore dari GitHub — semua perbaikan maju.

## Status terverifikasi

### Production compile
- Backend `go build ./...` — nol error.
- Mobile `lib/` — nol error setelah 2 perbaikan berikut.
- `content_notifier.dart:93` — konsumen tidak diikutkan saat interface
  `getContentsByAuthor` dimigrasi ke cursor pagination (C3B). Fixed:
  hapus param `offset` dari `fetchByAuthor`, arahkan ke
  `getContentsByAuthorPaged`. `content/` analyzer: no issues.
- `media_upload_component.dart:148` — pass `double` ke kontrak `int`
  `MediaUploadConfig.maxImageSizeMb/maxVideoSizeMb`. Fixed: `.round()`
  di call site (kontrak int taxang dipakai lintas app).

### Backend factual
- `go build ./...` nol error; `go vet` (commerce + discovery) nol error.
- Test commerce inti (auction, forsale, order/application, shipping) hijau.

### Mobile commerce suite
- `test/domains/commerce/` — 522 lulus, 4 gagal.
- 4 gagal = keluarga pre-existing terverifikasi baseline
  (`auction_detail_screen_runtime_test` x3, `auction_detail_header_media_test`)
  — masuk parkiran di bawah.
- 8 kegagalan lain dari ronde sebelumnya diperbaiki maju (detail di commit).

## Fixed maju (sesuai doctrine, test mengikuti codebase)

- `commerce_detail_media_network_contract_test` — `logicalCacheKey` →
  `reloadToken` (parameter factual StableNetworkImage).
- `payment_result_screen_widget_test` — fixture time-bomb
  (`expiredAt: 2026-08-02` hardcoded, sudah lewat) → relatif ke now;
  harness diberi GoRouter (factual: "Lanjutkan Pembayaran" push ke
  payment-webview route); tambah test negatif URL expired.
- `shipping_option_setup_screen_test` — tap `'Pengiriman'` → `'Shipping'`
  (label tile factual settings).
- `c1b2_new_chat_runtime_proof_test` — `HybridAvatar.initials` dibunuh
  (keputusan owner 2026-09-24: avatar = foto atau person icon, tanpa
  initials fallback); test align ke kontrak baru.
- `main_drawer_b1_avatar_handle_test` — idem, + assert person icon
  muncul untuk user tanpa foto.
- `chat_input_area_sendability_test` — `onSendMessage` kini
  `Future<void>`; `canSendMedia` tidak ada di widget; media-only empty
  body path sudah mati; whitespace draft = send icon tampil tapi guard
  trim mencegah send.
- `ws_envelope_contract_test` — room-level `context` dihapus dari DTO
  (chat-commerce boundary: hanya `linked_order_id` safe reference).

## Parkiran (perlu dikerjakan, belum dieksekusi)

### A. Di luar commerce — butuh audit factual dulu (bukan warisan audit)
1. ~~`c1b3_mention_rich_text_runtime_test`~~ ✅ SELESAI (1d2b134):
   resolver factual = {apiClient, logger}, error dienkapsulasi
   (catch → log → null); PaginationIntegrityException purged — test
   error-path lama digabung jadi kontrak baru. 5/5 lulus.
2. ~~`follow/*` 4 file~~ — ✅ SELESAI (124e7ca + 04c40c6): race-safety
   guards (ref.mounted, sequence per-target, principal guard, watch auth)
   di FollowStatusNotifier; 4 test file align ke kontrak factual; overflow
   akhirnya beres — akar: `_sanitizeProfileLoadError` passthrough raw
   `error.toString()` (300rb+ char stack dump) ke Text non-scrollable;
   kini full detail → logger, UI = first line capped 200 char. Suite
   follow **67/67**.
3. ~~Audit layout ProfileScreen (overflow) + duplikasi header~~ — ✅ SELESAI
   (04c40c6 + commit purge):
   - Tidak ada duplikat class ProfileScreen; tapi ada **duplicate header
     builder** (`profile_header_builder.dart`) tanpa konsumen produksi →
     PURGED bersama 3 file saudaranya (`profile_appbar_actions.dart`,
     `profile_sliver_delegates.dart` — nol konsumen).
   - 5 test kontrak identitas di-port ke screen kanonik
     (`profile_header_identity_canonical_test.dart`): @username-only,
     seller farmName, redaction degraded/deleted — 5/5.
   - **SellerTierBadge di-wire ulang ke header kanonik** (fitur hilang:
     data `sellerTier` disiapkan tapi tak pernah dirender; dokumen badge
     menyebut profile header sebagai surface).
   - **`profile_share_builder.dart` di-wire** ke `_handleShareProfile`
     (refactor setengah jalan agen sebelumnya: helper teruji 6 test tapi
     tak pernah dipakai; screen memakai logika duplikat tanpa lifecycle
     guard). Sekarang satu kebenaran.
   - `blocked_users_screen_test` — 2 test mengejar API
     `CircleAvatar(backgroundImage:)` lama → align ke kontrak ProfileAvatar
     kanonik (StableNetworkImage + person icon fallback).
   Suite profile **97/97**.
3. ~~`seller_wizard_helpers_test` — param `bio`~~ ✅ SELESAI (ea8aca6,
   sesuai keputusan owner purge bio seller).
4. Break lib lain: 7 warning `lib/` (unused import/element/field,
   hidden name RefundRequest/RefundStatus) — kecil, sekali jalan.
5. Test debt pre-existing commerce (terverifikasi baseline):
   `auction_detail_screen_runtime_test` x3, `auction_detail_header_media_test`.
6. Test debt non-commerce dari daftar lama (perlu re-baseline dulu):
   promoted feed HTTP-mock, `/settings` toggle, `saved_item` backend contract.

### B. Keputusan owner — SUDAH DIEKSEKUSI
- ✅ Drop kolom DB `for_sale_type` — commit b2dee89 (migrasi 000112,
  IF EXISTS aman di DB fresh & lama; README breadcrumb 000113).
- ✅ Purge bio seller — commit ea8aca6. Factual: bio hanya ada di
  `user_profiles` (user authority); domain seller backend & mobile sudah
  bersih; satu-satunya sisa (fixture wizard test) di-align.

### C. Keputusan owner — MENUNGGU
- Audit wire for_sale dengan lensa Scope 3 (raw status sold/withdrawn
  ke non-owner?).

## Komit acuan
e1fb4ae, c9ab974, a00b230, 94c1ce8, 7b0b1c8, 64e9f24, 23d36d9, 30874c6,
1b95590, b2dee89, ea8aca6, 124e7ca, 1d2b134.
