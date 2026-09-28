Labuda — Multi-Truth Codebase
Canonical Authority & Anti-Resurrection Guide
Engineering governance • Zero-to-one convergence

1. Masalah: Codebase Memiliki Terlalu Banyak “Kebenaran”
   Labuda telah melalui banyak iterasi sehingga satu konsep dapat memiliki beberapa implementasi yang semuanya masih tampak valid: service, repository, DTO, provider, route, adapter, fallback, test, fixture, komentar, dan dokumentasi. Inilah kondisi multi-truth codebase.
   Masalah utamanya bukan banyaknya file. Masalahnya adalah hilangnya kejelasan authority. Jika developer atau agent menjadikan implementasi yang sudah ada sebagai sumber kebenaran, architecture lama dapat terus dipertahankan dan bahkan dibangun kembali.
2. Analogi Owner → Bahasa Programmer
   Analogi Owner: satu perusahaan mempunyai toko bahan makanan dan loket telepon. Toko bahan makanan boleh memberi tahu pelanggan bahwa ada telepon/HP dan mengarahkan pelanggan ke loket telepon, tetapi toko bahan makanan tidak boleh mengambil alih harga, stok, transaksi, dan aturan bisnis loket telepon.
   Analogi Owner Padanan engineering
   Toko bahan makanan Owning domain / bounded context
   Loket telepon Domain/service yang memiliki business authority berbeda
   Banner bahwa HP tersedia Safe reference / projection / navigation
   Harga HP Canonical business state milik owning domain
   Stok HP Availability/inventory state milik owning domain
   Struk transaksi HP Order/payment/settlement state milik transaction domain
   Mengirim pelanggan ke loket HP Delegation ke owning domain
   Toko ikut menjual HP Domain A mengambil alih business authority Domain B — architectural violation
3. Prinsip Utama: Authority ≠ Dependency
   Dependency graph menjelaskan apa yang sekarang terjadi di codebase. Dependency graph tidak menentukan apa yang benar.
   Pola yang salah:
   OldService → OldRepository → OldDTO → OldProvider → banyak tests
   Jika canonical authority sudah terbukti berada pada NewService, banyak caller dan test tidak membuat OldService menjadi canonical. Caller harus dikonvergensikan ke authority baru atau dipurge jika obsolete.
   Dependency graph digunakan SETELAH authority ditentukan, untuk mengetahui urutan refactor/demolition dengan aman.
4. Hierarchy of Authority
   • Owner / business truth yang telah dikunci.
   • Canonical architectural decisions.
   • Current filesystem — fakta tentang implementasi yang ada.
   • Database/schema/runtime evidence.
   • Tests — evidence, bukan business authority.
   • Git/GitHub — historical/reference evidence, bukan design authority.
   Jika implementation bertentangan dengan canonical truth, implementation yang harus direfactor. Jangan mengubah canonical truth agar cocok dengan legacy implementation.
5. Audit Harus Dimulai dari Authority
   Authority → Tentukan business truth dan canonical authority.
   Classification → Tentukan CANONICAL / REQUIRED, OBSOLETE, INCOMPLETE / MISSING, atau AMBIGUOUS.
   Gap → Identifikasi implementation gap hanya pada capability yang memang canonical.
   Implementation → Bangun/refactor menuju canonical authority.
   Proof → Buktikan behavior dan invariants.
   Scoped Cleanup → Purge seluruh obsolete implementation dalam scope.
   Residue Search → Cari ulang dead references, adapter, fallback, test, export, comment, dan breadcrumb.
   Regression → Pastikan canonical path tetap sehat.
   Closure → Tutup hanya jika convergence dan cleanup terbukti.
6. Four-Way Classification
   Classification Makna Tindakan
   CANONICAL / REQUIRED Masih merupakan current truth dan diperlukan. KEEP
   OBSOLETE Tidak diperlukan dan telah digantikan/ditinggalkan. PURGE
   INCOMPLETE / MISSING Diperlukan tetapi implementasinya belum lengkap. STOP CLEANUP → implement minimum canonical → proof → lanjut cleanup
   AMBIGUOUS Authority belum cukup jelas. Jangan delete/implement → cari evidence atau Owner decision
