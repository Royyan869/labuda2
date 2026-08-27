# LABUDA — SOURCE OF TRUTH, GIT, GITHUB, RENDER, DAN ATURAN RECOVERY

## 1. Tujuan Dokumen

Dokumen ini menjelaskan hubungan antara:

- local codebase;
- Git;
- GitHub;
- Render;

serta aturan yang wajib dipatuhi oleh Owner, ChatGPT, Codex, dan developer lain ketika bekerja pada repository Labuda.

Tujuan utamanya adalah mencegah developer menganggap GitHub atau Git history sebagai sumber kebenaran produk, lalu secara tidak sengaja mengembalikan codebase ke kondisi lama yang sudah banyak masalahnya.

---

## 2. Prinsip Utama

### LOCAL CODEBASE ADALAH PRODUCT TRUTH

Untuk pekerjaan pengembangan Labuda, **current local filesystem/codebase adalah source of truth utama**.

Alasannya:

1. Pengembangan Labuda berlangsung terutama di local codebase.
2. Setiap scope yang dikerjakan bertujuan memperbaiki root cause, menghapus residue, menyatukan authority, dan menyelesaikan masalah yang ditemukan.
3. Seiring development berjalan, local codebase menjadi semakin bersih dan semakin dekat dengan business truth terbaru.
4. GitHub tidak selalu mengikuti kondisi local terbaru karena tidak semua owner work boleh atau perlu langsung dipush.
5. Commit lama di Git/GitHub dapat berisi desain, bug, residue, atau implementasi yang kemudian sudah diperbaiki dan tidak boleh dihidupkan kembali.

Karena itu:

> **Git/GitHub bukan product truth dan bukan sumber desain.**

Git digunakan sebagai alat keselamatan, checkpoint, integrasi, dan backup.

---

## 3. Status Masing-Masing Sistem

### 3.1 Local Codebase

Local codebase adalah:

- sumber kebenaran implementasi saat ini;
- tempat development utama;
- tempat perubahan owner dan developer berlangsung;
- baseline utama untuk audit dan implementasi;
- representasi codebase yang sedang dikonvergensikan menuju kondisi production-ready.

Jika local codebase dan GitHub berbeda, **jangan otomatis menganggap GitHub lebih benar**.

Periksa current filesystem dan business truth terlebih dahulu.

---

### 3.2 Git

Git adalah alat untuk:

- checkpoint;
- diff;
- history;
- recovery dalam kondisi tertentu;
- integrasi;
- keselamatan repository.

Git history **bukan sumber business truth**.

Commit lama tidak boleh dianggap sebagai desain yang masih valid hanya karena commit tersebut pernah bekerja atau pernah berada di `main`.

---

### 3.3 GitHub

GitHub terutama digunakan sebagai:

**BACKUP / REMOTE SAFETY COPY**

Tujuan utamanya adalah menjaga salinan codebase pada remote apabila terjadi masalah pada PC atau local drive, misalnya:

- kerusakan drive;
- kehilangan filesystem;
- kegagalan hardware;
- corruption pada local repository;
- kebutuhan recovery setelah local environment tidak dapat digunakan.

GitHub bukan mirror wajib dari setiap perubahan local.

GitHub boleh tertinggal dari local codebase apabila owner/developer memang sedang mengerjakan perubahan yang belum diotorisasi untuk dipush.

---

### 3.4 Render

Render digunakan terutama sebagai:

**REMOTE TEST / STAGING ENVIRONMENT**

Render bukan source of truth codebase.

Render dapat digunakan untuk:

- testing deployment;
- memeriksa apakah deployment berjalan di environment remote;
- integration/runtime verification;
- demonstrasi aplikasi;
- demo kepada investor;
- kebutuhan evaluasi atau presentasi lain;
- kebutuhan operasional sementara yang memang disetujui.

Keberadaan deployment di Render **tidak membuat code yang sedang berjalan di Render menjadi canonical source code**.

