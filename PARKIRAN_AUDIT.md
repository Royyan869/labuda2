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

### C. Keputusan owner — SUDAH DIEKSEKUSI
- ✅ Audit wire for_sale lensa Scope 3 (commit scope-3-forsale):
  **KEBOCORAN TERKONFIRMASI & DITUTUP.** Temuan audit:
  1. Detail for_sale mengirim raw `status` (draft/sold/withdrawn) +
     `sold_at`/`withdrawn_at` ke siapa pun — kini coarsened
     (`PublicLifecycle()` → {active, unavailable}) + `seller_status`
     owner-only (persis pola auction). Write surfaces (create/update)
     membawa `seller_status` untuk owner.
  2. Chat projection for_sale kirim raw `row.status` ke semua peserta
     chatroom (chat auction sudah coarsen) → kini coarsened, paritas
     chat chat-auction/chat-for-sale tertutup.
  3. Mobile DTO for_sale tanpa slot `seller_status` → ditambah + mapper
     precedence `sellerStatus ?? status` (persis auction mapper);
     `_mapStatus` mengenal vocab `unavailable` (→ draft, konservatif).
  4. Aman tanpa perubahan: guard draft (derived private → 404),
     SQL list/search (`status='active'`), owner-inventory branch.
  Test: backend boundary 3 test baru (vocab/owner-only/timestamps) +
  mobile boundary 6 test baru (precedence/vocab/fallback); for_sale
  mobile 59/59; backend forsale + serverboot chat tests hijau.
  Catatan: panic prometheus di full-suite serverboot = pre-existing
  isolation issue jalur InitServices (domain payout, kerja lain);
  test terkait lulus di isolasi.

## Scope #3 — Konvergensi Envelope (Laporan §11)

### 1. Verdict
SELESAI untuk backend: tahap 1 (canonical envelope shared) + tahap 2 (content
resolver) + tahap 3 (chat resolver) beres, compile & suite hijau. Sisa: tahap 4
(mobile parser converge) — BELUM dikerjakan.

### 2. Root cause
Satu resource diproyeksikan oleh DUA envelope private (chat vs content) —
dual authority. Payload tidak sepakat: price scalar vs objek {amount,currency},
image_url singular vs media[], flat ForSaleLiveSeller vs publiccard.SellerCard,
payload-level can_interact vs envelope-level capabilities, dan tombstone chat
meng-omit resource_id (padahal contract content menyimpannya).

### 3. Canonical behavior setelah perbaikan
- SATU tipe: `commerceshared.ResourceProjection` (`resource_projection_envelope.go`).
- Wire LIVE: `{state, resource_type, resource_id, canonical_url,
  viewer_capabilities, commerce_actions?, <payload>}`.
- Wire TOMBSTONE: `{state, resource_type, resource_id, viewer_capabilities}` —
  resource_id SELALU ada (omission rule chat MATI); canonical_url LIVE-only.
- Capabilities hanya di envelope; payload-level can_interact MATI.
- price = `LivePrice{amount,currency}` (konstanta `LivePriceCurrencyIDR`).
- seller = selalu `publiccard.SellerCard`; media = `[]mediaref.MediaRef` +
  `thumbnail_url`; mapping authority `ForSaleCommerceActions`/`AuctionCommerceActions`.
- Enum `ProjectionResourceType` {profile, content, for_sale, auction} — wire
  identik dengan dua enum lama; chat alias (`chat_resource_projection_contract.go`)
  tanpa alias constructor/payload tak sepakat → compiler memaksa konstruksi kanonik.

### 4. File yang berubah
- BARU: `commerce/shared/resource_projection_envelope.go` + `_test.go` (ratchet
  wire + payload contract), `interaction/chat/application/chat_resource_projection_contract.go`.
- DIHAPUS: `social/content/application/content_resource_projection.go`,
  `interaction/chat/application/chat_resource_projection.go`, dan ratchet lama
  `chat_resource_projection_test.go` (duplikat; otoritas ratchet pindah ke shared).
- Prod content: `content_resource_projection_resolver.go` + consumers
  (comment_response, comment_handler, content_handler, feed_handler,
  feed_share_projection, search_handler, search_projection_adapter).
- Prod chat: `chat_handler.go`, serverboot `chat_projection_resolver.go`,
  `chat_forsale/auction/content_projection_resolver.go`,
  `chat_resource_projection_aggregate_resolver.go`, `dependencies.go` (P2).
- Tests dimigrasi: aggregate unit + 7 file integration serverboot
  (profile/fps/f35b/auction/content/http/unified) + `chat_room_list_response_test.go`.

### 5. Desain/residue yang dihapus
Flat `ForSaleLiveSeller`; `image_url` singular; price scalar; payload-level
`can_interact`; omit resource_id di tombstone; constructor lama
`NewLiveProjection`/`NewTombstoneProjection` + `ResourceProjectionIdentity`/
`ResourceProjectionPayload` + private marshal chat; pointer `Thumbnail`/
`Lifecycle`/`Seller` di auction payload; duplikat ratchet envelope chat app.

### 6. Tests dan build yang dijalankan (command → hasil)
- `go build ./internal/...` → exit 0; `go vet ./internal/...` → bersih.
- `go test -run '^$'` unit & `-tags integration` → compile bersih (2x).
- Unit: `go test ./internal/commerce/shared/ ./internal/interaction/chat/...`
  → all ok; `./internal/serverboot/` unit → ok (satu FAIL = payment pre-existing).