7. Test Bukan Authority
   Test harus membuktikan canonical behavior. Dalam multi-truth codebase, test lama dapat menjadi mekanisme pelestarian legacy.
   • Test yang membuktikan canonical contract → KEEP.
   • Test yang hanya mempertahankan API/DTO/route/behavior obsolete → update atau PURGE.
   • Jangan membuat production compatibility layer hanya untuk membuat test obsolete tetap PASS.
   • Jika canonical contract berubah, validation harus dikonvergensikan ke contract baru.
8. Cleanup = Total Convergence
   Cleanup bukan sekadar menghapus file yang tidak dipanggil. Targetnya adalah filesystem yang hanya mengkomunikasikan current architecture dan tidak menyediakan breadcrumb yang dapat dianggap sebagai authority lama.
   • dead service/repository/provider/DTO/entity/route/export;
   • forwarding service, adapter, compatibility layer;
   • fallback, dual-read, dual-write;
   • duplicate producer/consumer/state authority;
   • obsolete tests dan fixtures;
   • stale comments dan documentation;
   • misleading registry/naming;
   • dead generated wiring;
   • artefak lain yang dapat menyebabkan resurrection.
9. Anti-Resurrection
   Architecture obsolete harus dibuat benar-benar mati. “Tidak dipakai” belum tentu cukup; jika artefak masih terlihat seperti bagian dari architecture, agent berikutnya dapat menghidupkannya kembali.
   • Jangan menyimpan legacy untuk “future use” tanpa current requirement.
   • Jangan mempertahankan compatibility hanya karena ada caller lama.
   • Jangan mempertahankan test hanya karena test tersebut sudah lama ada.
   • Jangan mempertahankan comment/documentation yang menggambarkan architecture obsolete.
   • Jangan menggunakan Git history untuk menghidupkan implementation lama.
   • Jika obsolete → PURGE. Jika caller valid → konvergensikan caller ke canonical authority.
10. Domain Boundary
    Setiap domain memiliki business authority sendiri. Domain lain boleh berkomunikasi melalui contract yang aman, reference, projection, atau navigation; domain lain tidak boleh menggandakan business truth.
    Contoh Chat ↔ Commerce: Chat boleh membawa resource reference dan safe projection. Chat tidak boleh menjadi authority atas harga, quantity, availability, seller commercial state, order, payment, settlement, atau transaction rules. Commerce tetap owning authority.
11. Closure Standard
    • canonical authority singular;
    • competing authority mati;
    • obsolete implementation dipurge;
    • dead references/imports/exports dibersihkan;
    • compatibility/fallback residue tidak tersisa tanpa requirement;
    • obsolete tests/fixtures tidak mengunci legacy;
    • stale comments/docs dikoreksi;
    • positive proof PASS;
    • negative proof/residue search PASS;
    • integration/runtime proof bila relevan;
    • protected scopes untouched;
    • tidak ada unexplained residue.
12. Engineering Rules
    • Existing code is evidence, not authority.
    • Existing tests are evidence, not authority.
    • Dependency count is not authority.
    • A caller does not make its callee canonical.
    • Complexity does not make an implementation correct.
    • Canonical business truth outranks legacy implementation.
    • If canonical truth is clear and old code is obsolete, purge it.
    • If canonical capability is missing, implement the minimum canonical path before continuing cleanup.
    • If authority is ambiguous, stop rather than inventing truth.
    • Cleanup is part of implementation, not optional post-work.
    • The goal is total convergence, not coexistence.
    • A clean repository should make obsolete architecture difficult to resurrect.
13. Final Mental Model
    CODEBASE tells us what EXISTS.
    AUTHORITY tells us what SHOULD EXIST.
    DEPENDENCY tells us HOW TO CHANGE IT SAFELY.
    CLEANUP removes what SHOULD NO LONGER EXIST.
    Tujuan Labuda bukan sekadar aplikasi yang dapat build atau test yang hijau. Tujuannya adalah codebase dengan satu kebenaran canonical per konsep, domain boundary yang jelas, proof yang dapat dipertanggungjawabkan, dan tanpa residue yang dapat menghidupkan kembali kesalahan lama.
