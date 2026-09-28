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
5. ~~Test debt pre-existing commerce~~ ✅ SELESAI (scope For Sale vs Auction,
   laporan §11 di bawah): `auction_detail_screen_runtime_test` **6/6 hijau**
   (akar: harness tidak menyediakan `savedItemRepositoryProvider` →
   `apiClientProvider` error state → `CommerceDetailSellerCard` melempar
   ProviderException saat `watch`; plus media shimmer vs fake-async),
   `auction_detail_header_media_test` **2/2 hijau**.
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
  `payment_result_notifier_test` (⚠️ KOREKSI 2026-09-28 malam-2: klaim "authority" ini SALAH — akarnya fixture time-bomb `expiredAt`; lihat bagian terakhir).
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
  `' [E]'` sebagai daftar gagal bertanggal.### Masih terbuka
- Commit — **ditahan atas permintaan owner**; risiko453+24 file tetap berdiri.
- Owner retest device: chat share for_sale/auction LIVE+TOMBSTONE, comment
  attachment, harga discovery.
- Keputusan: shorthand pasar `Rp1.5jt/Rp500rb` disatukan ke
  `CurrencyUtils.formatShorthand` (output berubah jadi `Rp 1.5Jt`),
  i18n layar seller (judul Inggris dipin test), guard DB untuk test backend.

## Batch keputusan tertahan (2026-09-27, sesi yang sama)

Semua keputusan yang tertahan kini dieksekusi — hasil, bukan opsi:

1. **Shorthand pasar DISATUKAN, arahnya ke format grouped.** Discovery harus
   menampilkan string yang sama untuk angka yang sama; `Rp75rb` di feed vs
   `Rp 75.000` di search row = dua kebenaran. `feed_renderers._formatPrice`
   kini `Rp ${formatGroupedAmount(minor ~/ 100)}`; allowlist `jt/rb` di DUA
   ratchet uang DIHAPUS (justru ratchet jadi lebih ketat). Shorthand kompak
   (`Rp 1.5Jt`, `Rp 500K`) tetap hidup sebagai fitur dashboard di
   `CurrencyUtils.formatShorthand` — satu implementasi, konteks terpisah.
   4 pin test promo di-align (`Rp75rb` → `Rp 75.000`, dst).
2. **i18n layar penghasilan seller.** `seller_earnings_screen` kini id-first
   (appbar, 4 kartu + subtitle, seksi info, tombol `Tarik Dana`, pesan error).
   Test exposure di-align maju sesuai doktrin "test mengikuti codebase";
   sisa pin Inggris yang tertinggal (`'Pending Balance'`) ikut dikejar.
   SISA: `withdraw_dialog`, `seller_renewal_screen`, `seller_upgrade_wizard`
   masih campur — pemindaian menyeluruh butuh sapuan terpisah.
3. **Guard DB backend.** Akar hang BUKAN koneksi, tapi `runMigrationsRaw`
   memakai `context.Background()` di semua query — sesi basi yang menahan
   lock DDL membuat suite membaca sebagai hang selamanya. Kini:
   `acquireLifecycleLock` + reset + `migration.Run` berbagi deadline
   (`connectionTimeout`10s / `migrationTimeout`60s), DB tak terjangkau atau
   migrasi macet → `t.Skip` cepat (CI paksa gagal dengan `REQUIRE_TEST_DB=true`).
   Bukti: paket subscription dulunya hang >90s, kini selesai **60.3s** dengan
   sisa kegagalan hanya unit test prasyarat.

### Verifikasi batch ini
- `flutter analyze lib test` → **185 issues, 0 error** (= baseline60+125).
- Ratchet uang/snapshot/paritas/object + exposure test → **55 PASS**.
-2 gagal tersisa = `home_screen_promoted_card_rendering_test` SCENARIO 4
  (state feed kosong = harness) — **terbukti pre-existing**: saat edit feed
  saya di-stash sementara, tes yang sama tetap gagal (bahkan lebih banyak,
  karena pin dan kode sengaja tidak cocok saat itu).
- `go vet ./pkg/testdb/` bersih; `go test ./internal/commerce/subscription/
  application/` → **tidak lagi hang**, selesai60s; gagal tersisa
  `TestProcessSuccessfulPaymentTx_SplitsPrincipalAndFeeFromSnapshot`
  (4 assert tanda ledger, unit test tanpa DB — pre-existing, di luar sapuan ini).
- **Temuan operasional:** ada `go.exe` yatim (PID15016, start12:08:08) dari
  run yang kena kill — kemungkinan ia yang memegang lock sehingga migrasi test
  DB macet. Tidak kubunuh karena kepemilikannya bisa jadi agen lain.

## Scope — For Sale vs Auction Consistency (Scope C + A + B) — laporan §11

### 1. Verdict
SELESAI dan HIJAU. Satu Product = satu authority konten: wire auction (list +
detail) kini membawa blok Product yang sama dengan for_sale dan parity-nya
dikunci test backend. Presentasi kedua channel dikonvergensikan ke satu
skeleton: grid/kartu discovery (2 kolom) dan detail (states + CustomScrollView +
product section + seller card). Zombie `auction_list_screen.dart` dimatikan.
Parkiran commerce (`auction_detail_screen_runtime_test` x3 +
`auction_detail_header_media_test` x2) MATI — akarnya dipin, bukan diwarisi.
Bukti akhir: mobile 271/271 hijau, `flutter analyze` 0 error; backend build +
3 paket commerce hijau.

### 2. Root cause
1. **Konten dual authority.** `auctionToResponseWithSeller` hanya mengirim
   ringkasan sementara for_sale mengirim Product penuh — satu Product, dua
   kebenaran di wire (detail auction bahkan memakai projection signature
   terpisah).
2. **Presentasi tumbuh per channel.** Kartu: `return Card(` + badge manual +
   deskripsi di kartu; grid: `ListView.builder`/`SliverList` yang berbeda di 4
   surface; detail: for_sale memakai `SingleChildScrollView` + seller card lokal
   + section prep lokal, auction memakai header media sendiri.
3. **Zombie.** `auction_list_screen.dart` = browse surface tanpa route.
4. **Parkiran test — akar sebenarnya (bukan "harness lama"):** harness
   `auction_detail_screen_runtime_test` tidak menyediakan
   `savedItemRepositoryProvider`; `CommerceSavedItemActionButton.initState`
   membaca repository → `apiClientProvider` throw → provider masuk error state.
   Setelah itu `CommerceDetailSellerCard` (membaca `profileStreamProvider` untuk
   foto toko — perilaku ini SUDAH ADA di HEAD `auction_seller_card.dart:27`)
   melempar `ProviderException: Tried to use a provider that is in error state`.
   Ditambah: shimmer media (`CachedNetworkImage`) tidak settle di fake-async →
   `pumpAndSettle` timeout. Harness for_sale hijau karena sudah menyediakan
   `_FakeSavedItemRepository` — pola kanonik yang sama dipakai
   `auction_detail_restriction_dispatch_test.dart`.

### 3. Canonical behavior setelah perbaikan
- **Product = 1 authority konten** di kedua channel; 14 key konten (media,
  media_urls, variety, size_cm, age_months, gender, breeder, bloodline,
  certificates, farm_address_id, preparation_time, preparation_note, …) di list
  & detail; `viewer_capabilities` hanya di DETAIL.
- **Discovery:** satu grid 2 kolom (`CommerceMarketplaceGrid`) di 4 surface;
  satu shell kartu + satu seller block (`CommerceCardSellerMetadata`); tanpa
  deskripsi di kartu (kartu = ringkasan, detail = kebenaran).
- **Detail:** satu skeleton identik antar channel — `CommerceDetailStates`
  (loading/error/notFound) + `AppBarCustom` + `CustomScrollView` + slot judul +
  `CommerceCommonProductDetailSection` + `CommerceDetailSellerCard` + action bar
  capability-driven; media `MediaCarouselWidget` rasio 4/3 di kedua channel.
- **Seller identity detail = 1 authority** (`CommerceDetailSellerCard`):
  @username line1, nama toko line2, degraded → label redaksi + tap mati, tier
  badge digate lifecycle; channel hanya menyuplai fakta entity.
- Auction: bottom bar `Chat` + `Pasang Bid`; ForSale: Chat/Ajukan
  Penawaran/Beli Sekarang; Promote = IconButton AppBar (owner + status active).
- Uang kanonik `formatGroupedAmount` (`Rp 50.000`) di semua permukaan.

### 4. File yang berubah (file inti scope; working tree multi-agen)
- **Backend (5):** `auction/delivery/http/auction_handler.go`,
  `auction/delivery/http/auction_detail_response_projection.go` (signature
  `(a, sellerCard, sellerInfo, product, viewerID)`),
  `commerce/shared/product_content_wire.go` (BARU, 14 key),
  `auction/…/auction_list_content_parity_test.go` (BARU),
  `forsale/…/for_sale_list_content_parity_test.go` (BARU).
- **Mobile lib (17):** auction → `auction.dart`, `data/dto/auction_dto.dart`
  (+`farmAddressId`), `data/mappers/auction_mapper.dart`,
  `presentation/presentation.dart`, `screens/auction_detail_screen.dart`,
  `screens/create_auction_screen.dart`, `widgets/auction_card.dart`,
  `widgets/detail/{auction_detail_header,auction_detail_info,auction_seller_card,
  auction_bid_section,auction_bid_history}.dart`; `screens/auction_list_screen.dart`
  (DIHAPUS); for_sale → `domain/entities/for_sale.dart`,
  `presentation/providers/for_sale_providers.dart`,
  `screens/create_for_sale_screen.dart`, `screens/for_sale_detail_screen.dart`
  (tulis ulang), `screens/for_sale_list_screen.dart`, `widgets/for_sale_card.dart`;
  shared → `presentation/sender_address_provider.dart` (BARU),
  `presentation/widgets/commerce_card_seller_metadata.dart` (BARU),
  `commerce_detail_seller_card.dart` (BARU), `commerce_detail_states.dart` (BARU),
  `commerce_marketplace_primitives.dart`; `shared/governance/seller_tier_badge.dart`.
- **Mobile test (9):** `for_sale/for_sale_detail_screen_runtime_test.dart`,
  `shared/commerce_detail_negative_contracts_test.dart` (kontrak paritas detail),
  `auction/…/auction_detail_header_media_test.dart`,
  `auction/auction_media_source_contract_test.dart`,
  `auction/auction_response_dto_shipping_origin_test.dart`,
  `auction/auction_detail_screen_runtime_test.dart` (parkiran dimatikan),
  `test/features/commerce/marketplace_surface_contract_test.dart`,
  `test/core/theme/theme_authority_contract_test.dart` (BARU),
  `test/shared/governance/seller_tier_stage2_test.dart`.

### 5. Residue yang dihapus
- `auction_list_screen.dart` + 2 export barrel (dengan komentar penanda).
- Kartu: `return Card(`, badge manual, deskripsi di kartu, `Positioned(`.
- Discovery: `ListView.builder(`/`SliverList(` di 4 surface.
- Detail for_sale: `_ForSaleSellerCard`, `_PromoteButton`, section prep lokal,
  `SingleChildScrollView`; detail auction: header PageView lokal, seller card
  lokal, section bid lokal → semua ke widget kanonik shared.
- Sapuan sisa: `SingleChildScrollView` hanya di dialog
  `auction_claim_shipping_modal.dart` (dialog, bukan detail — sengaja);
  "Fixed-Price" hanya di doc comment use case harga tetap (istilah benar).

### 6. Commands dan hasil (bukti akhir)
- `go build ./...` (backend) → exit 0.
- `go test ./internal/commerce/shared/... ./internal/commerce/auction/delivery/http/... ./internal/commerce/forsale/delivery/http/...` → ok (3 paket).
- `flutter analyze` (mobile) → **187 issues, 0 error**.
- `flutter test test/domains/commerce/catalog test/features/commerce test/core/theme test/shared/governance` → **+271, 0 gagal** (sebelum parkiran dimatikan: +268 −3; total test sama = 271, jadi nol degradasi).
- `flutter test …/auction_detail_screen_runtime_test.dart` → **6/6**.
- `flutter test …/auction_detail_header_media_test.dart …/for_sale_detail_screen_runtime_test.dart` → **9/9**.

### 7. Temuan di luar scope (tidak disentuh)
- Working tree multi-agen: 465 file berubah/untracked; scope ini ±31 file.
  File commerce lain (negotiation, order, checkout, subscription) milik sesi/agen
  paralel — jangan ikut di-stage.
- Parkiran lain di luar scope ini:
  - `saved_item_runtime_authority_test` — **diverifikasi ulang sesi ini: 4 gagal / 2 lulus**,
    semua asersi (judul item `For Sale Item`/`Saved ForSale`/`Saved Auction` tidak
    render). Domain `user/preference/saved_item` tidak tersentuh (git status =
    HEAD) dan nol referensi ke widget scope ini → baseline, bukan regresi scope.
  - harness feed/promoted, dsb. — statusnya masih seperti catatan sesi sebelumnya,
    tidak dijalankan ulang sesi ini.

### 8. Risiko / belum terbukti
- Verifikasi visual device belum (owner retest): kartu 2 kolom di kedua tab
  discovery dan urutan section detail di kedua channel.
- Test dengan fixture media + `pumpAndSettle` akan timeout (shimmer): pakai
  bounded pumps (pola sudah ditulis di dua file detail runtime).
- `CommerceDetailSellerCard` membaca `profileStreamProvider` (poller 30 detik)
  untuk foto toko — benar di produksi (authority FarmInfo), tapi harness yang
  merender kartu ini WAJIB menyediakan `savedItemRepositoryProvider`; kalau
  tidak, kegagalannya menyesatkan (ProviderException, bukan asersi).

### 9. Git status
Belum di-commit (owner belum minta). 1 file terhapus
(`auction_list_screen.dart`), 4 file baru di lib commerce shared, 3 test baru
(2 parity backend + 1 theme authority mobile).

### 10. Owner retest
Perlu — sekali lihat: (a) tab For Sale & Lelang di marketplace (grid 2 kolom,
kartu identik), (b) detail satu for_sale dan satu auction (urutan section sama,
harga tergrup, seller card sama), (c) Promote hanya muncul untuk owner listing
aktif.

## Sapuan bunuh tambahan — For Sale vs Auction (sesi sama, setelah laporan §11)

Klaim "bersih" sebelumnya TIDAK cukup bukti. Sapuan ulang dengan standar §4
(clean = total clean), §11 (zombie) dan §16 (decision rule) menemukan 5 sisa;
semuanya sudah DIEKSEKUSI, bukan dilaporkan:

1. **ZOMBIE — `for_sale_media_handler.dart` (335 baris) + test pemelihara
   `listing_media_handler_validation_test.dart`.** Authority faktual =
   `MediaGridUploader` (dipakai create_auction, create_for_sale, edit_for_sale)
   dan komentar orchestrator sudah menyatakan "Replaces duplicated logic".
   Kedua file DIHAPUS; roster theme di-align (entry file terhapus = dead, bukan
   migration — preseden auction_list_screen); komentar orchestrator dibetulkan
   (nama kelas mati dibuang); komentar "FACTUAL BLOCKER" di restriction_dispatch
   test dibetulkan (mekanisme kini MediaGridUploader, kesimpulan blocker tetap).
   **Catatan jujur:** file membawa 1 edit worktree milik sesi lain (warna icon
   `AppColors.primaryRed` → `colorScheme.primary`) yang ikut hilang bersama
   file; tidak ada fitur/klaim lain yang terbuang.
2. **PROXY — `_buildLoadingScaffold()` di auction_detail_screen** (forwarder 1
   baris) → inline ke `CommerceDetailStates.loading`; kontrak paritas ditulis
   ulang: kedua channel kini di-assert memanggil `CommerceDetailStates.` +
   negative proof (channel tidak boleh menggambar spinner/error icon sendiri;
   spinner tunggal auction = busy dialog aksi, bukan state halaman).
3. **TERMINOLOGI — `get_for_sale_share_reference_usecase.dart`**: "Fixed-Price-
   Sale" → "For Sale" (2 baris) → `lib/` bebas `Fixed-Price`.
4. **DUPLIKAT + INKONSISTENSI CHANNEL — dua `_buildAccessGate` private**
   (for_sale vs auction) dengan chrome sudah menyimpang (icon lock 56 vs
   storefront 64, spacing beda, appbar beda, wiring tombol beda), gate
   unauthenticated auction = teks telanjang tanpa judul/tombol, gate loading
   hanya di for_sale → authority baru **`CommerceAccessGate`**; kedua screen
   memakainya; auction kini hydrate → `CommerceDetailStates.loading('Buat
   Lelang')`, restricted → `AccountRestrictedScreen`, unauthenticated → gate
   penuh (paritas for_sale). 2 pin test di-align (loading → spinner; restricted
   → AccountRestrictedScreen); copy login TIDAK berubah (masih dipin dua channel).
5. **DIAGNOSTIK — helper `_block` mati** di `auction_media_source_contract_test`.

### Hasil setelah sapuan ini
- `flutter analyze` → **186 issue, 0 error**, nol issue di file yang disentuh.
- Suite scoped (catalog + features/commerce + core/theme + shared/governance) →
  **264/264 hijau** (271 − 7 test pemelihara zombie yang ikut dihapus; nol gagal).
- Sweep akhir: `ForSaleMediaHandler` lib=0, `_buildAccessGate` 0/0,
  `_buildLoadingScaffold` 0/0, `Fixed-Price` lib=0, `listing_media_handler_validation` 0/0.

### Sisa yang TIDAK dibunuh — dengan alasan, bukan alibi
- `seller_auctions_screen.dart:273 return Card(` — surface manajemen seller,
  bukan 4 surface discovery yang di-ratchet; di luar scope (§13).