- Integration serverboot (sekali per batch, tanpa run ulang yang hijau):
  - profile resolver: ok (285s, 4 test)
  - fps resolver: ok (322s + 271s, 8 test; termasuk JSONContracts wire)
  - content resolver paruh-1: ok (275s, 4 test)
  - content paruh-2 + auction LivePayloadContract + aggregate: ok (464s, 7 test)
  - auction batch: 4/5 ok; LivePayloadContract merah = pre-existing (lihat §8)
  - HTTP matrix + fallback + unified: 10/11 ok (532s); 1 FAIL = pre-existing (§8)
- Content package: `TestCommentListQueryCount_(TwentyAuctions|OneFPSOneAuction|
  TwentyFPSTwentyAuctions|Invariants)` → ok (570s) — **tahap 2 CLOSED**.

### 7. Hasil proof
- Ratchet shared: wire shape live fps, tombstone (resource_id ADA), profile
  forbids commerce_actions, 6 validation-rejection, marshal-validates,
  payload contract persis (profile/content/fps/auction) — semua PASS.
- f35b JSONContracts hijau: top-level live 7 key, media[]+thumbnail_url,
  SellerCard wire {user,farm_name,avatar_url,lifecycle}, tombstone 4 key
  + resource_id == source id.
- HTTP matrix hijau = wire kanonik benar lewat jalur HTTP nyata.
- Query counts content tetap (paruh-1 + paruh-2 hijau) → envelope tak menambah query.

### 8. Temuan di luar scope
- `TestPaymentReuseGuard_HandlesLookupErrors` — merah, pre-existing; file payment
  diubah agent lain (working tree).
- `TestUnifiedShareDepth1_DepthMatrixD1ToD30Evidence` — merah sejak commit
  checkpoint 4c3ab55: subtest D20 menanam share ke content id yang tidak ada,
  kena FK non-deferrable `content_resource_occurrences_content_source_id_fkey`;
  mustahil hijau sejak lahir; bukan regresi scope #3 (diff kita di file itu
  hanya import + literal D30). Perlu keputusan owner (tes vs FK).
- `TestAuctionProjectionResolver_LivePayloadContract` — merah di HEAD juga
  (seed `title+" product"` vs wantTitle lama); ekspektasi disesuaikan ke
  product title = canonical (paritas FPS).

### 9. Risiko / belum terbukti
- **Mobile (tahap 4) BELUM**: parser `chat_resource_projection.dart` masih
  strict-lawas (LIVE wajib image_url?, tombstone REJECT resource_id) → akan
  REJECT wire baru; parser content juga perlu converge. Belum ada smoke mobile.
- Owner retest ditunda (device belum ready); rendering harga di discovery
  (keputusan owner) belum diverifikasi di UI.

### 10. Git status
283 file berubah/untracked di working tree (multi-agent); Scope #3 menyentuh:
2 file D (envelope chat/content lama + ratchet chat), file baru shared envelope
+ chat contract (untracked), ~10 prod M (resolver/handler), ~10 test M.
Belum di-commit.

### 11. Owner retest diperlukan?
BELUM untuk tahap ini (perubahan wire-behind, mobile belum converge). Retest
setelah tahap 4 mobile: smoke buka share for_sale/auction di percakapan
(tombstone + card), lalu lensa discovery (harga tampil di discovery, tidak di
percakapan).

## Scope #3 tahap 4 — Konvergensi Mobile + audit boundary (laporan)

### 1. Verdict
SELESAI untuk boundary mobile. Dual authority di klien DIMATIKAN: dua tipe
projection Dart (chat + content) yang saling menentang dan menentang backend
diganti SATU tipe kanonik `lib/shared/domain/entities/resource_projection.dart`;
kedua file lama DIHAPUS. Aturan tombstone yang melawan authority backend
dibalik, bukan disinkronkan. Belum di-commit.

### 2. Root cause (temuan audit, read-only dulu)
Satu wire kanonik diparse oleh DUA parser yang berbeda pendapat — dan keduanya
salah di titik berbeda:
- **Chat** menolak `resource_id` pada TOMBSTONE (`_parseTombstone` throw) +
  `resourceId => null`; backend justru mem-pin `{state, resource_type,
  resource_id, viewer_capabilities}` ("identity survives death"). Efek: tiap
  tombstone kanonik → FormatException → `message_dto` menelan → kartu
  attachment HILANG SENYAP (justru kasus seller suspended/draft).
- **Content** belum converge: payload for_sale/auction masih mewajibkan
  `can_interact` + harga scalar, envelope tanpa capabilities → LIVE
  for_sale/auction SELALU FormatException. `comment_dto` menelan (kartu
  hilang); content/feed/search melempar keluar.
- **Media chat**: parser baca `image_url` (mati); backend kirim media[] +
  thumbnail_url → gambar listing hilang tanpa error.
- **False green**: 5 file test menaruh `can_interact` DI DALAM payload +
  fixture harga scalar; nol test memakai wire kanonik → suite hijau = parser
  cocok dengan fixture sendiri, bukan dengan authority backend.
