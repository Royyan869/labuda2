# LABUDA — ATURAN KERJA PENGEMBANGAN, AUDIT, DAN PENUTUPAN DOMAIN

## 1. Tujuan Utama

Pengembangan Labuda harus bergerak menuju aplikasi yang:

- benar secara bisnis;
- proper, scalable, robust, dan mudah dipelihara;
- aman untuk data dan transaksi uang;
- memiliki satu sumber kebenaran;
- dapat diuji dan digunakan secara nyata;
- tidak terus berputar dalam audit tanpa akhir;
- tidak menghidupkan kembali desain yang sudah ditolak.

Tim tetap terdiri dari:

- **Owner:** menentukan business truth dan menerima pengalaman produk.
- **ChatGPT:** memegang kontrol lintas produk, backend, mobile, admin, database, runtime, DevOps, QA, arsitektur, prioritas, dan closure.
- **Codex:** menjadi eksekutor teknis di repository, bukan penentu desain atau kebijakan bisnis.

Tidak perlu menambah tool atau anggota tim baru. Fokus utama adalah memperbaiki cara kerja.

---

## 2. Prinsip Dasar

### 2.1 Satu kebenaran

Setiap domain hanya boleh memiliki satu authority yang aktif.

Tidak boleh ada:

- dua aturan bisnis yang menghasilkan keputusan berbeda;
- dua model data untuk konsep yang sama;
- fallback ke desain lama;
- compatibility bridge yang tidak dibutuhkan;
- helper lama yang masih dapat dipanggil;
- route, provider, DTO, schema, atau UI alternatif yang mempertahankan desain tertolak.

Jika desain baru sudah dinyatakan benar, seluruh producer, consumer, lifecycle, test, schema, dokumentasi, dan residue desain lama harus diselaraskan atau dihapus.

### 2.2 Git bukan authority produk

Current filesystem dan business truth terbaru adalah sumber kebenaran implementasi.

Git digunakan untuk:

- backup;
- diff;
- checkpoint;
- integrasi;
- keselamatan repository.

Git history tidak boleh digunakan untuk menghidupkan kembali desain lama, rollback ke implementasi tertolak, atau menjadi sumber keputusan produk.

### 2.3 Audit untuk mengambil keputusan

Audit bukan tujuan akhir.

Audit harus berhenti ketika sudah cukup untuk membuktikan:

- perilaku yang salah;
- root cause;
- producer;
- consumer;
- invariant yang dilanggar;
- authority yang benar;
- impacted paths;
- perubahan yang diperlukan.

Setelah bukti cukup, lanjutkan ke implementasi. Jangan memperluas audit hanya karena masih mungkin menemukan ketidaksempurnaan lain.

---

## 3. Kapan Audit Mendalam Diperlukan

Audit mendalam wajib dilakukan apabila:

- terdapat dua authority atau dua desain aktif;
- desain lama pernah muncul kembali;
- business truth berubah;
- perubahan menyentuh uang, ledger, pembayaran, refund, withdrawal, authorization, database, concurrency, atau data integrity;
- producer dan consumer tidak konsisten;
- runtime berbeda dari test atau dokumentasi;
- desain lama belum pernah dibunuh secara end-to-end;
- ada bukti bahwa area yang sebelumnya hijau sebenarnya belum tertutup.

Audit mendalam harus memeriksa seperlunya:

- business invariant;
- producer;
- consumer;
- lifecycle;
- runtime wiring;
- database/schema;
- state/cache;
- route/navigation;
- background worker;
- test dan fixture;
- competing authority;
- fallback dan compatibility path;
- residue kode, komentar, dokumentasi, dan naming yang masih dapat memengaruhi implementasi.

Audit pertama tidak boleh langsung dianggap sebagai kesimpulan final apabila baru memetakan permukaan.

---

## 4. Kapan Audit Mendalam Tidak Diperlukan

Jangan membuka audit besar hanya karena:

- nama file atau helper kurang ideal;
- struktur dapat dibuat lebih elegan;
- ada komentar lama tanpa dampak runtime;
- terdapat analyzer info atau style issue;
- Codex menemukan cara implementasi lain;
- sebuah fitur tambahan mungkin berguna;
- ada duplikasi mati yang tidak dapat dipanggil dan tidak membingungkan authority;
- tidak ada bug, failing proof, atau perubahan bisnis.

