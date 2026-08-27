# Cross-Domain Relations

Hubungan antar domain di Labuda. Tujuan dokumen ini: **mencegah dokumentasi per-feature yang terisolasi.**

> Aturan baca:
> - `→` artinya "memicu / mengarah ke".
> - `↔` artinya "saling event / referensi dua arah".
> - `[event]` artinya jalur asinkron via outbox.
> - Anotasi dalam kurung adalah aktor / kondisi.

---

## 1. Buying Path (Direct Buy)

```
Listing
 → Pricing Preview (token + ongkir + diskon + coins opt-in)
 → Checkout
 → Order (created)
 → Payment (gateway init)
 → Payment Webhook
 → Wallet (Escrow HELD)
 → Order (PAID)
```

Pasca-payment:

```
Order (PAID)
 → Seller (Mark Shipped)
 → Order (SHIPPED)
 → Buyer (Confirm Delivery)
 → Order (DELIVERED)
 [auto-complete worker, post-window]
 → Order (COMPLETED)
 → Wallet (Escrow RELEASED → seller available)
 → Coins (earn untuk buyer)
 → Rating window terbuka
```

---

## 2. Buying Path (via Negotiation)

```
Buyer (Listing detail / Chat)
 → Negotiation (Create)
 ↔ Chat [event: negotiation.message_sent]
 ↔ Seller (Counter)
 → Buyer (Accept)
 → Checkout (with negotiationId, override price)
 → Order → Payment → Wallet (sama dengan Direct Buy)
```

---

## 3. Buying Path (Auction Win Claim)

```
Auction (lifecycle worker: started → live → ended)
 → Pemenang (highest bidder)
 → Auction Detail "Amankan Kemenangan" (winner-only UI)
 → Pricing Token for Claim
 → Checkout (with auctionId)
 → Order → Payment → Wallet (sama dengan Direct Buy)
```

Cabang sebelum berakhir:

```
Buyer
 → Auction (Place Bid) → Bid recorded → Notification (outbid kepada bidder lain)
Buyer (alternative)
 → Auction (Buy Now) → langsung ke Checkout → Order (skip lelang)
```

---

## 4. Chat ↔ Commerce

Chat **tidak mutate** ledger / order. Hubungan berupa link & event:

```
Order (created)
 → Chat (LinkOrderToChat) → Chat menampilkan banner status order

Chat
 → Negotiation (open) [event ke chat] → Negotiation room

Listing / Auction
 ↔ Chat Attachment (preview cards) — preview adalah snapshot, status live di-fetch ulang dari domain sumber
```

---

## 5. Refund & Cancellation Path

```
Buyer (Cancel Order, sebelum shipment)
 → Order (CANCELLED)
 → Refund (InitiateGatewayRefund, dengan idempotency key)
 → Gateway (Midtrans)
 → Refund Webhook (status update)
 → Wallet (ledger reversal)
 → Coins Refund Worker (jika buyer pakai coins)
```

> **Aturan keras:** refund tidak boleh memutasi balance buyer-side wallet langsung. Otoritas tetap di gateway.

---

## 6. Dispute Path

```
Buyer (pre-delivery) atau Seller (post-delivery)
 → Dispute (Open) → Order ditandai dispute
 → Wallet (Escrow Freeze atau DisputeFreeze post-release)
 → Notification → Support Admin
 → Admin (Resolve Dispute)
 ├─ Refund (buyer wins) → Refund Path (lihat #5)
 ├─ Release (seller wins) → Wallet Escrow RELEASE → seller available
 └─ Partial Split → kombinasi keduanya via WalletService
```

Cabang timeout:

```
Dispute (under_review, > timeout)
 → Dispute Timeout Worker
 → Mark overdue → Eskalasi ke admin (tidak auto-resolve)
```

---

## 7. Payout Path (Seller)

> Canonical: payout authority adalah **layer terpisah** dari selling authority. Lihat [Seller Authority Separation](./doctrine/seller-authority-separation.md).

```
Order (COMPLETED) → Wallet (seller available bertambah — liability seller)

Seller (Layer 5) → Request Withdrawal
 → Wallet (available → pending_withdrawal, ledger entry)
 → Finance Admin (review / approve)
 → Payout Worker → Bank API
 → Payout completed atau failed
 → Wallet (rollback jika failed)
```

Gating:

