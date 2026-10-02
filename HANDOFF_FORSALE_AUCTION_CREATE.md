# HANDOFF — ForSale & Auction (Create Path)

Status per 2026-10-02 (sesi lanjutan). Dokumen ini adalah handoff lintas sesi untuk scope ForSale/Auction create path.
Jangan mengaudit ulang dari nol; pakai tabel di bawah sebagai factual audit state.

---

## SCOPE 1 — Opsi pengiriman: seller tidak bisa menambah opsi setelah ada opsi aktif

**STATUS:** `CLOSED — CANONICAL DESIGN LOCKED` (owner retest on-device masih disarankan)

### Business truth

- Seller dengan >=1 opsi pengiriman aktif **harus tetap bisa menambah opsi baru** saat sedang membuat/mengedit ForSale atau Auction.
- Pembuatan opsi pengiriman punya **satu authority**: surface `RoutePaths.sellerShipping` (`SellerShippingScreen` → `ShippingSetupScreen`, one-package: identitas + destinasi dalam satu transaksi).
- Layar create **tidak boleh** membuat opsi sendiri (tidak ada form inline).

### Canonical authority

- `apps/mobile/lib/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart` — satu-satunya widget selector; satu call-site navigasi (`_openShippingManagement`) dipakai oleh keadaan empty dan populated.
- Daftar opsi di-refresh setiap kembali dari surface management (`setState(() { _future = _loadOptions(); })`), sehingga opsi baru langsung bisa dipilih.

### Forbidden / killed

- Form inline name+type di layar create (design lama, sudah dibunuh).
- Dua affordance berbeda untuk aksi yang sama (duplikat CTA) dalam satu keadaan.
- Wording `Tambah opsi pengiriman` / `Buat opsi pengiriman` (owner-locked: `Atur Pengiriman`).

### Perubahan

- `seller_shipping_options_selector.dart`: tambah `_openShippingManagement()` + refresh on return; affordance `Atur Pengiriman` muncul di **kedua** keadaan; `_EmptyOptionsBanner` menerima `onManage` (tidak lagi punya navigasi sendiri); copy dinetralkan dari "forSale" → netral untuk ForSale & Auction.
- Bug nyata yang ikut ditutup (satu file, satu keluarga cacat): `setState(() => _future = _loadOptions())` pada jalur retry error mengembalikan `Future` → assertion Flutter, retry tidak pernah bekerja. Diganti block body.
- Test `apps/mobile/test/domains/commerce/transaction/shipping/shipping_option_setup_screen_test.dart`: test lama yang **mengunci bug** ("populated shows chips not add button") dihapus dan diganti test kanonik.

### Positive proof

- `flutter test test/domains/commerce/transaction/shipping/shipping_option_setup_screen_test.dart` → **`+16 ALL PASS`**.
- `dart analyze` pada widget + test terdampak → **No issues found**.

### Negative proof

- Keadaan populated: `expect(find.text('Atur Pengiriman'), findsOneWidget)` → tepat satu affordance, tanpa duplikat.
- `expect(find.text('Tambah opsi pengiriman'), findsNothing)` dan `findsNothing` untuk `Buat opsi pengiriman` → wording tertolak tidak kembali.
- Residue sweep: tidak ada push kedua ke `RoutePaths.sellerShipping` dari dalam selector/layar create; tidak ada form pembuatan opsi inline.

### Known limitations

- Belum ada runtime proof di perangkat nyata (owner retest: buat ForSale saat sudah punya 1 opsi aktif → tambah opsi kedua → opsi baru muncul di selector).

---

## SCOPE 2 — Sertifikat produk (ForSale & Auction)

**STATUS:** `CLOSED — VERIFIED` (backend + mobile + test)

### Business truth

- Sertifikat = **teks biasa dari seller**, bukan upload dokumen.
- Jenis kanonik FINAL (owner memilih opsi A): `breeder`, `contest`, `import`, `health`. `ownership` **DIKILL** (digantikan `import`).
- Label mobile: `Breeder`, `Kontes`, `Import`, `Kesehatan`.