Masalah seperti itu masuk backlog engineering debt dan tidak boleh menghalangi progress menuju internal beta.

---

## 5. Aturan Anti-Resurrection

Desain yang sudah ditolak harus menggunakan prinsip:

# Kill Once, Lock Forever

Penutupan desain tidak cukup hanya dengan menghilangkan tampilan.

Setiap desain yang dibunuh harus memiliki lima bukti.

### 5.1 Canonical truth

Tuliskan dengan jelas satu desain yang sah dan harus tetap berlaku.

Contoh:

- siapa yang memiliki authority;
- state dan lifecycle yang diperbolehkan;
- field yang berlaku;
- tindakan yang tersedia;
- aturan backend dan frontend;
- perilaku saat error, expired, atau suspended.

### 5.2 Forbidden design

Tuliskan secara eksplisit apa yang tidak boleh kembali.

Contoh:

- toggle `allow comment` tidak boleh kembali ke create content;
- tombol yang sudah disederhanakan tidak boleh diduplikasi;
- opsi shipping lama tidak boleh dipasang kembali;
- route lama tidak boleh didaftarkan ulang;
- DTO atau payload lama tidak boleh diterima;
- helper dan fallback lama tidak boleh digunakan;
- field yang telah dihapus tidak boleh dibentuk ulang hanya karena masih muncul pada fixture atau dokumentasi lama.

Codex harus memperlakukan residue desain tertolak sebagai sesuatu yang harus dihapus atau dilaporkan, bukan sebagai petunjuk bahwa fitur tersebut perlu dikembalikan.

### 5.3 Removal manifest

Periksa dan bersihkan seluruh lapisan yang relevan:

- UI;
- reusable widget;
- route/navigation;
- form state;
- provider/controller;
- model;
- DTO/request/response;
- mapper;
- repository;
- service/domain;
- backend handler;
- schema/migration;
- cache;
- worker;
- tests dan fixtures;
- docs dan comments;
- compatibility bridge;
- fallback;
- feature flag lama.

Tidak semua domain menyentuh semua lapisan, tetapi setiap lapisan harus dipertimbangkan.

### 5.4 Negative contract

Tambahkan proof yang gagal apabila desain lama kembali.

Contoh:

- kontrol lama tidak ditemukan pada screen;
- request tidak membawa field lama;
- route lama tidak terdaftar;
- hanya satu CTA yang tersedia;
- parser menolak payload lama;
- shipping form tidak menampilkan opsi tertolak;
- source tidak lagi menggunakan helper atau type lama;
- expired seller tidak memperoleh market-increasing action;
- action yang diperbolehkan tetap dapat dilakukan.

Test positif membuktikan desain baru bekerja.

Test negatif membuktikan desain lama tetap mati.

Keduanya dibutuhkan untuk domain yang pernah mengalami resurrection.

### 5.5 Domain lock

Setelah seluruh proof lolos, domain ditandai:

`CLOSED — CANONICAL DESIGN LOCKED`

Domain terkunci tidak boleh dibuka kembali tanpa trigger yang sah.

---

## 6. Trigger Sah untuk Membuka Kembali Area Hijau

Area yang sudah hijau hanya boleh dibuka kembali apabila:

- owner menemukan bug runtime nyata;
- test, build, migration, atau integration proof gagal;
- perubahan domain lain terbukti berdampak langsung;
- kontrak backend dan consumer berubah;
- ditemukan authority ganda yang masih aktif;
- business truth resmi berubah;
- ada risiko keamanan, uang, data, atau authorization;
- negative contract menunjukkan regresi.

Area tidak boleh dibuka kembali hanya untuk cleanup tambahan tanpa dampak nyata.

---

## 7. Klasifikasi Masalah

Setiap temuan harus diklasifikasikan sebelum dikerjakan.

### P0 — Stop seluruh release

Contoh:

- kehilangan atau korupsi uang/data;
- unauthorized financial action;
- database rusak;
- settlement salah;
- security breach;
- aplikasi utama tidak dapat berjalan.

### P1 — Blocker domain atau internal beta

Contoh:

- core flow tidak dapat diselesaikan;
- authorization salah;
- seller/buyer memperoleh hak yang salah;
- mobile dan backend berbeda;
- state rusak setelah lifecycle tertentu;
- transaksi, order, auction, shipping, atau subscription salah;
- desain ganda menghasilkan perilaku runtime berbeda.