```
Seller di Layer 4 → Withdraw blocked (verification required)
Seller di Layer 6 → Withdraw blocked (trust downgraded)
Seller di Layer 5 dengan active DisputeFreeze → Withdraw amount dikurangi
```

> Liability invariant + revocable trust rules hidup di [Seller Authority Separation](./doctrine/seller-authority-separation.md) dan [Revocable Trust Model](./doctrine/revocable-trust-model.md).

---

## 8. Coins Loop

```
Order (COMPLETED) → Coins (earn, daily cap, idempotent insert)

Buyer (Checkout, opt-in)
 → Coins (AtomicDeductBalance) → Pricing (apply discount, jangan turunkan di bawah commission)

Order (Cancel/Refund) → Coins Refund (insert refund earn, idempotent)
```

> Coins ≠ uang. Tidak menyentuh ledger / wallet.

---

## 9. Reporting & Moderation Path

```
User → Submit Report
 → Moderation Case (pending)
 → Moderator (Decide)
 ├─ Approve → no action
 ├─ Remove → Content removed → Notification (creator)
 └─ Issue Warning → User violation record
 → (severe) Suspend / Ban → Notification policy filter aktif

Creator (post-removal)
 → Submit Appeal
 → Admin Review Appeal
 ├─ Approve → Reverse moderation case → Content restored
 └─ Reject → Decision upheld
```

---

## 10. Verification (KYC) Path

> Canonical: verification adalah **revocable financial trust decision**, bukan permanent moral approval. Lifecycle states + state machine hidup di [Verification Review Governance](./doctrine/verification-review-governance.md).

```
Seller (calon) → Submit Verification
 → Verification (pending_review)
 → Admin Review
 ├─ Approve → Seller approved (Layer 5)
 │ → Payout authority gate dibuka
 │ → ┌─(admin: suspend)──────▶ suspended
 │ ├─(admin: under_invest.)─▶ under_investigation
 │ └─(admin: revoke)────────▶ revoked
 ├─ Reject → Seller rejected → dapat resubmit
 └─ Needs resubmission → Seller dapat resubmit
```

Aturan canonical lintas-domain (ringkas — sumber kebenaran ada di doctrine):

- **Seller existence + balance + active obligations survive trust downgrade.** Lihat [Revocable Trust Model](./doctrine/revocable-trust-model.md).
- **Production trust escalation wajib attributable.** Hidden production auto-approve forbidden. Lihat [Trust Escalation Safety](./doctrine/trust-escalation-safety.md).
- **Governed review process.** Lifecycle visible, status semantics explicit, retry path exists. Lihat [Verification Review Governance](./doctrine/verification-review-governance.md).

---

## 11. Support Path

```
User → Create Support Ticket → Support Chat Room
 ↔ Support Admin (reply)
 → (jika eskalasi) → Dispute (eskalasi diputuskan Support Admin)
```

---

## 12. Subscription / Promotion Path

```
Seller → Subscription (tier free / paid)
 → Subscription Payment Service → Payment → Wallet

Seller → Promotion Package Selection
 → Subscription Payment Service (jalur pembayaran sama) → Payment
```

> Subscription dan Promotion Package adalah dua produk berbeda; jalur pembayaran-nya sama. Subscription mengontrol selling authority; Promotion adalah growth capability terpisah.

---

## 13. Notification Distribution

Domain emitter → outbox event → Notification consumer → DB record + (optional) push.

```
Order events ─┐
Chat events │
Dispute events │
Social events │ ─→ Outbox ─→ Notification Service
Auction events │ (Account status filter,
Moderation events │ Category policy filter)
Payment events ─┘ ├─→ DB record (selalu)
 └─→ Push (jika type & status memenuhi)
```

---

## 14. Realtime / WebSocket Layer

```
Domain emit event → Realtime Hub → WebSocket Channel → Mobile UI live update
```

Kanal canonical: chat room, auction room, notification stream.

> Realtime adalah signal-only. Truth tetap di domain DB / REST. Lihat ADR-005.

---

## 15. Read-Only Aggregations (Discovery / Bidding View)

```
Auction events → Bidding View (interaction/bidding) — read-only aggregation per user
Content events → Feed (social/feed)
Listings → Search Index (discovery)
```

Bukan jalur write — bukan owner data.

---

## 16. Identity Lifecycle (cross-domain)