- Klaim "panic prometheus pre-existing (P2)" di catatan lama sudah STALE:
  working tree kini punya `registerMetrics` toleran AlreadyRegisteredError
  (`serverboot/dependencies.go`, 2 call site, nol `MustRegister` tersisa).

### 3. Canonical behavior setelah perbaikan
- SATU tipe mobile: `ResourceProjection` (sealed: `LiveResourceProjection` /
  `TombstoneResourceProjection`) + payload sealed (profile/content/for_sale/
  auction) — mirror `commerceshared.ResourceProjection`.
- `resource_id` WAJIB dan diekspos di KEDUA state; `canonical_url` LIVE-only;
  `commerce_actions` hanya LIVE for_sale/auction (validasi actionable
  per-tipe persis Go: fps butuh can_buy|can_negotiate, auction can_bid|can_buy).
- `price` = `LivePrice{amount,currency}`; payload for_sale = `media[]` +
  `thumbnail_url` + getter `primaryImageUrl` (thumbnail → media.first).
- Kebijakan display keluar dari entity: kartu hanya merender; entity hanya
  data. (Kebijakan harga awalnya "chat tanpa harga", lalu DIREVISI owner pada
  task #4 — lihat section Task #4 di bawah.)
- Gagal-parse SERAGAM di semua permukaan: envelope malformed di-drop di edge
  (chat/comment/content/feed/search), dan wire dikunci secara lantang oleh
  ratchet.
- **Identity ≠ liveness**: handler buy chat sekarang memeriksa `!isLive`
  eksplisit (dulu bergantung pada `resourceId == null`) — tombstone tidak lagi
  bisa menembus ke checkout.

### 4. File yang berubah
- BARU: `lib/shared/domain/entities/resource_projection.dart`.
- DIHAPUS: `lib/domains/chat/.../chat_resource_projection.dart`,
  `lib/domains/social/content/domain/entities/content_resource_projection.dart`.
- Prod M: chat (`message_dto`, `chat_entities`, `chat_detail_screen`,
  `chat_resource_projection_card`), content (`comment_dto`, `comment.dart`,
  `comment_card`, `content_dto`, `content.dart`,
  `content_resource_projection_card`), features (`feed_dto`, `feed_renderers`,
  `search_dto`, `search_mapper`).
- Test M: 13 file (6 chat, 5 social/content, feed, search).
- Nama widget per-permukaan TETAP (`ChatResourceProjectionCard`,
  `ContentResourceProjectionCard`) — presentasi yang berbeda, tipe yang sama.

### 5. Ratchet mobile (pengganti dua pin yang saling menentang)
- Pin Go dan pin Dart kini membaca wire yang SAMA.
- `content_resource_projection_contract_test.dart` = ratchet baru: wire kanonik
  4 tipe × 2 state; negative (LIVE tanpa viewer_capabilities, payload tidak
  cocok, tombstone membawa payload/canonical_url/commerce_actions, tombstone
  tanpa id, payload legacy `can_interact` + harga scalar DITOLAK); plus source
  ratchet: dua file parser lama HARUS tidak ada, entity tanpa
  FALLBACK_ALLOWED/legacy conversion, dan BUKTI STRUKTURAL: tidak ada
  `can_interact`/`image_url` di dalam payload pada `toJson()`.
- Matrix chat: `TOMBSTONE containing resource_id is rejected` DIBALIK menjadi
  `TOMBSTONE missing resource_id is rejected` +
  `TOMBSTONE round trips canonically and keeps identity`.

### 6. Tests dan build yang dijalankan (command → hasil)
- `flutter analyze lib` → bersih (2 error tersisa di
  `profile_about_tab.dart` = agen UI paralel, di luar scope).
- `flutter analyze test` → 0 error.
- `flutter test test/domains/chat` → 383 PASS.
- 13 file test migrasi (satu batch) → 77 PASS.
- `flutter test test/domains/social/content` → 45 PASS;
  `test/domains/social/comment` → 39 PASS.
- `test/features/home` (17 file) & `test/features/search` (13 file) GAGAL LOAD:
  compile error di `lib/domains/user/profile/presentation/widgets/
  add_edit_address_dialog/*` (agen UI, in-flight) — bukan regresi scope ini.
  File yang menyentuh projection lulus sendiri-sendiri: feed render 4/4,
  search parity 5/5.
- Harness `feed_non_content_share_render_test` diperbaiki agar bisa jalan
  (override `authControllerProvider` = guest state; harness HEAD memang tidak
  pernah bisa load karena FeedCard kini membaca auth).

### 7. Risiko / belum
- Task #4 (contract harga: discovery tampil, chat tidak, LIVE saja,
  TOMBSTONE tanpa payload) belum ditulis sebagai test khusus — prasyaratnya
  kini terpenuhi.
- Task #5 (satu shell + bunuh N+1 `objectPreviewProvider`) SELESAI —
  lihat laporan Task #5 di bawah.
- Owner retest: smoke chat share for_sale/auction (LIVE + TOMBSTONE), comment
  attachment, feed/search/discovery (harga tampil di discovery).
- Working tree masih multi-agen; commit belum.

### 8. Catatan anti-resurrection
Yang dihapus bukan "parser kedua yang salah", tapi hak mobile untuk menetapkan
kebijakan: LIVE/TOMBSTONE, capabilities, harga, media semuanya datang dari
resolver kanonik. Arah sebaliknya (menyamakan dua parser) akan memelihara dual
authority dengan biaya sinkronisasi abadi.

