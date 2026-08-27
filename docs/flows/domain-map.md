# Domain Map

Peta domain bisnis Labuda. Domain di sini adalah **business domain** (bukan modul kode). Beberapa business domain dipecah menjadi beberapa folder kode di mobile/backend — itu wajar.

> **Konvensi atribut:**
> - `financial-critical` — domain ini menyentuh saldo / payment / refund / escrow.
> - `async-heavy` — domain ini bergantung pada worker / outbox / webhook / event.
> - `admin-sensitive` — domain ini punya jalur admin yang berdampak luas (override, freeze, decision).

---

## 1. Identity & Auth

- **Tujuan:** mengelola siapa user, status verifikasi, dan akses akun.
- **Actor utama:** Guest, Registered user.
- **Domain dependency:** Notification (email verifikasi).
- Financial-critical: tidak. Async-heavy: sedikit (email send via outbox). Admin-sensitive: tidak (admin sentuh user lewat domain Governance).
- **Canonical truth:** Identity domain wajib memisahkan empat layer trust secara eksplisit (Authentication / Identity Completion / Email Verification / Seller-Financial Trust). Lihat [Layered Identity & Trust Model](./doctrine/layered-identity-trust-model.md).

## 2. Profile & Preference

- **Tujuan:** menyimpan profil user dan preferensi (notif, privacy).
- **Actor utama:** Registered user.
- **Domain dependency:** Identity, Notification (preferensi block).
- Financial-critical: tidak. Async-heavy: tidak. Admin-sensitive: tidak.
- **Canonical truth:** username adalah identitas sosial yang **mutable but governed**. Lihat [Username Lifecycle](./doctrine/username-lifecycle.md). `profile_completed=true` = minimum public identity established (Layer B), bukan full bio / avatar / email-verified / seller-verified.

## 3. Address Book

- **Tujuan:** alamat user untuk shipping & sender.
- **Actor utama:** Buyer (alamat penerima), Seller (alamat asal).
- **Domain dependency:** Order/Checkout (snapshot alamat masuk ke order).
- Financial-critical: tidak. Async-heavy: tidak. Admin-sensitive: tidak.

## 4. Seller Capability (Onboarding & Verification)

- **Tujuan:** memberi/menarik kapabilitas menjual.
- **Actor utama:** Registered user → Seller; Admin (review).
- **Domain dependency:** Identity, Governance (verification), Finance (gating withdrawal).
- Financial-critical: tidak langsung (gating untuk payout). Async-heavy: tidak. Admin-sensitive: ya.
- **Canonical truth:** selling authority (subscription) ≠ payout authority (verification). Pre-verification seller MAY sell + receive order; tidak boleh extract money. Trust restriction reduce authority, bukan erase history. Lihat [Seller Authority Separation](./doctrine/seller-authority-separation.md) dan [Revocable Trust Model](./doctrine/revocable-trust-model.md).

## 5. Social — Content

- **Tujuan:** publikasi & konsumsi konten.
- **Actor utama:** Registered user.
- **Domain dependency:** Feed, Notification, Moderation/Report, Comment, Like, Share.
- Financial-critical: tidak. Async-heavy: sedang (feed fan-out, notification). Admin-sensitive: ya (moderasi).

## 6. Social — Graph (Follow / Block / Mute)

- **Tujuan:** relasi antar user.
- **Actor utama:** Registered user.
- **Domain dependency:** Feed, Notification, Chat (block dapat memblokir chat).
- Financial-critical: tidak. Async-heavy: sedang. Admin-sensitive: tidak.

## 7. Social — Engagement (Like, Comment, Share, Rating)

- **Tujuan:** interaksi pada konten / seller / order.
- **Actor utama:** Registered user, Buyer (rating).
- **Domain dependency:** Content, Order (untuk rating), Notification.
- Financial-critical: tidak. Async-heavy: sedang. Admin-sensitive: sebagian (komentar dapat kena moderasi).

