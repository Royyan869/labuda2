# HANDOFF — UI/UX SATU KEBENARAN (Labuda `apps/mobile`)

Tanggal: 2026-09-30. Semua kerja **belum di-commit**; working tree berisi ~241
path (campur kerja agen lain yang sedang berjalan — jangan di-revert).
Branch `main`.

Dua dokumen induk yang WAJIB dibaca dulu:
- `KONDISI_APP.md` — ledger keputusan (baris 128 TEMA SATU AUTHORITY, 130 SATU
  ROLE UNTUK AKSI & HARGA, 131 TIPOGRAFI SATU TANGGA, 134 TOKEN WARNA MATI, 138
  TINTA BUKAN LATAR, plus SATU KARTU ALAMAT, SATU PROYEKSI/HARGA).
- `TIPOGRAFI_MIGRATION_PLAN.md` — rencana bertahap + §1b temuan struktural +
  §GATE M3-LADDER DIPERBAIKI + Progres (Tahap 0/1/Irisan 1 selesai).

## 1. Prinsip yang memandu SEMUA keputusan

User menegaskan berulang: **"semuanya harus punya 1 kebenaran, bukan hardcode
hanya agar terlihat sama."** Artinya: pakai role/token/oytoritas tema, jangan
menyalin nilai per layar. Konsisten dengan `GUIDE_CLEANUP.md` (hapus total yang
obsolete) dan `LABUDA_CLEANUP_FIRST.md`.

## 2. Yang sudah SELESAI (urutan kronologis)

1. **Gate theme authority hijau**: 6 pelanggaran pixel-identical diperbaiki ke
   token (`AppMetrics.p2/p8/p4`, `AppShape.pill`, `AppType.s12/s28`) di
   `commerce_detail_seller_card`, `negotiation_proposal_card`,
   `feed_media_mosaic`.
2. **Auction detail → warna brand** (Opsi A): helper `_mainActionColor`
   DIBUNUH; CTA `Pasang Bid`/`Klaim Sekarang` tidak lagi menimpa
   `backgroundColor` → jatuh ke button theme (`scheme.primary`), identik dengan
   `Beli Sekarang` di for-sale. Nominal uang → `scheme.primary`. Hijau
   DIPERTAHANKAN hanya untuk "berhasil" (bid position menang, settlement aman,
   refund, OTP, banner "Anda Menang", badge status aktif, status leading list).
   Countdown: merah <1 jam, amber <6 jam, netral selebihnya.
3. **Token warna mati dibuang** dari `app_colors.dart` (5):
   `neutralGray700/800`, `koiBlue`, `successGradient`, `warningGradient`.
4. **Tangga tipografi KEDUA DIHAPUS**: `lib/core/src/theme/app_typography.dart`
   dihapus, export `core/core.dart` dicabut, 38 pemakaian di 11 widget
   dimigrasikan ke `textTheme`, dikunci test "the second typography ladder
   stays deleted".
5. **Halaman Address & Verifikasi diperbaiki** — akar masalah: peran TINTA
   (`onSurfaceVariant`) dipakai sebagai LATAR → teks abu di atas abu. 4 file
   duplikat mati dihapus. Kunci keputusan: **TINTA BUKAN LATAR**.
6. **Sweep "tinta jadi latar" lib-wide** (AddEditAddressDialog, phone
   verification, date picker, coordinate modal, create-content sheet,
   profile_avatar) + **GATE BARU** `inkAsFillScan()` di
   `test/support/theme_authority_gate.dart` + dua test kontrak (positif dengan
   lantai >300 blok fill, negative proof probe). Field `backgroundColor` pada
   banner chat di-rename `tone`.
7. **Kartu alamat di-dedupe**: widget bersama baru
   `lib/shared/widgets/shipping_address_card.dart` (`ShippingAddressCard` +
   `ShippingAddressEmptyState`, diekspor via `shared/shared.dart`); 4 salinan
   privat dihapus dari checkout & modal klaim lelang.