- `auction_claim_shipping_modal.dart:306 SingleChildScrollView` — badan modal,
  bukan scroll detail.
- `_buildErrorScaffold` / `_buildNotFoundScaffold` (auction_detail_screen) —
  punya tanggung jawab nyata (peta pola error → copy user; sumber tunggal copy
  "tidak ditemukan" untuk 2 pemanggil) → bukan proxy; inline justru menduplikasi copy.
- `Fixed-Price|fixed-price-sale` di **test** domain lain (10: 2 judul + 8 literal
  fixture) — bukan authority; domain lain (§13).
- Anti-hidup-lagi terkunci ratchet: `marketplace_surface_contract_test` (grid
  wajib; `ListView.builder(`/`SliverList(`/`return Card(`/`Positioned(` dilarang
  di 4 surface + 2 kartu) dan `commerce_detail_negative_contracts_test` (kedua
  channel wajib `CommerceDetailStates.`, dilarang spinner/icon sendiri).

## Sesi 2026-09-28 — Penutup scope media + gate tema lib-wide + konvergensi file authority (laporan §11)

### 1. Verdict
LIMA scope tertutup dan terverifikasi, semuanya bounded: (a) residu analyzer
scope media/satu-engine; (b) registry tema 161 entri → gate lib-wide;
(c) higiene toolchain yang menghambat tiap perintah; (d) 7 residu tema di
`lib/`; (e) konvergensi tiga file authority ganda yang terbukti mati total.
CANONICAL TRUTHS baru sudah dicatat di `KONDISI_APP.md` §14 (tema, satu file
satu authority, media satu engine).

### 2. Root cause
1. **Registry tema per-scope adalah kunci yang salah bentuk.** Ia mengunci 274
   file sambil meninggalkan **100 file UI (27%)** yang sudah bersih tapi tak
   dijaga, dan setiap closure menuntut entri manual. Enforcement-nya lebih
   lemah daripada `lib/` yang faktual sudah 100% bersih.
2. **Residu analyzer adalah penanda cleanup yang belum selesai**, bukan noise:
   variabel `scheme` mati, param `badgeText` yang tak pernah diberi, ctor
   non-const, `value:` deprecated — semuanya sisa konvergensi tema yang belum
   ditutup.
3. **`assets/images/payment_methods/` didaftarkan di pubspec padahal direktori
   itu tidak ada dan nol kode mereferensikannya** → warning tercetak di SETIAP
   perintah flutter (menutupi sinyal nyata).
4. **Histori Labuda: tiap fitur menumbuhkan salinan sendiri lalu konvergensi
   memindahkan konsumen tetapi meninggalkan salinannya.** Tiga salinan itu
   byte-identik, nol importer, dan tiap direktori salinannya hanya berisi file
   mati tersebut.

### 3. Canonical behavior setelah perbaikan
- Tema: satu authority `AppTheme.lightTheme/darkTheme`; widget `Theme.of(context)`;
  hanya 3 file boleh memegang warna; gate menyapu SELURUH `lib/` (1162 file)
  dengan allowlist eksplisit + lantai >1000 file supaya tidak bisa vakum.
- Satu file satu authority: `Province/City/District/Village` →
  `shared/models/wilayah_models.dart`; `PostLocation/LatLng` →
  `shared/entities/post_location.dart`; `BaseEntity/BaseModel` →
  `core/common/base_entity.dart`. Tidak boleh ada dua file `lib/` berbyte identik.
- Media: satu engine (`MediaUploadOrchestrator` + config) dan satu grid.
- Toolchain: pubspec hanya mendeklarasikan aset yang benar-benar ada; tiap
  import langsung punya deklarasi dependency.

### 4. File yang berubah
- **Tema (test):** `test/core/theme/theme_authority_contract_test.dart` —
  `_migratedUiPaths` (161 entri) DIBUNUH → `_authorityFiles` (3) +
  `_libDartFiles()` sweep + pola diangkat ke satu definisi + test negatif-proof
  baru; sweep snackbar memakai helper yang sama; import `material.dart` mati dihapus.
- **Gate baru:** `test/core/file_authority_contract_test.dart` (3 test).
- **Media:** `core/media/media_upload_config.dart` (doc menempel ke kelas),
  `core/media/media_upload_orchestrator.dart` (4× `use_build_context_synchronously`
  ditutup guard mounted + 1 html-in-doc), `shared/widgets/media_grid_uploader.dart`
  (2× curly braces, `(_, _)`, doc, baris kosong nyasar).
- **7 residu tema:** `auction_action_modal.dart` (var `scheme` mati),
  `notification_empty_state_widget.dart` (ctor const + 2 call site const, supaya
  tidak melahirkan `prefer_const_constructors`), `profile_screen.dart` (import
  berlebih), `seller_dashboard_screen.dart` (param/field `badgeText` mati + Row
  badge yang selalu kosong, 4× wildcard ganda), `canonical_promotion_create_screen.dart`
  (`value:` → `initialValue:`), `app_dropdown.dart` (`DEFAULT_VARIETY`
  SCREAMING_CASE dihapus, alias di-collapse), `features/home/data/dto/feed_dto.dart`
  (if ber-brace).
- **Gaya/format:** `test/core/media/`, `test/core/theme/` (milik sesi ini).
- **Toolchain:** `pubspec.yaml` + `pubspec.lock` (regenerasi `flutter pub get --offline`).

### 5. Residue yang dihapus
- `assets/images/payment_methods/` dari daftar aset pubspec (direktori tidak ada).
- `lib/domains/commerce/catalog/models/wilayah_models.dart` + direktori `models/`.
- `lib/domains/commerce/catalog/entities/post_location.dart` + direktori `entities/`.
- `lib/core/src/domain/base_entity.dart` + direktori `core/src/domain/`.
- Registry tema 161 entri + seluruh komentar migrasi per-scope di dalamnya
  (enforcement-nya digantikan gate; riwayat tetap ada di git sebagai backup,
  bukan authority).

### 6. Tests dan build yang dijalankan (command → hasil)
- `flutter analyze lib/core/media lib/shared/widgets/media_grid_uploader.dart`
  → **No issues found**.
- `flutter test test/core/media/` → **5 PASS**.
- `flutter test test/core/theme/theme_authority_contract_test.dart`
  → **16 PASS** (sebelumnya 14: registry 2 test → gate 3 test).
- `flutter analyze test/core/theme/theme_authority_contract_test.dart`
  → setelah import mati dihapus: bersih.
- `flutter pub get --offline` → **Got dependencies**; warning
  `unable to find directory entry in pubspec.yaml` HILANG dari keluaran flutter.
- `flutter test` dua pengimpor `fake_async` → **5 PASS**.
- `flutter analyze lib` → **37 → 26 issue, 0 error** (tidak ada lint baru).
- `flutter test` batch terarah (auction/promotion/seller/feed + gate tema)
  → **84 PASS**; batch kedua (11 file, termasuk 9 pengimpor wilayah/PostLocation)
  → **89 PASS**.
- `flutter analyze` (penuh) → **164 issue, 0 error**.
- **Negative proof gate file authority:** salinan mati dikembalikan sementara →
  **3/3 test GAGAL** (byte identik tertangkap; rumah kelas ganda tertangkap;
  path bangkit tertangkap); dihapus lagi → **3/3 PASS**.
- **Negative proof gate tema:** 5 baris kebangkitan (`AppColors.neutralWhite`,
  `Colors.white`, `Color(0x`, `isDark`, `brightness ==`) WAJIB memicu, dan 4
  token sah (`scheme.*`, `statusSuccess`, `coinPrimary`, `statusBar*Brightness`)
  WAJIB tidak memicu — dipin di dalam test.

### 7. Hasil proof
Sweep `lib/` (1162 file) untuk 6 pola warna terlarang: pelanggaran **nol** di
luar 3 file authority. Sapuan md5 seluruh `lib/`: **hanya 3 pasangan identik**
(yang sekarang sudah mati) — sebelum tindakan, tiap salinan nol importer via
`package:`, nol export barrel, dan direktori salinannya hanya berisi file itu.
Residue search pasca-hapus: 0 referensi ke `src/domain/base_entity`,
`catalog/models/wilayah_models`, `catalog/entities/post_location` di `lib/`,
`test/`, `tool/`; `flutter analyze lib` 0 error (kompilasi membuktikan tak ada
import menggantung).

### 8. Temuan di luar scope — PARKIRAN BARU (diklasifikasi, tidak dikerjakan)
1. **P1 kandidat (menyentuh uang → §3 audit mendalam wajib): authority
error/result ganda.** `NetworkFailure`, `ValidationFailure`, `UnknownFailure`
hidup dua kali: `core/errors/failure.dart` vs
`domains/finance/transaction/payment/domain/failures/payment_failure.dart`.
`RepositoryResult` juga dua kali: `order/domain/repositories/repository_result.dart`
vs `payment/domain/repositories/payment_repository.dart`. Plus kontrak domain
ganda: `DecisionContract` + `DisplayHints` di `order/domain/entities/order.dart`
vs `payment/domain/entities/payment.dart`.
2. **P2 — nama kelas publik terduplikasi lain (bukan byte-identik, butuh audit
tersendiri):** `ContentSearchResult` (content repo vs search repo),
`ContentSearchResultDto` (content dto vs search dto), `UserSearchResponseDto`
(follow dto vs search dto), `MessageDto` (chat dto vs support ticket dto),
`NotificationEntity` (interface core vs entity domain), `SearchState`
(content vs search), `ContentList` (freezed), `ProfileStats` +
`ProfileActions`, `BlockActionState` (dua deklarasi di `shared/providers/`).
Legit, bukan duplikat: `PlatformDetectorImpl` (conditional import io/web).
3. **AMBIGUOUS — butuh keputusan owner, TIDAK disentuh:** `KoiVarieties` ganda.
`core/constants/koi_varieties.dart` diexport `core.dart:35`, tetapi
`shared/widgets/app_dropdown.dart` mendeklarasikan kelas lokal dengan nama yang
sama (men-shadows export itu) memakai daftar berbeda ('Other' vs 'Lainnya',
nama antar-daftar berbeda). Menyatukannya MENGUBAH daftar variety yang dilihat
user → business decision, bukan cleanup.
4. **P2 zombie kecil di `lib/`:** `core/dependencies/provider_scope_reader.dart`
(import `core.dart` tak terpakai + `_ProviderScopeHolder` nol konsumen) dan
`core/api/config/api_config.dart` (import `foundation.dart` tak terpakai).
5. **P3 style (tidak boleh menghentikan progress):** 22× `use_null_aware_elements`
di 7 file data layer (chat/auction/for_sale/share/saved_item/seller datasource)
+ `USE_FIREBASE` bukan lowerCamelCase di `shared/services/firebase_wilayah_service.dart:18`.
6. Direktori `domains/commerce/catalog/models/`, `domains/commerce/catalog/entities/`,
   dan `core/src/domain/` sudah tidak ada — jangan dibuat ulang.

### 9. Risiko / belum terbukti
- **Full regression tidak dijalankan** (suite penuh >1105 file test). Justifikasi:
yang dihapus adalah file 0-importer, sisanya residu/format; bukti pengganti =
`flutter analyze` 0 error + residue sweep 0 + batch terarah 89 PASS. Jika owner
menginginkan gate rilis, jalankan `flutter test --reporter expanded` sekali.
- Working tree multi-agen: 3 file media yang saya edit juga membawa perubahan
sesi lain sebelumnya — hunk mereka tidak saya sentuh.
- Runtime/device proof: belum ada, dan belum diperlukan untuk scope ini
(nol perubahan perilaku UI: yang dihapus file mati, yang diedit residu analyzer;
pengecualian: `DropdownButtonFormField.initialValue` dan hilangnya Row badge
yang selalu kosong — keduanya nol perubahan visual).
- `KoiVarieties` tetap AMBIGUOUS sampai owner memutuskan daftar variety.

### 10. Git status
Belum di-commit (owner belum minta). Milik sesi ini saja: `M` pubspec.yaml,
pubspec.lock, 3 file media, 7 file residu tema, 1 test tema;
`A` `test/core/file_authority_contract_test.dart`;
`D` 3 file + 3 direktori kosong yang dirapikan. Sisa working tree (belasan file
commerce/UI) milik agen/sesi paralel — jangan ikut di-stage.

### 11. Owner retest diperlukan?
Belum untuk scope ini. Retest device yang masih tertunda tetap milik scope
projection/harga sesi sebelumnya (chat share for_sale/auction LIVE+TOMBSTONE,
comment attachment, harga di discovery).

## Sesi 2026-09-28 (lanjutan) — P1 audit: authority error/result & decision-contract jalur uang

### 1. Verdict
Audit mendalam SELESAI; satu eksekusi bounded SELESAI dan terkunci (rantai
`DecisionContract` phantom di payment di-PURGE). Sisa scope — konvergensi
`RepositoryResult` → `Result<T>` dan penyatuan taksonomi failure — **BELUM
dieksekusi dan TIDAK boleh dianggap PASS**. Audit membuktikan ia migrasi ~13
file lib + ~20 file test di jalur uang dengan satu jebakan senyap yang wajib
ditangani per call site.

### 2. Root cause (fakta, bukan asumsi)
1. **Tiga tipe result untuk satu konsep.** `Result<T>`
   (`core/common/result.dart`) = authority mayoritas (~60 file lib + ~35 test)
   dan **superset**: punya `statusCode` + `isSuccess` dari flag internal.
   `RepositoryResult<T>` (`order/domain/repositories/repository_result.dart`) =
   salinan lebih lemah (tanpa `statusCode`, `isSuccess => data != null`, factory
   `error` yang hanya alias `failure`). `RepositoryResult<T>` kedua di
   `payment/domain/repositories/payment_repository.dart` membawa payload beda
   (`PaymentFailure? failure`). Akibat nyata: dua repo commerce memakai tipe
   berbeda untuk konsep sama — for_sale memakai `Result`, auction/order/seller
   memakai `RepositoryResult`.
2. **JEBAKAN SENYAP — `fold` berurutan terbalik.**
   `RepositoryResult.fold(onSuccess, onError)` vs `Result.fold(onError, onSuccess)`.
   Keduanya hidup di file yang sama: `auction_remote_datasource.dart:61` memakai
   urutan error-dulu (Result), `auction_repository_impl.dart:344` memakai
   sukses-dulu (RepositoryResult). Rename buta = menukar jalur sukses/gagal di
   jalur uang.
3. **Decision contract phantom di payment.** Backend hanya punya SATU:
   `backend/internal/commerce/order/delivery/http/dto/decision.go`
   (`primary_action`/`secondary_actions`/`decision_version`/`display`);
   `grep allowed_actions` di backend = **nol**. Payment justru mem-parse
   `allowed_actions`, DTO-nya sendiri menulis "`GET /payments/:id` does not
   currently emit `decision`", nol pembaca `Payment.decision`, nol pin test.
   Pola identik sudah dibersihkan di permukaan auction (`auction.dart:38`
   menyimpannya sebagai breadcrumb) — purge ini menutup sisanya.
4. **Taksonomi failure per-domain (fakta).** core `Failure` (dipakai follow via
   `Either<Failure, T>` lewat barrel `core.dart:6`), `PaymentFailure`,
   `SupportFailure`, `ShareFailure`, `ReportFailure`, `WarningFailure`,
   `AppealFailure`, `AuthStateBackendFailure`. Hanya tiga nama yang bertabrakan
   persis: `NetworkFailure`/`ValidationFailure`/`UnknownFailure` (core DAN
   payment). Catatan: grep importer berbasis `package:` menyesatkan di sini —
   barrel re-export adalah jalur hidupnya.

### 3. Canonical behavior
- **Satu decision contract**: `order/domain/entities/order.dart` (parity key
  backend). Payment tidak memiliki decision contract.
- **DIPUTUSKAN, belum dieksekusi**: `Result<T>` = satu-satunya result authority;
  `RepositoryResult` (order + payment) mati.
- Payment state tetap dari wire `status` (`PaymentStatus`); client tidak
  menurunkan state bisnis sendiri.

### 4. File yang berubah
- `M` `lib/domains/finance/transaction/payment/domain/entities/payment.dart` —
  `DecisionContract` + `DisplayHints` (dua kelas), field `decision`, param ctor,
  entri copyWith, entri `props`, dan blok komentar P11 dihapus; penggantinya
  satu catatan akurat: state payment hanya dari `status` backend.
- `M` `lib/domains/finance/transaction/payment/data/dto/payment_dto.dart` —
  `DecisionContractResponseDto` + `DisplayHintsDto` + field `decision` + blok
  parse + konversi `toEntity` dihapus.
- `M` `lib/domains/commerce/transaction/order/domain/entities/order.dart` —
  vocabulary menyesatkan `decision.allowed_actions` (3 tempat) →
  `decision.hasActionType(...)`.
- `A` `test/domains/finance/transaction/payment/payment_decision_phantom_purge_test.dart`
  (4 test).

### 5. Residue yang dihapus
Salinan decision contract payment berikut seluruh kaki tangannya: 2 kelas
entity + 2 kelas DTO + 2 field `decision` (entity & DTO) + konversi DTO→domain +
komentar "P11 PHASE 2 / TRACK 8" + vocabulary `allowed_actions` yang menyesatkan
di rumah kanonik.

### 6. Commands dan hasil
- `flutter analyze lib/domains/finance/transaction/payment` → **No issues found**.
- `flutter analyze lib test/domains/finance/transaction/payment test/domains/commerce/transaction/order`
  → 33 issue, **0 error** (sisa = lint test pre-existing, mis.
  `unnecessary_underscores`).
- `flutter test` (seluruh dir payment + `dynamic_action_buttons_test` +
  `order_contract_p1_test`) → **61 lulus / 3 gagal**.