### P2 — Operational atau engineering debt

Contoh:

- UX membingungkan tetapi flow masih dapat diselesaikan;
- observability kurang;
- struktur kurang ideal;
- test coverage edge case belum lengkap;
- duplikasi yang belum aktif tetapi berisiko membingungkan.

P2 tidak otomatis menghentikan seluruh pengujian.

### P3 — Cosmetic/nonblocking

Contoh:

- style;
- komentar;
- analyzer info;
- wording minor;
- naming tanpa dampak authority;
- refactor estetika.

P3 tidak boleh menghentikan progress.

---

## 8. Batas Pekerjaan Aktif

Maksimum hanya boleh ada:

- satu scope implementasi aktif;
- satu scope verifikasi aktif;
- satu prioritas berikutnya dalam backlog.

Jangan menjalankan banyak domain secara bersamaan dalam satu percakapan.

Temuan di luar scope:

- dicatat;
- diklasifikasikan;
- tidak langsung dikerjakan;
- tidak boleh memperluas perubahan aktif kecuali P0/P1 yang benar-benar memblokir scope.

---

## 9. Siklus Kerja Wajib

Setiap task mengikuti urutan berikut.

### Tahap 1 — Problem statement

Nyatakan satu masalah konkret.

Contoh:

> Expired seller tidak dapat menarik For Sale meskipun withdrawal adalah risk-reducing action yang diperbolehkan.

Hindari scope kabur seperti:

> Audit seller secara global.

### Tahap 2 — Business truth

Nyatakan invariant yang tidak boleh dilanggar.

### Tahap 3 — Focused audit

Codex membuktikan:

- current behavior;
- root cause;
- producer;
- consumer;
- impacted paths;
- competing design;
- residue yang relevan.

Audit harus berhenti ketika keputusan implementasi sudah cukup aman.

### Tahap 4 — Design decision

ChatGPT menentukan:

- canonical authority;
- perubahan yang proper;
- desain yang harus dibunuh;
- path yang boleh disentuh;
- path yang dilindungi;
- acceptance gate.

Codex tidak boleh mengambil keputusan bisnis sendiri.

### Tahap 5 — Implementation

Codex:

- memperbaiki root cause;
- memperbarui semua consumer yang relevan;
- menghapus desain tertolak;
- tidak menambah compatibility layer;
- tidak mengubah area di luar scope;
- menambahkan proof yang diperlukan.

### Tahap 6 — Verification

Wajib menjalankan proof berlapis:

1. focused test;
2. relevant integration test;
3. build/analyze lapisan terdampak;
4. residue search untuk desain yang dihapus;
5. diff dan manifest review;
6. runtime/owner retest apabila perubahan memang memerlukan bukti perangkat nyata.

Full regression dijalankan saat closure besar, perubahan lintas lapisan, atau release gate—bukan otomatis untuk setiap perubahan kecil.

### Tahap 7 — Closure

Scope hanya boleh ditutup apabila:

- invariant terbukti;
- focused tests lolos;
- relevant integration proof lolos;
- build lapisan terdampak lolos;
- desain lama tidak lagi memiliki jalur aktif;
- negative contract tersedia jika diperlukan;
- diff tidak membawa perubahan di luar scope;
- protected paths tidak tersentuh;
- git status dipahami;
- owner retest selesai jika diperlukan.

Setelah itu domain dikunci dan tidak dibuka kembali tanpa trigger sah.

---

## 10. Aturan Prompt untuk Codex

Setiap prompt Codex harus memuat:

### Objective

Satu hasil yang harus dicapai.

### Canonical truth

Aturan bisnis atau teknis yang harus dipertahankan.

### Forbidden design

Desain yang telah ditolak dan tidak boleh dihidupkan kembali.

### Scope

Path dan lapisan yang boleh diperiksa atau diubah.

### Protected/out of scope

Area yang tidak boleh disentuh.

### Required proof

Test, build, residue search, runtime proof, dan laporan yang harus disediakan.

### Stop conditions

Codex harus berhenti dan melapor apabila:

- diperlukan keputusan bisnis baru;
- implementasi membutuhkan desain terlarang;
- ditemukan P0/P1 di luar scope;
- repository baseline tidak sesuai;
- protected path perlu disentuh;
- root cause tidak dapat dibuktikan.

Codex tidak boleh melanjutkan dengan asumsi produk sendiri.

---

## 11. Bentuk Laporan Codex

Setiap laporan minimal harus berisi:

1. **Verdict**
2. **Root cause**
3. **Canonical behavior setelah perbaikan**
4. **Daftar file yang berubah**
5. **Daftar desain/residue yang dihapus**
6. **Tests dan build yang dijalankan**
7. **Hasil proof**
8. **Temuan di luar scope**
9. **Risiko atau bagian yang belum terbukti**
10. **Git status**
11. **Apakah owner retest diperlukan**

Pernyataan “tests pass” tanpa nama command dan hasil yang jelas tidak cukup.

---

## 12. Aturan Owner Testing

Owner testing tidak harus menunggu seluruh aplikasi sempurna.

Pengujian dilakukan per jalur yang sudah aman.

Contoh:

- For Sale hijau → uji For Sale;
- chat dan shipping hijau → uji chat dan shipping;
- auction belum hijau → jangan gunakan auction lebih dahulu;
- withdrawal belum siap → jangan lakukan transaksi withdrawal.

Saat menemukan bug:

- laporkan perilaku yang terlihat;
- tuliskan langkah reproduksi;
- sebutkan akun dan kondisi;
- jangan menebak penyebab teknis.

ChatGPT yang menentukan severity, root-cause audit, dan prioritas.

---

## 13. Aturan Refactor

Refactor sekarang hanya jika:

- ada dua authority aktif;
- desain lama dapat dipanggil;
- desain ganda menghasilkan perilaku berbeda;
- compatibility residue mempertahankan kontrak tertolak;
- arsitektur sekarang menghalangi perubahan penting;
- ada risiko keamanan, data, uang, atau authorization;
- area tersebut memang sedang berada dalam scope perbaikan.

Jangan melakukan refactor hanya karena “mumpung ketemu”.

Debt yang aman dicatat untuk fase berikutnya.

---

## 14. Definisi Kecepatan yang Benar

Bergerak cepat bukan berarti:

- mengabaikan test;
- membiarkan authority ganda;
- menumpuk compatibility layer;
- menerima desain salah;
- membuat UI cepat tetapi runtime rapuh.

Bergerak cepat berarti:

- memilih masalah yang paling penting;
- menghentikan audit ketika bukti sudah cukup;
- memperbaiki root cause;
- membunuh desain tertolak secara total;
- memasang negative contract;
- menutup scope;
- tidak membuka area hijau tanpa alasan;
- segera melanjutkan ke penggunaan nyata.

---

## 15. Prinsip Akhir

Gunakan prinsip berikut sebagai aturan pengambilan keputusan:

> Audit untuk membuat keputusan, bukan untuk mencari seluruh ketidaksempurnaan.

> Satu scope, satu invariant, satu acceptance gate.

> Desain salah dibunuh end-to-end, bukan hanya disembunyikan dari UI.

> Desain yang sudah dibunuh harus dikunci dengan negative contract.

> Area hijau tidak dibuka kembali tanpa bukti regresi atau perubahan business truth.

> Temuan di luar scope dicatat, bukan langsung dikerjakan.

> P0/P1 diperbaiki sekarang; P2/P3 tidak otomatis menghentikan progress.

> Current filesystem dan keputusan terbaru adalah product truth; Git history bukan sumber desain.

> Owner menentukan pengalaman dan kebijakan bisnis; ChatGPT menentukan arah teknis; Codex mengeksekusi.

> Target akhirnya bukan audit sempurna, tetapi aplikasi yang benar, aman, dapat digunakan, dan terus bergerak menuju internal beta serta produksi.

---

# 16. DOCTRINE BARU — CROSS-SESSION CONVERGENCE & ANTI-RESURRECTION

Bagian ini merupakan penegasan wajib atas cara kerja Labuda setelah ditemukan bahwa pengembangan lintas sesi dan lintas chat dapat menghidupkan kembali desain yang sebelumnya telah dinyatakan mati.

## 16.1 Masalah Utama yang Harus Dicegah