## 8. Discovery (Search & Recommendation)

- **Tujuan:** membantu user menemukan content/listing/user.
- **Actor utama:** Any user.
- **Domain dependency:** Content, Listing, Graph.
- Financial-critical: tidak. Async-heavy: ya (indexing, projection). Admin-sensitive: tidak.

## 9. Chat

- **Tujuan:** komunikasi 1:1 antar user dan dengan support; jembatan ke commerce.
- **Actor utama:** Registered user, Buyer, Seller, Support Admin.
- **Domain dependency:** Negotiation (event-driven), Order (link), Listing/Auction (attachment), Notification.
- Financial-critical: tidak (boundary tegas: tidak boleh mutate ledger). Async-heavy: ya. Admin-sensitive: sebagian (support reuse infrastruktur chat).

## 10. Catalog — Listing

- **Tujuan:** mengelola item yang dijual.
- **Actor utama:** Seller (CRUD), Buyer (browse).
- **Domain dependency:** Pricing (preview), Shipping (config), Chat (origin), Order (downstream).
- Financial-critical: tidak langsung. Async-heavy: sebagian (search indexing). Admin-sensitive: sebagian (takedown).

## 11. Catalog — Auction

- **Tujuan:** mekanisme lelang.
- **Actor utama:** Seller (host), Buyer (bidder).
- **Domain dependency:** Pricing (token), Notification (outbid), Order (claim → order), Worker (lifecycle).
- Financial-critical: sebagian (pricing token + claim → payment). Async-heavy: ya (start/end/settlement workers). Admin-sensitive: sebagian (cancel/admin override).

## 12. Negotiation

- **Tujuan:** tawar-menawar harga sebelum order.
- **Actor utama:** Buyer (open), Seller (counter), Buyer (accept).
- **Domain dependency:** Chat (carrier), Pricing (final price), Order (downstream).
- Financial-critical: tidak langsung (mengubah harga checkout). Async-heavy: ya (expire worker). Admin-sensitive: tidak.

## 13. Pricing — Discount / Promotion / Token

- **Tujuan:** modulasi harga (kode diskon, promosi listing) dan pricing token (otoritas harga checkout).
- **Actor utama:** Seller (membuat diskon/promosi), Buyer (memakai), System (mengeluarkan token).
- **Domain dependency:** Listing, Order, Subscription, Coins.
- Financial-critical: sebagian (token = otoritas harga). Async-heavy: sebagian (token expiry). Admin-sensitive: tidak.

## 14. Subscription (Seller)

- **Tujuan:** tier seller / boost listing berbayar.
- **Actor utama:** Seller.
- **Domain dependency:** Payment, Promotion.
- Financial-critical: ya. Async-heavy: sebagian (renewal). Admin-sensitive: sebagian.

## 15. Transaction — Checkout

- **Tujuan:** menerjemahkan niat beli menjadi order + payment.
- **Actor utama:** Buyer.
- **Domain dependency:** Listing, Auction, Negotiation (input harga), Pricing (preview & token), Shipping (quote), Coins (opsional), Payment.
- Financial-critical: ya. Async-heavy: sebagian (payment webhook). Admin-sensitive: tidak.

## 16. Transaction — Order

- **Tujuan:** lifecycle order pasca-checkout.
- **Actor utama:** Buyer, Seller, System (worker), Admin (override).
- **Domain dependency:** Payment, Wallet (escrow), Shipping, Rating, Dispute, Refund, Coins.
- Financial-critical: ya. Async-heavy: ya (auto-complete worker, expire worker). Admin-sensitive: ya (mark delivered, dispute resolution).

## 17. Transaction — Shipping

- **Tujuan:** pilihan & quote pengiriman, proof.
- **Actor utama:** Buyer (pilih), Seller (config & ship), System (quote & reactivation).
- **Domain dependency:** Address, Order, Pricing.
- Financial-critical: tidak langsung (mempengaruhi total). Async-heavy: sebagian (quote reactivation). Admin-sensitive: tidak.