Identity lifecycle wajib dibaca sebelum domain manapun (Social, Commerce, Finance, Governance) menentukan gating action. Empat layer canonical:

```
Guest → Authenticated (Layer A) → Identity Complete (Layer B) → Email Verified (Layer C) → Seller / Financial Trust (Layer D)
```

Layer A/B/C/D, account stages, dan path convergence rule (email/password vs Google) hidup di [Layered Identity & Trust Model](./doctrine/layered-identity-trust-model.md). ALLOWED / BLOCKED list per layer hidup di [Email Gating Matrix](./doctrine/email-gating-matrix.md).

### Mapping ke domain konsumen

| Domain | Layer minimal | Catatan |
|--------|---------------|---------|
| Social — Content / Engagement / Graph | Layer C untuk write; Layer B untuk visibility default | Daftar lengkap di [Email Gating Matrix](./doctrine/email-gating-matrix.md). |
| Chat / Negotiation | Layer C | Akun pre-Identity-Complete tertahan Complete Profile gate. |
| Commerce — Checkout / Bid / Order | Layer C | Layer D tidak diperlukan untuk Buyer-side. |
| Commerce — Listing publish / Become Seller | Layer C + Layer 4 (subscription); Layer 5 untuk withdraw | Lihat [Become Seller](./foundation/become-seller.md). |
| Finance — Withdraw / Payout | Layer 5 | Lihat [Seller Authority Separation](./doctrine/seller-authority-separation.md). |
| Search / Mention | Layer B untuk visibility default | Pre-Complete-Profile user tidak terlihat di search / mention. |
| Moderation / Governance | Lintas semua layer | Tooling wajib melihat semua layer + handle history. |

---

## 17. Username Immutability (cross-domain)

Username **immutable** setelah ditetapkan — tidak ada rename event di domain Identity / Profile maupun domain lain. Canonical truth: username adalah identitas kanonik pengguna, dipilih saat registrasi, dan tidak pernah berubah. Lihat [Username Lifecycle](./doctrine/username-lifecycle.md).

Karena username tidak pernah berubah, referensi `@handle` di seluruh domain tetap stabil:

```
Identity / Profile (establishment once)
 ↓
 ├─→ Social — Mention      (referensi @handle stabil; tidak ada redirect semantics)
 ├─→ Social — Trust Graph  (follow / like menempel pada akun)
 ├─→ Social — Social Graph (friend / follower list TETAP utuh)
 ├─→ Search                (@handle lookups stabil; tidak ada fallback semantics)
 ├─→ Moderation            (handle history stabil by construction)
 ├─→ Notification          (tidak ada rename event)
 └─→ Seller — Reputation   (rating / review / aggregations menempel pada akun)
```

> Backend menegakkan immutability: `PATCH /users/me/profile` hanya menetapkan username saat belum ada, dan menolak perubahan dengan `409 USERNAME_ALREADY_SET`. Tidak ada rename endpoint di seluruh codebase.

---

## 18. Seller / Financial Trust — Layered Authority Matrix

> Canonical: **selling authority ≠ payout authority.** Subscription mengontrol selling authority; verification mengontrol payout authority.

Tujuh layer (0–6), capability matrix, liability invariant, revocable trust transitions, dan trust escalation safety hidup di:

- [Layered Identity & Trust Model](./doctrine/layered-identity-trust-model.md) — Layer A/B/C/D
- [Capability Matrix](./doctrine/capability-matrix.md) — per-capability lookup termasuk Layer 4/5/6
- [Seller Authority Separation](./doctrine/seller-authority-separation.md) — selling vs payout sub-gate
- [Revocable Trust Model](./doctrine/revocable-trust-model.md) — survive / cut lists pada trust downgrade
- [Trust Escalation Safety](./doctrine/trust-escalation-safety.md) — accountability rules

### Mapping ke domain konsumen (ringkas)

| Domain | Layer-relevant |
|--------|----------------|
| Catalog — Listing publish | Layer 4 atau Layer 5; Layer 6: publish baru MAY restricted |
| Catalog — Auction create | Layer 4 atau Layer 5; Layer 6: create baru MAY restricted |
| Order — receive (seller side) | Layer 4 atau Layer 5; active order lifecycle survive Layer 6 |
| Wallet / Escrow | Saldo seller liability — survives semua layer; tidak dimutate oleh trust downgrade |
| Payout / Withdraw | Layer 5 only; blocked di Layer 4 dan Layer 6 |
| Dispute | Active dispute lifecycle survive Layer 6 |
| Promotion / Growth | Layer 5 ideal; Layer 6 MAY restricted |
| Support | Semua layer (termasuk Layer 6) |
| Moderation / Governance | Lintas semua layer; audit trail wajib |

