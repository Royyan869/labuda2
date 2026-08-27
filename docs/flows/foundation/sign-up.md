# Sign Up

> **Status:** STABLE
> **Domain:** Identity & Auth
> **Last reviewed:** 2026-05-09

> **Doctrine references:**
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — sign-up clears Layer A and (for email/password) Layer B in one step.
> - [Username Lifecycle](../doctrine/username-lifecycle.md) — initial establishment; username immutable setelah ditetapkan.
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) — what the new account can do before verifying.
> - [Capability Matrix](../doctrine/capability-matrix.md) — capability lookup per stage.

## Purpose

Memberi cara bagi Guest untuk membuat akun baru di Labuda menggunakan email + password. Setelah sign up, Guest menjadi Registered User (Identity Complete Account) dan menerima email verifikasi.

## Actors

- **Guest** — pemicu sign up.
- **External (Firebase Auth)** — penyimpan kredensial (email + password) dan pengirim email verifikasi.
- **System** — mensinkronkan akun Firebase ke akun internal Labuda dan menerbitkan token sesi.

## Preconditions

- Guest belum memiliki akun dengan email yang sama.
- Guest belum memiliki akun dengan username yang sama.
- Username **wajib** diisi pada form sign-up untuk jalur email/password — jalur ini menggabungkan Authentication + Identity Completion dalam satu langkah, sehingga akun langsung menjadi Identity Complete Account saat dibuat.

## Main Flow

1. Guest mengisi formulir sign up: **email**, **password**, **nama lengkap**, dan **username**.
2. Sistem memvalidasi format email, kekuatan password, dan format username (lihat `### Business Rules`).
3. Akun kredensial dibuat di sisi External (Firebase Auth).
4. Sistem membuat record user internal dengan status akun **active** dan menautkan ke akun External.
5. Sistem membuat profile dengan username + nama lengkap, dan menandai `profile_completed=true`.
6. Sistem mengirim **email verifikasi** ke email Guest (via External).
7. Sistem menerbitkan token sesi (access token + refresh token).
8. Registered User masuk ke aplikasi sebagai **Identity Complete Account**. Browsing authority terbuka; interaction dan transaction authority masih tertutup sampai email diverifikasi (lihat [Email Gating Matrix](../doctrine/email-gating-matrix.md)). Mobile menampilkan banner verifikasi persistent — tidak ada blok navigasi global.

## Alternate Flows

- **Sign Up via Google OAuth** — tidak menggunakan flow ini. Lihat [Sign In (Google OAuth)](./sign-in-google.md).

## Failure / Rejection Cases

- **Email sudah dipakai akun lain** → `email-already-in-use`.
- **Password lemah** → `weak-password`.
- **Format email tidak valid** → `invalid-email`.
- **Username sudah dipakai user lain** → `INVALID_USERNAME`.
- **Username termasuk daftar reserved** (mis. `admin`, `root`, `support`, `labuda`) → `USERNAME_RESERVED`.
- **Username kosong atau melanggar format** (lihat `### Business Rules`) → ditolak.

## State Changes

- **Akun User**: tidak ada → `active`.
- **Email Verification**: tidak ada → belum-terverifikasi (verifikasi dilakukan di flow [Email Verification](./email-verification.md)).
- **Profile**: tidak ada → terisi (`username`, `full_name`); `profile_completed=true`.
- **Sesi**: belum ada → access token + refresh token aktif.
- **Account stage**: Guest → Identity Complete Account (Layer A + B lulus; Layer C belum).

## Financial Impact

Tidak ada. Tidak menyentuh Wallet/Escrow/Coins.

## Notifications

- **Guest**: email verifikasi dari External (Firebase Auth) berisi tautan aktivasi.
- Tidak ada notifikasi internal Labuda di tahap sign up.

## Cross-Domain Relations

- **Email Verification**: sign up otomatis memicu jalur verifikasi. Lihat [Email Verification](./email-verification.md).
- **Complete Profile**: tidak diperlukan untuk jalur email/password (username sudah dikumpulkan di sini). Lihat [Complete Profile](./complete-profile.md) untuk jalur Google.
- **Notification (preferensi)**: akun baru memakai default preferensi notifikasi. Lihat [Manage Preferences](./manage-preferences.md).

## Business Rules

- **Username** wajib pada saat sign up jalur email/password.
- **Username** unique global.
- **Username format (mobile)**: 3–30 karakter, hanya `a-z`, `0-9`, dan `_`. Backend menerapkan format yang sama via `identityusername.ValidateFormat` (kanonik).
- **Username reserved**: tidak boleh menggunakan nama yang masuk daftar reserved (admin, root, support, system, labuda, dan lain-lain).
- **Pemilihan username di sini adalah *initial establishment*** — tunduk hanya pada uniqueness + format + reserved list. Username immutable setelah ditetapkan; tidak ada rename. Lihat [Username Lifecycle](../doctrine/username-lifecycle.md).
- **Email** unique global di sisi External; satu email = satu akun.
- **Akun yang baru dibuat** secara otomatis ber-status `active`.
- **Idempotency**: pembuatan ulang dengan email yang sama selalu ditolak — tidak ada upsert.

## Forbidden Behaviors

- Sistem tidak boleh menerbitkan token sesi sebelum akun User berhasil dibuat di sisi internal.
- Sistem tidak boleh membuat dua akun User berbeda untuk satu email yang sama.
- Sistem tidak boleh menandai akun sebagai `email_verified` tanpa konfirmasi dari External.
- Mobile tidak boleh mem-blok navigasi global akun yang baru sign-up — lihat [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Notes

- Otoritas kredensial (email + password hash) tetap di External (Firebase Auth). Backend memegang otoritas atas: status akun, username, role, profil.
- Sign up jalur email/password menggabungkan Authentication + Identity Completion dalam satu form. Jalur Google memisahkan keduanya. Kedua jalur konvergen pada outcome canonical yang sama — lihat [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md).
