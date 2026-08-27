# Seller Verification Review

> **Status:** STABLE
> **Domain:** Verification (sisi admin)

> **Doctrine references:**
> - [Verification Review Governance](../doctrine/verification-review-governance.md) — canonical state machine + mandatory review properties.
> - [Revocable Trust Model](../doctrine/revocable-trust-model.md) — downgrade transitions + survive/cut lists.
> - [Trust Escalation Safety](../doctrine/trust-escalation-safety.md) — accountability rules untuk setiap transisi.
> - [Seller Authority Separation](../doctrine/seller-authority-separation.md) — approve membuka payout sub-gate, bukan selling.

## Purpose

Tahap di mana **Admin** menilai dokumen KYC yang dikirim Seller pada [Submit Seller Verification](./submit-seller-verification.md), lalu memutuskan transisi state canonical (approve / reject / needs_resubmission), dan — pasca-approve — MAY menjalankan trust downgrade (suspend / revoke / under_investigation).

Hasil keputusan menentukan apakah Seller dapat melakukan **withdraw** (payout authority — Layer 5). Trust downgrade pasca-approve (Layer 5 → Layer 6) tidak menghapus seller existence, balance, atau active obligations.

## Actors

- **Admin** — reviewer (sub-peran spesifik admin yang berhak menjalankan review vs trust downgrade vs investigation di-defer ke Governance RBAC).
- **Seller** — pemilik submission yang menunggu hasil; juga subject dari trust downgrade pasca-approve.
- **System** — menyimpan keputusan, mengubah state verifikasi, dan mencatat audit trail (mandatory).

## Preconditions

- **Untuk review submission baru:** ada submission Seller dengan status `pending_review`.
- **Untuk trust downgrade pasca-approve:** Seller berstatus `approved`.
- **Untuk re-evaluation pasca-investigasi:** Seller berstatus `under_investigation`.
- Admin memiliki akses ke admin panel verification queue dengan sub-peran sesuai authority.

## Main Flow (review submission baru)

1. Admin membuka antrean verifikasi (admin panel).
2. Admin memilih submission `pending_review`.
3. Admin meninjau dokumen (NIK, foto KTP, selfie) dan data nama lengkap.
4. Admin memutuskan transisi:
 - **Approve** → `pending_review` → `approved`. Payout authority (Layer 5) terbuka. Tidak terminal selamanya.
 - **Reject** → admin **wajib** memberikan **rejection reason**; `pending_review` → `rejected`.
 - **Needs resubmission** → `pending_review` → `needs_resubmission` dengan reason yang menjelaskan dokumen tambahan / perbaikan.
5. Sistem mencatat audit (mandatory): operator id, timestamp, reason (jika negatif).
6. UI Seller akan mencerminkan hasil; banner status berubah.

## Trust Downgrade Flow (post-approve)

Pasca-approve, admin MAY menjalankan transisi trust downgrade bila ditemukan fraud / dokumen palsu / pelanggaran berat / red flag investigasi:

1. Admin memilih Seller berstatus `approved` (atau Seller di state lain yang masih dalam scope authority).
2. Admin memilih jalur trust downgrade:
 - **Suspend** → `approved` → `suspended`. Reversible.
 - **Mark under_investigation** → `approved` → `under_investigation`. Payout di-hold.
 - **Revoke** → `approved` → `revoked`. Trust authority dicabut sepenuhnya; seller existence tetap.
3. Admin **wajib** memberikan reason.
4. Sistem mencatat audit: operator id, timestamp, alasan, transisi state.
5. UI Seller mencerminkan status baru + reason; canonical wording memberi recourse path.

> Survive/cut lists, liability invariant, dan obligation handling rules hidup di [Revocable Trust Model](../doctrine/revocable-trust-model.md). State machine canonical lengkap hidup di [Verification Review Governance](../doctrine/verification-review-governance.md).

## Alternate Flows

- **Request more info via `needs_resubmission`**: admin meminta perbaikan tanpa full reject.
- **Hapus dokumen** (sebelum keputusan): tidak diperbolehkan; dokumen yang sudah masuk antrean menunggu keputusan.
- **Re-evaluation pasca `under_investigation`**: outcome MAY berupa kembali ke `approved`, `needs_resubmission`, `suspended`, atau `revoked` sesuai temuan investigasi.

## Failure / Rejection Cases