- `flutter test` (gate baru + `dynamic_action_buttons` + `order_contract_p1` +
  `order_list_screen`) → **30/30 PASS**.
- Residue sweep: `DecisionContractResponseDto|DisplayHintsDto|allowed_actions`
  di `lib/`+`test/` = **nol kode hidup** (sisa hanya komentar akurat di
  `auction.dart:38` dan teks gate sendiri); `decision` di domain payment = nol
  (satu-satunya hit = kata Inggris di komentar UX `payment_webview_screen.dart`).

### 7. Hasil proof
`payment_decision_phantom_purge_test.dart` mengunci empat hal: (a) rantai
phantom tetap mati (source ratchet 3 kata kunci di 2 file payment); (b) decision
contract punya **tepat satu rumah** di `lib/`; (c) rumah kanonik masih membawa
key wire backend (`secondary_actions`, `decision_version`,
`time_remaining_seconds`); (d) wire kanonik dari
`test/fixtures/payment_wire_contract.json` tetap ter-parse **tanpa** `decision`
(status → `PaymentStatus.pending`) dan key `decision` asing **diabaikan**, bukan
diparse. Assertion-nya kesetaraan eksak (bukan substring longgar), jadi tidak
bisa lolos senyap.

### 8. Temuan di luar scope + PARKIRAN BARU
- **P1 — scope berikutnya (belum dikerjakan): konvergensi result + failure.**
  Manifest: `order/domain/repositories/repository_result.dart` DIHAPUS, diikuti
  ~13 file lib (`order_repository`/`_impl`, `refund_repository`/`_impl`,
  `auction_repository`/`_impl`, `bidding_repository`/`_impl`,
  `auction_remote_datasource`, `seller_repository`/`_impl`, `auction_detail_screen`)
  + ~20 file test (≈350 call site, mis. `seller_repository_impl` 35,
  `auction_repository_impl` 34, `seller_auctions_screen_test` 32).
  `payment/domain/repositories/payment_repository.dart` (deklarasi kedua) juga
  mati → menyeret `payment_repository_impl`, `payment_providers`, `data.dart`,
  `domain/domain.dart`, 2 test.
  Jebakan wajib per call site: (a) **urutan argumen `fold`**, (b) `isSuccess`
  semantik (`data != null` → flag; banyak call site menulis
  `isSuccess && data != null`, aman, tapi yang hanya `isSuccess` harus dibaca
  ulang), (c) `statusCode` adalah penambahan (aman), (d) `StructuredApiException`
  lahir khusus untuk memasok `RepositoryResult.error(code:)` di domain auction →
  audit ulang perannya SETELAH migrasi, jangan dihapus bersamaan.
- **P2 — blok "BACKWARD COMPATIBILITY … New code should use decision contract"**
  di `order.dart` (sekitar `isSellerActionRequired`): getter masih hidup dan
  dipakai; butuh audit apakah ia masih authority UI seller-action atau harus
  membaca decision contract. Tidak disentuh.
- **Pre-existing terverifikasi**: 3 gagal di `payment_result_notifier_test` — ⚠️ **KOREKSI 2026-09-28 malam-2: label "authority `order.status` vs payment resource" SALAH. Akar sebenarnya fixture time-bomb `expiredAt: 2026-08-02` yang sudah lewat; lihat bagian terakhir dokumen ini.**
  (authority `order.status` vs payment resource; baris 206/239/533). Bukti bukan
  regresi purge: file test **tidak tersentuh** (tidak muncul di `git status`),
  `grep -c decision` di file itu = **0**, dan nol pembaca `Payment.decision` di
  luar dua file yang saya edit → purge tidak punya permukaan perilaku. Ledger
  sesi sebelumnya juga sudah mencatat file ini merah.

### 9. Risiko / belum terbukti
- Konvergensi `RepositoryResult` **BELUM**: dua tipe result masih hidup
  berdampingan — kondisi yang saya temukan, bukan saya ciptakan, dan tidak boleh
  ditutup sebagai PASS.
- Taksonomi failure (core vs payment, tiga nama kembar) menunggu keputusan desain
  yang menyertainya.

### 10. Git status
Belum di-commit. Milik sesi ini: `M` 3 file lib + `A` 1 test. Sisa working tree
masih multi-agen — jangan di-stage borongan.

### 11. Owner retest
Belum perlu: purge menyentuh field yang **tidak pernah terisi** (backend tak
pernah mengirim `decision`) dan **tidak pernah dibaca**, jadi nol perubahan
perilaku — dibuktikan kompilasi bersih + 30 test hijau.

## Sesi 2026-09-28 (lanjutan 2) — Purge alias/backward-compat di fondasi result & status (§11)

### 1. Verdict
SELESAI & terverifikasi, bounded: alias dan jalur compat di `Result<T>` dan
`PaymentStatus` dibunuh; parser status payment kini **menolak** kosakata gateway
alih-alih mengoersinya diam-diam. Migrasi `RepositoryResult` (489 referensi / 48
file) TETAP parkir sebagai scope tersendiri dengan rencana slice di bawah.

### 2. Root cause
Alias hidup sebagai jalur kompatibilitas tanpa pemilik:
- `Result.isFailure` — komentarnya sendiri: "Alias for isError for backward
  compatibility".
- `PaymentStatus.isFinal` / `isSuccess` / `isSuccessful` — tiga nama untuk satu
  arti, **nol konsumen** (analyzer membuktikannya: menghapus ketiganya tidak
  memunculkan satu error pun).
- `PaymentStatus.fromString` memetakan 9 nilai kosakata gateway Midtrans
  (`settlement`, `capture`, `completed`, `challenge`, `deny`, `cancel`,
  `cancelled`, `process`, `expire`) yang **tidak pernah ada di wire**: backend
  memakai nama enum kanonik dan `midtrans_status` adalah forbidden response key.
  Bukti tambahan dari audit backend: `grep settlement|capture` di
  `backend/internal/commerce/order/` hanya menemukan **kata domain** (lifecycle
  settlement auction), bukan nilai `payments.status`. Coercion diam-diam ini
  berbahaya di jalur uang.

### 3. Canonical behavior
- Satu nama per arti: `isError` (bukan `isFailure`), `isTerminal` (bukan
  `isFinal`).
- `PaymentStatus.fromString`: cocokkan **nama enum** (trim + case-insensitive);
  nilai lain → `FormatException` (gagal keras, bukan diam-diam jadi `pending`).

### 4. File yang berubah
- `M` `lib/core/common/result.dart` — getter alias `isFailure` dihapus.
- `M` `lib/core/common/types/payment_types.dart` — 3 getter alias dihapus +
  `fromString` ditulis ulang.
- `M` `lib/domains/chat/chat/presentation/providers/chat_notifier.dart:605`,
  `M` `lib/domains/system/analytics/data/repositories/firebase_analytics_repository_impl.dart:35,100`,
  `M` `test/domains/social/follow/follow_identity_contract_test.dart:420,511`
  — `isFailure` → `isError` (blast radius dilisting analyzer: 5 situs).
- `M` `test/core/common/types/payment_types_test.dart` — pin legacy diganti
  negative contract.

### 5. Residue yang dihapus
Getter alias `Result.isFailure`; tiga getter alias `PaymentStatus`; 9 cabang
kosakata gateway + doc "legacy support"; 4 pin test yang menopang mapping
tertolak (test bukan authority).

### 6. Commands dan hasil
- `flutter analyze lib test` → **0 error** (5 perbaikan call site; analyzer yang
  melisting blast radius-nya).
- `flutter test test/core/common/types/payment_types_test.dart
  test/domains/finance/transaction/payment
  test/domains/social/follow/follow_identity_contract_test.dart` →
  **65 lulus / 3 gagal**, ketiganya `payment_result_notifier_test` pre-existing
  (file tidak tersentuh).

### 7. Hasil proof
- Analyzer membuktikan tiga getter payment **nol konsumen**.
- `isFailure` → `isError` semantiknya identik (`!_isSuccess`) → 5 perbaikan itu
  murni rename, nol perubahan perilaku.
- Negative contract baru: seluruh nama enum ter-parse (case-insensitive + trim),
  9 nilai kosakata gateway WAJIB ditolak.

### 8. INVENTARIS KERUMITAN (bahan desain ulang — diminta bagian “Complexity / Simplification” cara-kerja.md)
Semua **dicatat, belum dikerjakan**:
1. **Empat+ authority untuk satu konsep “hasil operasi”**: `Result<T>` (core,
   ~60 file), `RepositoryResult<T>` (order — hidup di folder domain tapi dipakai
   lintas domain: auction, seller, content, refund; **489 referensi / 48 file**),
   `RepositoryResult<T>` kedua (payment, payload `PaymentFailure`),
   `Either<Failure,T>` (dartz, dipakai follow). Ditambah `Withdrawal` yang punya
   `isSuccess`/`error` sendiri, dan `PaymentResult`/`PaymentResultStatus` untuk
   state layar hasil payment.
2. **Alias yang masih hidup** (butuh audit konsumen sebelum dipotong): factory
   `RepositoryResult.error(...)` (alias `failure`), pasangan
   `isError`/`isFailure` di tipe itu, `Withdrawal.isSuccessful =>`,
   `finance_gateway.isSuccessful => isSuccess`, `SupportFailure.isFailure`.
3. **Compat yang masih hidup**: blok “BACKWARD COMPATIBILITY” + getter
   `isSellerActionRequired` di `order.dart`; `AppFormatters`/`CurrencyUtils`
   sebagai mesin uang kedua (parkiran sesi sebelumnya).
4. **Layer berlebih**: `StructuredApiException` ada hanya untuk membawa error
   code menembus `fold→throw→catch` di auction — audit ulang perannya SETELAH
   `RepositoryResult` mati; jangan dihapus bersamaan.
5. **Pelanggaran boundary**: domain `social/content` mengimpor tipe generik milik
   folder `commerce/transaction/order` — tipe generik seharusnya milik fondasi
   (`core`), bukan milik satu domain.

### 9. RENCANA SLICE migrasi `RepositoryResult` → `Result<T>` (belum dikerjakan)
Urutan aman (pindahkan konsumen dulu, hapus deklarasi terakhir, supaya tree
TETAP kompilasi di setiap langkah — pola yang sama dengan migrasi envelope):
1. order + refund (domain pemilik): `order_repository{,_impl}`,
   `refund_repository{,_impl}` + test order (≈70 referensi).
2. auction: `auction_repository{,_impl}`, `bidding_repository{,_impl}`,
   `auction_remote_datasource`, `auction_detail_screen` + test auction (≈80).
3. seller: `seller_repository{,_impl}` + test sertifikasi seller (≈47).
4. content: `content_repository{,_impl}`, `content_notifier` + test content (≈59).
5. payment: `payment_repository{,_impl}`, `payment_providers`, `data.dart`,
   `domain/domain.dart` + test payment (≈30).
6. BARU setelah slice 5: hapus `order/domain/repositories/repository_result.dart`
   + export di `order/domain/domain.dart` + gate negative-proof (tipe & nama
   factory tidak boleh kembali).
Jebakan wajib per call site: (a) **urutan argumen `fold`** (RepositoryResult
sukses-dulu, Result error-dulu), (b) `isSuccess` semantik (`data != null` vs
flag; site yang menulis `isSuccess && data != null` aman), (c) `statusCode`
penambahan aman, (d) `RepositoryResult.failure(...)` → `Result.error(...)`.

### 10. Git status
Belum di-commit. Milik sesi ini (lanjutan 2): `M` 3 file lib + `M` 3 file test.
Sisa working tree multi-agen — jangan di-stage borongan.

### 11. Owner retest
Belum perlu: nol perubahan visual; satu-satunya perubahan perilaku yang disengaja
= status payment tak dikenal/gateway kini **gagal keras**. Ini keputusan sadar
(catatan: alternatifnya adalah fallback `pending` — dan itu kebohongan di jalur
uang). Bila owner ingin unknown status dirender sebagai state error yang ramah,
itu task UI tersendiri.

## Sesi 2026-09-28 (lanjutan 3) — Migrasi `RepositoryResult` → `Result<T>` + bunuh 2 duplikat (§11)

### 1. Verdict
SELESAI untuk 5 dari 6 slice, dihentikan sengaja di slice payment karena temuan
berubah bentuk: **dua** deklarasi duplikat dibunuh total
(`order/.../repository_result.dart` dan `ContentRepositoryResult` — deklarasi
**ketiga** yang tidak tercatat di inventaris sebelumnya), seluruh konsumennya
pindah ke `Result<T>`, dan file/kelas duplikatnya DIHAPUS. Sisa satu salinan
(payment) DIPARKIR sebagai keputusan owner karena payload-nya bukan string error
melainkan `PaymentFailure` bertipe — konvergensinya bukan rename.
Gate negative-proof baru: `test/core/result_authority_contract_test.dart`.

### 2. Root cause (inventaris sebelumnya meleset)
Catatan lanjutan 2 menyebut "empat+ authority"; audit lanjut menemukan bahwa
duplikatnya berjumlah **3 deklarasi untuk 2 nama**:
- `RepositoryResult<T>` — `order/domain/repositories/repository_result.dart`.
  `isSuccess => data != null` (sukses-null terbaca gagal), `fold(onSuccess,
  onError)` sukses-dulu, punya `failure` DAN `error` (alias).
- `ContentRepositoryResult<T>` — **deklarasi ketiga, tidak tercatat**:
  dideklarasikan di dalam `content/domain/repositories/content_repository.dart`
  (file interface!), lengkap dengan `dataOrThrow` sendiri. `fold`-nya kebetulan
  sudah error-dulu — jadi tiga tipe ini memakai **dua aturan urutan argumen
  berbeda** untuk operasi yang sama.
- `RepositoryResult<T>` (payment) — payload `PaymentFailure?` bertipe, `fold`
  menerima objek failure, `dataOrThrow` melempar `PaymentFailure`.
Akibatnya: `fold` berarti "sukses dulu" atau "error dulu" tergantung tipe mana
yang kebetulan dipakai, dan call site tidak bisa tahu. Dua dari tiga tipe juga
salah membaca sukses-yang-membawa-null.

### 3. Canonical behavior
- SATU tipe: `Result<T>` (`lib/core/common/result.dart`), `fold(onError,
  onSuccess)` — **error dulu**, seragam di seluruh app.
- `isSuccess`/`isError` = flag internal (bukan turunan `data != null`).
- `Result.error(msg, {code, statusCode, details})` = satu-satunya nama;
  `RepositoryResult.failure` (alias identik) tidak hidup lagi.

### 4. File yang berubah (milik sesi ini)
**Slice 1 — order + refund + use case**
- `M` `order/domain/repositories/order_repository.dart`,
  `order/domain/repositories/refund_repository.dart`,
  `order/data/order_repository_impl.dart`,
  `order/data/refund_repository_impl.dart` (import + rename + fold dibalik;
  `RepositoryResult.failure(` → `Result.error(`).
- `M` `order/presentation/providers/order_notifier.dart` — 7 `fold` dibalik
  urutannya (14 error analyzer sebelumnya).
- `M` `transaction/usecases/get_order_status_usecase.dart` — 2 `fold` dibalik.
- `M` `test/.../order/order_refund_history_controller_test.dart` (fake 9 method),
  `M` `test/.../order/recent_seller_orders_provider_test.dart`,
  `M` `test/domains/finance/transaction/payment/presentation/providers/
  payment_result_notifier_test.dart` (bagian fake order saja; bagian payment
  tetap memakai tipe parkir).

**Slice 2 — auction (blast radius lebih luas dari perkiraan)**
- `M` `auction/domain/repositories/auction_repository.dart`,
  `auction/domain/repositories/bidding_repository.dart`,
  `auction/data/remote/auction_remote_datasource.dart`,
  `auction/data/repositories/auction_repository_impl.dart`,
  `auction/data/repositories/bidding_repository_impl.dart`,
  `auction/presentation/screens/auction_detail_screen.dart` (komentar).
- `M` `auction/presentation/providers/auction_notifier.dart` — **12 `fold`**
  dibalik (termasuk `placeBid`/`updateAuction`/`cancelAuction`, closure error
  dipindah ke depan) + `bidding_notifier.dart` (1).
- `M` `auction/usecases/get_auction_usecase.dart` (3),
  `M` `auction/usecases/place_auction_bid_usecase.dart` (1).
- Test: 3 file auction + 8 file `test/features/home/**` + marketplace injection
  + `guest_welcome_home_router_test` (fake `_FakeAuctionRepository`).

**Slice 3 — seller**
- `M` `seller/domain/repositories/seller_repository.dart`,
  `M` `seller/data/repositories/seller_repository_impl.dart`,
  `M` `seller/presentation/providers/withdraw_notifier.dart` (1 `fold` dibalik).
- Test: `withdraw_history_pagination_test`,
  `seller_dashboard_earnings_exposure_test` (14 fake method),
  `seller_renewal_screen_test` (`RepositoryResult<X>.failure` →
  `Result<X>.error`), `seller_upgrade_wizard_screen_identity_test`.

**Slice 4 — content (mencakup deklarasi ketiga)**
- `M` `content/domain/repositories/content_repository.dart` — **kelas
  `ContentRepositoryResult<T>` + blok "Result Types" DIHAPUS**, import `Result`.
- `M` `content/data/content_repository_impl.dart` (40 referensi),
  `M` `content/presentation/providers/content_notifier.dart` (3).
- Test: `router_lifetime_preservation_test` (20),
  `content_media_failure_semantics_test` (20),
  `create_request_submission_contract_test` (20),
  `guest_welcome_home_router_test` (6), `feed_root_wiring_test` (6),
  `follow_status_provider_lifecycle_test` (2),
  `profile_header_identity_canonical_test` (2),
  `update_content_projection_contract_test` (`dataOrThrow` →
  `expect(isSuccess)` + `data!`).

