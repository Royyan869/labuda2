# Manage Profile

> **Status:** STABLE
> **Domain:** Profile
> **Last reviewed:** 2026-08-25

> **Doctrine references:**
> - [Username Lifecycle](../doctrine/username-lifecycle.md) — username adalah identitas kanonik dan **immutable** setelah ditetapkan; tidak ada rename capability.
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — profile field changes tidak melintasi layer.

## Purpose

Memberi Registered User kemampuan untuk memutakhirkan profilnya pasca-onboarding: foto profil, biodata, kontak sosial, dan informasi pribadi lain. Username **tidak** termasuk — username immutable setelah ditetapkan.

## Actors

- **Registered User** — pemilik profile.
- **External (Firebase Auth)** — otoritas atas perubahan email dan beberapa langkah verifikasi (re-verifikasi email, OTP nomor telepon).
- **System** — memvalidasi dan menyimpan perubahan, serta memberi tahu konsumen profil (mis. tampilan nama di chat, di komentar).

## Preconditions

- User sudah `active`.
- User sudah sign-in.

## Main Flow

1. User membuka layar Manage Profile (Edit Profile).
2. User memilih field yang ingin diubah dan menyuntingnya. Username ditampilkan **read-only** (tidak dapat diedit).
3. User menekan "Simpan".
4. Sistem memvalidasi perubahan (lihat `### Business Rules`).
5. Sistem menyimpan perubahan ke profile.
6. Tampilan profil di domain lain (Social, Chat, Listing) ikut ter-update — mengikuti mekanisme baca masing-masing domain.

## Alternate Flows

- **Mengubah email**: melewati alur khusus melalui External (re-verifikasi email tujuan baru). Tidak ditangani sebagai field biasa.
- **Mengubah nomor telepon**: memerlukan OTP — bukan field biasa.
- **Username**: immutable — tidak ada rename. Username ditetapkan sekali saat registrasi / Complete Profile dan tidak pernah berubah.
- **Mengubah avatar**: di-upload ke storage; URL ter-update di profile.

## Failure / Rejection Cases

- **Field tidak diizinkan** ditolak (mis. mengubah `firebase_uid`, `email_verified` langsung dari layar profile).
- **Username mutation** ditolak oleh backend dengan `409 USERNAME_ALREADY_SET` jika ada upaya mengubah username yang sudah ditetapkan.
- **Verifikasi email gagal** saat ganti email → email lama tetap berlaku.
- **OTP nomor telepon gagal** → nomor telepon tidak ter-update.

## State Changes

- **Profile**: field yang relevan ter-update.
- **`username`**: tidak pernah berubah setelah ditetapkan. Backend menegakkan immutability; tidak ada rename path di seluruh codebase.
- **Flag `email_verified` (di backend)**: ketika email diganti, flag dapat reset mengikuti perilaku External.

## Financial Impact

Tidak ada.

## Notifications

- Tidak ada notifikasi user-visible default. Beberapa perubahan sensitif (email, password reset) memunculkan email konfirmasi dari External.

## Cross-Domain Relations

- **Social (Content/Chat/Mention)**: tampilan nama, avatar, dan username di komentar/chat/feed mengikuti profile yang sudah disimpan. Username immutable sehingga referensi `@handle` tetap stabil.
- **Seller Profile**: nama toko / farm name **terpisah** dari username dan display name profile — perubahan profile pribadi tidak otomatis mengubah store name. Username tidak berubah, sehingga seller reputation continuity tetap utuh.
- **Search**: username tidak berubah, sehingga pencarian `@handle` tetap stabil.
- **Moderation**: username tidak berubah, sehingga moderation traceability tetap utuh.
- **Email Verification**: re-verifikasi email tujuan baru ditangani oleh flow [Email Verification](./email-verification.md).

## Business Rules

- **Field yang dapat di-update via Manage Profile**:
 - Display name / full name.
 - Bio.
 - Lokasi (kota, provinsi).
 - Avatar URL.
 - Gender.
 - Tautan sosial (Instagram, Facebook, Twitter, TikTok, website).
- **Field yang TIDAK dapat di-update via Manage Profile**:
 - **Username** — canonical identity, immutable setelah ditetapkan; ditampilkan read-only.
 - Email — perlu jalur ganti email + re-verifikasi.
 - Password — perlu jalur ganti password (lewat External).
 - Nomor telepon — perlu OTP.
 - Status akun (`active`/`suspended`/`banned`) — hanya admin (Governance).
- **Username uniqueness** dievaluasi pada setiap establishment (registrasi / Complete Profile); race condition ditangani dengan menolak submit yang gagal unique constraint.

## Forbidden Behaviors

- Sistem tidak boleh mengizinkan user mengubah `firebase_uid`, `email_verified`, status akun, atau role melalui Manage Profile.
- Sistem tidak boleh mengganti email tanpa re-verifikasi.
- Sistem tidak boleh mengganti nomor telepon tanpa OTP.
- Sistem tidak boleh menyimpan perubahan saat validasi format gagal.
- Sistem tidak boleh mengizinkan atau mengiklankan perubahan username — username immutable setelah ditetapkan.

## Notes

- Profile **pribadi** vs **toko (farm/seller)** adalah dua entitas berbeda. Perubahan satu tidak otomatis mengubah lainnya. Toko diatur di [Become Seller](./become-seller.md).
