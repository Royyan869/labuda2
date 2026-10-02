# TIPOGRAFI SATU AUTHORITY — RENCANA MIGRASI `AppType.s*` → role `textTheme`

Status: **Tahap 0 SELESAI**, **Tahap 1 SELESAI** (extension bernama terpasang),
**Irisan 1 SELESAI**, **Irisan 2 SELESAI**, **Irisan 3 SELESAI**,
**DEDUPE BADGE + TAIL 9 KE labelMicro SELESAI**, **gate M3-ladder DIPERBAIKI dari null==null → terbukti bisa gagal**,
**FONDASI 2026-10-02 SELESAI** (tangga tipe dipangkas 17 → 5 langkah);
irisan 2 ke atas tidak lagi tertahan.

> **FONDASI 2026-10-02 — tangga dipangkas 17 → 5.** `AppType` kini hanya
> `s12 · s14 · s16 · s20 · s24`, dan `AppTypeRoles` di-retune ke lima langkah
> yang sama (labelMicro 12 · bodyDense 14 · titleCompact 16 · titleSection 20 ·
> titleProminent 24). Tabel fakta di bawah adalah **histori pra-fondasi**
> (ukuran 8…36 masih hidup saat itu) — baca sebagai "sebelum", bukan keadaan
> sekarang. Census pasca-fondasi: **1117 rujukan di 246 file** (s12 387 ·
> s14 403 · s16 205 · s20 97 · s24 25), dan cap ratchet di-rebase sekali secara
> sengaja. Empat dari lima step adalah step M3 2021 (`bodySmall`/`bodyMedium`/
> `bodyLarge`/`headlineSmall`) — hanya 20 yang tak punya role M3 — jadi tujuan
> migrasi tetap: satu ROLE per situs, bukan satu angka.

Doktrin induk: `KONDISI_APP.md` (TEMA SATU AUTHORITY) dan komentar
`AppType`/`AppTheme` di `lib/core/src/theme/app_theme.dart`:
"a ROLE beats a size … the parallel ladder is DELETED — `textTheme` is the one
type authority". Migrasi ini menuntaskan kalimat itu.

## 1. Fakta terukur (bukan perkiraan)

Census: `test/support/type_role_migration_gate.dart` → `typeReferenceCensus()`,
1118 file `lib/` non-generated dipindai, komentar dibuang.

**1182 rujukan** `AppType.s*` (bukan ~1214: angka lama menghitung bentuk
langsung saja). Dari jumlah itu 1058 berbentuk `fontSize: AppType.s*`; 124
sisanya **ekspresi**, mis. `fontSize: isActive ? AppType.s10 : AppType.s8_5` —
justru inilah yang luput dari hitungan regex sederhana.

