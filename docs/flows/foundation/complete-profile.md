# Complete Profile (Identity Completion gate)

> **Status:** STABLE
> **Domain:** Profile / Identity & Auth
> **Last reviewed:** 2026-05-09

> **Doctrine references:**
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — flow ini adalah Layer B gate; tanpa lulus, tidak ada full app entry.
> - [Username Lifecycle](../doctrine/username-lifecycle.md) — initial establishment di sini berbeda dari rename pasca-onboarding.
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) — Layer C berlanjut independen dari flow ini.

## Purpose

Menjadi **canonical identity completion gate** untuk akun yang lulus Authentication tetapi belum memiliki canonical public identity (`username`).

Flow ini terutama dipicu untuk akun yang sign-up via Google. Akun email/password tidak memicu flow ini karena identity completion sudah terjadi di form sign-up.

> Flow ini **bukan** flow pelengkapan profil opsional (avatar, bio, lokasi, DOB). Flow ini **hanya** mengumpulkan minimum public identity (`username`). `profile_completed=true` setelah flow ini berarti minimum public identity established — bukan "full bio complete", "avatar complete", "email verified", atau "seller verified".

## Actors

- **Authenticated Account** — akun yang lulus Layer A tetapi belum lulus Layer B. Tidak diberi full participation; hanya boleh memicu Complete Profile gate.
- **System** — memvalidasi username, menyimpan ke profile, dan menandai `profile_completed=true` untuk transisi ke Identity Complete Account.

## Preconditions

- User sudah memiliki akun (`active`) — Layer A lulus.
- `profile_completed=false`.
- User sudah sign-in (sesi aktif).
- User belum mendapat full app entry — Authenticated-but-not-identity-complete account tidak boleh mengakses fitur lain (browse listing, view profile, dll.) sampai username dipilih.

## Main Flow

1. Setelah sign-in, sistem mendeteksi `profile_completed=false` dan mengarahkan user ke layar Complete Profile sebagai **gate** — bukan opsi yang dapat di-skip.
2. Sistem menampilkan saran username (turunan dari Google displayName atau bagian lokal email) — user dapat menerima atau mengganti.
3. User mengetik username yang diinginkan.
4. Sistem melakukan **availability check** dengan debounce.
5. Tombol "Lanjutkan" aktif hanya jika username valid dan tersedia.
6. User menekan "Lanjutkan".
7. Sistem menyimpan username dan menandai `profile_completed=true`. Akun bertransisi dari **Authenticated Account** ke **Identity Complete Account**.
8. User diarahkan ke layar utama sebagai full participant. Email verification gating tetap aktif sesuai [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Alternate Flows

- **Akun sign-up via email/password**: tidak melewati flow ini — username sudah dikumpulkan saat sign up.
- **Race condition** — username dipilih, lulus availability check, tetapi user lain menyimpan username yang sama lebih dulu. Sistem mengembalikan kegagalan dan user memilih ulang.

## Failure / Rejection Cases

- **Username kosong** → tombol Lanjutkan tidak aktif.
- **Username < 3 karakter** atau > 30 karakter → ditolak.
- **Username melanggar format** (lihat `### Business Rules`) → ditolak.
- **Username termasuk daftar reserved** → ditolak.
- **Username sudah dipakai user lain** → ditolak; user memilih ulang.

## State Changes

- **Profile**: belum punya canonical public identity → terisi (`username` ada).
- **Flag `profile_completed`**: `false` → `true`. Semantic: minimum public identity established. Bukan full bio complete, bukan avatar complete, bukan email verified, bukan seller verified.
- **Account stage**: Authenticated Account → Identity Complete Account.
- **Akses aplikasi**: gate aktif → user bebas masuk ke layar utama dengan otoritas browsing. Interaction & transaction authority masih dievaluasi terhadap [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Financial Impact

Tidak ada.

## Notifications

- Tidak ada notifikasi user-visible khusus pada flow ini.

## Cross-Domain Relations

- **Sign In Google**: jalur pemicu utama. Lihat [Sign In (Google)](./sign-in-google.md).
- **Sign Up (email/password)**: jalur email/password tidak memicu flow ini. Lihat [Sign Up](./sign-up.md).
- **Manage Profile**: setelah profile lengkap, user dapat memutakhirkan field lain melalui [Manage Profile](./manage-profile.md). Username immutable setelah ditetapkan — lihat [Username Lifecycle](../doctrine/username-lifecycle.md).
- **Email Verification**: kebijakan email gating berlaku **independen** dari complete-profile.

## Business Rules

- **Field mandatori untuk dianggap "complete"**: hanya **username**.
- **Field opsional** (boleh diisi sekarang atau di Manage Profile): display name (otomatis dari Google), avatar, bio, lokasi, tanggal lahir, gender, kontak sosial.
- **Username format (mobile)**: 3–30 karakter, hanya `a-z`, `0-9`, `_`. Backend menerapkan format yang sama via `identityusername.ValidateFormat` (kanonik).
- **Username unique** secara global.
- **Username daftar reserved** ditegakkan (admin, root, support, system, labuda, dan lainnya).
- **Tidak ada cool-down** atau gating ekstra setelah complete profile selesai — user langsung dapat memakai aplikasi.
- **Pemilihan username di sini adalah *initial establishment*** — tunduk hanya pada uniqueness + format + reserved list. Username immutable setelah ditetapkan; tidak ada rename di Manage Profile maupun di tempat lain. Lihat [Username Lifecycle](../doctrine/username-lifecycle.md).

## Forbidden Behaviors

- Sistem **TIDAK BOLEH** memberi akses ke layar utama jika `profile_completed=false`. Mobile WAJIB menegakkan gate ini secara global.
- Sistem **TIDAK BOLEH** memperlakukan flow ini sebagai opsional polish — flow ini adalah identity completion gate.
- Sistem tidak boleh menyimpan username yang melanggar format atau termasuk reserved.
- Sistem tidak boleh mengaktifkan tombol "Lanjutkan" sebelum availability check sukses.

## Notes

- Tujuan flow ini bukan mengumpulkan data demografi atau marketing — hanya memenuhi minimum public identity.
- Field DOB, gender, dan lainnya akan didokumentasikan pada [Manage Profile](./manage-profile.md).
