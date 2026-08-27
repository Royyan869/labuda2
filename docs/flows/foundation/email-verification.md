# Email Verification

> **Status:** STABLE
> **Domain:** Identity & Auth
> **Last reviewed:** 2026-05-09

> **Doctrine references:**
> - [Email Gating Matrix](../doctrine/email-gating-matrix.md) — canonical ALLOWED / BLOCKED lists, enforcement pattern, lifecycle of unverified accounts. **Authoritative for all gating rules.**
> - [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) — email verification is Layer C; independent from B and D.
> - [Capability Matrix](../doctrine/capability-matrix.md) — per-capability lookup including Email Verification gate.

## Purpose

Memastikan **kepemilikan email** Registered User sebagai prasyarat masuk ke interaction & transaction authority. Memberi platform kontak email yang reliable untuk financial-recovery, dispute notification, dan trust escalation.

## Actors

- **Registered User (unverified)** — boleh memasuki app dan menjelajah area read-only.
- **Registered User (verified)** — memiliki interaction & transaction authority sesuai role.
- **External (Firebase Auth)** — pengirim email verifikasi dan otoritas atas status `email_verified`.
- **System (mobile)** — menyajikan banner persistent + inline gate.
- **System (backend)** — menolak action gated bila `email_verified=false`.

## Preconditions

- User sudah membuat akun (via [Sign Up](./sign-up.md) atau [Sign In Google](./sign-in-google.md)) — Layer A lulus.
- User sudah **identity-complete** — Layer B lulus (`profile_completed=true`). Akun yang belum identity-complete tidak masuk flow ini; mereka tertahan di Complete Profile gate.
- Email pada akun Google **mungkin sudah otomatis verified** oleh External — flow tetap berlaku tetapi langkahnya dilewati.

## Main Flow

### Onboarding flow (pasca register, identity-complete)

1. Setelah sign-up (atau setelah Complete Profile pada jalur Google), External mengirim email verifikasi.
2. User yang sudah identity-complete masuk ke aplikasi dan dapat menjelajah area ALLOWED (lihat [Email Gating Matrix](../doctrine/email-gating-matrix.md)).
3. Mobile menampilkan **banner persistent** verifikasi (pengingat halus, bukan blocker navigasi).
4. User dapat membuka email dan menekan tautan verifikasi kapan saja.
5. External menandai email sebagai verified.
6. User kembali ke aplikasi — sistem mendeteksi status verified terbaru saat:
 - User menekan "Saya sudah verifikasi" pada banner / inline prompt, atau
 - Validasi sesi periodik berikutnya, atau
 - User mencoba memicu gated action (memicu re-check otomatis).
7. Backend menyinkronkan flag `email_verified_at`.
8. Banner verifikasi hilang; user kini memiliki interaction & transaction authority sesuai role.

### Inline gate flow (saat user memicu gated action)

1. User dalam keadaan unverified mencoba memicu action di daftar BLOCKED.
2. Backend menolak request dengan reason code stabil (`EMAIL_VERIFICATION_REQUIRED`).
3. Mobile menampilkan **inline prompt** di lokasi action: jelaskan kebutuhan verifikasi, tampilkan tombol "Verifikasi sekarang" dan tombol "Nanti".
4. User memilih: verifikasi sekarang → ke verification flow; nanti → kembali ke browsing.

> Wording banner persistent dan inline prompt adalah UX-level concern; tidak dikunci pada level flow ini.

## Alternate Flows

- **Sign In Google dengan email verified** → langkah verifikasi dilewati otomatis; `email_verified_at` disinkron langsung saat sign in.
- **Resend email verifikasi** → user dapat meminta pengiriman ulang; rate-limit dipegang oleh External.
- **User sign-out tanpa verifikasi** → akun tetap eksis. Tidak ada auto-delete. Tidak ada hard expiry.

## Failure / Rejection Cases

- **Tautan verifikasi kedaluwarsa** → user dapat meminta resend.
- **Tautan verifikasi sudah dipakai** → External menolak duplikat.
- **User mengubah alamat email setelah verifikasi** → flag verified **reset** ke false untuk email baru. User kembali ke status unverified untuk gated actions.
- **Server backend tidak sinkron** dengan klaim External → mobile dapat menampilkan status sementara "belum verified"; sinkronisasi terjadi pada percobaan berikutnya.
- **User mencoba gated action tanpa verified** → backend reject; mobile tampilkan inline prompt.

## State Changes

- **Email Verification (External)**: belum-verified → verified.
- **Akun User (backend)**: `email_verified_at` belum terisi → terisi dengan timestamp.
- **Mobile UI**: banner persistent aktif → banner hilang.
- **Authority user**: browsing-only → browsing + interaction + transaction (sesuai role).
- **Account stage**: Identity Complete Account → Email Verified Account.

## Financial Impact

Flow ini sendiri tidak memiliki dampak finansial langsung. Tetapi flow ini adalah **gate untuk semua transaction authority** — checkout, bid, withdraw, pembayaran subscription. Tanpa email verified, financial action ditolak backend.

Memori `Money authority model` (gateway-funded commerce) tetap berlaku: kebijakan ini tidak mengubah semantic refund / payout / escrow — hanya menambah / mempertegas gate akses ke action finansial.

## Notifications

- **External**: user menerima email verifikasi dari Firebase Auth.
- **Internal Labuda**: reminder verifikasi MAY dikirim; cadence tidak dikunci pada level doctrine.

## Cross-Domain Relations

- **Sign Up / Sign In Email / Sign In Google**: jalur masuk ke flow ini.
- **Become Seller / Subscription / Comment / Post / Chat / Negotiation / Checkout / Bid / Withdraw**: gating consumers — semua merujuk daftar BLOCKED canonical di [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Business Rules

- **Source of truth status email-verified** = External (Firebase Auth). Backend menyinkronkan dan mempersist timestamp.
- Backend = single source of truth untuk daftar action gated. Mobile menyesuaikan UX, tidak menambah / mengurangi gating.
- Pernah verified = **permanen** untuk email tersebut. Bila email diganti, status reset ke belum-verified untuk email baru.
- Tidak ada auto-delete / hard expiry untuk akun unverified.
- Tidak ada limit eksplisit di backend untuk re-send email verifikasi — rate-limit di External.

> Aturan gating canonical (ALLOWED / BLOCKED, mandatory enforcement pattern, daftar lengkap) hidup di [Email Gating Matrix](../doctrine/email-gating-matrix.md). Flow ini merujuk; tidak menggantikan.

## Forbidden Behaviors

- Sistem tidak boleh menandai `email_verified_at` di backend tanpa klaim verified dari External.
- Sistem tidak boleh men-skip verifikasi untuk email yang sudah pernah dipakai akun lain (mis. setelah delete-create-ulang) — verifikasi harus dilakukan ulang.
- Mobile tidak boleh men-cache `emailVerified=true` lokal melebihi durasi yang dijamin oleh validasi sesi periodik.

> Forbidden behaviors yang berlaku doctrine-wide (mobile blok navigasi, backend extend BLOCKED tanpa amendment, dll.) hidup di [Email Gating Matrix](../doctrine/email-gating-matrix.md).

## Notes

- Status verifikasi tersimpan di dua tempat: klaim External (otoritatif) + timestamp backend (snapshot). Bila beda, External menang.
- Backend Labuda tidak mengontrol pengiriman / format email verifikasi — itu sepenuhnya External (Firebase Auth).