14. Canonical Truths Terkunci — Convergence Record (Sept 2026)
    Keputusan authority yang telah dikunci owner dan dieksekusi total. Semua dianggap canonical; menghidupkan kembali pola lama = violation.

    • PRODUCT SATU AUTHORITY KONTEN. Title, description, media (termasuk typed metadata: type/dimensions/thumbnail/position), atribut ikan, farm address, preparation — semuanya milik Product. Entity ForSale dan Auction TIDAK lagi membawa salinan konten (13 alias field ForSale dihapus total; dual-write hydration dihapus). Konsumen membaca Product langsung dengan nil-guard eksplisit. Kolom DB for_sale_type dormant — drop = scope migrasi terpisah.

    • FORSALE TYPE TIDAK ADA. Konsep ForSaleType/FixedPrice dihapus dari entity, input, factory, wire, dan seluruh test. Jangan direkonstruksi.

    • WIRE MEDIA TYPED SATU BENTUK. commerce/shared MediaWireItems adalah satu helper projection media untuk kedua surface commerce (for_sale + auction detail). Blok `media` typed identik; `media_urls` flat tetap fallback universal untuk payload list.

    • AUCTION DISCOVERY SATU ENGINE. List discovery auction = Future engine (mirror for_sale), limit canonical 50. StreamProvider legacy, watchActiveAuctions/watchUserAuctions dihapus — jangan dihidupkan untuk "test compat". Polling detail/bids satu loop bersama dengan dedup snapshot; satu surface gagal tidak membunuh surface lain.

    • AUCTION STATUS BOUNDARY (keputusan owner). Wire publik `status` = kosakata phase tertutup {scheduled, active, waiting_settlement, ended, cancelled} (Status.PublicPhase). DRAFT TIDAK PERNAH menyeberang batas publik — dipetakan defensif ke cancelled. Nilai internal state-machine hanya lewat `seller_status`, hanya terisi untuk seller pemilik (owner surfaces), null untuk viewer lain. Detail draft 404 untuk non-owner. Search adapter coarsen juga. Mobile mapper memprioritaskan seller_status di atas status.

    • EMAIL_VERIFICATION_REQUIRED KANONIK = SNACKBAR. Semua kanal (auction bid, checkout, chat) menampilkan error snackbar dengan pesan spesifik aksi — bukan dialog. Test yang mengejar dialog hantu telah di-align.

    • FORSALE VISIBILITY TETAP DERIVED. visibility dihitung dari status+published_at; draft selalu private, detail di-guard 404 non-owner. Auction kini ber-paritas.

    • DIPURGE JUGA: field hantu DTO auction (expiredBNR, bid history hantu), deadline settlement kini derived (end_at + 24 jam) — bukan wire field. `expired_bnr`/`sold`/`expired` bukan backend state; parser mobile menormalkannya ke canonical (draft/ended) tanpa nilai enum hantu.

    • TEMA SATU AUTHORITY. `AppTheme.lightTheme/darkTheme` adalah satu-satunya authority warna/tipe; widget hanya membaca `Theme.of(context)`. Hanya 3 file boleh memegang warna: `core/src/theme/app_colors.dart` (token mentah), `app_theme.dart` (satu-satunya `ThemeData(`), `theme_provider.dart` (jembatan brightness OS → `ThemeMode`). JANGAN hidupkan kembali: registry per-scope 161 entri (DIMATIKAN, diganti gate yang menyapu seluruh `lib/`), fork `isDark`/`brightness ==` di widget, `Colors.white/grey`, hex mentah di luar `app_colors`. **SATU WARNA SATU NAMA** — alias `AppColors.primary`/`error`/`success`/`warning`/`successGreen`/`warningYellow` DIMATIKAN (semuanya cuma ejaan kedua dari warna yang sudah dimiliki tema); kanonik = `AppColors.statusSuccess/statusWarning/statusError/statusInfo` (token brand tanpa scheme role) dan role `colorScheme.*` (`primary`, `error`). Role M2 `Theme.of(context).primaryColor` DILARANG (melewati `colorScheme`); pakai `Theme.of(context).colorScheme.primary`. `Colors.transparent` = EXEMPTION sadar-sadar (nilai bebas-mode untuk scrim/immersive chrome, bukan pilihan palet). Gate: `test/core/theme/theme_authority_contract_test.dart` — sweep lib-wide (lantai >1000 file) + allowlist 3 file + lantai anti-vakum + negative proof pola (termasuk alias nama & `primaryColor`).

    • SATU FILE SATU AUTHORITY. Konsep yang dulu punya salinan per fitur kini satu rumah, dan salinan matinya dibunuh total beserta direktori kosongnya: `Province/City/District/Village` → `shared/models/wilayah_models.dart`; `PostLocation/LatLng` → `shared/entities/post_location.dart`; `BaseEntity/BaseModel` → `core/common/base_entity.dart`. DIMATIKAN: `domains/commerce/catalog/models/wilayah_models.dart`, `domains/commerce/catalog/entities/post_location.dart`, `core/src/domain/base_entity.dart`. Gate: `test/core/file_authority_contract_test.dart` — tidak boleh ada dua file `lib/` berbyte identik; tiap konsep dideklarasikan di tepat satu file; salinan yang dibunuh tetap mati.

    • RESULT SATU AUTHORITY. `Result<T>` (`core/common/result.dart`) adalah satu-satunya bentuk "hasil operasi repository" di seluruh app, dan `fold(onError, onSuccess)` mengambil **onError DULU** — seragam di setiap call site. DIMATIKAN TOTAL: `RepositoryResult<T>` (order domain; `isSuccess => data != null` sehingga sukses-membawa-null terbaca gagal, `fold` sukses-dulu, alias `failure`+`error`) dan `ContentRepositoryResult<T>` (dulu dideklarasikan di dalam file interface `content_repository.dart`, dengan `dataOrThrow` sendiri). Kedua file/kelas itu mati: `order/domain/repositories/repository_result.dart` DIHAPUS, `ContentRepositoryResult` DIHAPUS, export barrel `show RepositoryResult` dicabut, `dataOrThrow` tidak ada lagi — pakai `result.data!` setelah `isSuccess`. Ketiganya MATI, tanpa kecuali: `payment/domain/repositories/payment_repository.dart` (`RepositoryResult<T>` ber-payload `PaymentFailure`) DIHAPUS bersama `payment/domain/failures/payment_failure.dart` (+7 subclass: NetworkFailure, ValidationFailure, PaymentGatewayFailure, InsufficientBalanceFailure, PaymentExpiredFailure, PaymentNotFoundFailure, UnknownFailure) beserta `dataOrThrow`/`isFailure`-nya. **Repository payment MENERUSKAN, bukan menafsirkan**: kegagalan API diteruskan apa adanya (`code: source.errorCode`, `statusCode`, `details`) lewat satu `_forwardFailure<T>`; `_mapApiError` — yang menebak jenis dengan mencocokkan TEKS PESAN lalu membuang code backend, dan mengisi payload palsu seperti `PaymentNotFoundFailure('payment')` — DIBUNUH. Precondition lokal (id kosong / validate gagal) → `Result.error` tanpa code. Copy UI berbasis code kanonik (`api_error_codes.dart`: +`invalidPaymentStatus`, +`referenceRequired`), bukan tipe gagal buatan klien. **Vocabulary keempat & kelima `ApiResult` juga MATI.** Dua bentuk yang saling bertabrakan dibunuh sekaligus: `class ApiResult<T>` di support (fold NAMED error-KEDUA, `isSuccess => error == null`) dan `typedef ApiResult<T> = ({T? data, String? error})` yang dideklarasikan DUA KALI di domain search — bentuk record itu **tidak punya kanal error code sama sekali**, sehingga search meruntuhkan setiap kegagalan (termasuk transport) jadi `error.toString()`. Sekarang: search & support kembali ke `Result<T>` dan MENERUSKAN code kanonik. JANGAN hidupkan kembali: record `(data:, error:)` sebagai hasil repository, `fold` NAMED, `ApiResult` di klausa `show` barrel `search.dart`, atau klasifikasi kegagalan support lewat pencocokan TEKS pesan / status code sebagai STRING (`case '404'`). Jalur code: helper support meneruskan `code`+`statusCode` dari `ApiException`; `SearchApiService` punya SATU `_guard` yang mengubah `DioException` → `StructuredApiException` lewat `_apiClient.extractException` (tanpa tabel `DioExceptionType` kedua), dan impl search menerjemahkannya kembali lewat `_failure`/`_propagate`; `search_usecase` meneruskan `errorCode`/`statusCode`/`errorDetails` (dulu diratakan jadi pesan). Mapper support memutuskan dari `isTransportFailureCode(result.errorCode)` + `switch (result.statusCode)`. Sisa vocabulary "hasil/gagal" yang BELUM converge: `Either<Failure,T>` dartz di 6 use case follow (`core/errors/failure.dart` masih HIDUP lewat barrel `core.dart` — grep importer langsung memberi false negative; purge-nya DIBATALKAN dan dikembalikan ke owner), `Withdrawal.isSuccess`, `PaymentResult`/`PaymentResultStatus`. **`finance_gateway.dart` SUDAH DIPURGE** (dead file: `FinanceGateway` nol implementor, `FinanceResult`/`FinanceException` nol konsumen — vocabulary hasil keenam mati sebelum dipakai) — lihat butir DEAD FILE PURGE. Gate: `test/core/result_authority_contract_test.dart` — `Result` dideklarasikan di tepat satu file; nama duplikat (`RepositoryResult`/`ContentRepositoryResult`/**`ApiResult`**) DILARANG di `lib`&`test` **tanpa allowlist**; `ApiResult` juga masuk regex detektor deklarasi, jadi `class` MAUPUN `typedef`-nya tertangkap; support wajib memakai `isTransportFailureCode`/`result.statusCode` dan DILARANG `error.contains(`/`case '404'`; search wajib `extractException`+`StructuredApiException`+`code: error.code` dan DILARANG `DioExceptionType.` / `data: null, error:`; `repository_result.dart` + `payment_failure.dart` tetap mati; urutan `fold` (onError dulu) tidak boleh dibalik; repository payment wajib masih meneruskan `source.errorCode`/`source.statusCode` dan dilarang menamai `PaymentFailure`/`_mapApiError`; plus negative proof detektor (semua dibuktikan GAGAL dengan probe ditanam, PASS setelah dihapus).

    • TRANSPORT SATU TABEL. Kegagalan transport — permintaan yang tidak pernah menghasilkan envelope HTTP — punya identitas machine-readable: code kanonik di `core/api/api_error_codes.dart` (`BACKEND_UNREACHABLE`, `TIMEOUT`, `NETWORK_ERROR`, `SSL_ERROR`, `CANCELLED`, `UNKNOWN_ERROR`) yang mengalir keluar lewat `Result.errorCode`. Klasifikasi `DioExceptionType` → ApiException+code ada di TEPAT SATU tabel, `ApiExceptionFactory.fromTransport` (`core/api/exceptions/api_exception.dart`); `ErrorInterceptor` dan `ApiClient.extractException` hanya memanggilnya — dan `badResponse` (yang MEMANG punya envelope HTTP) sengaja tidak termasuk keluarga transport (mengembalikan `null`, bukan menebak). Satu predikat, `isTransportFailureCode(code)`, adalah satu-satunya cara bertanya "ini kegagalan transport?"; `unknownError` sengaja di luar predikat karena "tidak bisa diklasifikasi" ≠ "transport". JANGAN hidupkan kembali: literal code di luar authority, switch `DioExceptionType` kedua di file lain, `code: 'NETWORK_ERROR'` buatan domain (dulu di checkout — bahkan untuk 5xx), atau klasifikasi dengan mencocokkan TEKS pesan ('network'/'timeout'/'connection') di jalur `Result`. Pencocokan teks hanya sah untuk throw mentah yang tidak pernah punya code (Firebase SDK, `SocketException` Dart) — lihat `classifyAuthSyncError`. SUDAH converge di sesi lanjutan 6: `ApiResult` MATI TOTAL (search + support kembali ke `Result`, code diteruskan — lihat butir RESULT SATU AUTHORITY). Belum converge & diparkir: `SupportResult`/`SupportFailure` (vocabulary hasil kelima; `SupportFailureAlreadyResolved`/`CannotReopen` kini nol konstruktor karena pemetaan teks dibunuh), `auction_detail_screen` (state hanya `String`). **`retry_helper.dart` SUDAH DIPURGE** — lihat butir DEAD FILE PURGE. Gate: `test/core/api/transport_failure_classification_contract_test.dart` — tabel perilaku lewat pipeline Dio NYATA (adapter gagal → ErrorInterceptor → extractException → executeRequest → `Result.errorCode`), truth table predikat, sweep lib-wide TANPA allowlist (literal code & `case DioExceptionType.` hanya boleh di authority), positive proof konsumen memakai code, negative proof probe ditanam→GAGAL/hapus→PASS.

    • DEAD FILE PURGE — FILE MATI TIDAK BOLEH HIDUP LAGI. Dua file yang dipajang seolah kapabilitas hidup padahal NOL konsumen DIHAPUS dan dikunci: `lib/core/utils/retry_helper.dart` (284 baris; `RetryConfig`/`RetryHelper`/`RetryFutureExtension`, exponential backoff + jitter, dan `_isRetryableError` yang memutuskan retry dengan MENCOCOKKAN TEKS error 'network'/'timeout'/'500' — aturan klasifikasi yang sudah dioutlaw scope transport, plus `executeResult` yang membuang `errorCode`) dan `lib/domains/finance/finance_gateway.dart` (`abstract class FinanceGateway` + `FinanceResult<T>` + `FinanceException`, nol implementor/konsumen/barrel). Bukti kematian bukan asumsi: sweep repo-wide (`git grep` atas seluruh index semua tipe + grep `lib`/`test` termasuk file untracked) = nol rujukan di luar kedua file itu sendiri; nol barrel (`core.dart` meng-export `src/utils/...`, bukan `core/utils/` — direktori berbeda). JANGAN hidupkan kembali: identifier atau path kedua file itu di `lib`/`test`/pubspec (jalur kebangkitan nyata adalah baris `import`/`export`), atau membangun retry dengan mencocokkan teks error. Kalau retry dibutuhkan lagi, bangun di jalur `Result` lewat `isTransportFailureCode`/`errorCode` — dan kebijakan auto-retry backoff pada sync tetap DIBUNUH (Stage 3B), tidak boleh dikembalikan. Purge ini TIDAK memutuskan boundary commerce↔finance: klaim "commerce MUST go through FinanceGateway" di dok file itu sudah dibatalkan kenyataan dua arah (commerce→finance DAN finance→commerce) dan sekarang hanya hidup sebagai temuan parkir P1 yang butuh keputusan owner. Gate: `test/core/dead_file_purge_contract_test.dart` — kedua file wajib tetap mati; sweep identifier (word-boundary) + fragmen path atas `lib`+`test`+`pubspec.yaml` tanpa allowlist (dokumen dikecualikan karena ledger mencatat purge ini dengan nama); lantai surface + tiga live-neighbour wajib masih ada (purge harus ter-scope, bukan menghapus direktori); negative proof 2 probe (file dihidupkan ulang → GAGAL; satu baris komentar di file hidup → GAGAL).

    • MEDIA SATU ENGINE. `MediaUploadOrchestrator` + `MediaUploadConfig.for*` adalah satu-satunya jalur pick → validasi → upload; `MediaGridUploader`/`CompactMediaStrip` satu-satunya grid. JANGAN hidupkan kembali handler media per-domain, widget text-input media lama, atau cropper kedua (`custom_image_cropper`).

    Pre-existing debt terverifikasi baseline (bukan regression konvergensi): saved_item contract test, promoted feed HTTP-mock family, StableNetworkImage header test, /settings toggle, payment notifier compile debt. Sumber kebenaran commit: e1fb4ae, c9ab974, a00b230, 94c1ce8, 7b0b1c8, 64e9f24, 23d36d9.