- **Admin men-submit reject / needs_resubmission / suspend / revoke / under_investigation tanpa reason** → ditolak; reason wajib.
- **Admin mencoba men-trigger transisi yang invalid pada current state** → ditolak; setiap transisi wajib mengikuti state machine — mis. `rejected` → `approved` langsung tidak ada (Seller harus resubmit ke `pending_review` dulu).
- **Admin tanpa sub-peran sesuai authority** mencoba menjalankan trust downgrade → ditolak.

## State Changes

- **Seller Verification status**: transisi mengikuti state machine canonical di [Verification Review Governance](../doctrine/verification-review-governance.md).
- **Field `reviewed_at`**: terisi.
- **Field `reviewed_by` / operator id**: terisi dengan id admin (audit mandatory).
- **Field `rejection_reason` / `downgrade_reason`**: terisi (jika reject / needs_resubmission / suspend / revoke / under_investigation).

## Financial Impact

Flow ini tidak memutate uang, tetapi **memengaruhi gate** payout (lihat [Seller Authority Separation](../doctrine/seller-authority-separation.md)):

- **Source of truth status verifikasi**: backend Labuda (record `seller_verifications`).
- **Approval membuka payout authority** (Layer 5 — request withdraw / payout / financial settlement).
- **Tidak ada release saldo** terjadi otomatis akibat approval — saldo Seller mengikuti aliran transaksi normal di domain Wallet / Escrow. Approval hanya membuka **akses** ke withdraw.
- **Trust downgrade pasca-approve**: payout authority dipotong; saldo seller tidak dimutate. Memori `Money authority model` dipertahankan.
- **Active obligations survive trust downgrade**: active order lifecycle continues; active dispute lifecycle continues.
- **Coins**: tidak terlibat.

## Notifications

- **Saat approve / reject / needs_resubmission / suspend / revoke / under_investigation**: notifikasi formal MUST dikirim; outcome negatif MUST mencakup reason (recourse path). UI Seller banner berubah saat data dimuat ulang.

## Cross-Domain Relations

- **Submit Seller Verification**: flow yang men-supply submission.
- **Withdraw / Payout**: gate payout authority yang dibuka oleh approval. Saat trust downgrade, payout authority dipotong.
- **Wallet / Escrow**: liability invariant — saldo seller tidak dimutate saat trust downgrade.
- **Dispute**: active dispute lifecycle survive trust downgrade. DisputeFreeze adalah tool finansial paralel.
- **Audit (Governance)**: keputusan approve / reject / needs_resubmission / suspend / revoke / under_investigation **wajib** masuk audit log dengan operator id + reason + timestamp.
- **DevOps / Environment**: production trust escalation wajib attributable; sandbox / staging auto-approve flag tidak boleh leak ke production governance — lihat [Trust Escalation Safety](../doctrine/trust-escalation-safety.md).

## Business Rules

- **Reject / needs_resubmission / trust downgrade wajib menyertakan reason**. Reason ditampilkan kepada Seller (recourse path) dan masuk audit log.
- **Audit fields** wajib terisi (`reviewed_at`, `reviewed_by` / operator id, reason) untuk setiap transisi keputusan — termasuk trust downgrade pasca-approve.

> Aturan canonical revocable trust, governed review, trust escalation safety, selling vs payout separation — semua hidup di doctrine docs yang dirujuk di header.

## Forbidden Behaviors

- Admin tidak boleh men-submit reject / needs_resubmission / suspend / revoke / under_investigation tanpa reason.
- Admin tidak boleh meng-approve tanpa meninjau dokumen (prinsip operasi; tidak ditegakkan teknis tetapi merupakan aturan governance).
- Sistem tidak boleh menerima keputusan tanpa mencatat operator id, timestamp, dan reason di audit log.
- Sistem tidak boleh meng-bypass pemeriksaan state — setiap transisi wajib valid sesuai state machine canonical.

> Forbidden behaviors doctrine-wide (hidden production auto-approve, withdraw tanpa Layer 5, balance mutation pada downgrade, active obligation dipotong, seller deletion pada downgrade) hidup di [Trust Escalation Safety](../doctrine/trust-escalation-safety.md), [Seller Authority Separation](../doctrine/seller-authority-separation.md), dan [Revocable Trust Model](../doctrine/revocable-trust-model.md).

## Notes

- Detail user-facing untuk **Seller** mengikuti record yang sama; flow review ini hanya menulis dari sudut pandang admin.
- Tindakan admin yang lebih luas (issue warning, ban) hidup di domain Governance — di luar scope flow ini.