## Task #4 — Kebijakan harga lintas permukaan (laporan)

### 1. Keputusan owner (2026-09-27) — REVISI
Dari tiga opsi batas "percakapan", owner memilih **semua permukaan menampilkan
harga**. Kebijakan lama "chat = display layer tanpa harga" DIBATALKAN.
Konsekuensi: kartu chat kembali merender uang dari envelope, persis string yang
sama dengan discovery; status availability tetap tampil sebagai caption
(sehingga merender harga tidak menghilangkan kesaksian status).

### 2. Yang berubah
- `resource_projection.dart` kini memegang SATU formatter uang:
  `formatGroupedAmount` (grup ribuan dengan '.', gaya yang sudah dipakai
  commerce: `Rp 1.250.000`) + `LivePrice.formatted`,
  `ForSaleLivePayload.formattedPrice`, `AuctionLivePayload.formattedAmount`
  (current bid → fallback buy-now; null bila keduanya kosong — 0 tidak pernah
direkayasa).
- Kartu chat (`chat_resource_projection_card.dart`): value = uang kanonik
  (for_sale → `formattedPrice`, auction → `formattedAmount`), caption =
  availability/lifecycle (`Tersedia`/`Terjual`/`Berlangsung`/…). CTA dan badge
  tidak berubah.
- Kartu discovery (`content_resource_projection_card.dart`): formatter lokal
  dihapus, memakai getter envelope yang sama — nol duplikasi format.
- Baris hasil search (`search_result_extra_info.dart`) memakai
  `formatGroupedAmount` — sebelumnya `'Rp ${price.toInt()}'` tanpa grup, jadi
  dua permukaan discovery bisa memformat angka yang sama secara berbeda.

### 3. Ratchet baru
`test/shared/domain/resource_projection_price_policy_test.dart` (11 test):
- chat & discovery merender string uang yang IDENTIK untuk envelope yang sama
  (for_sale `Rp 1.250.000`; auction current bid menang atas buy-now; non-IDR
  membawa kode mata uangnya).
- auction tanpa bid & tanpa buy-now → tidak ada uang sama sekali di dua kartu.
- profile/content → nol uang di dua kartu.
- TOMBSTONE: wire persis 4 key kanonik (tanpa price/canonical_url/
  commerce_actions/payload), dua kartu merender nol uang, chat menampilkan
  `Tidak dapat ditampilkan`, dan tombstone yang membawa payload harga DITOLAK.
- "one formatting authority": source-scan dua kartu — tidak boleh ada literal
  `'Rp ` di kartu; entity wajib punya formatter + getter money.
- Test lama yang mem-pin kebijakan anti-harga dibalik: chat CTA contract
  (kini menuntut `Rp 1.250.000` tampil + caption status) dan consumer matrix.

### 4. Tests dan build yang dijalankan
- `flutter analyze` (entity, dua kartu, search row/dto/mapper) → No issues.
- 14 file keluarga projection (13 migrasi + policy baru) → 88 PASS.
- `test/domains/chat` → 383 PASS; `test/domains/social/content` +
  `test/domains/social/comment` → 84 PASS.
- `test/features/home` & `test/features/search` sebagai direktori masih GAGAL
  LOAD karena compile error agen UI paralel di
  `lib/domains/user/profile/presentation/...` (di luar scope); file search
  parity & feed render yang menyentuh projection lulus sendiri-sendiri.

### 5. Residu
- `share_preview_card.dart` masih punya formatter grup ribuan sendiri —
  SUDAH disatukan ke `formatGroupedAmount` (lihat laporan Task #5b).
- Task #5 (satu shell + bunuh N+1 `objectPreviewProvider`) selesai —
  lihat laporan Task #5.
- Owner retest: chat share for_sale/auction (harga + status, LIVE dan
  TOMBSTONE), comment attachment, feed/search/discovery.

## Task #5 — Satu shell snapshot-only + purge resolver N+1 (laporan)

### 1. Keputusan
`ObjectPreviewCard` turun pangkat menjadi **display shell snapshot-only**:
tanpa provider, tanpa state turunan, tanpa uang, tanpa badge status; tap selalu
aktif karena navigasi hanya memakai identitas dari snapshot transport
(`targetType` + `targetId`). Alasan: kartu ini hanya muncul pada baris yang
TIDAK punya projection kanonik (baris pra-authority). Di baris itu klien memang
tidak memegang authority — jadi kontrak paling jujur adalah "tampilkan apa yang
dikirim transport". Resolver N+1 milik klien (fetch per-kartu + LIVE/TOMBSTONE
versi klien + uang hasil hitungan sendiri) adalah cara klien membangun
kebenaran kedua — persis yang sedang diberantas kampanye ini.

### 2. Yang berubah
- `object_preview_card.dart` ditulis ulang: badge tipe, judul snapshot,
  thumbnail (content → `StableNetworkImage`; commerce/profile →
  `Image.network`), chevron. Nol uang, nol status, nol provider.
- `message_bubble.dart`: import `object_preview*` dihapus, parameter/field
  `preResolved` dihapus. Fallback tetap ada tapi kini display-only: baris dengan
  projection → kartu kanonik; tanpa projection → shell snapshot.
