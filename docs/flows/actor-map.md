# Actor Map

Peta seluruh actor yang muncul di flow bisnis Labuda.

> Actor di sini adalah **peran bisnis**, bukan tabel `users`.
> Satu user bisa memegang beberapa peran sekaligus (mis. Identity Complete + Buyer + Seller).
> System actor adalah jalur otomatis (worker, webhook, scheduler) yang men-trigger flow tanpa manusia.

---

## Hierarki

```
Guest
 └─ Authenticated Account               (Layer A)
    └─ Identity Complete Account        (Layer A + B — full app entry)
       └─ Email Verified Account        (Layer A + B + C — interaction & transaction authority)
          ├─ Buyer                      (capability checkout)
          └─ Seller                     (Layer D)
             ├─ Subscribed Seller (Unverified)   [Layer 4 — selling authority; payout blocked]
             ├─ Verified Seller                  [Layer 5 — payout authority; revocable]
             └─ Suspended / Revoked Trust        [Layer 6 — login + obligation handling only]

Admin
 ├─ Moderator
 ├─ Support Admin
 ├─ Finance Admin
 └─ Verification Admin

System
 ├─ Notification Worker / Outbox Consumer
 ├─ Auction Worker (start / end / settlement)
 ├─ Order Worker (auto-complete / expire)
 ├─ Negotiation Expire Worker
 ├─ Payment Webhook Consumer
 ├─ Refund Webhook Consumer
 ├─ Payout Worker
 ├─ Reconciliation Worker
 ├─ Dispute Timeout Worker
 ├─ Coins Refund Worker
 ├─ Risk / Fraud Detector
 └─ Realtime Hub (WebSocket dispatcher)

External
 ├─ Payment Gateway (Midtrans)
 └─ Courier API
```

> Definisi layer A/B/C/D dan stage Layer 4/5/6 hidup di [Layered Identity & Trust Model](./doctrine/layered-identity-trust-model.md) dan [Capability Matrix](./doctrine/capability-matrix.md). Dokumen ini hanya memetakan actor; doctrine docs adalah sumber kebenaran kapabilitas.

---

## 1. Guest

- **Capability:** browse listing publik, sign up, sign in.
- **Batasan:** tidak boleh post, checkout, chat, follow, atau report.
- **Domain yang disentuh:** Identity, Catalog/Listing (browse), Discovery (search dengan visibility filter).

## 2. Authenticated Account (Layer A)

Akun lulus Authentication, tetapi `username` belum diisi (`profile_completed=false`).

- **Capability:** memicu Complete Profile gate.
- **Batasan canonical:** tidak diberi full app entry — terhadang Complete Profile gate. Tidak boleh browse listing, view profile, atau memicu fitur sosial sampai username dipilih.
- **Domain yang disentuh:** Identity (Complete Profile gate).

## 3. Identity Complete Account (Layer A + B)

Username sudah dipilih, email belum terverifikasi.

- **Capability:** browse area read-only sesuai ALLOWED list di [Email Gating Matrix](./doctrine/email-gating-matrix.md). Edit basic profile.
- **Batasan canonical:** semua action di BLOCKED list ditolak backend. Mobile menampilkan banner persistent + inline gate; tidak ada blok-navigasi global.
- **Domain yang disentuh:** Identity, Profile, Social (read-only), Discovery, Notification (delivery).

## 4. Email Verified Account (Layer A + B + C)

- **Capability:** mengikuti seluruh flow social (post, comment, like, follow, share, rate); mengelola profile, preferensi, alamat; submit report dan appeal; membuka tiket support; menerima notifikasi.
- **Batasan:** belum bisa menjual sebelum upgrade ke Seller (Layer D).
- **Domain yang disentuh:** Identity, Profile, Social (semua), Discovery, Notification, Report/Moderation, Support, Chat.

## 5. Buyer

Email Verified Account yang melakukan transaksi pembelian. Buyer bukan capability tetap — berlaku per-order.

- **Capability:** checkout (direct, via negotiation, via auction claim); place bid; open negotiation; pay via gateway; confirm delivery; cancel order (sebelum shipment); open dispute (pre-delivery); memberi rating; earn dan spend coins.
- **Batasan:** tidak boleh mutate ledger / wallet langsung — semua via gateway atau service backend. Tidak boleh override status order.
- **Domain yang disentuh:** Catalog (Listing/Auction), Negotiation, Pricing, Checkout, Order, Shipping, Payment, Coins, Dispute, Rating.

## 6. Seller — Layer 4 (Subscribed Seller, Unverified)

Subscription `active`; verification belum `approved`.

- **Capability:** CRUD listing dan auction; konfigurasi shipping; counter offer; mark order shipped; upload shipping proof; open dispute (post-delivery); beli promotion / subscription; lihat earnings dan analytics; manage bank account (preparation). Saldo MAY accumulate internally.
- **Batasan canonical:** tidak boleh withdraw / payout / financial extraction sebelum verification approved.

## 7. Seller — Layer 5 (Verified Seller)

Subscription `active` AND verification `approved`.

- **Capability:** semua kapabilitas Layer 4 + request withdrawal / payout (payout authority terbuka). Full seller participation.
- **Batasan:** tidak boleh mutate ledger; saldo dikelola WalletService. Pricing wajib lewat pricing token.

## 8. Seller — Layer 6 (Suspended / Revoked Trust)

Sebelumnya `approved`, sekarang `suspended` / `revoked` / `under_investigation`. Existence + balance + active obligations survive — lihat [Revocable Trust Model](./doctrine/revocable-trust-model.md).