Labuda telah dikembangkan dalam banyak sesi dan melalui banyak putaran audit/fix. Akibatnya, sebuah domain dapat terlihat hijau secara lokal tetapi masih menyimpan:

- producer lama;
- consumer lama;
- DTO lama;
- mapper lama;
- test fixture lama;
- route/provider lama;
- schema/migration lama;
- komentar atau dokumentasi yang memberi sinyal desain lama;
- fallback;
- alias;
- compatibility shim;
- dead/zombie code.

Masalahnya bukan hanya adanya bug.

Masalah yang lebih berbahaya adalah **competing authority**:

> Dua bagian sistem memiliki pemahaman berbeda tentang konsep bisnis yang sama.

Jika hal tersebut terjadi, setiap bagian dapat terlihat benar sendiri tetapi sistem secara keseluruhan tidak memiliki satu kebenaran.

Tujuan pengembangan sekarang adalah **convergence**, bukan sekadar membuat setiap fitur hijau satu per satu.

## 16.2 Cross-Session Rule

Setiap sesi baru harus memperlakukan current codebase sebagai factual implementation state, tetapi tidak boleh menganggap setiap artefak yang ditemukan di codebase sebagai business truth.

Kode yang masih ada dapat merupakan:

- canonical implementation;
- stale implementation;
- residue;
- test-only artifact;
- historical artifact;
- accidental duplicate authority.

Karena itu agent WAJIB membedakan:

`factual existence` ≠ `canonical authority`.

Keputusan bisnis berasal dari Owner dan keputusan teknis yang telah disepakati bersama.

PRD adalah referensi product/technology blueprint, tetapi bukan otoritas final apabila bertentangan dengan keputusan Owner atau canonical implementation yang telah disepakati.

## 16.3 Authority First

Sebelum memperbaiki sebuah konsep, tentukan terlebih dahulu:

1. Apa business truth-nya?
2. Apa canonical authority-nya?
3. Siapa producer-nya?
4. Siapa consumer-nya?
5. Apa contract-nya?
6. Apa lifecycle/state-nya?
7. Apakah ada authority lain yang mengklaim konsep yang sama?

Jika ditemukan dua authority:

> Jangan membuat keduanya hidup berdampingan.

Tentukan satu yang canonical.

Kemudian **bunuh authority yang lain** beserta consumer dan residue yang membuatnya dapat hidup kembali.

## 16.4 "Kill Once, Lock Forever"

Setiap desain yang sudah ditolak harus diperlakukan sebagai desain terlarang.

Contoh:

- Content tidak memiliki type Post/Request;
- Content tidak memiliki allowComments;
- `contents.visibility` adalah authority visibility;
- seller bukan `users.role=seller`;
- username adalah identity authority;
- payment/order/pricing memiliki authority masing-masing sesuai canonical domain.

Jika desain lama ditemukan lagi:

1. Jangan menganggapnya sebagai requirement baru.
2. Jangan menghidupkannya untuk membuat test lama hijau.
3. Jangan membuat alias.
4. Jangan membuat compatibility parser.
5. Jangan membuat fallback ke desain lama.
6. Cari producer/consumer/residue-nya.
7. Hapus apabila memang obsolete.
8. Tambahkan negative proof bila risiko resurrection tinggi.

Tujuan cleanup bukan sekadar membuat grep menjadi nol.

Tujuannya adalah memastikan **tidak ada jalur aktif yang dapat menghidupkan desain lama kembali**.

## 16.5 Cleanup Adalah Penutup Scope

Cleanup bukan pekerjaan tambahan yang boleh ditunda.

Untuk setiap scope yang selesai:

`Audit → Decision → Implement → Proof → Scoped Cleanup → Residue Search → Regression → Close`

Cleanup harus dilakukan sebelum pindah ke area lain.

Namun cleanup harus:

- bounded;
- berbasis bukti;
- hanya pada scope yang telah selesai;
- tidak berubah menjadi global cleanup.

Jangan melakukan "sekalian bersihkan seluruh repository".

## 16.6 Safe Removal Principle

Labuda from zero tidak membutuhkan backward compatibility terhadap production data atau historical implementation.

Karena data production = 0, sesuatu yang:

- tidak canonical;
- tidak diperlukan oleh current scope;
- tidak memiliki consumer yang sah;
- tidak menjadi prerequisite;
- terbukti obsolete;

