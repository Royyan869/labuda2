# Sign In (Google OAuth)

> **Status:** STABLE
> **Domain:** Identity & Auth
> **Last reviewed:** 2026-05-09

> **Doctrine references:**
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — Google clears Layer A; Layer B is held behind the Complete Profile gate.
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) — gating after sign-in.

## Purpose

Memberi cara bagi Guest atau Registered User untuk masuk ke Labuda menggunakan akun Google. Jalur ini dipakai baik untuk **akun baru** (sign up via Google) maupun **akun lama** (sign in ulang via Google).

## Actors

- **Guest** (jika akun baru) atau **Registered User** (jika akun lama).
- **External (Firebase Auth + Google OAuth)** — penyedia identitas.
- **System** — menautkan identity Google ke akun User internal, atau membuat akun baru jika belum ada.

## Preconditions

- User memiliki akun Google.
- Aplikasi mobile dapat berkomunikasi dengan Google sign-in flow.

## Main Flow

1. User memilih "Sign in with Google" di layar sign in.
2. External (Google OAuth) menampilkan picker akun Google.
3. User memilih akun Google.
4. External mengembalikan identity token ke aplikasi.
5. Sistem memetakan ke akun User internal:
 - **Email Google sudah terdaftar di backend** → identity Google **ditautkan** ke akun yang sudah ada (1-email = 1-user).
 - **Email Google belum terdaftar** → akun User baru dibuat dengan status `active`.
6. Sistem mensinkronkan flag **email-verified** dari Google: jika Google menyatakan email-nya verified, backend menandai email terverifikasi otomatis. Bila tidak, akun masuk ke status unverified.
7. Sistem memeriksa **identity completion** (Layer B):
 - Jika `profile_completed=false` → user wajib diarahkan ke [Complete Profile](./complete-profile.md) sebagai gate. Akun pada state ini berstatus **Authenticated Account** — belum full participant.
 - Jika `profile_completed=true` → user masuk ke layar utama sebagai **Identity Complete Account**. Email verification gating tetap aktif untuk action gated.
8. Sistem menerbitkan access token + refresh token internal.

## Alternate Flows

- **Akun lama (email/password)** sign in via Google dengan email yang sama → sistem otomatis menautkan identity Google. User tetap satu akun.
- **Akun lama** sign in via Google dengan email **berbeda** → External memperlakukan sebagai identitas berbeda; akan terbentuk akun User baru terpisah. Tidak ada merge otomatis untuk dua email berbeda.

## Failure / Rejection Cases

- **User membatalkan Google picker** → kembali ke layar sign in tanpa perubahan.
- **Identity token Google tidak valid** → ditolak.
- **Akun internal yang ditemukan ber-status `suspended`/`banned`** → ditolak dengan pesan status akun.
- **Race kondisi tautan** (dua percobaan paralel) → satu percobaan gagal; user diminta coba lagi.

## State Changes

- **Akun User** (jika baru): tidak ada → `active`.
- **Email Verification**: jika Google email verified, langsung **terverifikasi**; jika tidak, mengikuti flow [Email Verification](./email-verification.md).
- **Tautan identity Google ↔ akun User**: belum ada → tertaut.
- **Sesi**: tidak ada → access token + refresh token aktif.
- **Account stage**: Guest → Authenticated Account (jika `profile_completed=false`) atau Identity Complete Account (jika `profile_completed=true`).

## Financial Impact

Tidak ada.

## Notifications

- Tidak ada notifikasi user-visible khusus.

## Cross-Domain Relations

- **Complete Profile**: jalur Google **wajib** melewati [Complete Profile](./complete-profile.md) untuk mengambil **username** sebelum user diberi full participation. Jalur email/password menggabungkan langkah ini di form sign-up.
- **Email Verification**: gating independen dari identity completion. Bila Google menyatakan email verified, backend sinkron otomatis. Bila tidak, gate tetap dievaluasi inline pada gated action. Lihat [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Business Rules

- **Satu email = satu akun User** — backend menegakkan dengan menautkan identity Google ke akun yang sudah ada bila email sama.
- **Sumber kebenaran email-verified** untuk akun yang sign-up via Google adalah klaim dari Google; backend menyinkronkannya ke flag internal `email_verified_at`.
- **Username** tidak dapat diturunkan dari email Google atau displayName Google secara final; **wajib** dipilih ulang oleh user di [Complete Profile](./complete-profile.md). Saran username dapat dihasilkan dari displayName, tetapi user harus konfirmasi.
- **Tautan akun**: jika seorang user pertama sign up via email/password lalu suatu hari memakai Google dengan email yang sama, tautan terjadi otomatis tanpa langkah konfirmasi tambahan.

## Forbidden Behaviors

- Sistem tidak boleh membuat dua akun User terpisah untuk satu email yang sama, baik via email/password atau via Google.
- Sistem tidak boleh menerima password Google atau membuat password lokal saat user sign-in via Google.
- Sistem tidak boleh menandai email-verified tanpa klaim dari External — jika Google memberi `email_verified=false`, backend tidak boleh menandainya verified.
- Sistem tidak boleh menggabungkan dua akun yang **email-nya berbeda** hanya karena nama sama atau Google ID terhubung — merge dua akun berbeda adalah operasi support manual, bukan otomatisasi.
- Mobile **WAJIB** mengarahkan akun yang `profile_completed=false` ke Complete Profile gate sebelum full app entry. Identity-incomplete user **TIDAK BOLEH** mendapat akses ke fitur lain (browse listing, view profile lain) sampai username dipilih.
- Sistem tidak boleh memberi user Google yang baru sign-in akses ke action gated email verification meski Google menyatakan email verified, **bila Layer B (Identity Completion) belum lulus**.

## Notes

- Logout: sama seperti jalur email/password — sisi-klien (hapus token).
- Flow ini tidak mendukung "Google linking dari user logged-in" sebagai aksi terpisah; tautan terjadi sebagai efek samping dari sign-in.