Jika Render berbeda dengan local codebase, local codebase tetap menjadi authority untuk development.

---

## 4. Alur Normal

Alur kerja yang benar adalah:

```text
                    ┌──────────────────────┐
                    │   LOCAL CODEBASE     │
                    │   SOURCE OF TRUTH    │
                    └──────────┬───────────┘
                               │
                    development / audit / fix
                               │
                               ▼
                    ┌──────────────────────┐
                    │        GIT           │
                    │ checkpoint / backup  │
                    └──────────┬───────────┘
                               │
                    explicit authorized push
                               │
                               ▼
                    ┌──────────────────────┐
                    │       GITHUB         │
                    │   remote backup      │
                    └──────────┬───────────┘
                               │
                       selected deployment
                               │
                               ▼
                    ┌──────────────────────┐
                    │       RENDER         │
                    │ test / staging / demo│
                    └──────────────────────┘
```

Arah ini **tidak berarti semua perubahan local harus langsung masuk GitHub atau Render**.

Push dan deployment harus dilakukan secara eksplisit dan sesuai scope.

---

## 5. ATURAN KERAS: DILARANG RESTORE DARI GIT SEBAGAI CARA NORMAL

### 5.1 Jangan melakukan restore/rollback untuk mendapatkan kembali kondisi lama

Developer/Codex **dilarang menggunakan Git untuk mengganti current local codebase dengan kondisi commit lama**, termasuk untuk tujuan:

- `git reset --hard`;
- checkout commit lama untuk menjadikan filesystem sebagai baseline kerja;
- restore branch lama;
- revert besar-besaran untuk mengembalikan desain lama;
- mengambil commit lama lalu menjadikannya canonical implementation;
- mengembalikan file dari Git hanya karena versi Git terlihat lebih rapi;
- menjadikan GitHub `main` sebagai baseline pengganti local codebase.

Perintah seperti `reset`, `rebase`, `checkout`, `restore`, `revert`, atau operasi Git lain yang mengubah current working codebase tidak boleh digunakan sebagai jalan pintas recovery tanpa keputusan dan otorisasi eksplisit dari Owner/ChatGPT.

---

## 6. Mengapa Restore dari Git Dilarang?

Karena GitHub/Git history dapat berisi kondisi yang **sudah tidak benar**.

Selama pengembangan Labuda:

- bug ditemukan dan diperbaiki;
- business truth berubah;
- authority lama dibunuh;
- compatibility layer dihapus;
- residue dibersihkan;
- schema diperbaiki;
- backend dan mobile diselaraskan;
- security/authorization diperbaiki;
- payment dan financial invariant diperketat;
- desain yang ditolak dikunci agar tidak resurrect.

Akibatnya, commit lama dapat terlihat sebagai code yang valid secara syntactic, tetapi **tidak lagi valid secara product truth**.

Contoh konsep:

```text
Commit lama
    ↓
masih memiliki bug / residue / desain lama
    ↓
development + audit
    ↓
local codebase diperbaiki
    ↓
business truth lebih baru
```

Jika developer melakukan restore ke commit lama:

```text
local codebase yang sudah bersih
          ↓
      RESTORE GIT
          ↓
codebase lama kembali
```

maka pekerjaan yang sudah diselesaikan dapat hilang atau desain yang sudah dibunuh dapat hidup kembali.

Ini bertentangan dengan prinsip:

> **Kill Once, Lock Forever.**

Desain yang sudah ditolak tidak boleh dihidupkan kembali hanya karena masih terdapat pada Git history.

---

## 7. Git History Hanya Digunakan Sebagai Referensi / Recovery Terbatas

Git tetap boleh digunakan untuk:

- melihat diff;
- membandingkan perubahan;
- mengetahui kapan sebuah perubahan dibuat;
- mengambil informasi dari commit;
- melakukan forensic investigation;
- menemukan file/commit tertentu;
- memulihkan repository setelah local filesystem benar-benar rusak, dengan prosedur recovery yang dikendalikan;
- membuat backup/checkpoint.

