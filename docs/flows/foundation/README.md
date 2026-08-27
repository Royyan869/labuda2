# Foundation Flows

Folder ini mendokumentasikan **flow foundation** Labuda — segala sesuatu yang harus benar **sebelum** seorang user bisa mengambil action di domain commerce, social, atau finance.

## Authority Layout

Folder ini punya dua lapis:

1. **Flow docs** (file di folder ini, mis. `sign-up.md`) — operational journey docs. Owner-readable; menjelaskan siapa, kapan, apa, kenapa, dan major rules per flow.
2. **Doctrine docs** ([`./doctrine/`](../doctrine/)) — canonical truth: invariant, authority semantics, layered trust, capability matrix. **Sumber kebenaran resmi.** Flow docs merujuk ke doctrine; tidak menggantikan.

Bila flow doc dan doctrine doc berkonflik, **doctrine menang**. Bila dokumentasi dan runtime berkonflik, **doctrine masih menang** — runtime adalah convergence target.

## Cakupan

| # | Flow | File | Domain |
|---|------|------|--------|
| 1 | Sign Up | [`sign-up.md`](./sign-up.md) | Identity & Auth |
| 2 | Sign In (Email/Password) | [`sign-in-email.md`](./sign-in-email.md) | Identity & Auth |
| 3 | Sign In (Google OAuth) | [`sign-in-google.md`](./sign-in-google.md) | Identity & Auth |
| 4 | Email Verification | [`email-verification.md`](./email-verification.md) | Identity & Auth |
| 5 | Complete Profile (Identity Completion gate) | [`complete-profile.md`](./complete-profile.md) | Profile |
| 6 | Manage Profile | [`manage-profile.md`](./manage-profile.md) | Profile |
| 7 | Manage Preferences | [`manage-preferences.md`](./manage-preferences.md) | Preference |
| 8 | Manage Address Book | [`manage-address-book.md`](./manage-address-book.md) | Address |
| 9 | Become Seller | [`become-seller.md`](./become-seller.md) | Seller Capability |
| 10 | Submit Seller Verification | [`submit-seller-verification.md`](./submit-seller-verification.md) | Verification |
| 11 | Seller Verification Review | [`seller-verification-review.md`](./seller-verification-review.md) | Verification |

## Doctrine Index

Doctrine docs hidup di [`./doctrine/`](../doctrine/).

| Doctrine | Cakupan |
|----------|---------|
| [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md) | Empat layer A/B/C/D + account stages + path convergence rule. |
| [Email Gating Matrix](../doctrine/email-gating-matrix.md) | Daftar canonical ALLOWED / BLOCKED + enforcement pattern. |
| [Username Lifecycle](../doctrine/username-lifecycle.md) | Username immutable setelah establishment + trust continuity invariants. |
| [Seller Authority Separation](../doctrine/seller-authority-separation.md) | Selling sub-gate ≠ payout sub-gate + pre-verification seller model. |
| [Revocable Trust Model](../doctrine/revocable-trust-model.md) | `approved` bukan terminal + survive/cut lists pada trust downgrade. |
| [Verification Review Governance](../doctrine/verification-review-governance.md) | Mandatory review properties + canonical 8-state lifecycle. |
| [Trust Escalation Safety](../doctrine/trust-escalation-safety.md) | Hidden production auto-approve forbidden + triple guard rules. |
| [Capability Matrix](../doctrine/capability-matrix.md) | Per-capability authority lookup (browse, comment, withdraw, dst.). |

## Outcome Bisnis

Foundation flows menetapkan:

1. **Lifecycle akun** — Guest → Authenticated → Identity Complete → Email Verified → Subscribed Seller → Verified Seller → (revocable) Suspended/Revoked Trust. Lihat [Capability Matrix](../doctrine/capability-matrix.md).
2. **Onboarding canonical** — jalur sign-up email/password vs Google. Kedua jalur konvergen pada outcome canonical: tidak ada full participation tanpa username (lihat [Layered Identity & Trust Model](../doctrine/layered-identity-trust-model.md)).
3. **Gating canonical**:
   - **Email verification** — prasyarat onboarding seller, pembayaran subscription, interaction-sensitive, dan transaction-sensitive actions. Daftar canonical: [Email Gating Matrix](../doctrine/email-gating-matrix.md).
   - **Seller verification (KYC)** — prasyarat **withdrawal** (payout authority).
   - **Subscription seller aktif** — prasyarat **listing publik** (selling authority).
   - Kedua gating seller terpisah — lihat [Seller Authority Separation](../doctrine/seller-authority-separation.md).
4. **Address book canonical** — address sebagai snapshot di order, dengan dua purpose berbeda (shipping vs sender).

## Dependency Antar-Domain

- **Social**: bergantung pada akun aktif & profil terisi.
- **Listing/Auction**: bergantung pada Seller Capability + Subscription aktif (selling sub-gate).
- **Order**: bergantung pada Address Book (snapshot) dan akun Buyer aktif.
- **Payout**: bergantung pada Seller Verification approved (payout sub-gate).

## Cara Membaca Folder Ini

- Section **Forbidden Behaviors** dan **Business Rules** di setiap flow adalah inti operasional — itulah yang tidak boleh berubah saat code berubah. Forbidden behaviors yang berlaku doctrine-wide hidup di doctrine docs.
- Semua referensi ke domain lain (Order, Wallet, Refund) **tidak dijelaskan di sini**; hanya di-link.
- Bila menemukan flow yang belum terdokumentasi (logout-all-devices, delete account, 2FA, dst.) — itu memang scope kerjaan berikutnya, bukan oversight.