## 18. Finance — Payment

- **Tujuan:** memproses pembayaran via gateway.
- **Actor utama:** Buyer, System (webhook).
- **Domain dependency:** Order, Wallet (escrow hold), Refund.
- Financial-critical: ya (sangat). Async-heavy: ya (webhook, retries, reconciliation). Admin-sensitive: sebagian.

## 19. Finance — Wallet & Escrow

- **Tujuan:** **single source of truth** untuk uang server-side; mengelola hold/release/freeze.
- **Actor utama:** System (mutator), Seller (lihat saldo), Admin (freeze).
- **Domain dependency:** Payment (input), Order (trigger release), Dispute (freeze), Payout (output).
- Financial-critical: ya (otoritas uang). Async-heavy: ya. Admin-sensitive: ya.
- **Aturan keras:** seluruh mutasi uang harus melewati `WalletService`. Buyer wallet **bukan** money authority. Refund melewati semantik gateway.

## 20. Finance — Refund

- **Tujuan:** mengembalikan dana ke buyer via gateway.
- **Actor utama:** System, Finance Admin (manual).
- **Domain dependency:** Payment, Wallet, Dispute.
- Financial-critical: ya. Async-heavy: ya. Admin-sensitive: ya.

## 21. Finance — Payout & Bank Account

- **Tujuan:** mencairkan saldo seller ke rekening.
- **Actor utama:** Seller (request), System (eksekusi), Finance Admin (review).
- **Domain dependency:** Wallet, Verification (gating payout authority), Bank account, Risk.
- Financial-critical: ya. Async-heavy: ya. Admin-sensitive: ya.
- **Canonical truth:**
  - Payout authority dibuka hanya saat Seller di Layer 5 (Verified Seller — `approved` AND tidak di Layer 6). Subscription `active` tidak grant payout authority.
  - **Liability invariant:** *blocked withdrawal does not erase seller ownership.* Trust downgrade tidak memutate ledger; hanya menutup gate extraction.
  - Active obligations survive trust downgrade — order COMPLETED tetap menghasilkan dana ke saldo seller meski trust downgraded.

## 22. Finance — Billing

- **Tujuan:** subscription billing dan platform fee billing.

## 23. Incentive — Coins

- **Tujuan:** loyalty point (bukan uang).
- **Actor utama:** Buyer (earn/spend), System.
- **Domain dependency:** Order (earn pada complete, refund pada cancel), Pricing (spend di checkout).
- Financial-critical: tidak (coins ≠ uang). Async-heavy: sebagian (refund worker). Admin-sensitive: tidak.

## 24. Governance — Moderation & Report

- **Tujuan:** menjaga kualitas & legalitas konten/akun.
- **Actor utama:** User (report), Admin/Moderator (decide), User (appeal).
- **Domain dependency:** Content, Comment, User, Notification.
- Financial-critical: tidak. Async-heavy: sebagian. Admin-sensitive: ya.

## 25. Governance — Verification

- **Tujuan:** KYC seller dan verifikasi identitas; gate payout authority.
- **Actor utama:** Seller (submit), Admin (review + trust downgrade pasca-approve).
- **Domain dependency:** Identity, Finance (gating payout).
- Financial-critical: tidak langsung. Async-heavy: tidak. Admin-sensitive: ya.
- **Canonical truth:** governed trust process; revocable financial trust decision; trust escalation must be attributable + environment-aware. Lifecycle states + state machine + invariants hidup di [Verification Review Governance](./doctrine/verification-review-governance.md), [Revocable Trust Model](./doctrine/revocable-trust-model.md), dan [Trust Escalation Safety](./doctrine/trust-escalation-safety.md).

## 26. Governance — Dispute