**Slice 6 — purge + gate**
- `D` `order/domain/repositories/repository_result.dart` (file duplikat).
- `M` `order/domain/domain.dart` (export `show RepositoryResult` dibuang).
- `M` `core/api/structured_api_exception.dart` (komentar menunjuk tipe mati).
- `M` `test/core/api/commerce_restriction_propagation_test.dart` — grup test
  `RepositoryResult<T> — errorCode preservation` DIHAPUS: duplikat persis dari
  grup `Result<T>` tepat di atasnya.
- `M` `test/.../order/order_contract_p1_test.dart` (nama test).
- BARU `test/core/result_authority_contract_test.dart`.

### 5. Residue yang dihapus
Dua file/kelas duplikat (`repository_result.dart` utuh + `ContentRepositoryResult`
di dalam file interface content), alias `RepositoryResult.failure` (mati bersama
tipenya), `dataOrThrow` milik content, export barrel `show RepositoryResult`, satu
grup test duplikat yang menopang tipe mati, dan 4 komentar basi yang menunjuk
tipe mati.

### 6. Commands dan hasil
- `flutter analyze lib test` → **0 error** setelah tiap slice (dicek 7 kali,
  terakhir di akhir sesi: 149 issues, 0 error — jumlah issue turun dari 180
  karena kerja agen lain, bukan karena sesi ini menambah/mengurangi lint).
- Slice 1: `flutter test test/domains/commerce/transaction/order/
  test/.../payment_result_notifier_test.dart` → **139 lulus / 3 gagal**;
  3 gagal = pre-existing terverifikasi (authority `order.status` vs payment
  resource).
- Slice 2: `flutter test test/domains/commerce/catalog/auction/` → **137 PASS**.
- Slice 3: `flutter test test/domains/user/preference/seller/
  test/domains/finance/withdrawal/` → **123 PASS**.
- Slice 4: `flutter test test/domains/social/content/
  test/domains/social/follow/follow_status_provider_lifecycle_test.dart
  test/domains/user/profile/profile_header_identity_canonical_test.dart` →
  **59 PASS**.
- Slice 6: `flutter test test/core/result_authority_contract_test.dart
  test/core/file_authority_contract_test.dart
  test/core/theme/theme_authority_contract_test.dart
  test/core/api/commerce_restriction_propagation_test.dart` → **21 PASS**.
- **Negative proof gate (dijalankan, bukan diklaim):** dibuat probe
  `lib/.../payment/domain/repositories/__temp_dup_probe.dart` berisi
  `class ContentRepositoryResult<T>` → gate **GAGAL 2 test** (deklarasi kedua +
  nama duplikat muncul); probe dihapus → gate **PASS**.

### 7. Hasil proof
- `grep -rn '\bRepositoryResult\b' lib test` → hanya 2 file payment + 1 test
  payment (yang memang diparkir). `ContentRepositoryResult` di luar gate →
  **nol** (7 kemunculannya hanya di dalam gate itu sendiri, dan gate mengecualikan
  dirinya dari sapuannya — kalau tidak, ia akan mendeteksi deskripsinya sendiri).
- `grep -rn "repositories/repository_result.dart"` → hanya 1, yaitu entri
  `_killedFiles` di dalam gate; nol import/export nyata.
- `lib/.../repository_result.dart` → **file tidak ada**.
- Urutan `fold` tiap call site kini satu arah; pergeserannya diverifikasi
  analyzer (0 error), bukan dibaca manual.

### 8. Temuan di luar scope (dicatat, TIDAK dikerjakan)
1. **10 gagal pre-existing di
   `test/features/home/presentation/providers/feed_promoted_click_destination_authority_test.dart`**
   — dibuktikan pre-existing, bukan regresi: helper `_tapPromotedCard` mencari
   `CommerceMarketplaceCardShell` / `PromotedExternalCard`, sedangkan
   `PromotedForSaleCard`/`PromotedAuctionCard` (`feed_renderers.dart`, **tidak
   tersentuh siapa pun**) merender `Card` + `InkWell` polos → helper tidak
   menemukan apa pun → tidak ada tap → semua assert navigasi/click-ack gagal.
   Diff sesi ini di file itu hanya **3 baris** (hapus import, 2 rename) dan tidak
   menyentuh helper. Klasifikasi **P2** (test tidak menguji yang diklaimnya);
   perbaikan = align helper ke kontrak widget kanonik — scope tersendiri.
   (Catatan: `feed_promoted_impression_authority_test` + marketplace injection
   LOLOS; seluruh 10 gagal berasal dari satu file ini.)
2. **`payment/.../payment_repository.dart:12` `RepositoryResult<T>`** —
   diparkir (lihat §9).
3. Satu momen `flutter test test/core/theme/theme_authority_contract_test.dart`
   melaporkan "Does not exist" padahal file ada (transien lingkungan saat banyak
   file test dijalankan sekaligus); dijalankan ulang → hijau.

### 9. Risiko / belum / parkiran baru (butuh keputusan owner)
- **Payment `RepositoryResult<T>` TIDAK bisa direname.** Payload-nya
  `PaymentFailure?` bertipe (`ValidationFailure`, `UnknownFailure`, …), `fold`
  menerima objek failure, dan `dataOrThrow` **melempar** `PaymentFailure`.
  Konvergensinya berarti mengubah jalur kegagalan payment dari *tipe* menjadi
  *string + code* (`Result.error`) — mengubah cara UI payment membedakan jenis
  gagal. Keputusan desain, bukan mekanik. Dua arah: (a) payment ikut `Result<T>`
  dengan `code` + `errorDetails` sebagai channel baru dan `PaymentFailure`
  menjadi konstruktor pesan di tepi; (b) `PaymentFailure` dipertahankan sebagai
  exception bertipe yang dilempar dari repository, `Result` untuk sisanya.
  Gate `result_authority_contract_test.dart` menuliskan allowlist-nya eksplisit
  supaya keputusan ini tidak menguap.
- Inventaris lanjutan-2 item #4 (`StructuredApiException` — perannya perlu
  audit ulang SETELAH tipe mati; jangan dihapus bersamaan) dan sisa item #1
  (`Either<Failure,T>` dartz di follow, `Withdrawal.isSuccess`,
  `PaymentResult`/`PaymentResultStatus`) tetap terbuka — belum disentuh, sesuai
  "satu scope aktif".
- Full-suite `flutter test` (gate rilis) dijalankan di sesi ini; hasilnya di §12.

### 10. Git status
Belum di-commit. Milik sesi ini (lanjutan 3): ~30 file `lib` (termasuk 1 `D`) +
~28 file `test` (1 `D` = `repository_result.dart`, 1 BARU = gate). Working tree
tetap multi-agen — **jangan `git add -A`**.

### 11. Owner retest
Belum perlu. Nol perubahan visual. Perubahan perilaku yang disengaja hanya satu,
dan hasil observabelnya tetap sama: pada jalur refund, `getRefundByOrderId` yang
dulu "sukses-tanpa-refund" terbaca gagal (karena `isSuccess => data != null`)
kini benar-benar sukses-dengan-null — layar tetap menampilkan `null` di dua
jalur, tetapi alasannya kini jujur, bukan kebetulan.

### 12. Full-suite (gate rilis) — dijalankan, 28 menit
`flutter test --reporter compact` (seluruh `test/`, default concurrency) →
**+2552 lulus / ~1 skip / -72 gagal, 28:00**.

Klasifikasi 72 gagal (semuanya pre-existing, **nol** terkait sesi ini):
- `grep -c "RepositoryResult|ContentRepositoryResult|Result.fold"` di seluruh
  log → **0**. Nol kegagalan menyebut identifier yang sesi ini ubah.
- Keempat gate sesi ini LOLOS di run penuh (tidak muncul di daftar `[E]`):
  `result_authority_contract_test`, `file_authority_contract_test`,
  `theme_authority_contract_test`, `commerce_restriction_propagation_test`.
  Begitu juga `payment_decision_phantom_purge_test`, `payment_types_test`,
  `order_status_wire_alignment_test`, `refund_action_api_test`,
  `order_refund_history_controller_test`, `recent_seller_orders_provider_test`,
  `seller_auctions_screen_test`, `auction_notifier_authority_test`,
  `auction_repository_polling_retention_test`, `withdraw_history_pagination_test`,
  `seller_dashboard_earnings_exposure_test`, `seller_renewal_screen_test`,
  `seller_upgrade_wizard_screen_identity_test`, `content_media_failure_semantics_test`,
  `create_request_submission_contract_test`, `update_content_projection_contract_test`,
  `follow_status_provider_lifecycle_test`, `profile_header_identity_canonical_test`,
  `feed_promoted_impression_authority_test`, `marketplace_promotion_injection_test`,
  `guest_welcome_home_router_test`, `order_contract_p1_test`.
- Akar 72 gagal (dari teks exception, bukan tebakan):
  `UnimplementedError: ApiClient must be provided externally` (22),
  `Bad state: No element` (10), `ProviderException` (10),
  finder `Found 0 widgets ... ("hello" / "Feed belum bisa dimuat" / SwitchListTile
  "Show Online Status")` (19+). Semuanya pola harness/provider-scope yang sama
  seperti yang sudah tercatat di sesi-sesi sebelumnya ("test/features/home ...
  penyebab sama (ApiClient/harness feed)"; "/settings toggle" ada di daftar
  test-debt lama di §Parkiran A.6).
- 13 dari 72 = `home_screen_feed_rendering_test`, 2 =
  `home_screen_promoted_card_rendering_test` SCENARIO 4 (dua-duanya sudah
  dinyatakan pre-existing + pernah dibuktikan dengan stash oleh sesi
  sebelumnya), 10 = keluarga `feed_promoted_click_destination_authority_test`
  (lihat §8), 3 = `payment_result_notifier_test` (lihat §6), 1 =
  `router_lifetime_preservation_test` `/settings preserves local toggles`
  (exception-nya soal `SwitchListTile 'Show Online Status'` = debt `/settings
  toggle`, bukan tipe result), sisanya keluarga c1b3_mention/chat CTA/saved_item/
  create_auction route — file-file yang sesi ini tidak sentuh.
- Catatan: `home_screen_feed_rendering_test` butuh ~19 menit sendirian
  (`pumpAndSettle` per test) — itu sebabnya run penuh memakan 28 menit.

## Sesi 2026-09-28 (lanjutan 4) — Konvergensi result payment + hapus vocabulary gagal (§11)

