# MEDIA — Status Menuju Production

> Sumber kebenaran: factual codebase + AWS/S3 + DB per 29 Sep 2026.
> Prinsip: 1 konsep = 1 authority. Test data boleh dibuang; desain mati dikunci.

## 1. SELESAI (dengan proof)

### 1.1 Read pipeline → CloudFront (CLOSED)
- Satu resolver: `mediaresolve` → `https://d358tu61i1wrtt.cloudfront.net/<key>`.
- Semua permukaan read terkonvergensi: content/feed/search, forSale/auction (list+detail+search+card), comment `media[]`, external product, avatar feed/seller/search, chat projection.
- Presigned read 5-menit mati sebagai default (CDN di-set) — placeholder-stuck & refresh-lama sembuh di akar.
- Bukti: `go test -count=1` paket shared/feed/search/commerce/mediaupload hijau.

### 1.2 Render tunggal (CLOSED)
- `AppImage` (cached) satu-satunya widget gambar network; loading vs error selalu beda widget.
- Di-purge total: `StableNetworkImage`, `resolveNetworkImageUrl`, `normalizeMarketplace*`, `awsS3BaseUrl`/`useCloudFront`, `ContentMediaHandler`, 2 modal picker mati, `showImageSourcePicker`, cropper stub, `TextInput*` orphan, dep `image_cropper`, UCrop manifest.
- Detail/carousel/viewer selalu original penuh + zoom; daftar pakai thumbnail (di bawah).
- Bukti: sapu residu `lib/` nol; 35+ test kontrak media hijau.

### 1.3 Upload 1 mesin (CLOSED)
- `MediaUploadOrchestrator` + `MediaUploadConfig` (termasuk preset `forEvidence` baru).
- Termigrasi: content (handler 148 baris dihapus), dispute/refund/external (headless engine), evidence validation MB kanonis.
- Negative contract test mengunci: 1 pick engine, limit hanya di config, deteksi video 1 helper, folder namespaced.

### 1.4 Crop/avatar anti-balik-home (CLOSED, P1)
- Catch-all `pop` yang membunuh wizard dibuang; `StorePhotoPreview` tunggal (server-truth menang + veil uploading); flag mati `showAdvancedCropper` dipurge; `uploadAvatar`/`clearImageCache` paralel dipurge.
- Submit-guard saat uploading sudah ada dan dipertahankan.

### 1.5 Key namespace per domain (CLOSED)
- Backend allowlist eksak: `images|videos` × `content|commerce|chat|evidence`.
- Mobile preset kirim subpath; fixed-key kirim 2 segmen; backend tidak validasi folder untuk fixed-key (key-nya sendiri otoritas).
- Lambda skip `avatars/stores/profile-covers` (varian sia-sia berhenti).

### 1.6 Varian thumbnail tersambung (CLOSED)
- Aturan `ThumbnailVariantKey` = Lambda `getVariantKey` (idempoten, unit-tested).
- Backend isi `thumbnail_url`: commerce wire + card, search previews, feed `media[]` (additive).
- Mobile daftar pakai thumbnail (feed, kartu dagangan, search); codegen `.g.dart` via build_runner.
- Backfill 8/8 objek test → HTTP 200 via CDN (script temp sudah dihapus).

### 1.7 AWS deploy (CLOSED, diverifikasi langsung)
- Origin → `labuda-uploads.s3.us-east-1`, OAC, bucket privat (S3 langsung 403, CDN 200 Hit dari edge Jakarta).
- Bucket fosil `labuda-videos` dihapus (nol referensi DB + kode).
- DB: storage key murni (`sisa_bucket_url = 0`); 6 baris test dibersihkan tanpa langgar FK (2 media-row DELETE, 4 products dikosongkan medianya karena dirujuk for_sale/auction).
- S3 test objects dihapus owner; struktur kanonis terbentuk ulang saat upload real.

## 2. BELUM (terurut, siap eksekusi)

### 2.1 Video sebagai major surface (berikutnya, desain siap)
- Poster klien saat upload (`video_thumbnail` → `_poster.jpg`, jalur backend sudah izin).
- Player visibility-aware (jangan `initialize()` semua kartu); durasi + mute; error Retry (sudah ada).
- Urutan owner-controlled dipertahankan di UI; rebucket images-first backend sudah dibunuh.
- HLS/transcoding: HANYA jika metrik menuntut (titik picu: % error/timeout video di <3G).

### 2.2 P2 tertunda (desain siap, butuh 1 keputusan owner)
- Trust-embedded-avatar (bunuh GET per-instance; butuh restu basi ≤5 mnt + umur daftar).
- `likeStats` per-kartu: rekomendasi PARKIR (butuh kolom wire `/feed` + risiko optimistic).
- Thumbnail server: tetap strategi C (client downsampling) sampai metrik menuntut.

### 2.3 Butuh tangan owner (bukan kode)
- `backend/.env` di server (`CDN_BASE_URL` + region `us-east-1`) + restart backend.
- Retest perangkat (daftar seller berfoto, konten campur, refund/dispute, attach external, diam >5 mnt).
- TTL pendek / invalidasi untuk key fixed (avatar/toko/cover ditimpa di tempat).

## 3. Aturan yang dikunci untuk semua pekerjaan lanjutan
1. Original tidak pernah diubah; hanya tambah varian.
2. Mobile tidak pernah membangun URL; backend tidak pernah mengirim URL mentah.
3. `media.first` = cover = `thumbnail` kartu; `position` = urutan owner.
4. Satu picker, satu crop entry, satu deteksi video, satu render gambar, satu render video.
5. Test data boleh dibuang; desain mati dikunci negative contract; migration applied tidak diedit.