- **Tujuan:** menyelesaikan sengketa transaksi.
- **Actor utama:** Buyer/Seller (open), Admin (resolve), System (timeout).
- **Domain dependency:** Order, Wallet (freeze), Refund, Notification, Support.
- Financial-critical: ya (mengarahkan refund/release). Async-heavy: ya. Admin-sensitive: ya.

## 27. Governance — Audit

- **Tujuan:** forensik event bisnis kritis.
- **Actor utama:** Admin (compliance).
- **Domain dependency:** semua domain (sebagai event source).
- Financial-critical: tidak (tetapi mengamati domain financial-critical). Async-heavy: ya. Admin-sensitive: ya.

## 28. Governance — Support

- **Tujuan:** komunikasi antara user dan support.
- **Actor utama:** User, Support Admin.
- **Domain dependency:** Chat (reuse), Dispute (jalur eskalasi).
- Financial-critical: tidak (langsung). Async-heavy: sedikit. Admin-sensitive: ya.

## 29. Notification

- **Tujuan:** distribusi event ke user (DB + push).
- **Actor utama:** System (router), User (consumer).
- **Domain dependency:** semua domain emitter (Order, Chat, Dispute, dll).
- Financial-critical: tidak. Async-heavy: ya (outbox). Admin-sensitive: tidak.

## 30. Risk / Fraud

- **Tujuan:** deteksi pola penipuan / abuse.
- **Actor utama:** System.
- **Domain dependency:** Order, Payment, Payout, User.
- Financial-critical: ya (efek tidak langsung — gating). Async-heavy: ya. Admin-sensitive: ya.

## 31. Realtime / Workers / Integration / Projection

Domain **infrastruktur bisnis** — bukan domain feature, tetapi mendorong banyak flow async.

- **Realtime** — WebSocket hub untuk chat, auction, notif live.
- **Workers** — auction lifecycle, payout, expire, reconciliation, fraud detect.
- **Integration** — payment gateway, courier, dll.
- **Projection** — read-model untuk feed/discovery/ratings.

Karakter:
- Async-heavy: selalu ya.
- Admin-sensitive: sebagian (workers yang dapat di-pause).
- Financial-critical: sebagian (payout, refund webhook).

## 32. Admin Operasi (apps/admin)

Admin app **memanfaatkan** domain governance, finance, dll. — bukan domain bisnis sendiri. Surface admin: Login Admin, Users, Orders, Payments, Payouts, Withdrawals, Audit Logs, Disputes, Moderation, Appeals, Warnings, Verifications, SLA Dashboard.

---

## Ringkasan Atribut Domain

| Domain | Financial-critical | Async-heavy | Admin-sensitive |
|---|---|---|---|
| Identity & Auth | – | sedikit | – |
| Profile/Preference | – | – | – |
| Address Book | – | – | – |
| Seller Capability | gating | – | ya |
| Social — Content | – | sedang | ya |
| Social — Graph | – | sedang | – |
| Social — Engagement | – | sedang | sebagian |
| Discovery | – | ya | – |
| Chat | tidak (boundary) | ya | sebagian |
| Listing | – | sebagian | sebagian |
| Auction | sebagian | ya | sebagian |
| Negotiation | – | ya | – |
| Pricing/Discount/Promotion | sebagian | sebagian | – |
| Subscription | ya | sebagian | sebagian |
| Checkout | ya | sebagian | – |
| Order | ya | ya | ya |
| Shipping | – | sebagian | – |
| Payment | ya | ya | sebagian |
| Wallet & Escrow | **otoritas uang** | ya | ya |
| Refund | ya | ya | ya |
| Payout & Bank | ya | ya | ya |
| Coins | tidak (loyalty) | sebagian | – |
| Moderation/Report | – | sebagian | ya |
| Verification | gating | – | ya |
| Dispute | ya | ya | ya |
| Audit | – | ya | ya |
| Support | – | sedikit | ya |
| Notification | – | ya | – |
| Risk/Fraud | ya (gating) | ya | ya |
| Realtime/Workers | sebagian | **ya** | sebagian |