### 1. Verdict
SELESAI. Duplikat result yang terakhir MATI: `RepositoryResult<T>` milik payment
+ `PaymentFailure` beserta 7 subclass-nya dihapus; repository kini meneruskan
`code`/`statusCode`/`details` dari backend apa adanya, dan UI payment bercabang
pada `errorCode`. **Allowlist parkir di gate `result_authority_contract_test.dart
DIKOSONGKAN** — sekarang tidak ada satu pun pengecualian: nol duplikat result di
seluruh `lib/` & `test/`.

### 2. Audit (bukti yang memutuskan bentuk konvergensi)
1. **Payload bertipe tidak pernah dibaca siapa pun.** `ValidationFailure.field`,
   `PaymentExpiredFailure.expiredAt`, `PaymentNotFoundFailure.paymentId`,
   `UnknownFailure.originalError` — nol konsumen.
2. **2 dari 7 subclass mati sejak lahir**: `PaymentGatewayFailure`,
   `InsufficientBalanceFailure` — tidak pernah dikonstruksi, tidak pernah
   didiskriminasi.
3. **Data palsu**: `_mapApiError` membangun `PaymentNotFoundFailure('payment')`
   (string literal `'payment'` sebagai paymentId) dan
   `PaymentExpiredFailure(DateTime.now())`.
4. **Fabricator klasifikasi**: `_mapApiError` menebak jenis dengan *mencocokkan
   teks pesan* (`errorStr.contains('network'/'not found'/'expired'/'invalid')`)
   lalu **membuang** code backend yang sudah dibawa `Result.errorCode` dari
   lapisan API. Anti-pattern yang doctrine sebut eksplisit.
5. **Satu-satunya diskriminasi bertipe** ada di
   `payment_initiation_notifier._getUserFriendlyErrorMessage` — 4 cabang
   `is NetworkFailure` / `is PaymentNotFoundFailure` / `is PaymentExpiredFailure`
   / `is ValidationFailure`, semuanya hanya untuk memilih pesan, dan hanya bisa
   menyala secara kebetulan (karena berasal dari tebakan #4).
6. **Code backend yang benar-benar ada untuk payment** (dari
   `platform/response/error_mapper.go`): `INVALID_PAYMENT_STATUS` (409) dan
   `REFERENCE_REQUIRED` (400). Tidak ada padanan untuk 4 jenis buatan klien.
7. **Blast radius kecil**: 4 call site di `lib` + 2 konsumen lintas domain yang
   baru terlihat dari analyzer (`checkout_screen_logic`,
   `order_detail_handlers`) + 2 test.

### 3. Canonical behavior
- SATU tipe hasil: `Result<T>`. `PaymentRepository` mengembalikan
  `Result<PaymentIntent>` / `Result<Payment>` / `Result<List<PaymentMethodOption>>`.
- **Repository meneruskan, tidak menafsirkan**: kegagalan API diteruskan apa
  adanya (`code: source.errorCode`, `statusCode: source.statusCode`,
  `details: source.errorDetails`) lewat satu helper `_forwardFailure<T>`.
- **Precondition lokal** (paymentId/orderId kosong, `request.validate()` gagal)
  → `Result.error(pesan)` **tanpa code**, karena tidak ada kebenaran backend
  yang boleh diklaim.
- `dataOrThrow`, `isFailure`, `fold(onSuccess, onFailure)` payment: MATI.
- Copy UI berbasis **code kanonik** (2 konstanta baru di
  `core/api/api_error_codes.dart`: `invalidPaymentStatus`, `referenceRequired`),
  sisanya pesan dari authority.

### 4. File yang berubah (milik sesi ini)
- `M` `payment/domain/repositories/payment_repository.dart` — kelas
  `RepositoryResult<T>` (50 baris) DIHAPUS; import `result.dart`; 3 signature
  jadi `Result<...>`.
- `M` `payment/data/repositories/payment_repository_impl.dart` — `_mapApiError`
  (fabricator) DIGANTI `_forwardFailure<T>`; `RepositoryResult.failure(...)` →
  `Result.error(...)`; 3 `RepositoryResult.success(...)` → `Result.success(...)`.
- `M` `payment/presentation/providers/payment_initiation_notifier.dart` —
  1 `fold` dibalik; `_getUserFriendlyErrorMessage(PaymentFailure)` →
  `_getUserFriendlyErrorMessage(String? code, String message)`; import
  `payment_failure.dart as payment_failures` dibuang.
- `M` `payment/presentation/providers/payment_notifier.dart` — 2 `fold` dibalik
  (`failure.message` → `error`).
- `M` `payment/presentation/providers/payment_result_notifier.dart` —
  `paymentResult.isFailure` → `isError`, `${paymentResult.failure}` → `.error`.
- `M` `payment/domain/domain.dart` — `export 'failures/payment_failure.dart'`
  dibuang.
- `D` `payment/domain/failures/payment_failure.dart` (+ direktori `failures/`).
- `M` `core/api/api_error_codes.dart` — +`invalidPaymentStatus`,
  +`referenceRequired` (authority code mobile, sesuai doc file itu sendiri).
- `M` `checkout/presentation/screens/checkout_screen_logic.dart` +
  `order/presentation/screens/order_detail/order_detail_handlers.dart` —
  `fold<List<PaymentMethodOption>>` dibalik (2 konsumen lintas domain).
- `M` `test/.../payment_result_notifier_test.dart` —
  `payment_repo.RepositoryResult` → `Result`, `.failure(` → `.error(`, import
  `payment_failure.dart` dibuang.
- `M` `test/.../payment_wire_contract_test.dart` — `result.failure?.message` →
  `result.error` (3 situs).
- `M` `test/core/result_authority_contract_test.dart` — allowlist `_parked`
  DIHAPUS; `homes['RepositoryResult']` wajib `isNull`; `_killedFiles` +=
  `payment_failure.dart`; **ratchet baru** "payment forwards the backend code
  instead of inventing a failure kind" (source scan: `\bPaymentFailure\b` &
  `_mapApiError` terlarang di `lib/domains/finance/transaction/payment`; PLUS
  positive proof `source.errorCode` & `source.statusCode` wajib ada).

### 5. Residue yang dihapus
Kelas `RepositoryResult<T>` payment; `PaymentFailure` + 7 subclass
(`NetworkFailure`, `ValidationFailure`, `PaymentGatewayFailure`,
`InsufficientBalanceFailure`, `PaymentExpiredFailure`, `PaymentNotFoundFailure`,
`UnknownFailure`); method `dataOrThrow`; getter `isFailure`; `_mapApiError`
(tebakan berbasis teks pesan); 4 cabang `is ...Failure` di UI; import + export
`payment_failure.dart`; 1 direktori `failures/`.

### 6. Commands dan hasil
- `flutter analyze lib test` → **0 error** (149 issues non-error, sama seperti
  sebelum sesi ini — tidak ada lint baru).
- `flutter test test/domains/finance/transaction/payment/` → **39 lulus / 3
  gagal**; 3 gagal = pre-existing terverifikasi (`payment_result_notifier_test`,
  authority `order.status` vs payment resource — bukan soal tipe result).
- `flutter test test/domains/finance/` → **98 lulus / 3 gagal** (tiga yang sama).
- `flutter test test/domains/commerce/transaction/checkout/
  test/domains/commerce/transaction/order/
  test/core/result_authority_contract_test.dart
  test/core/file_authority_contract_test.dart` → **187 lulus / 1 skip, semua
  lulus**.
- Gate `result_authority_contract_test.dart` → **6/6 PASS**.

### 7. Hasil proof
- **Negative proof ratchet payment DIJALANKAN**: file probe
  `lib/domains/finance/transaction/payment/domain/failures/payment_failure.dart`
  berisi `abstract class PaymentFailure { ... }` ditanam kembali → gate GAGAL
  2 test ("killed authorities stay deleted" + "payment forwards the backend
  code"); probe dihapus → 6/6 PASS.
- `grep -rn '\bRepositoryResult\b' lib test` → hanya di dalam gate itu sendiri
  (yang mengecualikan dirinya dari sapuannya). `ContentRepositoryResult` → idem.
- `grep -rn "payment_failure\|PaymentFailure" lib test` → hanya
  `_getPaymentFailureReason(Payment? payment)` (alasan status pembayaran, bukan
  vocabulary gagal yang mati) + gate.
- `dataOrThrow` di payment → nol.
- Analyzer yang menemukan 2 konsumen lintas domain (checkout, order detail) yang
  tidak terlihat dari sapuan direktori payment — pelajaran: dependency yang
  benar-benar mengikat ditemukan compiler, bukan grep.

### 8. KOREKSI CATATAN SESI INI (penting — klaim saya sendiri yang salah)
Saat memutuskan purge `core/errors/failure.dart`, saya melaporkan ke owner
"**nol importer**" berdasarkan `grep "errors/failure.dart"` yang hanya menemukan
`core/core.dart:6` (satu-satunya import *langsung*). Itu **salah**: `Failure`
dipakai lewat barrel `core.dart` sebagai tipe `Left` di `Either<Failure, T>` oleh
**6 use case follow** (`follow_user_use_case`, `get_followers_use_case`,
`get_following_use_case`, `get_follow_stats_use_case`, `search_users_use_case`,
`unfollow_user_use_case`). Grep importer langsung = **false negative**.
Konsekuensi: **purge `core/errors/failure.dart` DIBATALKAN** (tidak dieksekusi) —
owner menyetujuinya atas dasar data saya yang cacat, jadi saya tidak
melaksanakannya, dan keputusannya dikembalikan ke owner dengan data yang benar.
`core/errors/failure.dart` sekarang diklasifikasi: **HIDUP**, milik vocabulary
`Either<Failure,T>` (dartz) di follow — satu-satunya authority "hasil/gagal" yang
masih tersisa di luar `Result`, dan penggabungannya adalah scope tersendiri
(converge follow off dartz) yang akan sekaligus membuka purge file itu.
Tidak ada perubahan file yang dilakukan atas dasar klaim cacat ini.

### 9. Temuan di luar scope (dicatat, TIDAK dikerjakan)
1. **`lib/domains/finance/finance_gateway.dart` = DEAD FILE total.** `abstract
   class FinanceGateway` + `dataOrThrow` sendiri; nol importer, nol export di
   barrel mana pun, nol rujukan di `test/`. Kandidat purge (P2). Ditemukan saat
   menyisir `dataOrThrow`.
2. **Regresi copy kecil, disengaja-dan-dicatat.** Cabang lama
   `is NetworkFailure → 'Koneksi internet bermasalah. Silakan cek koneksi
   Anda.'` MENYALA untuk kegagalan transport (pesan `ErrorInterceptor`
   'Network error. Please check your connection.' mengandung 'network'). Setelah
   konvergensi, kegagalan transport membawa `code == null`, jadi user melihat
   pesan Inggris dari lapisan API. Perbaikan yang benar dan bounded: lapisan API
   memberi code pada kegagalan transport dari **`DioExceptionType`** (enum
   bertipe, bukan cocok-cocokan teks) di `base_api_repository.executeRequest`,
   lalu UI memetakan code itu. Belum dikerjakan — menyentuh SEMUA domain (setiap
   `Result.error` transport akan punya code), jadi ia scope tersendiri, bukan
   ekor dari scope payment.
3. `payment_result_notifier_test` 3 gagal (authority `order.status` vs payment
   resource) tetap pre-existing; bukan soal tipe result.

### 10. Risiko / belum
- Tidak ada perubahan perilaku yang tidak disengaja: satu-satunya perubahan
  copy adalah untuk 2 code backend nyata (kini Indonesia) dan kasus transport di
  §9.2.
- Sisa vocabulary "hasil/gagal" di app: `Either<Failure,T>` (follow, 6 use case),
  `Withdrawal.isSuccess`/`error`, `PaymentResult`/`PaymentResultStatus`,
  `SupportFailure`, `finance_gateway.dart` (mati). Semua belum disentuh.

### 11. Git status
Belum di-commit. Milik sesi ini (lanjutan 4): 9 file `lib` (1 `D` + 1 direktori
dihapus) + 3 file `test`. Working tree tetap multi-agen — **jangan `git add -A`**.

### 12. Owner retest
Perlu, ringan: alur payment gagal yang paling mungkin terlihat beda adalah
(1) `INVALID_PAYMENT_STATUS` (409) — sekarang copy Indonesia yang eksplisit;
(2) kegagalan koneksi saat initiate payment — pesan kini dari lapisan API
(Inggris). Kalau owner ingin copy Indonesia untuk transport, itu §9.2.
Alur sukses tidak tersentuh.

---

## Sesi 2026-09-28 (lanjutan 5) — satu tabel klasifikasi kegagalan transport

**Scope aktif (tunggal):** kegagalan transport — permintaan yang tidak pernah
menghasilkan envelope HTTP — harus punya identitas machine-readable di
`Result.errorCode`, diklasifikasi dari `DioExceptionType` di SATU tempat, dan
tidak ada konsumen hidup yang mencocokkan teks pesan untuk mengenalinya.

### 1. Kondisi awal (terverifikasi, bukan asumsi)

**Koreksi klaim sesi sebelumnya.** Sesi lanjutan 4 menulis "kegagalan transport
membawa `code == null`". Itu **SALAH**, dan §9.2 lanjutan 4 ikut salah. Bukti:
`error_interceptor.dart` cabang `connectionTimeout` mengembalikan
`const TimeoutException(...)` yang default code-nya `'TIMEOUT'`; cabang
`connectionError`/`badCertificate` memberi code eksplisit. Jadi code transport
SUDAH ADA untuk 3 dari 7 cabang. Yang benar-benar rusak berbeda:

1. **Tiga rumah untuk satu identitas.** Literal di `error_interceptor.dart`
   (`'BACKEND_UNREACHABLE'`, `'SSL_ERROR'`) + default kelas di
   `api_exception.dart` (`'TIMEOUT'`, `'CANCELLED'`, `'NETWORK_ERROR'`,
   `'UNKNOWN_ERROR'`). Authority code (`api_error_codes.dart`) tidak tahu satu
   pun dari mereka, padahal file itu MENDEKLARASIKAN dirinya sebagai satu-satunya
   authority identitas code.
2. **Fabrikator di domain.** `checkout_repository_impl.dart` memberi
   `code: 'NETWORK_ERROR'` untuk SEMUA `DioException` yang tidak ter-map —
   termasuk 5xx yang justru envelope HTTP nyata. Nol konsumen membaca code itu
   (`grep "== 'NETWORK_ERROR'"` = 0), jadi itu klasifikasi palsu yang dipajang
   seolah fakta.
3. **Tabel klasifikasi terfragmentasi.** `DioExceptionType` → code hanya ada di
   `_convertToApiException`; jalur `ApiClient.extractException` untuk DioException
   yang tidak sempat dibungkus interceptor MENEBUANG `e.type` dan memberi
   `'UNKNOWN_ERROR'` — informasi klasifikasi hilang tepat di jalur yang paling
   tidak terduga.
4. **Konsumen mencocokkan teks.** Bukti paling telanjang: `error_interceptor.dart`
   sengaja menyimpan kata "network" di pesan `connectionError` KHUSUS supaya
   `_isBackendUnavailableError` (substring) tetap bekerja — komentarnya ada di
   kode — dan `test/core/api/interceptors/error_interceptor_test.dart` mengunci
   perilaku itu sebagai kontrak yang diinginkan. Test auth juga mencetak premis
   itu di namanya: "…falls back to free-text matching, which classifies it as
   backendUnavailable" — lulus karena kata "network", bukan karena code.

Alasan copy Indonesia transport hilang setelah konvergensi payment: bukan
karena `code == null`, tapi karena `_getUserFriendlyErrorMessage` tidak mengenali
code transport mana pun dan jatuh ke pesan lapisan API (Inggris). Perbaikannya
sama; alasannya diperbaiki.

### 2. Authority yang dikunci

- `core/api/api_error_codes.dart` = satu-satunya rumah identitas code. Keluarga
  transport baru: `backendUnreachable`, `requestTimeout`, `networkError`,
  `sslError`, `requestCancelled`, `unknownError`, plus predikat
  `isTransportFailureCode(code)` sebagai satu-satunya authority "ini kegagalan
  transport". `unknownError` SENGAJA di luar predikat: "tidak bisa
  diklasifikasi" bukan klaim yang sama dengan "ini kegagalan transport".
- `ApiExceptionFactory.fromTransport(DioException)` = SATU tabel
  `DioExceptionType` → `ApiException` + code (9 cabang). Mengembalikan `null`
  untuk `badResponse` — itu envelope HTTP, bukan transport — sehingga dua
  keluarga kegagalan itu tidak bisa tertukar oleh siapa pun yang memanggilnya.
- `ApiClient.extractException` (jalur DioException yang tidak lewat interceptor)
  sekarang mengklasifikasi dari `e.type`, bukan menebak `'UNKNOWN_ERROR'`.
- Kelas transport di `api_exception.dart` merujuk konstanta, bukan literal:
  identitas dideklarasikan sekali, dirujuk berkali-kali.

### 3. Perubahan

`lib` (8 file):

1. `core/api/api_error_codes.dart` — +6 konstanta transport + predikat.
2. `core/api/exceptions/api_exception.dart` — import `dart:io`/`dio`; default code
   kelas transport → konstanta; +`ApiExceptionFactory.fromTransport`.
3. `core/api/interceptors/error_interceptor.dart` — switch 45 baris → 2 baris
   (`ApiExceptionFactory.fromTransport(err) ?? _parseErrorResponse(err.response)`),
   `dart:io` dibuang, komentar "pertahankan kata network" DIHAPUS.
4. `core/api/api_client.dart` — `extractException` mengklasifikasi dari type.
5. `core/errors/failure.dart` — `FailureFactory.network` memakai konstanta
   (literal transport terakhir di luar authority).
6. `domains/user/identity/authentication/.../auth_controller.dart` — +cabang
   `isTransportFailureCode(errorCode)` → `backendUnavailable`, sebelum cek 5xx.
7. `domains/commerce/transaction/checkout/data/repositories/
   checkout_repository_impl.dart` — fabrikator `'NETWORK_ERROR'` dibunuh;
   transport memakai code dari lapisan API, non-transport tidak menemukan code
   (null, jujur), `'UNKNOWN_ERROR'` → konstanta.
8. `domains/finance/transaction/payment/.../payment_initiation_notifier.dart` —
   +copy transport Indonesia; temuan §9.2 lanjutan 4 DITUTUP.

`test` (3 file): 1 gate baru + 2 test yang premisnya (substring match) sudah mati:
`error_interceptor_test.dart` (kini membuktikan code kanonik, bukan kata
"network") dan `auth_sync_error_classification_test.dart` (grup transport
struktural + grup terpisah untuk throw mentah non-HTTP).

### 4. Gate & proof

`test/core/api/transport_failure_classification_contract_test.dart` (15 test):

1. Tabel perilaku lewat pipeline NYATA — adapter Dio gagal → `ErrorInterceptor`
   → `ApiClient.extractException` → `BaseApiRepository.executeRequest` →
   `Result.errorCode` — untuk 4 timeout, connectionError, badCertificate,
   cancel, `unknown`+`SocketException`, `unknown` biasa, dan `badResponse` 500.
2. `badResponse` tetap keluarga HTTP: `statusCode == 500`, code dari envelope,
   dan `isTransportFailureCode(code) == false`.
3. Truth table `isTransportFailureCode`: true untuk 5 code transport, false untuk
   `unknownError`, `null`, `''`, dan code backend (`COMMERCE_RESTRICTED`,
   `INVALID_PAYMENT_STATUS`, `MARKET_AUTHORITY_REQUIRED`).
4. `case DioExceptionType.` hanya boleh ada di `api_exception.dart` (sweep
   lib-wide, tanpa allowlist).
5. Literal code transport DILARANG di luar authority (sweep lib-wide, tanpa
   allowlist) + positive proof konstanta masih dideklarasikan.
6. Konsumen hasil konvergensi memakai code/predikat (source scan `fromTransport`
   di 2 ujung pipeline + `isTransportFailureCode` di 2 konsumen).
7. Negative proof detektor.

**Negative proof nyata dijalankan:** probe `lib/core/api/_zz_probe_resurrection.dart`
(switch fork + `code: 'NETWORK_ERROR'`) ditanam → gate GAGAL di 2 test
("the DioExceptionType table lives in exactly one file" +
"no transport code literal exists outside the authority file"); probe dihapus →
15/15 PASS.

### 5. Verifikasi

- `flutter analyze lib test` = **0 error**, 149 issue non-error (= baseline).
- `test/core/api` + gate result/file/theme authority = **144 PASS**.
- `test/domains/user/identity/authentication` + `test/domains/finance` = 319
  total, **3 gagal pre-existing** (`payment_result_notifier_test.dart`: 2 otoritas
  `order.status` + 1 helper) — sama dengan baseline, tidak tersentuh sesi ini.
- `test/domains/commerce/transaction/checkout` + `order` = **178 PASS / 1 skip**.
- `test/core` = 332 total, **4 gagal pre-existing** (3
  `create_auction_route_contract_test` redirect `/seller/upgrade`, 1
  `router_lifetime_preservation_test`) — router, tidak tersentuh.
- `test/domains/user` = 402 total, **4 gagal pre-existing**
  (`saved_item_runtime_authority_test`).
- Grep penutup: literal code transport di `lib` = **hanya** `api_error_codes.dart`;
  `case DioExceptionType.` = **hanya** `api_exception.dart`; `fromTransport`
  dipakai 3 file; `isTransportFailureCode` dipakai 2 konsumen hidup.

### 6. Temuan luar scope (diparkir, tidak dikerjakan)

- **P2 `lib/core/utils/retry_helper.dart` = DEAD FILE TOTAL** (284 baris). Bukti:
  `grep -rn "RetryHelper|RetryConfig|RetryFutureExtension" lib test` hanya
  menemukan file itu sendiri (nol importer). Ia masih menyimpan
  `_isRetryableError` yang mencocokkan teks ('network'/'connection'/'timeout'/
  'socket'/'internet') dan `executeResult` yang membuang `errorCode` sebelum
  memutuskan retry. Karena mati, ini bukan cacat konsumen hidup — purgenya
  masuk scope dead-file bersama `finance_gateway.dart`.
- **P1/P2 `ApiResult` = vocabulary result KEEMPAT & KELIMA, TAK TERLIHAT gate.**
  Tiga deklarasi, dua bentuk yang saling bertabrakan:
  `class ApiResult<T> {data, error, code}` di
  `domains/system/support/data/datasources/support_api_datasource.dart`
  (fold named, **error KEDUA**, `onError(String, String?)`) DAN
  `typedef ApiResult<T> = ({T? data, String? error})` (record, **tanpa kanal
  code sama sekali**) di `features/search/search/domain/repositories/
  search_repository.dart` + `search_history_repository.dart`. Gate
  `result_authority_contract_test.dart` hanya menyapu 3 nama (`Result`,
  `RepositoryResult`, `ContentRepositoryResult`), jadi buta terhadap nama ini.
  Konsekuensi nyata: domain search tidak punya kanal code → kegagalan transport
  di search tidak bisa diklasifikasi. Scope tersendiri.
- **P2 `support_repository_api._mapApiErrorToFailure`** masih mencocokkan teks
  ('network'/'connection'/'timeout') untuk memilih `SupportFailureNetwork`, dan
  `code` yang diterimanya adalah status code sebagai STRING (`'404'`, `'403'`).
  Satu paket dengan penggabungan `ApiResult`/`SupportResult`/`SupportFailure`.
- **P2 `auction_detail_screen._buildErrorScaffold`** mencocokkan teks
  ('network'/'connection'/'not found'/'permission'/'expired'). State notifier-nya
  hanya membawa `String? error`, jadi butuh mengalirkan `errorCode` ke state
  presentasi — scope state, bukan scope klasifikasi API.
- **P2 default code berbasis status HTTP** di `api_exception.dart`
  (`'BAD_REQUEST'`, `'UNAUTHORIZED'`, `'FORBIDDEN'`, …): `_parseErrorResponse`
  selalu memanggil `fromStatusCode(code: code)` dengan `code` nullable, dan
  argumen eksplisit `null` MENIMPA default — jadi default itu tidak pernah
  bertahan di jalur nyata. Fabricated-but-dead; nol konsumen
  (`grep "== 'UNAUTHORIZED'"` dst = 0). Jangan dihapus bersamaan dengan scope ini.
- **P3 `FailureFactory.network/validation/unexpected`** di `core/errors/failure.dart`:
  nol pemanggil (`grep -rn "FailureFactory\."` = 0). `network()` kini merujuk
  konstanta; dua literal lain mati bersama pemanggilnya.
- **P2 `lib/domains/finance/finance_gateway.dart`** masih dead (temuan sesi
  sebelumnya), belum dipurge.

### 7. Git status

Belum di-commit. Milik sesi ini (lanjutan 5): 8 file `lib` + 3 file `test`
(1 baru). Working tree tetap multi-agen — **jangan `git add -A`**.

### 8. Owner retest

Ringan: (1) matikan backend lalu lakukan initiate payment → copy
"Koneksi bermasalah. Periksa koneksi internet Anda lalu coba lagi." (bukan lagi
Inggris dari lapisan API); (2) matikan backend saat buka app → state
`backendUnavailable`, kini dari code `BACKEND_UNREACHABLE`, bukan dari kata
"network" di pesan. Alur sukses tidak tersentuh.

### 9. Next (kandidat scope berikutnya — pilih satu)

Purge dead-file (`retry_helper.dart` + `finance_gateway.dart`) · converge
`ApiResult` (search) + `SupportResult`/`SupportFailure` (support) ke `Result` ·
alirkan `errorCode` ke state presentasi (auction detail) · baseline bertanggal
untuk 72 gagal pre-existing.

---

## Sesi 2026-09-28 (lanjutan 6) — `ApiResult` (vocabulary result keempat & kelima) dibunuh; search & support berhenti membuang `errorCode` (§11)

**Scope aktif (tunggal):** `ApiResult` — satu `class` di support + DUA `typedef`
bernamasama di search — harus mati, dan search/support harus berhenti membuang
identitas kegagalan machine-readable (`errorCode`/`statusCode`) saat gagal.
Bukan sekadar ganti nama tipe: tanpa memperbaiki kanal code, "migrasi" ini hanya
memindahkan kebohongan lama ke tipe baru.

### 1. Kondisi awal (terverifikasi, bukan asumsi)

Tiga deklarasi, dua bentuk yang saling bertabrakan (temuan lanjutan 5, kini
dikerjakan):

1. `class ApiResult<T>` di `support_api_datasource.dart:293` — field
   `data`/`error`/`code`; `fold` NAMED dan **error KEDUA**
   (`onError(String error, String? code)`); `isSuccess => error == null`.
2. `typedef ApiResult<T> = ({T? data, String? error})` di
   `search_repository.dart` **dan** `search_history_repository.dart` — nama sama
   dideklarasikan dua kali, **tanpa kanal code sama sekali**.

Kerusakan nyata (alasan scope ini ada):

- **Search meruntuhkan code jadi teks.** Impl search menangkap semua dengan
  `catch (e)` dan mengembalikan `(data: null, error: 'Failed to ...: ${e}'`.
  Kegagalan transport (`NETWORK_ERROR`/`TIMEOUT`/`BACKEND_UNREACHABLE`) datang
  sebagai `DioException`→`ApiException` ber-code kanonik dari scope transport,
  lalu dibuang. Konsumen tidak punya jalan lain selain cocok-cocokan teks.
- **Support memakai status code sebagai STRING.** `_mapApiErrorToFailure`
  switch pada `'403'/'401'/'404'/'409'/'400'`, sisanya mencocokkan TEKS pesan
  ('already assigned', 'already resolved', 'network'/'connection'/'timeout').
  Setelah scope transport, code transport adalah `'NETWORK_ERROR'` dst — bukan
  status numerik — jadi cabang network berbasis teks itu hanya benar kalau
  pesannya kebetulan memuat kata "network".

### 2. Authority yang dikunci

- `core/common/result.dart` `Result<T>` = satu-satunya vocabulary hasil. Sesi ini
  menambah anggotanya, bukan membuat yang baru.
- `ApiExceptionFactory.fromTransport` + `isTransportFailureCode` tetap SATU
  authority klasifikasi. Sesi ini **tidak** menambah tabel `DioExceptionType`
  kedua; gate transport menegakkannya (lihat §4).
- `StructuredApiException` = jembatan `throw` → `Result` untuk lapisan data yang
  masih melempar (pola yang sudah dipakai auction), sehingga klasifikasi tetap
  terjadi sekali di lapisan API dan repo hanya menerjemahkan.

### 3. Perubahan

`lib` (10 file):

1. `domains/system/support/data/datasources/support_api_datasource.dart` —
   `class ApiResult<T>` + blok `API RESULT TYPE` DIHAPUS; 2 helper + 7 method →
   `Future<Result<T>>`; cabang envelope sekarang meneruskan
   `code` **dan** `statusCode`; cabang `DioException` meneruskan
   `code: exception.code, statusCode: exception.statusCode` (tidak lagi hanya
   `code`).
2. `domains/system/support/data/repositories/support_repository_api.dart` —
   `fold(onError:, onSuccess:)` NAMED dibuang → `if (result.isError)`;
   `_mapApiErrorToFailure` kini menerima `Result<Object?>` dan memutuskan dari
   `isTransportFailureCode(result.errorCode)` + `switch (result.statusCode)`;
   cabang `code == '404'` di `getTicket` → `statusCode`; SELURUH cocok-teks
   dihapus.
3. `features/search/search/domain/repositories/search_repository.dart` —
   `typedef ApiResult` DIHAPUS; 5 signature → `Result<...>`.
4. `features/search/search/domain/repositories/search_history_repository.dart` —
   `typedef ApiResult` DIHAPUS; 4 signature → `Result<...>`.
5. `features/search/search/data/remote/search_api_service.dart` — +satu
   `_guard<T>` (`DioException` → `StructuredApiException(message, code,
   details)` lewat `_apiClient.extractException`) membungkus 8 pemanggilan
   `_apiClient`; nol literal code, nol `DioExceptionType`.
6. `features/search/search/data/search_repository_impl.dart` — record literal →
   `Result.success`/`Result.error`; +`_failure` (membaca `StructuredApiException`)
   dan +`_propagate` (meneruskan `errorCode`/`statusCode`/`errorDetails` saat satu
   domain gagal di `searchAll`); cast `as ApiResult<...>` → `as Result<...>`.
7. `features/search/search/data/search_history_repository_impl.dart` — idem,
   +`_failure`.
8. `features/search/search/domain/usecases/search_usecase.dart` — `data != null`
   → `isError`, dan kegagalan diteruskan **dengan** `errorCode`/`statusCode`/
   `errorDetails` (sebelumnya diratakan jadi pesan).
9. `features/search/search/presentation/providers/search_history_notifier.dart` —
   `.error != null` / `.error == null` → `.isError` / `.isSuccess`.
10. `features/search/search/search.dart` — `ApiResult` dicabut dari klausa
    `show` barrel (satu-satunya jalur publik vocabulary itu).

`test` (4 file): 3 wiring test search-history (fake `ApiResult` → `Result`) +
`test/core/result_authority_contract_test.dart` (gate diperluas).

### 4. Gate & proof

Gate lama (`result_authority_contract_test.dart`) **buta** terhadap nama ini — ia
hanya menyapu `Result`/`RepositoryResult`/`ContentRepositoryResult`. Sekarang:

1. `ApiResult` masuk `_duplicateNames` (sweep `lib`+`test`, tanpa allowlist) DAN
   masuk regex deklarasi (`class ApiResult<T> {` maupun
   `typedef ApiResult<T> = ({...})`), jadi `homes['ApiResult']` wajib `null`.
2. Positive proof surface: 3 file (datasource support + 2 interface search)
   wajib masih menyebut `Result<`.
3. Positive proof support: `isTransportFailureCode(code)` + `result.statusCode`
   ada; **negative proof**: `error.contains(` dan `case '404'` DILARANG — teks
   pesan bukan lagi authority keluarga kegagalan.
4. Positive proof search: `extractException` + `StructuredApiException` ada di
   `search_api_service.dart`, `error is StructuredApiException` + `code:
   error.code` ada di kedua impl; **negative proof**: `DioExceptionType.`
   DILARANG di service (tabel kedua), `data: null, error:` DILARANG di impl
   (bentuk record tanpa code).
5. Negative proof detektor ditambah dua bentuk `ApiResult` di atas.

**Negative proof nyata dijalankan (3 probe terpisah, bukan satu):**

- Probe A: `typedef ApiResult<T> = ({T? data, String? error});` ditanam ulang di
  `search_repository.dart` → **GAGAL** di 2 test ("Result is declared in exactly
  one file under lib/" menampilkan `homes['ApiResult']` = file itu, dan "no file
  anywhere names a duplicate result type").
- Probe B: `isTransportFailureCode(code)` diganti `error.contains('network')` +
  `case '404'` ditanam di mapper support → **GAGAL** di test klasifikasi support.
- Probe C: `code: error.code` dibuang dari `_failure` dan satu call site
  dikembalikan ke `Result.error('Failed to search contents: $e')` → **GAGAL** di
  test "search carries the API failure code across its throw boundary".

Semua probe dihapus → gate result **10/10 PASS**, dan bersama gate transport
**24/24 PASS**.

**Gate gate bekerja lintas scope:** komentar doc saya di mapper support sempat
menyebut literal `NETWORK_ERROR`/`TIMEOUT`; gate transport
("no transport code literal exists outside the authority file") menangkapnya
sebagai pelanggaran → komentar ditulis ulang tanpa literal → PASS. Tidak ada
pengecualian yang ditambahkan ke gate itu.

### 5. Verifikasi

- `flutter analyze lib test` = **0 error**, **149 issue** (= baseline persis).
- `test/core/result_authority_contract_test.dart` + `test/features/search` +
  `test/domains/system/support` = **129 PASS** (termasuk 3 wiring test search
  history dan 4 test kontrak support; tanpa skip).
- Gabungan lebih luas (`test/core` + `test/features/search` +
  `test/domains/system/support` + 2 test mention widget + proof chat) = **512
  test, 18 gagal**. Ketiga belas+lima-nya **bukan milik sesi ini**: 13 mention
  (`UnimplementedError: ApiClient must be provided externally` — keluarga
  pre-existing), 4 router (3 `create_auction_route_contract_test` + 1
  `router_lifetime_preservation_test`, pre-existing), 1 gate media
  (`media_pick_engine_authority_test`, lihat §6). Nol regresi dari perubahan ini.
- Grep penutup: `ApiResult` di `lib` = **0**; `ApiResult` di `test` = **hanya**
  gate yang sengaja menamainya sebagai detektor.

### 6. Temuan luar scope (diparkir, tidak dikerjakan)

- **P2 `SupportResult`/`SupportFailure` masih hidup = vocabulary hasil kelima.**
  Sesi ini hanya menyembuhkan jalur MASUK-nya (code). Keluarganya sendiri masih
  punya `dataOrThrow`, `fold` NAMED sukses-dulu, dan `isSuccess => failure ==
  null`. Paket tersendiri: converge ke `Result<T>` atau kanonikkan sebagai tipe
  domain — jangan setengah jalan.
- **P2 `SupportFailureAlreadyResolved` + `SupportFailureCannotReopen` kini NOL
  konstruktor.** Satu-satunya jalan menuju `AlreadyResolved` dulu adalah
  substring `'already resolved'` yang sesi ini bunuh. Varian itu sekarang mati
  tapi masih diekspor; jangan dihidupkan kembali lewat pencocokan teks, dan
  jangan dihapus terpisah dari scope `SupportFailure`.
- **P3 semantik mapper support dipertahankan apa adanya:** 409 →
  `AlreadyAssigned`, 400/422 → `Validation`, 401/403 → `Permission`, 404 →
  `NotFound`. Sesi ini hanya memindahkannya dari string status ke
  `result.statusCode`; apakah keluarga 409/422 itu benar untuk API user-side
  (reopen/conflict) adalah pertanyaan kontrak yang belum diadili.
- **P2 gate media merah oleh file agen lain.**
  `test/core/media/media_pick_engine_authority_test.dart` gagal karena
  `lib/domains/commerce/transaction/order/presentation/widgets/
  evidence_media_gallery.dart` (untracked, bukan milik sesi ini) mendeteksi video
  secara lokal: `url.toLowerCase().contains('/videos/')` dan
  `.split('?').first.endsWith('.mp4')`. Authority-nya
  `MediaUploadOrchestrator.isVideoUrl/isVideoFile`. Gate bekerja benar; yang
  perlu dibereskan file itu (agen pemiliknya).
- **P2 pre-existing, tidak tersentuh:** 13 test mention
  (`ApiClient must be provided externally`), 3 `create_auction_route_contract`
  + 1 `router_lifetime_preservation`, keluarga feed promoted & saved_item.
  Baseline bertanggal masih belum dibuat.
- **Sisa vocabulary "hasil/gagal" yang BELUM converge (tidak berubah sesi ini):**
  `Either<Failure,T>` dartz di 6 use case follow (purge `core/errors/failure.dart`
  DIBATALKAN atas koreksi data — `Failure` masih tipe `Left`), `Withdrawal.isSuccess`,
  `PaymentResult`/`PaymentResultStatus`, `SupportResult`/`SupportFailure`,
  `finance_gateway.dart` (dead file), `retry_helper.dart` (dead file).

### 7. Git status

Belum di-commit. Milik sesi ini (lanjutan 6): **10 file `lib` + 4 file `test`**
(0 file baru). Working tree tetap multi-agen (`evidence_media_gallery.dart`
untracked dari agen lain, backend Go & file media lain ikut berubah) —
**jangan `git add -A`**.

### 8. Owner retest

Ringan, dua jalur yang dulu kehilangan code: (1) matikan backend lalu lakukan
pencarian → state error kini berasal dari code transport (`BACKEND_UNREACHABLE`),
bukan dari kalimat pesan, dan pencarian tidak lagi "berhasil" dengan daftar
kosong; (2) matikan backend lalu buka/balas ticket support →
`SupportFailureNetwork` dipilih karena code, bukan karena kata "network" di
pesan. Alur sukses (search & support) tidak tersentuh.

### 9. Next (kandidat scope berikutnya — pilih satu)

Converge `SupportResult`/`SupportFailure` (vocabulary hasil kelima) · alirkan
`errorCode` ke state presentasi (`auction_detail_screen` masih `String`) · purge
dead-file (`retry_helper.dart` + `finance_gateway.dart`) · converge 6 use case
follow off dartz `Either<Failure,T>` · baseline bertanggal untuk 72 gagal
pre-existing.

---

## Sesi 2026-09-28 (lanjutan 7) — purge dead-file: `retry_helper.dart` + `finance_gateway.dart` (§11)

**Scope aktif (tunggal):** dua file yang sesi-sesi sebelumnya sebut "dead total"
harus DIBUKTIKAN mati lewat sweep referensi seluruh repo, lalu dihapus, lalu
dikunci supaya tidak bisa hidup lagi. Klaim mati tanpa bukti bukan bukti; klaim
mati yang salah = menghapus kapabilitas.

### 1. Bukti kematian (sweep, bukan asumsi)

Sweep dilakukan atas file TRACKED (`git grep`, seluruh index, semua tipe file)
DAN file UNTRACKED (termasuk milik agen lain: `evidence_media_gallery.dart`,
`store_photo_preview.dart`, gate-gate baru):

- `git grep -n -I -e RetryHelper -e RetryConfig -e RetryFutureExtension -e
  retry_helper -e FinanceGateway -e FinanceResult -e finance_gateway` di SELURUH
  repo → hanya 3 kategori: (a) file itu sendiri, (b) catatan ledger
  (`PARKIRAN_AUDIT.md`, `KONDISI_APP.md`) yang menyebutnya sebagai bahan audit,
  (c) NOL rujukan kode/test.
- `grep -rn` di `apps/mobile/lib` + `apps/mobile/test` → nol rujukan di luar
  kedua file itu.
- `tool/`, `assets/`, `android/`, `ios/` → nihil. Satu-satunya "match" adalah
  `android/.gradle/8.12/executionHistory/executionHistory.bin`, cache build
  Gradle lama yang kebetulan memuat string hasil kompilasi lampau — artefak,
  bukan jalur hidup (dan tidak boleh "dibersihkan" oleh sesi ini).
- Nol barrel: `lib/core/core.dart` meng-export `src/utils/...` (direktori BEDA
  dari `lib/core/utils/`), dan `lib/domains/finance/` tidak punya file `.dart`
  lain di root-nya — jadi tidak ada `export` yang menyembunyikan importer.

Isi keduanya 100% self-contained — nol importer, nol implementor, nol konsumen,
nol test:

1. `lib/core/utils/retry_helper.dart` (284 baris) — `RetryConfig` (termasuk
   `RetryConfig.network`/`.critical`), `RetryHelper.execute`/`executeResult`
   (exponential backoff + jitter), extension `RetryFutureExtension.retry`.
   `_isRetryableError` memutuskan retry dengan MENCOCOKKAN TEKS error
   ('network'/'connection'/'timeout'/'socket'/'internet'/'500'..'504'/'429'),
   dan `executeResult` membuang `errorCode` sebelum memutuskan. Jadi: kebijakan
   retry yang tidak pernah dijalankan app, dengan aturan klasifikasi yang JUSTRU
   sudah dilarang total oleh scope transport (lanjutan 5).
2. `lib/domains/finance/finance_gateway.dart` — `abstract class FinanceGateway`
   (7 method: charge/releaseEscrow/refund/checkBalance/getBalance/holdFunds/
   verifyPayment) + `FinanceResult<T>` (`dataOrThrow`, `isSuccessful`) +
   `FinanceException`. Nol implementor, nol konsumen, nol barrel.

**Authority check (kenapa purge ini TIDAK melanggar "authority dulu").**
`finance_gateway.dart` MENGKLAIM dirinya otoritas arsitektur di dok headernya:
"commerce CANNOT directly import or call finance repositories … ✅ REQUIRED: Use
FinanceGateway interface for all finance operations". Klaim itu sudah dibatalkan
kenyataan di DUA arah: `lib/domains/commerce/**` mengimpor `domains/finance/**`
(coins, payment entity, payment presentation, `payment_result_notifier`), dan
`lib/domains/finance/**` mengimpor `domains/commerce/**` (`order`, `order_status`,
`order_providers`). Dokumen owner (`KONDISI_APP.md`, `GUIDE_CLEANUP.md`,
`cara-kerja.md`) tidak satu pun menyebut `FinanceGateway`/`RetryHelper` sebagai
wajib (sweep `-- "*.md"` = nol). Jadi ini *fake authority*, bukan authority —
yang berhak menentukan boundary adalah owner, dan keputusan boundary
commerce↔finance yang sebenarnya masuk §5 sebagai scope terpisah.

### 2. Perubahan

- `lib/core/utils/retry_helper.dart` **DIHAPUS** (284 baris).
- `lib/domains/finance/finance_gateway.dart` **DIHAPUS** (`FinanceGateway` +
  `FinanceResult` + `FinanceException`) — sekaligus membunuh vocabulary hasil
  KEENAM sebelum sempat dipakai.
- `test/core/dead_file_purge_contract_test.dart` **BARU** (gate, 4 test).
- Tidak ada file lain disentuh. Direktori induk tetap hidup dan diverifikasi:
  `lib/core/utils/` masih berisi `polling_monitor.dart` (dipakai
  `auction_repository_impl` + `seller_remote_datasource`) dan
  `notification_navigation_handler.dart` (dipakai 3 service notifikasi);
  `lib/domains/finance/` masih berisi `transaction/` + `wallet/`.

### 3. Gate & proof

`test/core/dead_file_purge_contract_test.dart` mengeklaim empat hal:

1. Kedua file yang dipurge **tetap mati** (`File.existsSync() == false`).
2. **Nol kode menamainya lagi** — sweep `lib` + `test` (semua `.dart`,
   termasuk file untracked) + `pubspec.yaml`, mencocokkan DUA bentuk: identifier
   ber-word-boundary (`RetryHelper`, `RetryConfig`, `RetryFutureExtension`,
   `FinanceGateway`, `FinanceResult`, `FinanceException`) DAN fragmen path
   (`utils/retry_helper.dart`, `finance/finance_gateway.dart`) — karena jalur
   kebangkitan yang sebenarnya adalah baris `import`/`export`, bukan nama kelas.
   Dokumen (`*.md`) SENGAJA di luar sweep: ledger mencatat purge ini DENGAN NAMA,
   dan prosa tidak bisa menghidupkan kode terkompilasi.
3. Surface sweep nyata + purge ter-scope: lantai jumlah file (>1400) dan tiga
   "live neighbour" wajib masih ada (`polling_monitor.dart`,
   `notification_navigation_handler.dart`, `payment_repository.dart`) — gate ini
   tidak boleh bisa lulus dengan menghapus direktori.
4. Negative proof detektor (identifier word-boundary menangkap `class
   RetryHelper {`/`FinanceResult>` tetapi TIDAK menangkap `_isRetryableError`
   maupun `FinanceResultX`; fragmen path menangkap bentuk `import`).

**Negative proof nyata dijalankan (2 probe terpisah):**

- Probe A: `lib/core/utils/retry_helper.dart` dibuat ulang (`class RetryHelper {
  static void execute() {} }`) → **GAGAL di 2 test** ("purged dead files stay
  deleted" + "no code names a purged dead file or its vocabulary", pelanggar
  `lib/core/utils/retry_helper.dart names RetryHelper`). Probe dihapus.
- Probe B: satu baris komentar ditanam di file HIDUP
  (`lib/core/utils/polling_monitor.dart`: `// probe: RetryConfig and
  finance/finance_gateway.dart …`) yang tetap KOMPILASI — untuk membuktikan
  detektor jalan pada kode yang sehat, bukan hanya pada file yang dihapus →
  **GAGAL di test 2**. File dikembalikan dan diverifikasi byte-identik
  (`git diff --stat` = kosong).

Semua probe bersih → gate **4/4 PASS**.

### 4. Verifikasi

- `flutter analyze lib test` = **0 error**, **149 issue** (= baseline persis,
  tidak berubah walau 2 file hilang — bukti keduanya nol issue/nol konsumen).
- `test/core` + `test/domains/finance` = **436 test, 8 gagal**, ketujuhnya
  pre-existing dan tidak satu pun menyentuh purge ini: 3
  `payment_result_notifier_test` (authority `order.status` vs resource payment),
  3 `create_auction_route_contract_test` + 1 `router_lifetime_preservation_test`
  (router), 1 `media_pick_engine_authority_test` (file agen lain).
- Grep penutup: `RetryHelper|RetryConfig|RetryFutureExtension|retry_helper|
  FinanceGateway|FinanceResult|FinanceException|finance_gateway` di `lib` =
  **0**; di `test` = hanya gate yang sengaja menamainya sebagai detektor.

### 5. Temuan luar scope (diparkir, tidak dikerjakan)

- **P1 boundary commerce↔finance TIDAK PERNAH DITEGAKKAN.** Satu-satunya tempat
  aturan itu dinyatakan adalah dok header file yang kini mati, dan kenyataan
  sudah dua arah: commerce → finance (coins, payment entity/presentation,
  `payment_result_notifier`) dan finance → commerce (`order`, `order_status`,
  `order_providers`). Purge ini TIDAK memutuskan boundary mana yang benar; ia
  hanya berhenti memajang aturan yang tidak dijalankan. Butuh keputusan owner
  (tegakkan lewat gate import, atau nyatakan boundary dua arah sebagai kanonik
  dan hapus aturannya).
- **P2 retry TIDAK ADA di jalur hidup mana pun sekarang.** `retry_helper.dart`
  mati dan tidak ada pengganti; `ApiClient`/`BaseApiRepository` tidak punya
  retry. Ini konsisten dengan
  `LABUDA_IDENTITY_AUTH_DESIGN_PROPOSAL.md` yang menyatakan auto-retry backoff
  pada sync "sudah dibunuh (Stage 3B) … tidak boleh dikembalikan" — jadi
  keputusan sadar, bukan kehilangan. Kalau nanti retry dibutuhkan, bangun di
  jalur `Result` berbasis `isTransportFailureCode` + `errorCode`,
  JANGAN hidupkan kembali `_isRetryableError` yang mencocokkan teks.
- **P3 `RetryConfig.critical`** menyebut "critical operations like user sync" —
  user sync di app sudah tanpa retry otomatis; tidak ada lagi yang merujuk
  konsep itu.
- **P3 cache Gradle** `apps/mobile/android/.gradle/**/executionHistory.bin`
  masih memuat string kelas yang sudah mati. Artefak build lokal (bukan tracked),
  tidak berdampak; tidak dibersihkan oleh sesi ini.
- Sisa vocabulary "hasil/gagal" yang belum converge (tidak berubah):
  `SupportResult`/`SupportFailure` (kelima — sisa berikutnya), `PaymentResult`/
  `PaymentResultStatus`, `Either<Failure,T>` dartz di 6 use case follow,
  `Withdrawal.isSuccess`. Vocabulary keenam (`FinanceResult`) mati bersama purge
  ini.

### 6. Git status

Belum di-commit. Milik sesi ini (lanjutan 7): **2 file `lib` DIHAPUS + 1 file
`test` BARU**. Working tree tetap multi-agen (backend Go, file media agen lain,
`evidence_media_gallery.dart` untracked) — **jangan `git add -A`**.

### 7. Owner retest

Tidak ada alur pengguna yang berubah: kedua file tidak punya call site, jadi
nol permukaan UI/network. Cukup smoke test biasa (buka app, checkout, buka
support) untuk memastikan tidak ada regresi tak terduga dari penghapusan.

### 8. Next (kandidat scope berikutnya — pilih satu)

Converge `SupportResult`/`SupportFailure` (vocabulary hasil kelima, sisa
terdekat) · tegakkan atau batalkan boundary commerce↔finance sebagai gate import
(butuh keputusan owner) · alirkan `errorCode` ke state presentasi
(`auction_detail_screen`) · converge 6 use case follow off dartz
`Either<Failure,T>` · baseline bertanggal untuk 72 gagal pre-existing.

## Sesi 2026-09-28 (malam) — Tema: SATU WARNA SATU NAMA + purge 2 zombie lib

### 1. Verdict
Dua scope ditutup dan terverifikasi, keduanya bounded: (a) konvergensi nama
warna status — satu warna satu nama, **253 situs di 66 file** dipindahkan ke
nama kanonik **tanpa perubahan visual** (const yang sama, hanya beda ejaan);
(b) purge 2 zombie `lib/` yang diparkir sesi siang. Scope tema yang sudah hijau
(gate lib-wide, fundasi AppTheme) **tidak dibuka kembali** — diperkuat saja.

### 2. Root cause
`AppColors` menyimpan satu warna dengan **tiga nama**: `error = statusError`,
`success = statusSuccess = successGreen`, `warning = statusWarning =
warningYellow`, plus `primary = primaryRed`. Karena nilainya identik, tiap
layar boleh memilih ejaan sendiri dan tidak ada yang merasa salah — duplicate
authority yang tak terlihat. Dari sisi lain,
`Theme.of(context).primaryColor` (role M2) menjawab pertanyaan "warna brand"
di luar `colorScheme`, dan `AppColors.primary` mengulanginya dari sisi token.
Gate lib-wide tidak memblokir nama-nama ini (cuma `primaryRed`/`primaryBlue`).

### 3. Canonical behavior
- Kanonik status (token brand tanpa scheme role): `AppColors.statusSuccess` /
  `statusWarning` / `statusError` / `statusInfo`.
- Jawaban kanonik untuk warna role: `Theme.of(context).colorScheme.*`
  (`primary`, `error`). `Theme.of(context).primaryColor` **DILARANG**.
- `AppColors` tidak lagi punya alias sama sekali (blok alias dihapus total).
- `Colors.transparent` = exemption terdokumentasi (nilai bebas-mode untuk
  scrim/immersive chrome, bukan pilihan palet).
- `DetailChipWidget.color` kini `Color?` (null → inherit `scheme.primary`),
  jadi varian `.size` tidak mengikat token brand di initializer const.
- Zombie dibunuh: `core/dependencies/provider_scope_reader.dart` DIHAPUS
  (+ direktori kosongnya) dan import `foundation.dart` mati di `api_config.dart`.

### 4. File yang berubah
- **66 file `lib/`**: rename `AppColors.error|success|warning|successGreen|warningYellow` → `status*` (253 situs).
- **6 file `lib/`**: `AppColors.primary` / `Theme.of(context).primaryColor` → `colorScheme.primary` (`help_center_screen`, `profile_about_tab` ×2, `search_result_type_helper`, `location_picker_component`, `notification_navigation_service`, `saved_item_screen`).
- `core/src/theme/app_colors.dart`: 8 baris alias dihapus (+2 blank sisa).
- `shared/widgets/detail_chip_widget.dart`: field `Color?`, tint di-resolve dari scheme di `build`, `.size` menginit `color = null`.
- `core/api/config/api_config.dart`: import `foundation.dart` mati dihapus.
- **Dihapus**: `lib/core/dependencies/provider_scope_reader.dart` + direktori `core/dependencies/`.
- **Test**: `test/core/theme/theme_authority_contract_test.dart` (+3 pola larangan: alias nama & `.primaryColor`; +3 sample resurrection, +3 sample legit, +6 asersi alias di `AppColors`), `test/core/dead_file_purge_contract_test.dart` (file ke-3 + identifier `_ProviderScopeHolder` + path fragment + negative proof).
- **Dokumen**: `KONDISI_APP.md` §14 butir TEMA diperbarui (satu warna satu nama, `primaryColor` dilarang, exemption `Colors.transparent`).

### 5. Residue yang dihapus
6 alias const di `AppColors`; 2 baris komentar alias; 1 import mati; 1 kelas
zombie (`_ProviderScopeHolder`, nol konsumen di `lib`/`test`/`tool`); 1 direktori
kosong; 4 blank line sisa.

### 6. Proof (command → hasil)
- residue grep `AppColors.(primary|error|success|warning|successGreen|warningYellow)` + `.primaryColor` + path/nama file mati, di `lib`+`test`+`tool` → **0**.
- `flutter analyze lib` → **23 issue, 0 error** (baseline 26: 3 warning zombie hilang, **tidak ada lint baru**).
- `flutter analyze test/core/theme test/core/dead_file_purge_contract_test` → **No issues found**.
- `flutter test test/core/theme test/core/dead_file_purge_contract_test test/core/file_authority_contract_test` → **23 PASS**.
- `flutter test` 2 test checkout terdampak (satu-satunya test non-tema yang menyebut `AppColors`/`DetailChipWidget`) → **10 PASS**.
- **Negative proof**: `AppColors.primary`, `AppColors.successGreen`, `Theme.of(context).primaryColor` WAJIB memicu gate; `scheme.primary`, `AppColors.statusWarning`, `Colors.transparent` WAJIB tidak — dipin dalam test, terbukti saat test ke-8 gagal sebelum pola `successGreen` ditambahkan.

### 7. Kenapa tidak full regression
Perubahan = rename nilai identik (const sama persis) + hapus file nol
konsumen; bukti pengganti = `flutter analyze lib` 0 error (seluruh konsumen
terkompilasi), residue grep 0, dan 33 test kontrak hijau. `test/` tidak pernah
menyebut `AppColors.` (0 hit), jadi tidak ada fixture yang bergantung pada nama
lama. Full regression tetap milik gate rilis.

### 8. Parkiran (di luar scope, sudah diklasifikasi)
1. **P2 — butuh keputusan owner (visual): dua hijau untuk semantik sukses.** `primaryGreen` #10B981 (32 situs/18 file: refund approved, chat category, operational queue) vs `statusSuccess` #059669 (197 situs). Menyatukannya MENGUBAH tampilan.
2. **P2 — butuh keputusan owner (visual) + arsitektur: retune tone status untuk dark.** `scheme.error` = `statusError` = #DC2626 di KEDUA mode; kontras di atas `surface` dark #161B22 ≈ **3.6:1 (di bawah WCAG AA 4.5)**; `statusSuccess` ≈ 4.6:1 (pas), `statusWarning` ≈ 5.5:1. Perbaikan butuh authority per-brightness (ThemeExtension) karena `AppColors` bersifat const — kalau hanya `ColorScheme.error` yang diubah, `scheme.error` (96 situs) dan `AppColors.statusError` (182 situs) berbeda nilai = divergensi.
3. **P2** — `scheme.error` (96) vs `AppColors.statusError` (182) masih dua nama untuk nilai sama; menyatu otomatis kalau status diangkat ke ThemeExtension.
4. **P2** — regex gate tersalin di `checkout_theme_authority_contract_test.dart:367` (versi subset) → risiko drift; hoist ke satu definisi.
5. **P2** — `appBarTheme`/`cardTheme`/`elevatedButtonTheme`/`inputDecorationTheme` masih bind token mentah (nilai kebetulan = role scheme, drift laten) dan **tidak ikut dipin** test "component themes pinned to scheme"; plus 2 override AppBar per-widget (`main_app_bar.dart:51`, `media_viewer_widget.dart:67`) dan 31 `Scaffold(backgroundColor:)`.
6. **P3** — typography scale ad-hoc: 0 `textTheme:` di `AppTheme`, 234 `TextStyle(fontSize:)` dengan 16 nilai berbeda (14/12/16/13/…) — menetapkan skala tipografi = keputusan owner.
7. **P3** — 22× `use_null_aware_elements`, `USE_FIREBASE` bukan lowerCamelCase, 19 file masih mengimpor `app_colors.dart` langsung (saat ini hanya token status/brand, sah menurut gate).

### 9. Git status
Belum di-commit (owner belum minta). Milik sesi ini: 66 file rename, 8 file
edit, 1 file `D` + 1 direktori kosong, 2 file test, 1 dokumen. Sisa working
tree (±160 file) milik sesi/agen lain — jangan ikut di-stage.

### 10. Owner retest diperlukan?
Belum. **Nol perubahan visual** di scope ini (nilai warna identik, file yang
dihapus nol konsumen). Yang menunggu keputusan owner ada di parkiran #1 dan #2.

---

## Sesi 2026-09-28 (malam 2) — Boundary commerce↔finance jadi SATU ARAH + koreksi klaim ledger (§11)

### 1. Verdict
Scope: buang coupling dua arah commerce↔finance. **SELESAI dan terverifikasi.** Arah canonical = **commerce → finance** (commerce boleh bergantung pada finance; finance TIDAK boleh). Recon hasil pembayaran dipindahkan dari finance ke `commerce/transaction/checkout/`, lalu boundary-nya dikunci oleh gate baru.

### 2. KOREKSI KLAIM LEDGER — WAJIB dibaca sesi berikutnya
Klaim "3 gagal `payment_result_notifier_test` = authority `order.status` vs payment resource" (muncul di baris 534, 1236, 1311, 1486, 1599, 1704, 1763, 1908, 2299) **SALAH**. Akar sebenarnya = **fixture time-bomb**: helper `_payment()` memakai `expiredAt: DateTime.utc(2026, 8, 2)` yang sudah lewat, sehingga `hasReusablePaymentUrl` dan `canContinuePayment` selalu false. Tiga assertion yang gagal (baris 206/239/531) semuanya bergantung expiry — **nol hubungan dengan authority maupun tipe result**. Diperbaiki dengan 1 baris (expiry relatif ke now) → **18/18 PASS**, nol perubahan produksi.
Pelajaran yang harus dipakai: label "pre-existing" yang diwariskan lintas sesi harus **diverifikasi**, bukan disalin. Tiga sesi berturut-turut memakai angka itu tanpa membacanya, dan diagnosis yang salah hampir menyebabkan perubahan kontrak jalur uang yang tidak perlu.

### 3. Root cause coupling
`payment_result_notifier.dart` + `payment_result_state.dart` tinggal di `lib/domains/finance/...` tetapi mengimpor `order_providers.dart` (repository commerce), `Order` dan `OrderStatus` (entity/enum commerce), lalu memutuskan hasil pembayaran dari state machine order milik commerce. Konsumen satu-satunya = screen **commerce** (`checkout/presentation/screens/payment_result_screen_impl.dart`). Backend justru sebaliknya: payment upstream (webhook menyettle payment row), dan `order_completion_service.go:597` menolak menyelesaikan order sampai payment settlement/capture → kebenaran mengalir **payment → order**, sedangkan mobile membalikkannya.

### 4. Yang dilakukan
- **Dipindah**: `payment_result_notifier.dart` + `payment_result_notifier.g.dart` (generated, untracked) + `payment_result_state.dart` → `lib/domains/commerce/transaction/checkout/presentation/providers/`. Test → `test/domains/commerce/transaction/checkout/presentation/providers/`.
- **Diubah**: barrel `finance/.../presentation/presentation.dart` (2 export dicabut, bagian "Payment result" dihapus); screen commerce (2 import); 2 file test (import diarahkan ke jalur commerce — satu di antaranya, `payment_result_screen_widget_test.dart`, terlewat di sweep pertama karena sweep hanya menyentuh `lib/`).
- **BARU**: `test/core/domain_boundary_contract_test.dart` (5 test, gate).
- **Semantik runtime: NOL perubahan.** Operasi OR (`order.status || payment resource`) **DIPERTAHANKAN**, karena itu memang kontrak yang diuji ("succeeds when payment resource is settled", "keeps polling when payment resource is processing"). Yang BUKAN authority adalah field proyeksi `order.paymentStatus` — sudah dipin oleh test "does not succeed even when paymentStatus is paid".
- **Slice media (authority)**: `evidence_media_gallery.dart` dulu mendeteksi video sendiri (`contains('/videos/')`, `endsWith('.mp4')`) → kini `MediaUploadOrchestrator.isVideoUrl`.

### 5. Gate (diranah DIRECTIVE, bukan prosa)
Pola: `^\s*(import|export|part)\b[^;]*commerce/` atas `lib/domains/finance` + `test/domains/finance`.
Prosa dan string data TIDAK dihitung — mis. `test/domains/finance/.../payment_decision_phantom_purge_test.dart:25` menyebut path commerce sebagai data scan, itu sah dan tidak boleh di-allowlist.
Floor anti-vakum: >40 file disapu + 4 file finance wajib masih hidup + arah canonical (commerce→finance) wajib >0, supaya gate tidak bisa lulus dengan mengosongkan finance atau menghapus arah yang sah.

### 6. Proof (command → hasil)
- Residu: `domains/commerce` di `lib/domains/finance` = **0**; jalur lama `finance/.../providers/payment_result*` di `lib`+`test` = **0**.
- `flutter analyze` (checkout + payment + gallery + test checkout) = **0 error** (2 warning pre-existing di test checkout lain, bukan scope ini).
- `flutter test test/core/domain_boundary_contract_test.dart` = **5/5 PASS** (setelah 1 sampel negative-proof milik saya sendiri diperbaiki: `providers/commerce_state.dart` tidak mengandung `commerce/`).
- `flutter test` gate + seluruh `test/domains/commerce/transaction/checkout` + gate media = **81 PASS / 1 skip**.
- `payment_result_notifier_test.dart` = **18/18 PASS** (sebelumnya 15/3).
- `test/core/media/media_pick_engine_authority_test.dart` = **5/5 PASS** (sebelumnya merah).
- **Negative proof nyata (probe)**: `import 'package:labuda/domains/commerce/...order.dart';` ditanam di `payment_providers.dart` (finance) → gate **GAGAL** dan menyebut pelanggar persis; probe dilepas → `git diff` file itu **kosong (byte-identik)** → gate PASS.

### 7. Temuan luar scope (dicatat, TIDAK dikerjakan)
- `payment_decision_phantom_purge_test.dart` (finance) menginspeksi file entity commerce lewat string path. Bukan coupling kompilasi, tapi letaknya janggal: kandidat pindah ke tree commerce.
- OR dua-sumber tetap ada karena diuji, bukan karena ada dokumen authority yang menetapkannya. Kalau nanti ingin satu sumber, ubah di backend dulu (order = proyeksi ketat dari payment), jangan di klien.
- Parkiran lama tetap hidup (tidak tersentuh): `SupportResult`/`SupportFailure`, `Either<Failure,T>` dartz ×6 use case follow, `Withdrawal.isSuccess`, `PaymentResult`/`PaymentResultStatus` **yang HIDUP** (dipakai `PaymentState.paymentSuccess` → `payment_notifier`) sehingga ada dua gagasan "payment result" di finance, `errorCode` belum mengalir ke `auction_detail_screen`, baseline bertanggal, dua hijau + kontras dark WCAG, komponen theme belum dipin.
- Gate ini hanya memetakan SATU arah (finance→commerce = 0). Boundary domain lain belum dipetakan dan tidak diklaim.

### 8. Git status
Belum di-commit. Milik scope ini: 3 file `R` (2 lib + 1 test), 1 file baru (gate), 1 file `M` (`evidence_media_gallery.dart`), 1 `M` (barrel finance), 1 `M` (screen commerce), 1 `M` (widget test), plus 1 baris fixture di test yang dipindah. `.g.dart` ikut dipindah tetapi tidak di-track (`.gitignore:77 **/*.g.dart`).

### 9. Owner retest
Boundary: **tidak perlu** — nol perubahan semantik runtime (hanya lokasi modul + import).
Slice media: smoke ringan — buka order yang punya bukti media video, pastikan thumbnail video tetap ter-render (bukan ikon gambar rusak).

### 10. Next (kandidat, pilih satu)
Converge `SupportResult`/`SupportFailure` (sisa vocabulary terdekat) · salurkan `errorCode` ke presentasi (`auction_detail_screen`) · baseline bertanggal untuk gagal pre-existing yang BELUM diverifikasi (jangan salin daftar lama — lihat §2) · keputusan owner: dua hijau + kontras dark WCAG · pindahkan `payment_decision_phantom_purge_test.dart` ke tree commerce.

---

## Sesi 2026-09-28 (malam 3) — Satu vocabulary status payment di wire (§11)

**Status scope: BELUM CLOSED — 1 keputusan authority menggantung + 1 perilaku residu yang saya nyatakan terang-terangan, tidak dibungkus.**

### 1. Authority yang ditemukan (bukan asumsi)
Satu konsep (`status pembayaran`) punya DUA kebenaran sekaligus:
- `core/common/types/payment_types.dart` (tipe kanonik, dokumennya sendiri): "the vocabulary IS the enum names; backend never puts gateway vocabulary on this wire … a settled payment that silently reads as pending is a money-safety lie" → MENOLAK kosakata gateway.
- `commerce/.../order_mapper.dart`: peta kosakata gateway privat (`settlement`/`capture`/`challenge` — `challenge` bahkan tidak ada di backend) + **unknown → `pending` diam-diam**.
- Producer: backend menuang status payments-table mentah ke wire di **7 situs** (order detail `decision.go:772`, order list `order_query_service.go:278`, payments-create ×5 di `dependencies.go`).

### 2. Yang dieksekusi
- Authority baru di **pemilik** state: `integration/payment/infrastructure/repository/entity.go` → `canonicalWireStatus`, `CanonicalWireStatus`, `CanonicalWireStatusPtr`, `WireStatusUnknown`.
- 7 produsen mentah → 0 (`grep '"status": *payment\.Status,'` = kosong).
- Competitor dibunuh: tabel gateway di `order_mapper.dart` DIHAPUS; klien kini `PaymentStatus.fromString` (strict, lantang) + empty → pending.
- Test mengikuti kodebase (bukan sebaliknya): `order_payment_status_mapper_test.dart` (13 PASS), `order_contract_p1_test.dart:547` fixture `settlement` → `paid`, `decision_cancelled_timeout_test.go`, `order_detail_active_refund_test.go`, `order_query_service_payment_status_test.go`.
- Proof: `go build ./...` OK; `go test ./internal/commerce/order/... ./internal/integration/payment/infrastructure/repository/` hijau; `flutter test test/domains/commerce/transaction/order` **122/122**; analyze 0 error.

### 3. YANG BELUM CLOSED — jangan dibaca sebagai PASS
1. **Satu keputusan authority menggantung (business truth, bukan teknis).** Authority backend membedakan 6 status tersimpan (`pending`, `settlement`, `capture`, `deny`, `cancel`, `expire`) + predikat sendiri-sendiri (`IsSettledStatus` = settlement|capture; `IsExpired()` = expire; `IsFailed()` = deny|cancel|expire). Vocabulary kanonik klien hanya punya 4 nama terminal (`paid`/`failed`/`expired`/`refunded`). Jadi kolaps 3→2 itu **sebagian penemuan saya**, bukan turunan authority: `settlement|capture → paid` dan `expire → expired` punya dasar predikat, tetapi `deny|cancel → failed` adalah penggabungan yang saya pilih. Dua opsi yang harus diputuskan owner: (a) perluas vocabulary kanonik agar mencerminkan 6 status authority (nama paling jujur), atau (b) tetapkan aturan kolaps secara eksplisit.
2. **Perilaku residu yang saya nyatakan, bukan disembunyikan:** `CanonicalWireStatusPtr` masih mengembalikan `nil` untuk status tak dikenal → di wire order field-nya OMIT → klien membacanya sebagai `pending`. Itu masih persis "unknown silently reads as pending" untuk kasus itu. `CanonicalWireStatus` (dipakai 5 situs payments-create) sudah memakai sentinel `WireStatusUnknown` yang DITOLAK lantang oleh klien, jadi jalur itu jujur. Menyamakan `Ptr` dengan perilaku itu belum dikerjakan (butuh penulisan ulang fungsi, bukan penggantian satu baris).
3. `wire` payments-create kini bisa memancarkan `"unknown"`; mobile `payment_dto.dart` mem-parse `json['status'] as String` lalu `PaymentStatus.fromString` → menolak lantang. **Nol test kontrak untuk kasus ini** — perlu ditambahkan sebelum scope boleh CLOSED.

### 4. Next
Keputusan owner untuk §3.1 (perluas vocabulary kanonik vs aturan kolaps) · samakan `CanonicalWireStatusPtr` ke perilaku sentinel jujur · tambah test kontrak "status tak dikenal ditolak" di kedua wire · lalu scope ini boleh CLOSED.

---

## Sesi 2026-09-28 (malam 4) — Kebenaran TUNGGAL status pembayaran: CLOSED (3 tahap)

**Prinsip yang dipakai (koreksi penting):** kebenaran sesungguhnya adalah **kebutuhan bisnis**, bukan internal gateway. Status pembayaran didefinisikan oleh PERTANYAAN PEMBELI dan AKSI yang terbuka — bukan oleh daftar teknis Midtrans. Entri malam-3 (§3.1/3.2/3.3) ditutup oleh entri ini.

### Keputusan bisnis (dikunci owner)
| Pertanyaan pembeli | Wire | Aksi yang terbuka |
|---|---|---|
| Masih harus bayar? | `pending` | bayar / lanjutkan (URL + deadline) |
| Uang sudah masuk? | `paid` (settlement **atau** capture) | pesanan lanjut; escrow terisi |
| Bisa coba lagi? | `failed` (deny) | coba dengan metode lain |
| Sudah terlambat? | `expired` (expire) | tidak bisa bayar lagi |
| Tidak ada putusan | **tanpa nilai** | tidak ada aksi pembayaran; kebenaran ada di `order.status` |
| — | `cancel` | **TANPA PUTUSAN**, bukan `failed` |

Alasan `capture` = `paid`: backend sendiri sudah memutuskan itu (`IsSettledStatus = {settlement, capture}`, dan penyelesaian pesanan digerbangi keduanya). Uang terkunci = cukup untuk melanjutkan pesanan.
Alasan `cancel` ≠ `failed`: di sistem ini `cancel` adalah ekor dari pembatalan **pesanan**, bukan penolakan gateway. Menyebutnya "Pembayaran Gagal" menuduh pembeli atas pembayaran yang tidak pernah diminta diulang.
Alasan `pending` harus sempit: sebelumnya satu kata itu menanggung tiga makna (belum ada baris payment, status tak dikenal, benar-benar menunggu bayar) — di layar uang.

### Tahap 1 — authority di pemilik state (backend)
`canonicalWireStatus` (integration/payment/infrastructure/repository): `pending→pending` · `settlement|capture→paid` · `deny→failed` · `expire→expired` · **`cancel→tanpa putusan`** · tak dikenal→tanpa putusan. Konstanta `WireStatusNoVerdict = ""`. `go build ./...` OK, nol sisa `WireStatusUnknown`.

### Tahap 2 — klien berhenti mengoersi
`Order.paymentStatus` → `PaymentStatus?` (+ `clearPaymentStatus` di `copyWith`, supaya putusan bisa benar-benar dikosongkan). `_mapPaymentStatus`: kosong/absen → **null** (bukan `pending`); selain itu strict `PaymentStatus.fromString`. `order_payment_info_card`: tanpa putusan → badge TIDAK dirender, warna netral. `flutter analyze lib` = **0 error**.

### Tahap 3 — kunci anti-kebangkitan
Gate baru `test/core/payment_status_authority_contract_test.dart` (4 test):
1. Nol **literal gateway ber-quote** (`'settlement'`/`'capture'`/`'challenge'`/`'deny'`) di seluruh `lib/`, **tanpa allowlist** — prosa boleh menyebut katanya, kode tidak boleh memegangnya sebagai nilai. `waiting_settlement` (fase auction, konsep lain) tetap sah.
2. Floor anti-vakum (>600 file) + pemetaan kanonik wajib tetap hidup (gate tidak boleh lulus dengan menghapus terjemahannya).
3. `pending` HANYA bisa datang dari wire `pending`; kosong → null; kosakata tak dikenal → ditolak lantang.
4. Negative proof detektor: menangkap bentuk tabel lama, tidak menangkap prosa.

### Proof (command → hasil)
- `flutter analyze lib` = **0 error**.
- `flutter test` gate payment-status + gate boundary + gate media + seluruh `order` + `checkout` = **207 PASS / 1 skip**.
- Backend: `go build ./...` OK; `go test ./internal/commerce/order/... ./internal/integration/payment/infrastructure/repository/` hijau.
- **Probe nyata**: literal `'settlement'` ditanam di `order_mapper.dart` → gate **GAGAL** dan menyebut pelanggar persis (`holds 'settlement'`); probe dilepas → bersih, gate PASS.

### Status
**CLOSED — SATU AUTHORITY, SATU VOCABULARY.** `pending` kembali berarti satu hal saja. Yang tersisa dari model ini untuk sesi lain: guard UI/copy yang memakai kata "gagal"/"pending" untuk status pembayaran (audit copy, **kini dikerjakan — lihat malam 5**) dan parkiran lama yang tidak tersentuh.

---

## Sesi 2026-09-28 (malam 5) — Audit copy UI status pembayaran: judul gagal tidak boleh menuduh (§11)

**Scope**: sisa parkiran malam-4 ("guard UI/copy kata 'gagal'/'pending'"). Commit sesi ini (authority tunggal + boundary) masuk `637dc32` lebih dulu.

### Temuan audit (1 kebohongan nyata, sisanya sudah jujur)
- **LIAR — `payment_result_screen_impl.dart` judul hardcode `'Pembayaran Gagal'`** untuk SEMUA state failed. Order `cancelled` (wire `cancel` → tanpa putusan) masuk layar merah berjudul "Pembayaran Gagal" dengan subtitle "Pesanan dibatalkan." — menuduh pembeli gagal bayar = melanggar keputusan terkunci. Refund ("Pembayaran telah dikembalikan.") dan dispute ("Pesanan sedang dalam sengketa.") kena tuduhan sama. Alasan subtitle sudah jujur; yang bohong judulnya.
- Sudah jujur, TIDAK diubah: reason notifier (cancel → "Pesanan dibatalkan."), layar timeout ("Status Pembayaran Belum Diketahui"), checking ("Menunggu Konfirmasi Pembayaran"), label order-level list/timeline/chat ("Dibatalkan"/"Kedaluwarsa"), badge `order_payment_info_card` (render hanya non-null; `pending` → "BELUM" sesuai makna sempit).

### Perbaikan — satu authority untuk copy gagal
- `PaymentResultState`: field `title` (ikut `props`+`copyWith`); factory `failed` kini `required title`.
- Notifier: `_getFailureReason` + `_getPaymentFailureReason` menyatu → **satu switch per kasus, return `(title, reason)`** (`_getOrderFailure`/`_getPaymentFailure`). Judul per kasus: cancel/cancelledTimeout → "Pesanan Dibatalkan"; refunded/partiallyRefunded → "Pembayaran Dikembalikan"; disputeOpen → "Pesanan Dalam Sengketa"; expired → "Pembayaran Kedaluwarsa"; **hanya `PaymentStatus.failed` (deny) yang jujur "Pembayaran Gagal"**.
- Screen render `state.title ?? 'Pembayaran Tidak Berhasil'` — nol tuduhan hardcode. Ejaan diseragamkan "kadaluarsa" → "kedaluwarsa" (2 alasan notifier).
- Test mengikuti kode: notifier — deny → title "Pembayaran Gagal"; cancelled → title "Pesanan Dibatalkan" DAN `isNot('Pembayaran Gagal')`; expired → `contains('kedaluwarsa')` + title "Pembayaran Kedaluwarsa". Widget test baru: failed state dengan title authority → dirender, `'Pembayaran Gagal'` absent (menangkal hardcode ulang).

### Proof (command → hasil)
- `flutter analyze lib` = **0 error** (23 info pre-existing, nol di file tersentuh).
- `flutter test` 2 file tersentuh = **31 PASS**.
- Lingkar penuh: gates payment-status + boundary + result-authority + seluruh `checkout` + `order` = **212 PASS / 1 skip**.

### Status
**CLOSED.** Copy gagal kini berasal dari satu switch di notifier; screen murni render. Sisa parkiran lama (`SupportResult`/`SupportFailure`, dartz ×6 use case follow, `Withdrawal.isSuccess`, dll.) tidak tersentuh — sesi lain.
