# LABUDA — AUTH IDENTITY & EMAIL VERIFICATION (DESIGN SCOPE v2)

Status: **DIIMPLEMENTASIKAN (stages 1–5 §10). D3 dieksekusi (drop DB + migrate + reseed, 2026-09-22) dan integration test matriks exchange 14/14 PASS di `labuda_test`. Tersisa runtime proof §8.6 uji end-to-end dua provider dari device (skip dulu — device belum tersedia) sebelum closure §16.12.**

> **⚠️ AKAR MASALAH RUNTIME TERUNGKAP (2026-09-22, sesi uji device pertama):**
> Gejala "daftar → Gagal Memuat Data / akun ada di Firebase tapi tidak di PostgreSQL" BUKAN bug desain D1–D4 — backend berjalan di **`DEV_MOCK_FIREBASE_AUTH=true`** (backend/.env): `VerifyIDTokenMock` menerima token APA PUN dan memfabrikasi identitas dari string mentah token (UID = `mock-` + hash(token), email = base64 header JWT → `eyjhbg...@test.com`). Token Firebase berganti tiap refresh → identitas palsu berubah per request → sesi ditendang → `no-current-user` → splash degraded. Row fabrikasi di dev DB sudah dihapus, `DEV_MOCK_FIREBASE_AUTH=false` diberlakukan, backend di-restart dengan Firebase Admin asli (project `labuda-79de2`) dan dibuktikan: token sampah kini ditolak `INVALID_TOKEN`, tidak ada row baru terfabrikasi. Perbaikan ikutan di mobile: `no-current-user` diklasifikasi `identityInvalid` (sign-out bersih, bukan degraded) + auto-lowercase input username (signup & complete-profile). Runtime proof §8.6 tinggal diulang di device dengan backend ini.
Klasifikasi masalah: **P1** — core flow registrasi tidak dapat diselesaikan (signup email & Google gagal; EMAIL_NOT_VERIFIED jatuh ke splash "Gagal Memuat Data" yang buntu).

Dokumen ini adalah satu-satunya design authority untuk scope ini. Tidak ada dokumen desain kedua.

---

## 1. Objective

Satu hasil: **registrasi dan login (email/password & Google) selesai end-to-end dengan satu model identitas sederhana** — satu manusia = satu Firebase user = satu baris `users` — dan verifikasi email menjadi gerbang eksplisit sebelum exchange, bukan error yang jatuh ke splash degraded.

## 2. Keputusan (Owner & authority teknis)

| # | Keputusan | Otoritas | Status |
|---|-----------|----------|--------|
| D1 | **Firebase linking tunggal**: satu Firebase user per manusia (linkWithCredential di klien); backend tetap satu kolom `firebase_uid` sebagai kredensial. TIDAK ada tabel `user_identities`. | Owner (via diskusi desain) | ✓ diputuskan |
| D2 | **Hard gate verifikasi**: email/password wajib verified SEBELUM exchange. Tidak ada jalur "exchange dulu, gating belakangan". | Owner | ✓ diputuskan |
| D3 | **DB dev di-drop & reseed** — tanpa data migration (§16.10 Test Data Is Disposable). | Owner | ✓ diputuskan |
| D4 | **Bound + UID berbeda = TOLAK** (409 `IDENTITY_CONFLICT`). Tidak ada re-bind otomatis. Anomali dua UID untuk satu email adalah kondisi error, bukan fallback. | Owner (diputuskan 2026-09-22) | ✓ diputuskan |

## 3. Canonical truth

### 3.1 Model

```
Manusia 1──1 Firebase user (N provider ter-link: password, google.com)
        1──1 users row (PostgreSQL)
              email        = account key (normalized, UNIQUE) — INV-1
              firebase_uid = kredensial binding (NULL = unbound; migrasi 000107) — INV-2
```

### 3.2 Invariant

| INV | Pernyataan |
|-----|------------|
| INV-1 | Satu email ternormalisasi ⇒ paling banyak satu baris `users` aktif. |
| INV-2 | Satu `firebase_uid` ⇒ paling banyak satu baris `users` aktif (UNIQUE). |
| INV-3 | `POST /auth/firebase/exchange` satu-satunya penulis `firebase_uid` di luar `createUser`. |
| INV-4 | Exchange membuat akun / men-bind ke row unbound **hanya** jika token Firebase `email_verified=true`. Tanpa pengecualian. |
| INV-5 | Row **bound** + UID berbeda = 409 `IDENTITY_CONFLICT`, selalu. Tidak ada re-bind, tidak ada audit-log-re-bind, tidak ada fallback. (D4) |
| INV-6 | Exchange tidak pernah mengubah email, role, account_status. |
| INV-7 | EMAIL_NOT_VERIFIED / USERNAME_* adalah keputusan bisnis dengan alur UI-nya sendiri (verify screen / form username). Splash degraded HANYA untuk kegagalan infrastruktur (network/5xx/timeout). |
| INV-8 | Verifikasi email satu jalur: verify → exchange. Tidak ada jalur kedua (tanpa "coba exchange dulu"). |