Namun, mengambil sesuatu dari Git **tidak otomatis berarti sesuatu tersebut boleh kembali menjadi canonical implementation**.

Jika recovery diperlukan, setelah filesystem pulih harus dilakukan:

1. audit kondisi hasil recovery;
2. bandingkan dengan business truth terbaru;
3. identifikasi perubahan owner yang hilang;
4. identifikasi desain lama/residue;
5. konvergensikan kembali ke canonical design;
6. lakukan verification sebelum digunakan sebagai development baseline.

---

## 8. GitHub Tidak Harus Sama Dengan Local

Perbedaan antara local dan GitHub **tidak otomatis merupakan masalah**.

Contoh kondisi yang sah:

```text
Local HEAD
    c13def0

origin/main
    a650a17
```

Jika `c13def0` dan commit setelahnya adalah owner work yang belum diotorisasi untuk push, maka kondisi tersebut benar.

Developer tidak boleh berpikir:

> "GitHub lebih lama, jadi local harus di-reset ke GitHub."

Yang benar:

> "GitHub sedang menjadi remote checkpoint/backup, sedangkan local adalah current product truth."

---

## 9. Contoh Push Aman

Jika hanya satu commit tertentu yang diizinkan untuk dipush, gunakan explicit refspec.

Contoh:

```bash
git push origin a650a17:refs/heads/main
```

Perintah ini digunakan ketika tujuan memang:

> push hanya commit `a650a17` ke `origin/main`.

Setelah push, wajib verifikasi remote:

```bash
git ls-remote origin refs/heads/main
```

Expected result harus cocok dengan commit yang diotorisasi.

Jangan menggunakan:

```bash
git push origin main
```

jika local `main` berisi owner commits atau perubahan lain yang belum diotorisasi.

---

## 10. Protected Owner Work

Owner work yang masih berada di local harus dianggap protected apabila belum secara eksplisit diotorisasi untuk push.

Developer/Codex tidak boleh:

- memasukkannya ke commit lain;
- staging tanpa instruksi;
- push secara tidak sengaja;
- reset;
- rebase;
- checkout;
- stash;
- revert;
- restore;
- mengubah file protected;
- membersihkan perubahan owner hanya agar Git menjadi clean.

Jika kondisi repository menunjukkan bahwa operasi yang diminta akan menyentuh owner work, **STOP dan laporkan**.

---

## 11. Render Deployment Tidak Sama Dengan Source Control

Render dapat menjalankan commit tertentu dari GitHub untuk kebutuhan remote testing/demo.

Tetapi:

```text
Render runtime ≠ source of truth
GitHub       ≠ source of truth
Git history  ≠ source of truth
```

Canonical source untuk development tetap:

```text
CURRENT LOCAL CODEBASE
```

Karena itu, developer tidak boleh mengambil code dari Render lalu menganggapnya sebagai baseline development hanya karena deployment tersebut sedang berjalan.

---

## 12. Sebelum Melakukan Push

Setiap push harus menjawab:

1. Commit mana yang diotorisasi?
2. Apakah commit tersebut memang canonical untuk tujuan push?
3. Apakah ada owner commits setelah commit tersebut?
4. Apakah working tree memiliki owner changes?
5. Apakah push akan mengirim perubahan yang tidak diotorisasi?
6. Apakah explicit refspec diperlukan?
7. Bagaimana remote akan diverifikasi setelah push?

Jika jawaban tidak jelas:

**STOP. Jangan push.**

---

## 13. Sebelum Melakukan Recovery

Jika local PC/drive mengalami masalah dan recovery dari Git benar-benar diperlukan:

### Jangan langsung:

```bash
git reset --hard origin/main
```

atau menjadikan `origin/main` sebagai baseline tanpa pemeriksaan.

### Lakukan:

1. selamatkan filesystem/data yang masih tersedia;
2. identifikasi local state terakhir;
3. identifikasi commit backup terakhir yang tersedia;
4. recover menggunakan Git hanya sejauh diperlukan;
5. jangan menganggap hasil recovery otomatis canonical;
6. audit terhadap business truth terbaru;
7. identifikasi perubahan yang hilang;
8. konvergensikan codebase;
9. jalankan verification;
10. baru tetapkan filesystem hasil recovery sebagai development baseline.

Recovery adalah **proses penyelamatan**, bukan perubahan product truth.

---

## 14. Aturan untuk Codex / Developer

Setiap task yang menyentuh Git harus mengikuti aturan berikut:

### Objective

Nyatakan secara eksplisit apakah task:

- hanya membaca Git;
- membuat checkpoint;
- melakukan push;
- melakukan recovery.

### Protected

Nyatakan owner commits dan working-tree changes yang tidak boleh disentuh.

### Required proof

Untuk push:

```text
exact push command
push result
git ls-remote result
local HEAD
git status --short
confirmation that protected owner work was not pushed
```

Untuk recovery:

```text
recovered source/commit
files recovered
owner changes identified
business-truth comparison
verification result
remaining risks
```

### Stop conditions

STOP jika:

- baseline repository tidak sesuai;
- diperlukan reset/rebase/restore;
- protected owner work harus disentuh;
- push akan mengirim commit yang tidak diotorisasi;
- GitHub state bertentangan dengan instruksi;
- recovery membutuhkan keputusan product/business;
- ada risiko menghidupkan kembali desain lama.

---

## 15. Hubungan Dengan Aturan Pengembangan Labuda

Aturan ini merupakan penerapan langsung dari prinsip:

> Current filesystem dan keputusan terbaru adalah product truth; Git history bukan sumber desain.

Audit dan development harus tetap mengikuti prinsip:

- satu authority;
- audit untuk mengambil keputusan;
- tidak menghidupkan desain lama;
- protected paths tidak disentuh;
- scope tetap kecil;
- perubahan di luar scope tidak dikerjakan;
- Git digunakan sebagai keselamatan repository, bukan sebagai sumber kebenaran produk.

---

## 16. Ringkasan Singkat Untuk Developer Baru

Jika developer baru hanya membaca satu bagian, pahami ini:

```text
LOCAL
= canonical development codebase
= product truth

GIT
= checkpoint / history / safety tool

GITHUB
= remote backup / authorized integration point

RENDER
= remote test / staging / demo environment
```

### Jangan pernah berasumsi:

```text
GitHub lebih baru → GitHub pasti lebih benar
GitHub main → harus menjadi local baseline
commit lama → boleh dipulihkan
Render berhasil → Render adalah source code
local berbeda dari GitHub → local harus di-reset
```

### Yang benar:

```text
Business truth terbaru
        +
Current local codebase
        ↓
    DEVELOPMENT
        ↓
   authorized Git
        ↓
      GitHub
        ↓
   selected deploy
        ↓
      Render
```

**Local codebase adalah authority. GitHub adalah safety copy. Render adalah environment untuk test/demo.**

---

## 17. Final Rule

> **JANGAN RESTORE PRODUCT CODE DARI GIT HANYA KARENA GIT TERLIHAT LEBIH RAPI, LEBIH BARU, ATAU LEBIH AMAN.**

> **Current local codebase adalah baseline development.**

> **GitHub digunakan terutama untuk mengamankan codebase apabila local PC/drive mengalami masalah dan sebagai remote checkpoint/integration yang memang diotorisasi.**

> **Render digunakan untuk remote testing, staging, demo investor, atau kebutuhan lain yang memang diperlukan — bukan sebagai source of truth.**

> **Jika local codebase dan GitHub berbeda, jangan melakukan reset/restore. Audit dulu.**

> **Jika recovery benar-benar diperlukan karena local filesystem rusak, Git hanya digunakan sebagai bahan recovery. Hasil recovery wajib dikonvergensikan kembali dengan business truth terbaru sebelum dianggap canonical.**