lebih baik **dihapus** daripada dipertahankan sebagai compatibility.

Analogi kerja:

> Kita sedang merakit sepeda dan menemukan baut yang tidak sesuai. Jangan memaksa baut tersebut agar cocok. Jika baut itu belum dibutuhkan dan desainnya belum jelas, lepaskan/buang. Ketika benar-benar dibutuhkan, buat atau pilih baut baru yang tepat untuk desain canonical saat itu.

Prinsip ini berlaku untuk:

- code;
- DTO;
- API field;
- schema;
- test fixture;
- helper;
- route;
- provider;
- mapper;
- migration;
- documentation;
- configuration;
- feature flag.

### Batas penting

"Belum dipahami" tidak otomatis berarti "hapus".

Sebelum menghapus, pastikan tidak sedang menghapus:

- canonical business rule;
- required production path;
- security control;
- financial invariant;
- data-integrity mechanism;
- prerequisite scope;
- active consumer yang sah.

Jika aman dan tidak dibutuhkan sekarang, removal lebih sehat daripada compatibility.

Jika ternyata dibutuhkan di masa depan, buat ulang berdasarkan kebutuhan nyata dan canonical design saat itu.

## 16.7 Test dan Fixture Bukan Business Authority

Test lama tidak boleh memaksa business design.

Jika test fixture menggunakan kontrak lama sementara backend canonical menggunakan kontrak baru:

> Perbaiki atau hapus fixture/test.

Jangan mengubah production code agar kompatibel dengan fixture lama kecuali ada alasan canonical yang sah.

Jika test sudah obsolete:

> Hapus test tersebut.

Namun setiap penghapusan test harus memiliki alasan dan, bila kontraknya penting, digantikan dengan proof canonical yang lebih tepat.

## 16.8 Positive + Negative Proof

Untuk domain yang pernah mengalami resurrection:

### Positive proof

Membuktikan desain baru bekerja.

### Negative proof

Membuktikan desain lama tidak lagi memiliki jalur aktif.

Contoh:

- field lama tidak dikirim;
- route lama tidak terdaftar;
- old type tidak diterima;
- old provider tidak digunakan;
- old helper tidak dipanggil;
- old UI control tidak muncul;
- old schema writer tidak ada.

Tidak semua domain memerlukan negative test formal, tetapi **residue search dan authority review wajib dilakukan** untuk desain yang sebelumnya sering hidup kembali.

## 16.9 Runtime Proof > Local Green

"Tests pass" tidak otomatis berarti domain selesai.

Jika masalah menyangkut runtime integration, proof harus mengikuti jalur nyata:

`DB → Backend → API → Auth → Mobile → State → UI`

Gunakan test untuk membuktikan contract, tetapi gunakan runtime proof ketika masalah memang berada pada integration/runtime path.

Contoh Feed:

`real PostgreSQL → real backend → authenticated GET /feed → real wire JSON → Flutter DTO → mapper → rendered feed`

Itu lebih kuat daripada sekadar mock fixture hijau.

## 16.10 Test Data Is Disposable

Data development/test bukan production authority.

Jika data test:

- inconsistent;
- stale;
- menghalangi proof;
- berasal dari desain lama;
- memiliki graph yang tidak valid;

data tersebut boleh:

- dihapus;
- di-reset;
- di-reseed;
- dibuat ulang.

Data test tidak boleh menjadi alasan untuk:

- backward compatibility;
- alias;
- legacy API;
- schema residue;
- business compromise.

Walaupun sebagian akun menggunakan email atau identity provider yang nyata, statusnya dalam environment development/test tetap tidak menjadikannya production data yang harus mengendalikan architecture.

## 16.11 Jangan Takut Membuat Ulang

Jika sebuah implementasi lama sudah dibuang dan suatu saat kebutuhan nyata muncul kembali:

> Jangan menghidupkan implementasi lama hanya karena "dulu pernah ada".

Buat ulang dari:

`current business requirement → canonical authority → current architecture → current proof`

Dengan demikian implementasi baru tidak mewarisi asumsi lama secara tidak sengaja.

## 16.12 Domain Closure Contract

Sebelum sebuah scope ditutup, agent harus menjawab:

1. Apa satu canonical authority?
2. Apa desain yang dibunuh?
3. Apakah producer lama sudah hilang?
4. Apakah consumer lama sudah hilang?
5. Apakah DTO/schema/route/provider lama sudah hilang jika relevan?
6. Apakah test fixture lama sudah diselaraskan atau dihapus?
7. Apakah ada fallback/alias/compatibility?
8. Apakah runtime proof tersedia bila diperlukan?
9. Apakah negative proof/residue search dilakukan?
10. Apakah perubahan tetap bounded?
11. Apakah ada ambiguity bisnis yang belum diputuskan?
12. Apakah scope benar-benar aman untuk ditinggalkan oleh sesi berikutnya?

Jika jawaban belum cukup:

> Jangan declare PASS.

## 16.13 Green Area Protection

Scope yang telah CLOSED tidak boleh dibuka kembali hanya karena:

- agent berikutnya menemukan nama lama;
- ada test lama yang ingin dibuat hijau;
- ada refactor yang terlihat lebih bagus;
- ada kemungkinan kebutuhan masa depan;
- ada historical implementation yang terlihat mudah dipakai.

Scope hanya dibuka kembali apabila ada trigger sah:

- runtime bug;
- regression;
- failing integration/build/migration proof;
- authority conflict nyata;
- perubahan backend contract;
- security/data/financial/authorization risk;
- perubahan business truth dari Owner.

## 16.14 Agent Skepticism Protocol

Laporan agent harus selalu diperlakukan sebagai **klaim yang harus diverifikasi**, bukan sebagai fakta final.

Khusus klaim:

- "production code clean";
- "no competing authority";
- "root cause found";
- "test failure hanya infrastructure";
- "legacy ini legitimate";
- "safe to delete";
- "scope closed";

agent wajib memberikan evidence yang cukup.

ChatGPT harus mencoba mencari counter-evidence sebelum menerima closure.

Jika laporan mengatakan PASS tetapi evidence tidak cukup:

> status tetap UNPROVEN, bukan PASS.

## 16.15 Scope Discipline

Satu waktu hanya satu scope aktif.

Temuan di luar scope:

- catat;
- klasifikasikan;
- jangan langsung dikerjakan.

Pengecualian hanya untuk P0/P1 yang benar-benar memblokir scope atau release safety.

Scoped cleanup tidak boleh berubah menjadi global cleanup.

Tujuannya adalah meninggalkan setiap scope dalam kondisi bersih tanpa kehilangan kontrol atas perubahan.

## 16.16 Prompt Contract untuk Agent

Setiap prompt harus secara eksplisit menyebut:

### Objective

Satu hasil konkret.

### Canonical truth

Business/technical invariant yang wajib dipertahankan.

### Forbidden design

Desain lama yang telah dibunuh.

### Scope

Area yang boleh diperiksa dan diubah.

### Protected / Out of scope

Area yang tidak boleh disentuh.

### Required proof

Test, integration, build, runtime, residue search, dan regression yang wajib dilakukan.

### Cleanup requirement

Semua residue yang terbukti obsolete dalam scope wajib dibersihkan sebelum closure.

### Stop conditions

Agent wajib berhenti apabila:

- business decision diperlukan;
- canonical authority tidak dapat ditentukan;
- protected path harus disentuh;
- root cause tidak dapat dibuktikan;
- P0/P1 di luar scope memblokir pekerjaan;
- current baseline tidak dapat dipercaya;
- solusi memerlukan desain yang telah ditolak.

## 16.17 Business Decision Gate

Owner adalah otoritas final untuk keputusan bisnis.

ChatGPT wajib memberikan masukan apabila menemukan requirement yang:

- tidak masuk akal;
- berpotensi merusak UX;
- berisiko secara teknis;
- bertentangan dengan domain invariant;
- tidak scalable;
- membuka security/financial risk.

Tetapi ChatGPT tidak boleh mengubah business policy secara diam-diam.

Jika perlu keputusan:

`Fakta → Risiko → Opsi → Rekomendasi → Owner Decision`

Setelah Owner memutuskan:

`Decision → Canonical implementation → Cleanup → Proof`

## 16.18 Target Arsitektur

Labuda harus dibangun sebagai aplikasi yang:

- proper;
- scalable;
- robust;
- secure;
- observable;
- maintainable;
- mudah dikembangkan lintas sesi;
- memiliki satu authority per concern;
- tidak membawa sejarah legacy ke production.