> Severity-spesifik (Layer 6 listing freeze, dispute escalation matrix, GMV/saldo threshold) sengaja **tidak** dikunci canonical pada level operasional — lihat doctrine docs untuk daftar lengkap deferred parameters.

---

## Diagram Big Picture (text version)

```
 ┌──────────────┐ ┌──────────────┐
 │ Identity │───▶│ Profile / │
 │ & Auth │ │ Preference │
 └──────┬───────┘ └──────┬───────┘
 │ │
 ▼ ▼
 ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
 │ Address │ │ Social │◀──▶│ Discovery │
 │ Book │ │ (Content/ │ │ (Search/Feed)│
 └──────┬───────┘ │ Engage/ │ └──────────────┘
 │ │ Graph) │
 │ └──────┬───────┘
 │ │
 │ ▼
 │ ┌──────────────┐
 │ │ Chat │◀────────┐
 │ └──────┬───────┘ │
 │ │ events │
 │ ▼ │
 │ ┌──────────────┐ │
 │ │ Negotiation │ │
 │ └──────┬───────┘ │
 │ │ │
 ▼ ▼ │
 ┌──────────────────────────────────┐ │
 │ Catalog (Listing + Auction) │─────────┤
 └──────────┬───────────────────────┘ │
 ▼ │
 ┌──────────────────────────────────┐ │
 │ Pricing │◀── Coins│
 │ (Discount/Promotion/Token) │ │
 └──────────┬───────────────────────┘ │
 ▼ │
 ┌──────────────────────────────────┐ │
 │ Checkout │ │
 └──────────┬───────────────────────┘ │
 ▼ │
 ┌──────────────────────────────────┐ │
 │ Order │◀────────┘ (Chat link)
 └──────────┬───────────────────────┘
 │
 ┌──────────────┼──────────────┬──────────────┐
 ▼ ▼ ▼ ▼
 ┌──────────┐ ┌──────────┐ ┌──────────────┐ ┌──────────┐
 │ Shipping │ │ Payment │ │ Wallet/ │ │ Dispute │
 └──────────┘ └────┬─────┘ │ Escrow │ └────┬─────┘
 │ └──────┬───────┘ │
 ▼ │ │
 ┌─────────┐ │ │
 │ Refund │◀─────────┤ │
 └────┬────┘ ▼ │
 │ ┌──────────┐ │
 └────────▶│ Payout │◀─ Verif. │
 └──────────┘ │
 ▼
 ┌────────────────┐
 │ Governance │
 │ (Moderation/ │
 │ Verif./ │
 │ Audit/ │
 │ Support) │
 └────────┬───────┘
 │
 ▼
 ┌────────────────┐
 │ Notification │
 └────────────────┘
```

> Diagram ini adalah **peta orientasi**, bukan diagram arsitektur. Tidak menggambarkan setiap edge yang sebenarnya ada di code.

---

## Pola Hubungan Berulang

1. **Outbox-driven notification** — hampir semua write-domain (Order, Dispute, Chat, Moderation) emit event ke outbox; Notification Service konsumsi.
2. **Wallet sebagai single mutator uang** — Payment, Refund, Order Complete, Dispute Resolution, Payout: semua mendelegasikan mutasi ke `WalletService`.
3. **Pricing token sebagai otoritas harga** — Checkout dan Auction Claim selalu menerbitkan token berumur pendek; client tidak boleh mengirim harga sendiri.
4. **Snapshot vs Reference** — alamat dan harga disimpan sebagai snapshot di order, bukan reference.
5. **Idempotency key** — chat message, payment init, refund init: semua memiliki idempotency key.

---

## Pola Anti-Pattern yang Harus Diawasi

- Mobile **tidak boleh** memutasi balance / escrow langsung. Hanya kirim niat (intent) ke backend.
- Chat **tidak boleh** memutate order / wallet. Hanya link & event.
- Coins **tidak boleh** dipakai untuk merepresentasikan uang (mis. refund cash sebagai coins).
- Promotion **tidak boleh** menabrak Subscription tanpa pembedaan jelas.