### 3.3 Matriks exchange (canonical, final)

| Kondisi row `users` untuk email tsb | token verified? | Hasil |
|---|---|---|
| Tidak ada row | true | **Create** + bind (INV-4) |
| Tidak ada row | false | 403 `EMAIL_NOT_VERIFIED` |
| Row unbound (NULL) | true | **Bind** |
| Row unbound | false | 403 `EMAIL_NOT_VERIFIED` |
| Row bound, UID sama | — | Jalur normal |
| Row bound, UID lain | true/false | **409 `IDENTITY_CONFLICT`** (INV-5/D4) |

Seis baris. Tidak ada kasus ketujuh.

## 4. Forbidden design (Kill Once, Lock Forever)

1. **Auto-linking / re-bind berbasis email** di backend (era B2: `EMAIL_ALREADY_REGISTERED` otomatis; era "single binding rule" re-bind). `bindFirebaseIdentity` hanya boleh menulis ke row **unbound**. Bound + UID lain → `IDENTITY_CONFLICT`.
2. **Exchange sebelum verified** di mobile. Tidak ada path signup yang memanggil exchange dengan `email_verified=false`.
3. **Progressive verification gating untuk user authenticated** (banner + gate per fitur). Dengan D2+D3, user authenticated selalu verified ⇒ seluruh gate ini dead code:
   - `email_verification_banner.dart`
   - `isEmailUnverifiedProvider` consumer yang MEMBLOKIR aksi: `follow_button.dart`, `chat_list_screen.dart`, `new_chat_user_list_widget.dart`, `order_user_info_card.dart`, `auction_detail_screen.dart`, `feed_renderers.dart`, `seller_upgrade_wizard_screen.dart` (gate verifikasi; display "Verified/Not verified" murni informatif boleh tinggal bila membaca source of truth yang sama).
   - Alur "kirim verifikasi dari profile" (`personal_information_screen._sendEmailVerification` + tombol resend di `email_verification_field`) — digantikan oleh verify screen onboarding yang satu-satunya.
4. **Dua datasource exchange** — `AuthApiDatasource.exchangeFirebaseSession` sudah dihapus di diff D2-A; `UserApiDatasource.exchangeFirebaseSession` adalah satu-satunya. Tidak boleh dihidupkan lagi.
5. **Auto-retry backoff timer** pada sync failure — sudah dibunuh (Stage 3B); degraded bersifat terminal + manual retry. Tidak boleh dikembalikan.
6. **Pre-flight `fetchSignInMethodsForEmail`** — TIDAK diintroduksi. Firebase sudah menolak `email-already-in-use`; dua mekanisme untuk satu truth = authority ganda.
7. **Client-side username authority** — backend tetap otoritas format/reserved/uniqueness.
8. **"Gagal Memuat Data"** sebagai permukaan untuk keputusan bisnis (verifikasi/username).

## 5. Removal manifest (bounded — hanya lapisan yang disentuh scope ini)

- **Mobile**: state/enum klasifikasi (`EMAIL_NOT_VERIFIED` → state verifikasi, bukan degraded); gate-gate pada butir 4.3; banner verifikasi; path resend dari profile; artefak format diff D2-A (assignment menempel di baris `if`).
- **Backend**: sisa kondisi re-bind / komentar yang mengarah ke auto-linking; error `EMAIL_NOT_VERIFIED` tetap (defense-in-depth) + tambah `IDENTITY_CONFLICT`.
- **Test/fixture**: test yang menegakkan re-bind otomatis (mis. bagian `auth_handler_email_identity_integration_test.go` era B2) diperbaiki/hapus → diganti negative contract (§7).
- **Docs/komentar**: bagian `cara-kerja` tidak disentuh; komentar kode yang masih menyebut "linking rejection" lama diselaraskan.
- **DB**: drop + reseed (D3) — bukan migrasi data.

## 6. Model & alur

### 6.1 State machine (mobile)

```
AuthStatePendingEmailVerification { email, username?, pendingCredential? }
  → router: /auth/verify-email (pola redirect eksklusif seperti
    AuthStateRequiresProfileCompletion / AuthStateAccountRestricted)
  → VerifyEmailScreen (screen existing, DIADAPTASI — bukan diduplikasi):
    polling reload() 3–5 dtk · resend + cooldown 60 dtk · escape: keluar bersih
  → verified → exchange:
      signup: bawa username (intent EmailSignupIntent dipertahankan untuk retry USERNAME_TAKEN)
      mixed-provider: linkWithCredential(pendingCredential) dulu → exchange
```