8. **Format Rupiah disatukan**: `negotiation_proposal_card` memakai
   `formatGroupedAmount` (ratchet `resource_projection_price_policy_test` hijau).
9. **Test rusak diperbaiki**: `store_photo_preview_test` dulu meng-`substring(-1)`
   karena memindai marker di file yang sudah menjadi delegator; kini memindai
   SETIAP blok catch di `avatar_image_processor.dart` dengan pencocokan kurung
   + lantai anti-vakum.
10. **Purge slice 2** (nol konsumen): `lib/features/home/domain/entities/main_tab.dart`
    (`MainTabEntity`) + `lib/docs/feature_skeleton_template.dart` (8 KB murni
    komentar) + dir `lib/docs/` mati; dikunci BY PATH + identifier di
    `dead_file_purge_contract_test`.
11. **MIGRASI TIPOGRAFI — Tahap 0**: census + ratchet.
    - `test/support/type_role_migration_gate.dart` (`typeReferenceCensus`,
      `typeReferencesInFile`, floors ≥900 file & ≥900 rujukan, komentar dibuang).
    - `test/core/theme/type_role_migration_ratchet_test.dart`: 17 token dibekukan
      (total **1182 rujukan / 265 file**), cap tidak boleh naik, token baru
      merah, `_slicesLockedToZero` (by path), negative proof.
    - Angka: **858 rujukan (73%) ukurannya ada di tangga M3; 324 (27%) tidak**
      (s13 160, s18 73, s10 40, s20 29, s15 17, s9 3, s8 1, s8_5 1).
      Bentuk: 1058 `fontSize:` langsung + 124 ekspresi (`fontSize: isActive ? …`),
      528 blok ber-`fontWeight`, 44 `height`, 7 `letterSpacing`, 74 `const TextStyle`.
12. **Tahap 1 — keputusan owner = OPSI C**: langkah luar tangga diresmikan
    sebagai role bernama. `AppTypeRoles` (ThemeExtension) di `app_theme.dart`
    dengan 5 role: `labelMicro` 10, `bodyDense` 13, `titleCompact` 15,
    `titleSection` 18, `titleProminent` 20; dibaca `context.typeRoles.*`;
    `AppTypeRoles.fromBody(bodyMedium)` (satu sumber metrik, `copyWith(fontSize:)`);
    fallback `Typography.material2021().englishLike`.
    **Penyempitan yang dicatat**: tail badge 8/8.5/9 (5 rujukan) SENGAJA tidak
    diresmikan — 8 dan 9 maksud sama di 3 widget badge nyaris kembar → konvergen
    saat badge widgets didedupe, baru masuk `labelMicro`.
13. **Irisan 1 selesai** (`shared/object`): caption tipe badge
    `object_preview_card.dart` → `labelSmall` sesuai mapping kanonik AppTheme
    (w600 + warna brand tetap di call site). Delta piksel dicatat: 12→11,
    ls 0.25→0.5, tinggi 1.43→1.45. Cap `s12` 278→277, file masuk
    `_slicesLockedToZero`, proof resolver ditulis di
    `reference_attachment_snapshot_shell_test.dart`.
14. **TEMUAN STRUKTURAL (§1b)** — penting:
    - `ThemeData.textTheme` MENTAH **tidak punya geometri** (fontSize/weight/
      height/letterSpacing = null; hanya color + family).
    - `Theme.of(context)` menambahkannya via `ThemeData.localize(theme,
      theme.typography.geometryThemeFor(ScriptCategory.englishLike))`.
    - Konsekuensi: `AppTypeRoles.fromBody` awalnya salah (role berukuran tanpa
      metrik baris) → sudah diperbaiki memakai ekspresi sama dengan `Theme.of`.