- **Yang TETAP berlaku:** login; akses support / dispute; active order lifecycle; active dispute lifecycle; audit + historical visibility; saldo seller tetap visible dan tetap liability.
- **Yang DIPOTONG / MAY direstriksi:** withdraw / payout (blocked); publish listing baru / create auction baru; growth / promotional capability; trust escalation actions.

> Seluruh detail capability per layer hidup di [Capability Matrix](./doctrine/capability-matrix.md). Dokumen ini hanya peta actor.

## 9. Admin (super-set)

Akses ke admin panel (`apps/admin`). Sub-peran membatasi area tindakan; admin generic adalah peran maksimal.

## 10. Moderator

- **Capability:** review moderation case (approve/reject/remove); issue warning / violation; review appeal.
- **Batasan:** tidak menyentuh finance; tidak menyentuh dispute resolution.
- **Domain yang disentuh:** Governance/Moderation, Notification.

## 11. Support Admin

- **Capability:** balas tiket support; resolve dispute (refund / release / partial split); pantau SLA.
- **Batasan:** refund dilakukan via WalletService / RefundService — bukan mutasi langsung.
- **Domain yang disentuh:** Support, Dispute, Wallet (via service), Notification.

## 12. Finance Admin

- **Capability:** review dan approve payout / withdrawal; inisiasi gateway refund manual; pantau reconciliation.
- **Batasan:** mutasi uang lewat WalletService dan jalur gateway; tidak boleh edit ledger langsung.
- **Domain yang disentuh:** Payment, Refund, Wallet, Payout, Bank Account, Audit.

## 13. Verification Admin

- **Capability:** approve / reject / mark `needs_resubmission` untuk seller verification; trust downgrade pasca-approve (`suspended` / `revoked` / `under_investigation`). Setiap action wajib menyertakan reason + audit log entry (operator + timestamp).
- **Batasan canonical:** tidak boleh men-trigger transisi tanpa reason; tidak boleh menghapus seller record / saldo seller saat trust downgrade; tidak boleh memutus active obligation handling; tidak boleh mengaktifkan hidden auto-approve di production. Lihat [Trust Escalation Safety](./doctrine/trust-escalation-safety.md).
- **Domain yang disentuh:** Verification, Identity, Finance (gating payout), Audit, Notification.

## 14. System Worker (umbrella)

System adalah aktor non-manusia yang menjalankan flow async.

| Sub-tipe | Tugas |
|----------|-------|
| Notification Worker / Outbox Consumer | Distribusi event ke recipient (DB + push); filter berdasarkan account status & kategori |
| Auction Worker | Lifecycle auction: mulai, akhir, settlement |
| Order Worker | Auto-complete order delivered + lewat window; trigger coins earn |
| Negotiation Expire Worker | Menutup sesi negosiasi yang basi |
| Payment Webhook Consumer | Memproses webhook gateway; otoritatif terhadap status payment |
| Refund Webhook Consumer | Memproses ack refund dari gateway |
| Payout Worker | Mengeksekusi payout setelah approve |
| Reconciliation Worker | Auto-repair orphaned webhook / mismatch |
| Dispute Timeout Worker | Menandai dispute lewat batas review |
| Coins Refund Worker | Mengembalikan coin yang dipakai jika order dibatalkan |
| Risk / Fraud Detector | Pattern detection |
| Realtime Hub | WebSocket dispatcher |

**Batasan umum System Worker:**
- Untuk uang: hanya melalui WalletService / Gateway.
- Untuk content: keputusan moderasi tetap pada Moderator; system hanya menjalankan pipeline.

## 15. External Actors

- **Payment Gateway (Midtrans)** — bukan actor internal, tapi men-drive flow (webhook → backend). Mengontrol otoritas status payment dan refund.
- **Courier API** — memberi quote pengiriman dan validasi coverage.

---

## Tabel Ringkas Capability vs Domain

| Actor | Identity | Social | Chat | Listing | Auction | Negotiation | Checkout | Order | Payment | Wallet/Escrow | Payout | Coins | Moderation | Dispute | Support | Notification |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Guest | r/w (auth) | – | – | r | r | – | – | – | – | – | – | – | – | – | – | – |
| Email Verified Account | r/w | r/w | r/w | r | r | – | – | – | – | – | – | – | r (report) | – | r/w | r/w |
| Buyer | r | r/w | r/w | r | r/w (bid) | r/w (open/accept) | r/w | r/w (own) | r/w (init) | r (own) | – | r/w | – | r/w (open) | r/w | r/w |
| Seller | r | r/w | r/w | r/w | r/w | r/w (counter) | – | r/w (own seller-side) | – | r (own) | r/w (Layer 5) | – | – | r/w (open post-deliv) | r/w | r/w |
| Moderator | – | r/w (mod actions) | – | r (mod) | r (mod) | – | – | – | – | – | – | – | r/w | – | – | r |
| Support Admin | – | – | r/w (support chat) | – | – | – | – | r (audit) | – | r (audit) | – | – | – | r/w (resolve) | r/w | r |
| Finance Admin | – | – | – | – | – | – | – | r (audit) | r/w | r/w (via service) | r/w | – | – | r (financial side) | – | r |
| Verification Admin | r (audit) | – | – | – | – | – | – | – | – | – | gating only | – | r (audit) | – | – | r |
| System Worker | – | – | r/w (events) | r/w (index) | r/w (lifecycle) | r/w (expire) | – | r/w (auto-complete) | r/w (webhook) | r/w (via service) | r/w | r/w (refund earn) | r (filter) | r/w (timeout) | r | r/w |

> Legend: `r` = read, `w` = write, `r/w` = both. Kolom kosong = tidak ada interaksi tipikal.