- `chat_detail_screen.dart`: `_MessagesBatchWidget` → `_MessageListWidget`;
  STEP 1 (kumpulkan referensi), STEP 2 (watch batch provider), STEP 3 (lookup
  `preResolved`) dihapus — chat tidak lagi melakukan lookup per-kartu.
- `attachment_widget.dart`: cabang `objectReference` dihapus total (import
  kartu, field, parameter, pemanggilan). Kini murni renderer workflow payload
  (negotiation/shipping/location) — satu resource, satu jalur render.
- `object_preview.dart`: satu-satunya file yang tersisa di `lib/shared/object/`;
  dokumen menyatakan "transport snapshot"; factory `fromForSale`/`fromAuction`
  dihapus (klien tidak lagi menyusun snapshot turunan). Field `price`/`status`
  dipertahankan apa adanya dengan komentar eksplisit "tanpa authority
  uang/status".
- Komentar sisa dibersihkan: `discussion_screen.dart` (STEP 1 comment) dan
  `chat_mapper.dart` ("Handle ObjectReference" → "Handle a shared object
  reference").

### 3. Yang dihapus (jalur resolver N+1)
- `lib/shared/object/object_preview_provider.dart`
- `lib/shared/object/object_preview_batch_provider.dart`
- `lib/shared/object/object_reference_bridge.dart`
- `lib/shared/object/object_reference.dart`
- `test/shared/object/object_preview_card_stale_target_test.dart` (mem-pin
  perilaku resolver yang sudah tidak ada; diganti dua ratchet baru)

### 4. Ratchet baru
- `test/shared/object/reference_attachment_snapshot_shell_test.dart` (3 test):
  render identitas lengkap (badge/judul/thumbnail/chevron); snapshot "gone"
  TIDAK memblokir navigasi (tap tetap aktif); NOL uang (`'Rp'`) dan nol klaim
  status (`Terjual`/`Tersedia`/`Ditarik`/`Dihapus`).
- `test/shared/object/reference_attachment_live_fetch_purge_test.dart` (3 test):
  empat file resolver HARUS tidak ada; source-scan empat surface
  (`chat_detail_screen.dart`, `message_bubble.dart`, `object_preview_card.dart`,
  `attachment_widget.dart`) terlarang memuat `objectPreviewProvider`/
  `objectPreviewBatchProvider`/`ObjectReference`/`preResolved`/`batchPreviews`;
  `object_preview_card.dart` bebas `'Rp '` dan `toStringAsFixed`.
- Migrasi 4 file test yang dulu menyetel override provider:
  `content_shared_reference_media_render_contract_test.dart`,
  `comment_commerce_preview_contract_test.dart`,
  `feed_non_content_share_render_test.dart`,
  `content_card_media_network_contract_test.dart`.

### 5. Tests dan build yang dijalankan (command → hasil)
- `flutter analyze lib` → 61 issues, **0 error** (sisa info/lint agen paralel).
- `flutter analyze test` → 125 issues, **0 error**.
- `flutter test test/domains/chat` → **383 PASS** (baseline pra-Task #5 utuh).
- `flutter test test/domains/chat test/domains/social/comment
  test/domains/social/content` → 465 PASS (sebelum edit terakhir
  `attachment_widget.dart`; baris di atas adalah ulangan SETELAH edit).
- `flutter test test/shared/object
  test/shared/domain/resource_projection_price_policy_test.dart` → **21 PASS**
  (10 ratchet object + 11 ratchet harga).
- `flutter test test/features/home/feed_non_content_share_render_test.dart
  test/features/home/content_card_media_network_contract_test.dart` → 8 PASS.
- `flutter test test/shared` → 246 PASS / 23 FAIL. Semua 23 gagal karena
  harness di luar scope: `seller_tier_stage2_test` = `Bad state: No
  ProviderScope found` (lib `shared/governance/seller_tier_badge.dart` sedang
  diedit agen lain — terlihat `M` di `git status`), `c1b3_mention_*` +
  `popup_more_options_button_test` = `UnimplementedError: ApiClient must be
  provided externally`. Nol kegagalan menyentuh ObjectPreviewCard/projection.
- `flutter test test/features/home` → 122 PASS / 20 FAIL, penyebab sama
  (`ApiClient`/harness feed), bukan regresi purge.

### 6. Residu
- `share_preview_card.dart` + `link_picker_modal.dart` (`'Rp${price}'`) masih
  memformat uang sendiri — SUDAH disatukan ke `formatGroupedAmount`
  (lihat laporan Task #5b).
- `attachment_widget.dart` masih punya `_formatCurrency` sendiri (dipakai
  negotiation/shipping payload, bukan projection) — kini delegasi ke
  `formatGroupedAmount`, jadi nol regex lokal.
- `for_sale_repository.getForSalesByIds` kini tanpa pemanggil di lib (hanya
  deklarasi + implementasi + satu fake di test) — kandidat purge mati.
- `live_status_provider.dart` masih menurunkan status dari
  `shareReference.preview` (`*LiveStatus.fromSnapshot`): jalur ini bukan
  projection dan belum dipurge; perlu keputusan owner apakah badge status
  komunikasi ikut pindah ke projection atau tetap snapshot.
- Owner retest: smoke chat share for_sale/auction (LIVE + TOMBSTONE),
  comment attachment, feed/search/discovery.

## Task #5b — Satu implementasi pemisah ribuan (laporan)

### 1. Temuan
Ratchet Task #4 hanya menjaga dua kartu projection. Sapuan penuh ke `lib/`
menemukan **8 salinan** pemisah ribuan yang sama
(`replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'))`), semuanya lolos
dari grep satu-baris karena pemanggilannya dipecah beberapa baris:
`attachment_widget`, `share_preview_card`, `checkout_action_bar` (DUA
formatter dalam satu file), `for_sale.formattedPrice`,
`shipping_quote_creation_modal`, `coin_balance_card`; plus `link_picker_modal`
yang bahkan tidak menggrup sama sekali (`'Rp${price}'` → `Rp150000`).

### 2. Yang berubah
Semuanya kini memanggil `formatGroupedAmount` (authority di entity envelope),
yang menghasilkan string identik untuk semua pemakaian `'.'` — nol perubahan
visual kecuali `link_picker_modal` yang naik ke `Rp 150.000`.

### 3. Ratchet baru
`resource_projection_price_policy_test.dart` → test
`thousand separators are implemented once, not per widget`: memindai SELURUH
`lib/` (rekursif), melarang regex pemisah ribuan di mana pun, dengan SATU
pengecualian eksplisit: `price_input_component.dart` (input mask yang menggrup
dengan `','` saat user mengetik lalu mem-parse `','` itu kembali — bukan display
money). Ratchet lama "cards never format money themselves" tetap ada.

### 4. Verifikasi
- `flutter analyze lib test/shared/domain/resource_projection_price_policy_test.dart`
  → 61 issues, **0 error** (jumlah tidak naik dari sebelum sapuan).
- `flutter test test/shared/domain/resource_projection_price_policy_test.dart`
  → **12 PASS** (11 lama + ratchet baru; output-nya menyebut test baru).
- `flutter test test/domains/commerce/transaction/checkout
  test/domains/social/share test/domains/commerce/catalog/for_sale` →
  **120 PASS, 1 skip**.
- `flutter test test/domains/chat test/domains/finance` → 477 PASS / 3 FAIL:
  `payment_result_notifier_test` (authority `order.status` vs payment resource).
  Di luar scope — `payment_remote_datasource.dart` +
  `payment_method_picker_sheet.dart` sedang diedit agen lain (`M` di git
  status) dan test itu tidak menyentuh satu pun file sapuan ini.

### 5. Residu (belum satu authority)
- `AppFormatters.formatCurrency` → `CurrencyUtils` (intl `NumberFormat`,
  locale `id_ID`) adalah authority kedua yang hidup untuk layar seller/wallet.
  Output identik, tapi dua mesin: satu loop, satu ICU.
- Sekitar 30 tempat masih `'Rp ${x.toStringAsFixed(0)}'` TANPA grup sama sekali
  (bid auction, detail for_sale, dialog withdraw, saved item, dst) — SUDAH
  dikonvergensikan (lihat laporan Task #5c).
- `discount_card.dart` memakai `NumberFormat('#,###','id_ID')` sendiri — SUDAH
  ikut dikonvergensikan (Task #5c).

## Task #5c — Tampilan harga ikut grup + bug "Rp Rp" (laporan)

### 1. Temuan
- **Bug nyata:** 7 tempat merender `"Rp Rp 1.000.000"` karena menginterpolasi
  `AppFormatters.formatCurrency(...)` — yang SUDAH membawa simbol `Rp ` —
  setelah literal `'Rp '`: `seller_renewal_screen` (3), 
  `seller_upgrade_wizard_screen` (3), `seller_wizard_preview_widget` (1).
  (Agen lain menemukan satu lagi jenis yang sama di
  `payment_method_picker_sheet` dan menandainya "never prefix it a second
  time" — jadi bug ini kelas, bukan kasus tunggal.)
- **±30 tampilan uang tanpa pemisah ribuan** (satu angka tampil `Rp 1250000`
  di satu layar, `Rp 1.250.000` di layar lain): seluruh keluarga auction
  (card, bid section/history/info/position/countdown/settlement/action modal,
  notifier, use case, detail screen), detail for_sale, saved item, picker
  komentar, dialog/state/repository withdraw, `bidding_screen` (engine
  `NumberFormat.currency` sendiri), `discount_card` (engine `NumberFormat`
  sendiri).

### 2. Yang berubah
- Semua di atas → `formatGroupedAmount`; `for_sale_detail_screen` memakai
  getter kanonik `forSale.formattedPrice`.
- `bidding_screen`: engine `NumberFormat.currency` + parameter
  `currencyFormat` pada `_BidInfo` DIHAPUS, bukan direplikasi.
- `discount_card`: `NumberFormat('#,###','id_ID')` → `formatGroupedAmount`.
- Tiga layar seller: prefix `'Rp '` ganda dihapus.
- `auction_action_modal`: teks tampilan dikonvergensikan, TAPI `hintText`
  input bid dibiarkan digit-mentah — field-nya `digitsOnly` dan validatornya
  menolak titik, jadi hint bergrup akan mengundang input yang pasti gagal.
  Ini pengecualian desain, bukan kelalaian.

### 3. Ratchet baru
`resource_projection_price_policy_test.dart` → test
`no surface builds a Rupiah string from a raw number`: melarang regex
`Rp ... ${...toStringAsFixed...}` di SELURUH `lib/`, dengan dua pengecualian
tertulis: `lib/generated/**` dan `currency_utils.dart` (shorthand kompak
`Rp 500K`), plus pengecualian baris `}jt'`/`}rb'` di `feed_renderers.dart`
(shorthand pasar `Rp1.5jt`) — sisa apa pun di file itu tetap gagal.

### 4. Verifikasi
- `flutter analyze lib` → 60 issues, **0 error**.
- `flutter test test/shared/domain/resource_projection_price_policy_test.dart`
  → **13 PASS** (11 lama + ratchet regex grup + ratchet raw-number ini).
- `test/domains/commerce/catalog/for_sale` + `social/comment` + `social/share`
  + `finance/withdrawal` + `commerce/pricing` → **238 PASS**.
- `test/domains/chat` → **383 PASS** (bagian dari 436 PASS bersama seller).
- Pin uang auction `expect(find.text('Rp 50.000'))` LULUS di
  `auction_detail_screen_runtime_test` — bukti angka kini grup.
- Kegagalan sisa yang ditemui, semuanya dibuktikan BUKAN regresi sapuan ini:
  - `saved_item_runtime_authority_test` (4) + `seller_upgrade_wizard_screen_
    identity_test` (1): diuji dengan **revert sementara** editanku → gagal
    PERSIS SAMA (`Bad state: No element`, item tidak muncul), lalu edit
    dikembalikan.
  - `seller_dashboard_operational_action_queue_test` (6) +
    `auction_detail_header_media_test` (1): file yang tidak pernah kusentuh,
    dan `grep Rp` di seluruh test seller = nol pin uang.
  - `auction_detail_screen_runtime_test` (3): `takeException` = harness
    `ApiClient must be provided externally`, bukan teks harga.
  - Catatan lingkungan: `feed_renderers.dart` dan `search_result_extra_info.dart`
    sempat berstatus error kompilasi karena agen paralel mid-edit (transien,
    pulih sendiri).

### 5. Residu
- `CurrencyUtils`/`AppFormatters` (intl `NumberFormat`) tetap engine kedua
  untuk layar seller/wallet; output identik dengan `formatGroupedAmount`,
  tapi dua mesin.
- Shorthand pasar di `feed_renderers` (`Rp1.5jt`/`Rp500rb`) masih formatter
  sendiri — kandidat penyatuan ke `CurrencyUtils.formatShorthand`
  (output akan berubah: `Rp 1.5Jt` dengan spasi).
- `seller_earnings_screen.dart:271` memuat literal hardcoded berbahasa
  Inggris: `'Rp 10.000 minimum withdrawal amount'` — copy, bukan formatter,
  tapi salah bahasa dan bisa melenceng dari nilai minimum sebenarnya.

## Komit acuan
e1fb4ae, c9ab974, a00b230, 94c1ce8, 7b0b1c8, 64e9f24, 23d36d9, 30874c6,
1b95590, b2dee89, ea8aca6, 124e7ca, 1d2b134.

## Audit ulang + plan baru (2026-09-27, sesi baru — tanpa warisan asumsi)

Prinsip: audit READ ONLY dulu → verifikasi ulang tiap klaim lama → plan baru.
Status: **Fase 1 & 2 SELESAI, belum di-commit** (owner melarang sentuh git;
working tree tetap 453 M + 24 U — risiko kehilangan tetap berdiri).

### Delta audit (klaim lama yang salah / kedaluwarsa)
1. **`live_status_provider.dart` bukan "keputusan badge yang tertunda" —
   itu DEAD CODE total.** `liveStatusProvider`, `LiveStatus` (+ 4 subclass),
   `ForSaleAvailabilityStatus`, `AuctionDisplayStatus`,
   `forSaleAvailabilityProvider`, `auctionStatusProvider`, dan 4 helper
   semuanya nol konsumen di `lib/` dan `test/`. Satu-satunya kemunculan
   "LiveStatus" lain adalah `attachment.supportsLiveStatus` — bendera
   kejujuran payload yang berbeda. Jadi arahnya **purge**, bukan migrasi badge.
2. **Celang ratchet uang = bug tampilan nyata.** Koreksi temuan lama: semua
   situs membawa `int`, jadi gejalanya "Rp 1000000" tanpa grup, bukan desimal
   `Rp 50000.0`.4 situs audit + **2 offender yang ratchet baru temukan
   sendiri** (`coin_balance_card` menyembunyikan delegation di wrapper
   `_formatNumber`; `feed_renderers` fall-through `'Rp$rupiah'`).
3. **Kartu Inggris `seller_earnings_screen` = bagian dari layar bilingual**;
   ada test yang mem-pin judul Inggris ("Available Balance") sebagai kanonik,
   jadi yang dibetulkan hanya angka mati → kini turun dari
   `WithdrawRequest.minAmount` (otoritas yang sama dengan validator).
4. **Uang non-IDR teoritis** — backend mematok `LivePriceCurrencyIDR`, jadi
   ditutup sebagai "won't fix sampai backend multi-currency".
5. **Temuan baru di luar rencana:** `go test ./...` backend **HANG** di
   `internal/commerce/subscription/application` (pgxpool menunggu Postgres,
   tanpa fail-fast) — setiap run memakan jatah10 menit lalu mati. Butuh DB
   (docker-compose) atau guard skip-bila-tanpa-DB.

### Fase 1 — otoritas uang (SELESAI)
- 6 situs `'Rp $x'` mentah → `formatGroupedAmount`:
  `seller_auctions_screen` (2 baris + import), `auction_detail_screen`
  (snackbar juga dialihkan ke "Bid berhasil!"), `place_auction_bid_usecase`,
  `checkout_action_bar` (label kini membawa prefix `Rp`-nya sendiri),
  `coin_balance_card` (wrapper `_formatNumber` dihapus, 2 call site),
  `feed_renderers` fall-through.
- **Ratchet baru:** `a Rupiah string never interpolates an unformatted number`
  (scan seluruh `lib/`; allowlist eksplisit: generated, engine shorthand
  `currency_utils`, baris `jt/rb` feed_renderers, dan `hintText:` input bid
  digitsOnly = pengecualian desain tertulis).
- **Engine kedua DIMATIKAN:** `CurrencyUtils.format/formatInt/formatNumber/
  formatNumberInt` kini delegasi ke `formatGroupedAmount`; `intl` keluar dari
  display (tersisa hanya input mask `currency_input_formatter.dart`).
  Paritas dibuktikan dulu terhadap mesin ICU SEBELUM delegasi:
  `test/shared/utils/currency_utils_single_engine_test.dart` (9 test —
  grouping, tanda minus `-Rp 1.500` di depan simbol, pembulatan .5 =
  half-away-from-zero persis ICU, shorthand, parse, facade) + ratchet
  source "CurrencyUtils tidak boleh memuat `NumberFormat`".
- Copy: `'Rp 10.000 minimum withdrawal amount'` → nilai hidup dari
  `WithdrawRequest.minAmount`.

### Fase 2 — bunuh otoritas/status mati (SELESAI)
- **Purge `live_status_provider.dart` (688 baris)** + 2 export barrel
  (`shared/shared.dart`, `chat/attachment/attachment.dart`) + komentar basi
  `shared/attachment/attachment.dart`.
- **Ratchet baru:** `test/shared/domain/snapshot_live_status_purge_test.dart`
  (file harus tidak ada + scan `lib/`&`test/` untuk 13 identifier dengan
  `\b` agar `supportsLiveStatus` tetap legal).
- **Purge `getForSalesByIds`**: interface + impl + datasource + 4 fake test.
  Bukti tambahan: backend **tidak punya route `/for-sale/batch` sama sekali**
  — endpoint itu sudah pasti 404 sejak lahir.
- **Ratchet** ditambahkan ke `reference_attachment_live_fetch_purge_test.dart`
  (4 test) — needle disusun literal berdempetan supaya tidak self-match.

### Verifikasi (command → hasil)
- `go build ./...` → 0 error; `go test ./internal/commerce/shared
  ./internal/interaction/chat/...` → **ok**; delegasi `Evaluate*` di 4 resolver utuh.
- `flutter analyze lib` → **60 issues, 0 error** (identik baseline; purge tidak
  menambah unused-import).
- Ratchet: price policy **14 PASS**, snapshot purge **2 PASS**, live-fetch purge
  **4 PASS**, currency parity **9 PASS**.
- for_sale domain + marketplace injection → **64 PASS**.
- Batch verifikasi terarah (checkout, auction, earnings exposure, feed render,
  shared/object, currency) → **215 PASS / 5 FAIL**: 4 = pre-existing terverifikasi
  (`auction_detail_screen_runtime` x3 harness `ApiClient must be provided
  external`, `auction_detail_header_media` x1) + 1 = ratchet baru kena
  self-match di komentarnya sendiri → **diperbaiki, re-run 4 PASS**.

### Fase 3 — baseline (SEBAGIAN; sisa butuh1 perintah luar limit tool)
Terekam & terverifikasi:
- Mobile pre-existing: `saved_item` 4 fail (di-sample ulang sesi ini, persis
  sama), auction runtime x3, auction header media x1; `test/shared` harness
  lama (ApiClient/ProviderScope) — daftar lengkap35 itu milik sesi sebelumnya,
  tidak saya ulang karena suite penuh tidak muat di batas10 menit/perintah.
- Backend pre-existing: `subscription/application` hang tanpa DB; dulu
  `TestPaymentReuseGuard_HandlesLookupErrors` + `TestUnifiedShareDepth1_D20`.
- **Untuk baseline penuh (sekali jalan, di luar tool):**
  `cd backend && docker compose up -d postgres && go test -timeout 120s ./...`
  dan `cd apps/mobile && flutter test --reporter expanded` → simpan grep
  `' [E]'` sebagai daftar gagal bertanggal.

### Masih terbuka
- Commit — **ditahan atas permintaan owner**; risiko 453+24 file tetap berdiri.
- Owner retest device: chat share for_sale/auction LIVE+TOMBSTONE, comment
  attachment, harga discovery.
- Keputusan: shorthand pasar `Rp1.5jt/Rp500rb` disatukan ke
  `CurrencyUtils.formatShorthand` (output berubah jadi `Rp 1.5Jt`),
  i18n layar seller (judul Inggris dipin test), guard DB untuk test backend.