15. **Gate M3-ladder DITUTUP & terbukti bisa gagal**:
    - Helper bersama `resolvedTextTheme(ThemeData)` di
      `test/support/theme_authority_gate.dart` (dipakai gate ladder + ratchet).
    - Test `typography stays on the M3 ladder` kini: floor `isNotNull` untuk 4
      metrik SEBELUM membandingkan, dua sisi di-resolve, `Inter` wajib bertahan.
    - Negative proof baru: (a) fork `labelSmall` 11→13 tertangkap (tepat 1
      mismatch); (b) canary jalur mentah (`raw.labelSmall.fontSize == null`);
      (c) end-to-end via `ThemeData(typography: Typography.material2021(englishLike: …))`.
    - **Empiris**: fork nyata ditanam sementara di `_build` →
      `Expected: <11.0> Actual: <13.0>` → dicabut → hijau lagi.

## 3. Status verifikasi terakhir (SEMUA HIJAU)

- `cd apps/mobile && dart analyze lib` → **0 error/warning** (24 info lint lama
  di file yang tidak disentuh).
- `flutter test test/core test/shared` → **+670 All tests passed, 0 gagal**.
- Gate tema (`test/core/theme/`) = 36 test; ratchet = 8 test; suite besar yang
  pernah dijalankan sesi ini: auction 146, checkout+contract 249, chat 398,
  user/{identity,verification,preference,profile} 194 (1 kegagalan lama sudah
  diperbaiki), shared+profile+chat+seller 882.

## 4. ANTREAN / LANGKAH BERIKUTNYA

1. **Irisan 2 — `features/marketplace` (2 rujukan / 1 file)**: migrasi ke role,
   lock di `_slicesLockedToZero`, turunkan cap, tulis proof resolver. Ulangi
   resep irisan 1. Urutan penuh di `TIPOGRAFI_MIGRATION_PLAN.md` Tahap 2
   (search 14 → finance 25 → home 35 → chat 40 → social 44 → system 84 →
   shared/widgets 161 → commerce 362 → user 398). **Satu irisan hijau dulu
   sebelum berikutnya.**
2. **Sweep proof vakum** di semua contract test: cari pola `expect(a, b)` yang
   bisa lulus saat dua-duanya null/kosong (masalah yang baru ditemukan di gate
   M3-ladder; kemungkinan masih ada di tempat lain).
3. **Dedupe 3 widget badge** (`chat_badge_widget`, `notification_badge_widget`,
   `saved_item_badge_widget`) → setelah itu tail 8/8.5/9 konvergen ke `labelMicro`
   dan semuanya enshrined.
4. Opsi lain yang sudah diajukan & belum dikerjakan: desain gate "CTA tidak
   pernah status tone" (perlu allowlist untuk tombol destruktif/selesai —
   ada 23 bind `statusColors` yang sah di `styleFrom`); sisa hijau dekoratif di
   81 file `statusColors.success` di luar auction; **verifikasi visual (screenshot)
   — semua klaim baru dibuktikan lewat role + test, BELUM lewat mata di layar**.

## 5. CAUTIONS (jangan dikuliti ulang)

- **`rg` tidak ada**; `code_search` tool pernah gagal (ENOENT). Pakai
  `grep -rn` / `grep -rho` via terminal. `perl` tersedia.
- Selalu jalankan test dengan wrapper, cwd `apps/mobile`:
  `cd apps/mobile && (timeout 300 flutter test test/core/theme/ > /tmp/x.log 2>&1 || true)`
- Windows/MSYS: ansih `git` memperingatkan LF→CRLF; jangan pakai `del`/`move`
  cmd, pakai `rm`/`mv` bash.
- **Jangan revert** file lain yang bukan milik sesi ini (working tree bercampur).
- Jangan hidupkan kembali: `AppTypography`, token warna mati, `MainTabEntity`,
  `lib/docs/`, fill tinta, `fontSize:` mentah, role di luar `AppTypeRoles`,
  atau proof dua sisi mentah (`null == null`).