| ukuran | rujukan | role M3 | catatan |
|---|---|---|---|
| s12 | 278 | `bodySmall` (12/w400) | tinggi berbeda (1.43 → 1.33) |
| s14 | 271 | `bodyMedium` (14/w400) | **satu-satunya yang identik ambient** |
| s16 | 198 | `bodyLarge` (16/w400) | tinggi berbeda (1.43 → 1.5) |
| s13 | 160 | — | **di luar tangga** |
| s11 | 85 | `labelSmall` (11/**w500**) | role lebih tebal dari ambient w400 |
| s18 | 73 | — | **di luar tangga** |
| s10 | 40 | — | **di luar tangga** |
| s20 | 29 | — | **di luar tangga** |
| s24 | 17 | `headlineSmall` | |
| s15 | 17 | — | **di luar tangga** |
| s28 | 4 | `headlineMedium` | |
| s9 | 3 | — | **di luar tangga** |
| s22 | 3 | `titleLarge` | |
| s8 | 1 | — | **di luar tangga** |
| s8_5 | 1 | — | **di luar tangga** |
| s32 | 1 | `headlineLarge` | |
| s36 | 1 | `displaySmall` | |

Ringkas: **858 rujukan (73%) ukurannya ada di tangga M3**, **324 (27%) tidak
punya role sama sekali**. Tidak ada jalan memigrasikan 324 itu tanpa keputusan
Tahap 1.

Bentuk pemakaian (perl census, 1058 bentuk langsung):

- 1049 di dalam blok `TextStyle(…)`; 9 di luar (mis. `copyWith`).
- 528 blok punya `fontWeight:` eksplisit; 44 punya `height:`; 7 punya
  `letterSpacing:`.
- 74 blok `const TextStyle(…)` → kehilangan `const` saat membaca theme.
- 265 file; 1 pemakaian bukan teks: `size: AppType.s12` (ikon,
  `commerce_detail_seller_card.dart:238`) — ikut aturan geometri, bukan teks.

Sebaran rujukan per area (untuk urutan irisan):

| area | rujukan | file |
|---|---|---|
| `shared/object` | 1 | 1 |
| `features/marketplace` | 2 | 1 |
| `core/src/router` | 5 | 1 |
| `shared/{src,ui,domain,utils,models}` | ~11 | 5 |
| `features/search` | 14 | 6 |
| `domains/finance` | 25 | 5 |
| `features/home` | 35 | 8 |
| `domains/chat` | 40 | 11 |
| `domains/social` | 44 | 10 |
| `domains/system` | 84 | 23 |
| `shared/widgets` | 161 | 53 |
| `domains/commerce` | 362 | 66 |
| `domains/user` | 398 | 74 |

## 2. Kenapa ini bukan `sed`

`fontSize: 14` pada `Text` **bukan nilai mandiri**: `Text` me-merge style ke
`DefaultTextStyle` (di Material = `bodyMedium`). Konsekuensinya:

- `s14` tanpa bobot pada teks body = **redundan**; menghapus `fontSize`-nya
  **nol piksel** karena memang sudah 14/w400/h1.43 dari ambient.
- `s12`/`s16` → role: ukuran sama, tapi **tinggi baris berubah** (17.2 → 16.0 px
  untuk 12; 20.0 → 24.0 px untuk 16) — terlihat pada teks multi-baris.
- `s11` → `labelSmall` menambah **w500** yang tadinya w400 → tebal berubah.
- Ambient bukan selalu `bodyMedium`: di dalam `ElevatedButton` ambient-nya
  `labelLarge` (14/w500), di `AppBar` `titleLarge`, di `ListTile` lain lagi.
  Memetakan ukuran mentah ke role tanpa tahu ambient = bobot berubah diam-diam.
- 1058 titik tersebar di 265 file; perubahan tinggi baris berarti **layout bisa
  bergeser** (`maxLines`/overflow), bukan sekadar huruf lebih kecil.
- 324 rujukan memakai ukuran yang **tidak ada** di tangga M3.

Ini juga alasan tidak ada `sed` yang aman: yang menentukan hasil adalah
ambient widget di sekitarnya, dan itu hanya terbukti lewat render.

## 3. Tahapan

### Tahap 0 — sensus + ratchet (tanpa ubah piksel) — **SELESAI**
- `test/support/type_role_migration_gate.dart` — `typeReferenceCensus()`,
  `typeReferencesInFile()`, `typeCallSite`, `typeReference`, floors.
- `test/core/theme/type_role_migration_ratchet_test.dart` — 4 kontrak:
  1. census menyapu app nyata dan tidak bisa vakum (lantai ≥900 file & ≥900
     rujukan);
  2. **tiap token dibatasi jumlah hari ini dan tidak boleh naik** (cap beku
     untuk 17 token); token baru langsung merah;
  3. `_slicesLockedToZero` — file yang sudah selesai wajib 0, dikunci by path;
  4. negative proof: bentuk kondisional terhitung 2, prosa/`///` dan file
     pemilik token tidak dihitung.

Gunanya: setiap tahap berikutnya hanya bisa **menurunkan** angka, dan
`fontSize: AppType.s*` baru langsung merah.

### Tahap 1 — keputusan tangga untuk 324 rujukan di luar tangga — **SELESAI (opsi C)**

Owner memilih **C**: langkah di luar tangga diresmikan sebagai role bernama di
`app_theme.dart`, bukan angka mentah per widget. Yang terpasang:

- `class AppTypeRoles extends ThemeExtension<AppTypeRoles>` dengan **5 role**:
  `labelMicro` 10, `bodyDense` 13, `titleCompact` 15, `titleSection` 18,
  `titleProminent` 20.
- **Satu sumber metrik**: `AppTypeRoles.fromBody(TextStyle body)` menurunkan
  kelima role dari `bodyMedium` dengan `copyWith(fontSize:)`. Tidak ada ukuran/
  bobot/tinggi yang ditulis ulang — jadi kalau tangga M3 di-retune, role ikut.
- Registrasinya **setelah** theme jadi: `_build` kini `final theme = ThemeData(…)`
  lalu `theme.copyWith(extensions: [status, AppTypeRoles.fromBody(theme.textTheme.bodyMedium!)])`,
  sehingga role memakai style milik theme itu sendiri — termasuk
  `fontFamily: 'Inter'`. Karena itu gate `return ThemeData(` di tema diperbarui
  menjadi hitungan **konstruksi** (`\bThemeData(`) yang tetap 1, sekaligus
  menolak substring `DialogThemeData(`/prosa.
- Ditolak: **A** (normalisasi ke role terdekat — mengubah piksel di UI padat
  tanpa batch screenshot) dan **B** (sisakan `AppType` untuk ukuran luar tangga
  — mempertahankan dua otoritas tipe).
- Akses: `context.typeRoles.labelMicro` / `.bodyDense` / `.titleCompact` /
  `.titleSection` / `.titleProminent`, dengan fallback
  `AppTypeRoles.fallback` (metrik body M3 stok, untuk `ThemeData()` telanjang
  yang selalu dipompa widget test — postur yang sama dengan `AppStatusColors.light`).
- **GATE** di `type_role_migration_ratchet_test.dart`: kedua theme wajib
  mendaftarkannya; tiap role wajib sama bobot/tinggi/letterSpacing/family dengan
  `bodyMedium` theme itu (bukti penurunan, bukan janji); sebuah role **tidak boleh**
  menabrak langkah M3 mana pun (dua nama untuk satu langkah = duplikasi yang
  dibunuh); `context.typeRoles` wajib resolve di theme nyata **dan** di
  `ThemeData()` telanjang.
- SATU penyempitan yang saya putuskan dan catat di sini: **tail badge (8, 8.5, 9
  — 5 rujukan di 4 file) sengaja TIDAK diresmikan.** 8 dan 9 adalah maksud yang
  sama (angka di dalam badge) di tiga widget badge yang nyaris kembar;
  mengabadikan dua langkah untuk satu maksud justru duplikasi yang ingin
  dibunuh. Ketiganya konvergen saat widget badge itu didedupe, lalu masuk ke
  `labelMicro`. Kalau owner ingin tetap diresmikan sekarang, tinggal tambah dua
  role di class yang sama — nol perubahan di call site.

Konsekuensi penting yang jujur dicatat: role ini membawa metrik **keluarga
body** (w400/height 1.43/letterSpacing 0.25) pada ukurannya sendiri, karena
itulah yang di-inherit situs-situs itu hari ini. Jadi memigrasikan situs yang
ambient-nya tombol/AppBar tetap wajib mempertahankan `fontWeight:` eksplisitnya
(resep 3), dan judul (18/20) yang kelak diberi metrik keluarga title adalah
keputusan visual tersendiri — bukan efek samping migrasi.

### Irisan 1 — `shared/object` — **SELESAI**

Satu rujukan: `object_preview_card.dart` → caption tipe badge.

- Dipetakan ke **`labelSmall`** mengikuti mapping kanonik yang tertulis di
  `AppTheme` ("caption/badge → `labelSmall`"), bukan ke `bodySmall` (12px):
  kebenaran role mengalahkan pixel yang mirip. Bobot w600 dan warna brand tetap
  di call site, tidak di-bake ke role.
- **Delta piksel yang diterima (dicatat, bukan disembunyikan)**: 12 → 11,
  letter-spacing 0.25 → 0.5, line-height 1.43 → 1.45 (kotak baris 17.2 → 16.0 px).
  Teks satu baris dalam Column tanpa batas tinggi → tidak ada risiko overflow.
- Ratchet: cap `s12` 278 → **277**; file masuk `_slicesLockedToZero`.
- **Bukti resolver** di `reference_attachment_snapshot_shell_test.dart`
  (`resolver proof: the type caption is the theme role`): memompa widget nyata
  dengan theme nyata lalu membuktikan ukuran/tinggi/letter-spacing yang dirender
  **adalah** role dari theme itu, plus bobot & warna milik call site.
- Verifikasi: `dart analyze` jalur tersentuh 0 issue; `test/core` +
  `test/shared` = **+669 All tests passed**.

## 1b. TEMUAN STRUKTURAL dari irisan 1 (baca sebelum irisan berikutnya)

Probe membuktikan sesuatu yang mengubah asumsi rencana ini:

1. **`ThemeData.textTheme` mentah TIDAK punya geometri.**
   `AppTheme.lightTheme.textTheme.labelSmall` berisi hanya `color` + `family`;
   `fontSize`/`fontWeight`/`height`/`letterSpacing` semuanya **null**. Yang
   membawa ukuran adalah `englishLike` (2021).
2. **`Theme.of(context)` menambahkannya**: `Theme.of` memanggil
   `ThemeData.localize(theme, theme.typography.geometryThemeFor(category))`,
   yaitu `textTheme: englishLike.merge(theme.textTheme)`. Karena itu gaya yang
   benar-benar dirender **punya** metrik penuh — situs yang sudah termigrasi lewat
   `Theme.of(context).textTheme.*` (11 file) tidak ada yang rusak.
3. **Kesalahan Tahap 1 yang sudah diperbaiki**: `AppTypeRoles.fromBody` awalnya
   memakai `theme.textTheme.bodyMedium` (mentah) → role-nya berukuran tapi
   **tanpa metrik baris**, yaitu ketergantungan ambient yang justru mau dihapus.
   Kini `_build` memakai `ThemeData.localize(theme, geometryThemeFor(englishLike))`
   — ekspresi yang sama dengan `Theme.of` — dan `fallback` memakai
   `Typography.material2021().englishLike` (bukan `black`, yang ternyata hanya
   warna).
4. **Dua bukti ternyata kembar**: (a) proof ratchet `style.height == body.height`
   membandingkan `null == null` → sudah ditambahkan floor `isNotNull`;
   (b) **gate lama `typography stays on the M3 ladder` juga kembar**: ia
   membandingkan `AppTheme.lightTheme.textTheme` vs
   `ThemeData(useMaterial3: true).textTheme`, keduanya MENTAH dan keduanya null,
   jadi keempat asersinya lulus sebagai `null == null` — hanya `fontFamily ==
   'Inter'` yang non-kosong. **HOLE INI SUDAH DITUTUP** (lihat
   `GATE M3-LADDER DIPERBAIKI` di bawah).

### GATE M3-LADDER DIPERBAIKI — bisa gagal, dan terbukti

Perbaikan (satu definisi, dipakai dua test):

- Helper bersama **`resolvedTextTheme(ThemeData)`** di
  `test/support/theme_authority_gate.dart` — memanggil `ThemeData.localize(
  theme, typography.geometryThemeFor(ScriptCategory.englishLike))`, yaitu
  ekspresi yang persis dipakai `Theme.of`. Salinan ekspresi ini di ratchet test
  sekarang memakai helper yang sama (tidak ada dua salinan).
- Test `typography stays on the M3 ladder` kini: (1) **floor `isNotNull` untuk
  keempat metrik + eksistensi style + `theirStyle?.fontSize` non-null** sebelum
  membandingkan — kalau resolusi gagal, gate merah duluan, bukan lulus; (2)
  membandingkan hasil resolusi terhadap `ThemeData(useMaterial3: true)` yang
  juga diresolusi; (3) memastikan `fontFamily: Inter` bertahan selepas merge.
- **Negative proof baru** `the ladder comparison can actually fail`: (a) sebuah
  ladder yang di-FORK (`labelSmall` 11→13) dilaporkan sebagai
  `labelSmall fontSize` oleh komparasi yang sama, tepat satu mismatch;
  (b) canary bahwa jalur mentah memang tak punya geometri (alasan gate harus
  meng-resolve) — kalau suatu saat Flutter menanam geometri ke `ThemeData`,
  canary ini gagal dengan sendirinya dan resolusi bisa disederhanakan; (c) bukti
  end-to-end lewat jalur fork yang realistis: `ThemeData(typography:
  Typography.material2021(englishLike: … fontSize 15))` → masuk lewat path yang
  sama dengan gate → tertangkap.
- **Bukti empiris end-to-end**: fork NYATA ditanam sementara di `_build`
  (`typography` dengan `labelSmall` 13) → test gagal dengan
  `Expected: <11.0> Actual: <13.0>` → probe dicabut → gate hijau lagi (36 test).

### Tahap 2 — irisan, satu per satu, kecil → besar
Urutan: mulai dari satu file, supaya resepnya terbukti dulu sebelum menyentuh
permukaan besar. **Satu irisan wajib hijau sebelum irisan berikutnya dimulai.**

1. `shared/object` (1 rujukan / 1 file) — bukti resep pada satu file
2. `features/marketplace` (2 / 1)
3. `core/src/router/router_error_page.dart` (5 / 1)
4. `shared/{src,ui,domain,utils,models}` (~11 / 5)
5. `features/search` (14 / 6)
6. `domains/finance` (25 / 5)
7. `features/home` (35 / 8)
8. `domains/chat` (40 / 11)
9. `domains/social` (44 / 10)
10. `domains/system` (84 / 23)
11. `shared/widgets` (161 / 53) — permukaan bersama, setelah resep stabil
12. `domains/commerce` (362 / 66) — terbesar & paling kompleks
13. `domains/user` (398 / 74) — terbesar

Tiap irisan: `dart analyze` jalur tersentuh 0 issue; suite test irisan hijau;
path irisan masuk `_slicesLockedToZero`; cap token di ratchet **diturunkan**
sebesar yang dimigrasikan (kalau tidak, gate tetap hijau padahal tidak ada
kemajuan yang terkunci).

### Resep per titik (urut aman → berisiko)
1. **Redundan** (= ambient): `s14` tanpa bobot/`height` pada teks body →
   **hapus** `fontSize`-nya. Nol piksel.
2. **Tanpa bobot + tinggi cocok**: petakan ke role (`s12`→`bodySmall`,
   `s16`→`bodyLarge`, dst.) dan terima perubahan tinggi baris — dikumpulkan
   sebagai batch "tinggi baris" dan diperiksa dengan screenshot pada situs
   multi-baris/`maxLines`.
3. **Berbobot** (528 blok): role sebagai ukuran, **pertahankan** `fontWeight:`
   eksplisit di call site (itu keputusan produk, bukan tangga).
4. **Punya `height:`/`letterSpacing:`** (44 + 7): override tetap, hanya ukuran
   yang pindah.
5. **Di luar tangga** (324): diparkir sampai Tahap 1 diputuskan.
6. **`const TextStyle(`** (74): `Theme.of(context)` bukan const → `const` lepas;
   hanya di irisan yang tidak sedang mengubah logika.

### Bukti per irisan (bukan hanya analyze)
- `dart analyze` jalur tersentuh = 0 error/warning.
- Suite test domain irisan hijau.
- **Proof resolver**: 3–5 situs perwakilan diuji lewat widget test dan
  membuktikan `(fontSize, fontWeight, height)` efektif **sama** dengan sebelum
  migrasi — analyzer tidak bisa melihat perbedaan ini, dan inilah satu-satunya
  bukti "tidak ada drift" yang jujur.

### Kriteria selesai
`AppType` dihapus dari `app_theme.dart`, ratchet 0, larangan `fontSize:` literal
tetap hidup, dan `AppType` masuk daftar "JANGAN hidupkan kembali" di
`KONDISI_APP.md`.

## 4. Progres

- [x] Tahap 0 — sensus + ratchet + lock path + negative proof (hijau).
- [x] Tahap 1 — extension `AppTypeRoles` (5 role) + gate penurunannya (hijau).
- [x] Irisan 1 — `shared/object` (1 rujukan) → `labelSmall`, cap `s12` 278→277,
      proof resolver ditulis (hijau).
- [ ] **Selesai**: tutup gate `typography stays on the M3 ladder` yang lulus
      sebagai `null == null` (§1b.4) — resolusi bersama + floor + negative proof
      + fork nyata ditanam/cabut (36 test hijau).
- [x] Irisan 2 — `features/marketplace` (2 rujukan) → `labelLarge` + bobot tetap
      di call site, cap `s14` 271→269, path dikunci, proof resolver ditulis
      (hijau).
- [x] Irisan 3 — `core/src/router/router_error_page.dart` (5 rujukan) →
      `headlineSmall` (judul), `bodyLarge` (detail), `bodySmall` (debug),
      `labelLarge` di KEDUA tombol (teks tombol kanonik; halaman ini satu-satunya
      yang menaikkannya ke 16 → diterima 16→14), cap `s12` 277→276 / `s16`
      198→195 / `s24` 17→16, path dikunci, proof resolver lewat jalur error
      GoRouter NYATA (hijau).
- [x] Dedupe 3 widget badge → SATU renderer `CountBadgeOverlay`
      (`shared/widgets/count_badge.dart`): layout mati dari ketiga badge
      (yang tersisa cuma seam data masing-masing domain), tail `s9` (3
      rujukan) konvergen ke `labelMicro` (9→10; w600 + height 1.1 tetap di
      renderer), cap `s9` 3→0. Sisa tail: 8 (badge toolbar social) & 8.5
      (wizard) tetap di luar tangga — bukan badge app-bar (hijau).
- [ ] Tahap 2 — irisan 4…13 (shared ~11 → search 14 → finance 25 → dst.).