Kecepatan tidak diukur dari banyaknya code yang ditulis.

Kecepatan diukur dari seberapa cepat kita:

`menentukan kebenaran → memperbaiki root cause → membunuh desain salah → membuktikan → menutup scope → bergerak maju`

---

# 17. CROSS-SESSION HANDOFF STANDARD

Setiap scope yang selesai harus meninggalkan codebase dalam kondisi yang dapat dipahami oleh agent pada sesi berikutnya.

Minimum handoff:

- scope;
- canonical authority;
- business decisions yang relevan;
- forbidden legacy;
- perubahan yang dilakukan;
- cleanup yang dilakukan;
- proof;
- known limitations;
- unresolved Owner decisions;
- exact scope status.

Jangan meninggalkan "mental state" hanya di chat.

Jika keputusan penting belum tercermin di codebase atau dokumentasi kerja, risiko resurrection tetap tinggi.

---

# 18. FINAL OPERATING RULE

Gunakan aturan ini sebagai ringkasan paling penting:

> **Satu konsep bisnis = satu authority.**

> **Satu scope = satu invariant = satu acceptance gate.**

> **PRD adalah referensi, bukan final business authority.**

> **Owner memutuskan business truth; ChatGPT menjaga arah teknis dan mengoreksi requirement yang bermasalah; agent mengeksekusi.**

> **Current codebase adalah factual implementation state, bukan bukti bahwa semua yang ada di dalamnya masih benar.**

> **Git adalah backup, bukan sumber kebenaran dan bukan mesin waktu.**

> **Tidak ada rollback, restore, alias, atau backward compatibility hanya demi mempertahankan sejarah.**

> **Desain yang salah dibunuh end-to-end.**

> **Cleanup adalah penutup scope, tetapi selalu bounded dan evidence-based.**

> **Test data boleh dibuang; test lama tidak boleh mengendalikan architecture.**

> **Jika sesuatu yang belum diperlukan aman untuk dihapus, lebih baik hapus dan buat ulang dengan desain yang benar ketika benar-benar dibutuhkan daripada mempertahankan zombie.**

> **Positive proof membuktikan desain baru hidup; negative proof/residue search membantu memastikan desain lama tetap mati.**

> **Agent report adalah klaim, bukan kebenaran. Verifikasi sebelum menerima PASS.**

> **Jika business truth ambigu, STOP dan tanya Owner.**

> **Jika technical truth salah, perbaiki.**

> **Jika authority ganda, pilih satu dan bunuh yang lain.**

> **Jika sesuatu sudah CLOSED, jangan buka lagi tanpa trigger yang sah.**

> **Tujuan akhir bukan codebase yang penuh compatibility, tetapi aplikasi Labuda yang sehat, bersih, proper, scalable, robust, dan siap berkembang tanpa terus menghidupkan kesalahan masa lalu.**

MAP → AUTHORITY → BUSINESS GATE → TRACE → PROVE → FIX → VERIFY → SCOPED CLEANUP → SKEPTICAL REVIEW → CLOSE.

# cleanup bukan sekadar merapikan file. Dalam konteks Labuda, yang harus dibuang ketika sudah terbukti obsolete adalah:

dead code
zombie implementation
duplicate authority
obsolete branch
compatibility/fallback shim yang tidak diperlukan
producer/consumer lama
DTO/schema/test yang hanya menopang desain rejected
stale comments/docs yang masih mengarahkan ke desain lama
test contract yang bertentangan dengan business truth
residue dari desain sebelumnya

Sementara yang belum terbukti obsolete tidak boleh disentuh sembarangan.

PRINCIPLE:
Setiap residu/dead code/zombie implementation yang terbukti obsolete WAJIB dihapus.
"Masih dipakai" bukan alasan cukup. Trace business purpose dan authority-nya.
"Deprecated tapi masih aktif" WAJIB diselesaikan statusnya.

# Prinsip yang dikunci

Jika ditemukan desain yang rumit tapi sebenarnya bisa dibuat sederhana tanpa mengurangi nilai bisnis
Business truth → architecture → implementation

Bukan:
kode lama → tambal → tambah kondisi → tambah compatibility → akhirnya terlihat benar.

Tetapi:
business truth → tentukan model sederhana → satu authority → implementasikan ulang.