### Canonical authority

- Backend satu authority: `backend/internal/commerce/product/entity/product_validation.go` (`CertificateImport`, `CanonicalCertificateOrder` = breeder→contest→import→health, error `invalid certificate value %q: allowed breeder, contest, import, health`).
- `backend/internal/commerce/shared/certificates.go` mendelegasi ke entity — tidak lagi menyimpan daftar sendiri.
- DB kolom `certificates text[]` tanpa CHECK (migration 000001).

### Perubahan

- Backend: entity + shared normalizer ditulis ulang + test (negative test `ownership` mati, single-authority lock).
- Mobile: `commerce_certificate_selector.dart` = vocabulary+label kanonik; `commerce_common_product_detail_section.dart` menurunkan label dari daftar itu dan membaca `forSale.certificates` (sebelumnya hardcoded `const []`); entity `ForSale` + mapper + copyWith + props mendapat `certificates`; create ForSale, create Auction (`KoiDetails.certificates`), edit ForSale (hydrate + save) mengoleksi & mengirim sertifikat.

### Positive proof

- `flutter test` pada `commerce_certificate_selector_test.dart` + `for_sale_certificate_read_model_test.dart` + `auction_detail_screen_runtime_test.dart` → **+12 ALL PASS**; `auction_detail_restriction_dispatch_test.dart` dengan fixture `['import']` → PASS.
- Fixture auction kanonik: `['import','health']` dan label `'Import, Kesehatan'`.

---

## SCOPE 3 — Hapus catatan persiapan end-to-end

**STATUS:** `CLOSED — VERIFIED` (field, wire, snapshot, proyeksi, UI, DB)

### Business truth