Klasifikasi error — satu pemetaan, tanpa jalur kedua:

```
EMAIL_NOT_VERIFIED  → AuthStatePendingEmailVerification (bukan kind degraded baru)
IDENTITY_CONFLICT   → error terminal + signOut bersih (pesan jelas)
INVALID_TOKEN       → identityInvalid (tetap)
ACCOUNT_*           → tetap
5xx/timeout/network → backendUnavailable → splash degraded (satu-satunya pemakai splash)
```

### 6.2 Alur per provider

- **Signup email**: form → `createUserWithEmailAndPassword` → `sendEmailVerification` → `pendingEmailVerification` → verified → exchange(username) → complete-profile bila perlu.
- **Login email**: `signInWithEmail` → unverified? → `pendingEmailVerification` (TIDAK exchange) → verified → exchange.
- **Google**: `signInWithCredential` → email Firebase selalu verified → exchange. (`email_verified=true` untuk akun Google adalah klaim yang WAJIB dibuktikan runtime — lihat §8.)
- **Mixed provider (D1)**: Google → `account-exists-with-different-credential` → simpan pendingCredential → sign-in metode lama (bila unverified → verifikasi dulu) → `linkWithCredential` → satu UID → exchange. Kegagalan link = pesan eksplisit, tanpa retry diam-diam.

## 7. Negative contract (proof desain lama mati)

- Tidak ada kode yang memanggil re-bind pada row bound: test — exchange UID lain ke row bound harus 409 `IDENTITY_CONFLICT` (bukan bind, bukan 500).
- Tidak ada pemanggil exchange dengan token unverified dari jalur signup: test — listener/`signUpWithEmail` tidak boleh memicu exchange sebelum verified.
- Gate-gate terhapus: test — follow/chat/order/auction tidak lagi menampilkan dialog "verifikasi email" untuk user authenticated.
- Tidak ada `AuthApiDatasource.exchangeFirebaseSession`: residue search = 0 hit.
- Tidak ada field timer auto-retry: residue search = 0 hit.

## 8. Required proof

1. **Focused test**: klasifikasi error & state machine (positive + negative §7).
2. **Integration test backend**: matriks §3.3 enam baris, termasuk concurrency advisory-lock (satu email, dua exchange paralel ⇒ satu row).
3. **Build/analyze**: Go backend + Flutter analyze lapisan terdampak.
4. **Residue search**: butir §4 & §7 (grep = 0).
5. **Runtime proof (§16.9)**: `DB (drop+reseed) → backend → exchange (Google & email-verified fixture) → mobile → state → UI`, dua provider, satu email yang sama untuk signup email lalu login Google.
6. Klaim "Google selalu verified" diverifikasi di runtime proof sebelum diterima.

## 9. Scope / Protected

**Scope**: `apps/mobile` auth domain (controller, state, router redirect, verify screen, repositori Google/signup), `backend/internal/identity/auth`, migrasi 000107, seed/dev-firebase-admin (sudah sesuai), test terkait.
**Protected / out of scope**: domain lain yang hanya membaca `is_email_verified` untuk display; seller wizard business rule di luar gate verifikasi; struktur router umum; perubahan promotion/billing yang sedang berjalan di working tree; `cara-kerja.md`.

## 10. Rollout & closure

1. Rapikan artefak diff D2-A (tanpa mengubah perilaku).
2. Backend: `IDENTITY_CONFLICT` (hapus re-bind), snapshot verified monotonic, test matriks §3.3.
3. Mobile: state `pendingEmailVerification` + adaptasi VerifyEmailScreen + klasifikasi + linking D1.
4. Removal manifest §5 (cleanup = penutup scope, §16.5).
5. Drop DB + reseed + rebuild (D3).
6. Runtime proof §8.6 → closure checklist §16.12 → `CLOSED — CANONICAL DESIGN LOCKED`.

Closure wajib menjawab 12 pertanyaan §16.12; jika ada jawaban belum cukup → tidak PASS.

## 11. Stop conditions

- D4 (tolak re-bind) ditolak owner → kembali ke §3.3 untuk keputusan baru, jangan implementasi dua-duanya.
- Runtime proof membantah asumsi (mis. Google mengirim `email_verified=false` di environment tertentu) → STOP, laporkan, keputusan baru.
- Ditemukan P0/P1 di luar scope yang memblokir (mis. token/session layer rusak) → STOP dan laporkan.
- Baseline working tree berubah signifikan saat implementasi → verifikasi ulang sebelum lanjut.
