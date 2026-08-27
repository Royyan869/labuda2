# Submit Seller Verification (KYC)

> **Status:** STABLE
> **Domain:** Verification

> **Doctrine references:**
> - [Verification Review Governance](../doctrine/verification-review-governance.md) — canonical lifecycle states + state machine.
> - [Seller Authority Separation](../doctrine/seller-authority-separation.md) — verification approves payout sub-gate, **bukan** selling sub-gate.
> - [Revocable Trust Model](../doctrine/revocable-trust-model.md) — `approved` bukan terminal.
> - [Trust Escalation Safety](../doctrine/trust-escalation-safety.md) — accountability pada setiap transisi.
> - [Capability Matrix](../doctrine/capability-matrix.md) — Layer 4 → Layer 5 transition impact.

## Purpose

Mengumpulkan data identitas Seller agar dapat diverifikasi oleh admin sebagai bagian dari governed trust process. Approved verification membuka **payout authority** (Layer 5).

## Actors

- **Seller** — pengirim data verifikasi.
- **System** — menyimpan dokumen, menjaga state machine verifikasi, menampilkan status terbaru kepada Seller.
- **Admin** — peran reviewer di flow [Seller Verification Review](./seller-verification-review.md).

## Preconditions

- **Layer A — Authentication:** user sign-in.
- **Layer B — Identity Completion:** `profile_completed=true`.
- **Layer C — Email Verification:** email sudah terverifikasi.
- **Layer D pre-state:** user sudah memiliki Seller capability ([Become Seller](./become-seller.md)).
- Belum ada submission `pending_review` aktif (user tidak dapat submit ulang saat sedang menunggu review).
- Boleh re-submit jika status terakhir adalah `rejected` atau `needs_resubmission`.
- Setelah status mencapai `approved`, submission baru tidak diperlukan untuk operasi normal — tetapi `approved` bukan terminal (lihat [Revocable Trust Model](../doctrine/revocable-trust-model.md)).

## Main Flow

1. Seller membuka layar Seller Verification.
2. Seller mengisi formulir KYC (identitas):
 - **Nama lengkap** (3–100 karakter).
 - **Nomor identitas (NIK)** — 16 digit untuk KTP Indonesia.
 - **Foto KTP** — gambar dokumen identitas.
 - **Selfie dengan KTP** — bukti pemegangan identitas.
3. Seller menekan **Submit**.
4. Sistem menyimpan dokumen dan transisi state: `not_submitted` / `rejected` / `needs_resubmission` → `pending_review`.
5. UI Seller menampilkan status "Menunggu Review".
6. Submission menunggu review admin di flow [Seller Verification Review](./seller-verification-review.md).

## Alternate Flows

- **Resubmit setelah `rejected` atau `needs_resubmission`**: Seller dapat submit ulang. Retry cooldown duration adalah parameter operasional yang sengaja tidak dikunci pada level doctrine. State transisi: `rejected` / `needs_resubmission` → `pending_review`.
- **Edit submission saat `pending_review`**: tidak diperbolehkan. Seller harus menunggu hasil review.
- **Re-submission setelah `under_investigation`**: tergantung outcome admin investigation. Lihat [Verification Review Governance](../doctrine/verification-review-governance.md) untuk state machine lengkap.

## Failure / Rejection Cases

- **Field wajib kosong** (nama, NIK, foto KTP, selfie) → ditolak.
- **NIK bukan 16 digit** → ditolak.
- **Sudah ada submission pending atau sudah verified** → submit baru ditolak.
- **Upload dokumen gagal** (storage error) → submit ditolak; data formulir tetap di klien sampai user mencoba lagi.
- **Akun ber-status `suspended`/`banned`** → submit ditolak.

## State Changes

- **Seller Verification status**: transisi mengikuti state machine canonical di [Verification Review Governance](../doctrine/verification-review-governance.md).
- **Field `reviewed_at`, `reviewed_by`, `rejection_reason`**: dibersihkan saat resubmission dari `rejected` / `needs_resubmission`.
- **Dokumen identitas**: tersimpan; akan dievaluasi admin.

## Financial Impact

Flow ini tidak memutate uang, tetapi **memengaruhi gate** payout (lihat [Seller Authority Separation](../doctrine/seller-authority-separation.md)). Liability invariant berlaku: saldo seller pre-verification (Layer 4) tetap visible dan tetap liability seller. Trust downgrade pasca-approve tidak memutate ledger; hanya menutup gate payout.

## Notifications

- **Saat submit (`pending_review`)**: UI menampilkan banner "Menunggu Review". Notifikasi formal MAY dikirim sesuai doctrine.
- **Saat `approved` / `rejected` / `needs_resubmission` / `suspended` / `revoked` / `under_investigation`**: UI banner berubah; notifikasi formal MUST mencakup reason saat outcome negatif (recourse path).

## Cross-Domain Relations

- **Become Seller**: prasyarat — tanpa Seller capability, flow verification tidak relevan.
- **Seller Verification Review**: tahap admin yang merespon submission ini. Lihat [Seller Verification Review](./seller-verification-review.md).
- **Withdraw / Payout**: gating utama dari verification `approved`. Saat verification berpindah ke Layer 6, payout authority dipotong.
- **Listing publik**: **tidak** di-gate oleh verification — gating-nya adalah subscription aktif.
- **Wallet / Escrow**: liability invariant berlaku. Lihat [Revocable Trust Model](../doctrine/revocable-trust-model.md).
- **Dispute**: active dispute lifecycle survive trust downgrade.
- **Governance / Moderation**: trust downgrade authority adalah admin / governance action.

## Business Rules

- **Dokumen identitas hanya KYC orang**: data NIK + foto KTP + selfie. Verifikasi badan usaha (NPWP, SIUP, NIB) **tidak dalam scope** KYC ini — keputusan owner permanen, dihapus dari enum database di migration 000205.
- **NIK uniqueness antar akun**: tidak ada pemeriksaan apakah satu NIK sudah dipakai untuk verifikasi akun lain.

> Aturan canonical revocable trust, governed review process, trust escalation safety, selling vs payout separation — semua hidup di doctrine docs yang dirujuk di header. Flow ini tidak menggantikan.

## Forbidden Behaviors

- Sistem tidak boleh mengizinkan submission baru saat status sedang `pending_review` atau `approved`. Submission baru hanya valid dari `not_submitted` / `rejected` / `needs_resubmission`.
- Sistem tidak boleh menyimpan NIK / dokumen identitas di lokasi yang dapat dibaca user lain. Dokumen KYC disimpan di private S3 bucket; admin membaca via presigned GET URL (TTL 5 menit) yang di-generate server-side dari storage_key — bukan URL permanen.
- Sistem tidak boleh menampilkan kepada Seller status `approved` tanpa konfirmasi backend — banner status harus mencerminkan state real-time, bukan optimisme klien.

> Forbidden behaviors doctrine-wide (withdraw tanpa Layer 5; trust state tanpa attribution; balance mutation pada downgrade; active obligation dipotong; seller deletion) hidup di [Seller Authority Separation](../doctrine/seller-authority-separation.md), [Revocable Trust Model](../doctrine/revocable-trust-model.md), dan [Trust Escalation Safety](../doctrine/trust-escalation-safety.md).

## Notes

- Mobile menampilkan banner status real-time dari sumber backend; jangan caching agresif klaim status verifikasi.