- Catatan persiapan DIHAPUS total (owner: "sampai akar tanpa sisa sedikitpun"). Tidak ada producer, tidak ada reader, tidak ada kolom.
- Yang tersisa sebagai vocabulary persiapan hanyalah `preparation_time` (target perubahan berikutnya, lihat antrian #1).

### Yang dihapus (satu keluarga)

- Backend: product entity/patch/repo (INSERT/SELECT/UPDATE/scan), `shared/product_content_wire`, for_sale & auction service+handler (request/response/projection/`hasProductContent`), order entity + `NewOrderFromSource` + creation service (2 call site) + decision DTO + `order_repository.go` (4 scan + INSERT) + `order_repository_extensions.go`, `chat_auction_projection_resolver` (SQL+scan+struct).
- Mobile: `auction_dto`, `auction_mapper`, `auction_repository_impl`, `auction` entity, `auction_repository`, `auction_notifier`, `create_auction_screen`; `for_sale_dto`, `for_sale_dto_mapper`, `for_sale_repository_impl`, `for_sale` entity, `create_for_sale_screen`; `order_mapper`, `order_api_response_dtos`, `order` entity, `order_detail_screen`; `commerce_common_product_detail_section` (field, `hasPreparationInfo`, blok note).
- DB: `backend/migrations/000120_drop_preparation_note.{up,down}.sql` — drop `products.preparation_note` + `orders.preparation_note_snapshot` (keduanya `IF EXISTS`); terdaftar di `backend/migrations/README.md`.

### Positive proof

- Residue sweep mobile+backend: `grep -rn -E "preparation_?[Nn]ote"` pada `apps/mobile/lib`, `apps/mobile/test`, `backend` → **0 hit** (satu-satunya sisa string adalah deskripsi `preparation_time` yang sah di `preparation_time.dart`).
- `dart analyze lib test` (mobile) → **0 error** (151 info pre-existing di domain lain, nol terkait preparation).
- `flutter test` batch 9 test terdampak (read-model preservation, auction/for_sale detail runtime, create auction contract, shipping-origin DTO, 4 authority/seller test dengan fake lama) → **+62 ALL PASS**.
- Backend: `go build ./...` bersih; `go test -count=1 ./internal/commerce/... ./internal/serverboot/... ./internal/worker/... ./internal/platform/...` → semua paket ok, dengan satu pengecualian **bukan milik pekerjaan ini** (lihat Temuan di luar scope).

### Negative proof

- Test detail auction & for_sale kini mengunci ketiadaan note: `expect(find.text('Packing aman sebelum kirim'), findsNothing)` — anti-regresi eksplisit.
- Fixture `auction_response_dto_shipping_origin_test.dart` tidak lagi menyelundupkan `preparation_note`; 4 fake repo test tidak lagi mendeklarasikan param `preparationNote`.
- Migration down me-restore kolom tanpa producer — jujur, tanpa jalur tulis palsu.

---

## KEPUTUSAN OWNER (FINAL — tidak ada lagi yang menunggu)

- Sertifikat: teks biasa, jenis `breeder`, `contest`, `import`, `health` (opsi A).
- Catatan persiapan: dihapus total end-to-end.
- Vocabulary waktu persiapan: **SELESAI (Scope 4)** — 3 rentang `1_3_days`/`4_7_days`/`8_15_days`, default 1–3; deadline order = upper bound 3/7/15 hari.
- ForSale default **no-nego**: **SELESAI (Scope 6)**.
- CTA create Auction: **SELESAI (Scope 5)** — disabled sampai field wajib lengkap.

---

## SCOPE 4 — Vocabulary waktu persiapan 3 rentang (end-to-end)

**STATUS:** `CLOSED — VERIFIED` (DB + backend + mobile + wiring create Auction)

### Business truth

- Waktu persiapan = kapan seller bisa kirim SETELAH checkout (ikan kadang butuh karantina).
- Tepat 3 opsi (owner): **1–3 hari (DEFAULT)**, **4–7 hari**, **8–15 hari**. Rentang = batas MAKSIMUM (upper bound) yang dijanjikan ke buyer.

### Canonical authority

- Token wire/DB: `1_3_days` | `4_7_days` | `8_15_days` (keputusan owner 2026-10-02).
- Backend: `forsale/entity/preparation_time.go` (Parse/IsValid/Days = 3/7/15), `product/entity/product_validation.go` (`ValidatePreparationTime`), binding `oneof=1_3_days 4_7_days 8_15_days` (4 titik: for_sale & auction × Create+Update), deadline `order/entity/order.go` `preparationTimeToDays` (3/7/15, unknown → 3 — upper bound).
- Mobile: `core/common/types/preparation_time.dart` (`days1_3`/`days4_7`/`days8_15`, default `days1_3`), shared widget `catalog/shared/presentation/widgets/commerce_preparation_time_selector.dart` (diangkat dari `_PreparationTimeSelector` private di create ForSale — satu authority, dipakai ForSale + Auction).
- DB: `000121_preparation_time_three_ranges.{up,down}.sql` — guard menolak nilai di luar 4 nilai lama sebelum konversi; mapping TIDAK PERNAH mempersingkat janji (immediate/short → `1_3_days`, medium → `4_7_days`, long → `8_15_days`); snapshot NULL tetap NULL; down memetakan balik kasar (1_3→short, 4_7→medium, 8_15→long). Terdaftar di `migrations/README.md` (head = `000121`).

### Perubahan kunci

- Vocabulary lama (`immediate/short/medium/long`) DIHAPUS total dari Go + Dart + SQL fixtures — residue sweep = 0 (satu-satunya sisa `"immediate"` adalah alert/worker `action.Channel`, domain berbeda yang sah).
- `order/delivery/http/dto/decision.go`: snapshot `preparation_time` selalu ikut bila non-kosong (konsep "immediate = tak ada informasi" ikut mati).
- **Wiring create Auction PENUH**: `CreateAuctionDto` (+wire key `preparation_time`) → `CreateAuctionParams`/`toMap` → `AuctionMapper.toCreateDto` → `AuctionRepository` → impl → `AuctionNotifier` → screen (state default `days1_3` + selector di antara durasi & opsi pengiriman).
- `isImmediate`/`requiresPreparation` DIHAPUS; semua call site (badge order detail, chat order banner, `_PreparationInfoBanner`) disederhanakan ke satu pola "Estimasi siap kirim: N hari" + deskripsi.
- Guard test enum di-rename/di-rewrite: `tests/migration_000121_preparation_time_three_ranges_test.go` (build tag `integration`) — mengunci label tepat 3 nilai, consumer tepat 2 kolom, dan menolak 10 nilai purged.
- Panggilan `NewOrderFromSource` bertag integration yang kehilangan 1 argumen saat purge catatan persiapan (5 titik) ikut dibereskan.

### Positive proof

- `dart analyze lib test` → **0 error** (151 info pre-existing, jumlah identik dengan sebelum scope ini).
- `flutter test` batch 14 file terdampak → **81/81 PASS** (79 pada batch pertama; 2 gagal karena CTA/validator tertiadi lazy ListView → test diperbaiki scroll-into-view → re-run file 7/7).
- `go build ./...` bersih; `go test -count=1` pada `./internal/commerce/... ./internal/serverboot/... ./internal/worker/... ./internal/interaction/... ./internal/pricing/... ./internal/social/... ./internal/discovery/... ./tests/` → **69 paket ok** (satu-satunya FAIL: `TestPaymentReuseGuard_HandlesLookupErrors`, milik pekerjaan lain).
- **Bukti migrasi nyata**: `go test -tags integration -run TestMigration000121 ./tests/` → **PASS (150s)** — seluruh chain 000001→000121 dieksekusi canonical runner (`testdb`), enum efektif = tepat `1_3_days, 4_7_days, 8_15_days`.

### Negative proof

- Sweep: `PreparationTimeImmediate/Short/Medium/Long` = 0; label lama (`Siap kirim langsung`, `1–2 hari`, `3–5 hari`, `7+ hari`, `Siap dikirim segera`) = 0 di lib+test dan internal.
- DB menolak 10 nilai purged (immediate/short/medium/long + 6 legacy) — diuji langsung di guard integrasi.

### Known limitations / out of scope (dicatat, tidak dikerjakan)

- `go test -tags integration ./internal/...` masih RED oleh pekerjaan LAIN di worktree (tidak terkait preparation): test `order/application`, `order/tests`, `negotiation/consumer` masih memakai `[]string` untuk `ProductMedia` (perubahan `product.go` tak terkomitmen); syntax error `n8d_stale_resource_guard_integration_test.go` (edit `SellerID→ActorID` setengah jadi); `quantity_persistence_test.go` memanggil `entity.NewForSale` yang tidak pernah ada di HEAD; `SellerDashboardResponse.TotalListings/ActiveListings` hilang + arg `string` vs `*string` di seller handler tests. Sebagian kecil (`discovery`) sudah sekalian dibetulkan agar paketnya hijau.
- `TestPaymentReuseGuard_HandlesLookupErrors` (`internal/serverboot`) tetap gagal — diff `dependencies.go` milik pekerjaan lain.

---

## SCOPE 5 — CTA create Auction disable sampai field wajib lengkap

**STATUS:** `CLOSED — VERIFIED` (behavioral + source contract)

### Perubahan (`create_auction_screen.dart`)

- `bool get _isFormComplete` — gate membaca: judul & deskripsi non-kosong (trim), ≥1 media, varietas, ukuran >0, harga awal >0, kenaikan bid >0, buy now (bila diisi) harus parse + ≥ harga awal, durasi, waktu mulai (wajib bila mode scheduled), ≥1 opsi pengiriman, sender address (`senderAddressIdProvider.value != null`).
- `ElevatedButton.onPressed: _isSubmitting || !_isFormComplete ? null : _submitForm` + caption saat disabled: `Lengkapi semua field wajib untuk mengaktifkan tombol terbit.`
- **Prasyarat diperbaiki**: `onChanged` Ukuran & Usia kini menulis lewat `setState(...)`; 5 controller (judul, deskripsi, harga awal, kenaikan bid, buy now) di-listen via `_onFormFieldsChanged` (initState/dispose) sehingga gate re-evaluate tiap ketikan.
- Form validators + guards `_submitForm` tetap sebagai defense-in-depth — gate hanya menjawab "sudah terisi", bukan "sudah sah".

### Proof

- `flutter test` batch 7 file screen → **+41 ALL PASS**. Dua test behavioral baru: form kosong & partial → CTA `onPressed == null`, tap no-op, validator tidak jalan, `createCalls == 0`; satu source contract mengunci gate expression, 12 kondisi field owner, fix setState Ukuran/Usia, dan listener controller.
- `dart analyze` pada dua file tersentuh → **No issues found**; full `dart analyze lib test` → **0 error** (151 info pre-existing, jumlah identik).

### Known limitation

- Bukti positif "CTA menyala saat semua lengkap" tidak bisa dijalankan di widget test: `_mediaUrls` private tanpa seam (aturan yang sama yang sudah didokumentasikan di kepala test file) + ketergantungan pada `senderAddressIdProvider` → kepatuhan penuh dikunci via source contract; guard `_submitForm` yang sudah ada tetap membuktikan tolakan media/koi saat CTA terbuka.

---

## SCOPE 6 — ForSale default no nego

**STATUS:** `CLOSED — CANONICAL DESIGN LOCKED`

### Perubahan

- `create_for_sale_screen.dart`: `_isNegotiable = true` → `false` (owner: listing dibuat TANPA NEGO; seller mengaktifkan nego sendiri bila mau).
- `edit_for_sale_screen.dart`: hydrate `_isNegotiable = forSale.price > 0` → `_isNegotiable = forSale.isNegotiable` (kebenaran server, bukan turunan harga — desain lama di-KILL); initial pre-hydrate ikut `false`.
- Backend tidak berubah: `negotiation_enabled` bool biasa; DTO/request/read mobile sudah default `false` di lapisan data — satu-satunya offender memang dua layar.

### Proof

- Positive: `flutter test` batch 4 file (kontrak baru + authority + router + contract) → **+16 ALL PASS**; `dart analyze` 3 file → **No issues found**.
- Negative (kill once, lock forever): `test/.../for_sale/negotiation_default_contract_test.dart` — source contract mengunci `bool _isNegotiable = false;` ADA dan `= true` TIDAK ADA; `_isNegotiable = forSale.isNegotiable;` ADA dan `forSale.price > 0` TIDAK ADA.

---

## ANTRIAN SCOPE BERIKUTNYA

**KOSONG** — semua keputusan owner pada handoff ini sudah dieksekusi dan terverifikasi (Scope 1–6).

Temuan di luar scope tetap tercatat di bagian bawah; tunggu arahan owner untuk scope baru.

---

## TEMUAN DI LUAR SCOPE (dicatat, tidak dikerjakan)

- Create Auction: `mediaTypes: List.filled(_mediaUrls.length, AuctionMediaType.photo)` → video ditandai photo.
- `print('DEBUG-SOURCE _submit entered')` di `shipping_option_setup_screen.dart` (debug residue).
- `my_for_sales_screen.dart` memakai `Navigator.of(context).pushNamed(RoutePaths.sellerShipping)` sementara surface lain memakai `context.push` → dua gaya navigasi ke route yang sama.
- `edit_for_sale_screen.dart` hydration nego (lihat antrian #2).
- **Gagal di luar scope (worktree, non-preparation):** `go test -tags integration ./internal/...` merah karena pekerjaan ProductMedia (`[]string` → `[]ProductMedia` di test order/negotiation), syntax error `n8d_stale_resource_guard` (edit `ActorID` setengah jadi), `entity.NewForSale` tak pernah ada di HEAD (`quantity_persistence_test`), field `SellerDashboardResponse` hilang. Sebagian `discovery` sudah sekalian dibetulkan agar hijau.
- **Gagal di luar scope:** `TestPaymentReuseGuard_HandlesLookupErrors` (`internal/serverboot`) gagal karena perubahan `dependencies.go` milik pekerjaan lain di worktree (diff nol baris preparation; test file bersih di HEAD). Bukan residu scope ini.
