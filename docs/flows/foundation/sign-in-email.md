# Sign In (Email/Password)

> **Status:** STABLE
> **Domain:** Identity & Auth
> **Last reviewed:** 2026-05-09

> **Doctrine references:**
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — sign-in clears Layer A only; B/C/D evaluated separately.
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) — gating after sign-in.

## Purpose

Memberi Registered User cara untuk masuk kembali ke akunnya menggunakan email + password yang dipakai saat sign up.

## Actors

- **Registered User** — pemilik akun yang ingin masuk.
- **External (Firebase Auth)** — memvalidasi kredensial.
- **System** — memeriksa status akun, menerbitkan token sesi internal, dan menerapkan layer evaluation.

## Preconditions

- Akun User sudah pernah dibuat melalui [Sign Up](./sign-up.md) atau [Sign In (Google)](./sign-in-google.md).
- User mengetahui email dan password yang valid.

## Main Flow

1. User memasukkan **email + password** di layar sign in.
2. Sistem memvalidasi kredensial ke External (Firebase Auth).
3. Sistem menerima identity token dari External.
4. Sistem mencari akun User internal yang tertaut ke identity tersebut.
5. Sistem memeriksa **status akun**: harus `active` (lihat `### Failure / Rejection Cases`).
6. Sistem menerbitkan **access token** dan **refresh token** internal.
7. Sistem memeriksa **identity completion** (Layer B):
 - Akun email/password: `profile_completed=true` selalu, sehingga user langsung masuk sebagai **Identity Complete Account**.
 - Akun yang pernah sign-up via Google dan kini tertaut email yang sama dengan `profile_completed=false` → user wajib menyelesaikan [Complete Profile](./complete-profile.md) sebelum full app entry.
8. User masuk ke layar utama.
 - Jika **email belum terverifikasi** (Layer C belum) → user dapat menjelajah area read-only; mobile menampilkan banner persistent + inline gate untuk action gated. Lihat [Email Gating Matrix](../doctrine/email-gating-matrix.md).
 - Jika email terverifikasi → interaction & transaction authority sesuai role.

## Alternate Flows

- **Sign in via Google OAuth dengan email yang sama**: sistem menautkan identity Google ke akun yang sudah ada. Lihat [Sign In (Google OAuth)](./sign-in-google.md).

## Failure / Rejection Cases

- **Password salah** → ditolak; pesan generik dari External.
- **Email tidak ditemukan di External** → ditolak.
- **Akun ada di External tapi tidak ada di backend internal** → ditolak; user diminta sign up ulang.
- **Akun internal `suspended`** → "Account is suspended".
- **Akun internal `banned`** → "Account is banned".
- **Akun terhapus** (soft-deleted) → ditolak.
- **Identity token tidak valid / kedaluwarsa** → ditolak.

## State Changes

- **Sesi**: tidak ada → access token + refresh token aktif.
- Tidak ada perubahan status akun (status ditentukan admin / governance, bukan oleh login).

## Financial Impact

Tidak ada.

## Notifications

- Tidak ada notifikasi user-visible di flow sign in normal.
- Jika akun di-suspend/ban, user melihat pesan in-screen.

## Cross-Domain Relations

- **Email Verification**: gating utama setelah sign in. Lihat [Email Verification](./email-verification.md) dan [Email Gating Matrix](../doctrine/email-gating-matrix.md).
- **Account Status (Governance)**: status `suspended` / `banned` ditetapkan dari domain Governance (Moderation / Warning).

## Business Rules

- Kredensial **email + password** dipegang oleh External; backend Labuda **tidak menyimpan password**.
- **Single source of truth** untuk validitas password = External (Firebase Auth).
- **Single source of truth** untuk status akun (`active`/`suspended`/`banned`) = backend Labuda.
- Token sesi (access + refresh) **stateless** — backend tidak menyimpan whitelist token.
- Refresh token TTL hingga **30 hari**; access token TTL pendek.
- Validasi sesi periodik di sisi mobile dapat memicu auto sign-out jika status akun berubah ke non-`active`.

## Forbidden Behaviors

- Sistem tidak boleh menerima password User di backend Labuda; password hanya boleh dipertukarkan dengan External.
- Sistem tidak boleh menerbitkan token sesi untuk akun yang `suspended` atau `banned`.
- Sistem tidak boleh melewati pemeriksaan status akun saat refresh token diminta — refresh harus tetap mengevaluasi status terbaru.
- Sistem tidak boleh memberi clue spesifik apakah suatu email "ada" atau "tidak ada" lebih dari yang dikembalikan External (anti probing).

## Notes

- "Sign in" di sini adalah otentikasi (Layer A), bukan otorisasi. Layer B/C/D dievaluasi terpisah.
- Logout adalah aksi sisi-klien (hapus token); tidak ada endpoint backend khusus untuk invalidasi token.
