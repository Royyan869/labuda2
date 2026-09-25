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

    Pre-existing debt terverifikasi baseline (bukan regression konvergensi): saved_item contract test, promoted feed HTTP-mock family, StableNetworkImage header test, /settings toggle, payment notifier compile debt. Sumber kebenaran commit: e1fb4ae, c9ab974, a00b230, 94c1ce8, 7b0b1c8, 64e9f24, 23d36d9.
