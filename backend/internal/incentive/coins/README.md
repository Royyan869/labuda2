# Coins Domain — Canonical Business Truth

> **Owner canonical 2026-06-16. All code comments and tests must align with this document.**

---

## 1. What Coins Are

Coins adalah **hak penggunaan platform Labuda** (platform usage rights), bukan uang,
bukan saldo dompet, dan bukan kewajiban finansial Labuda kepada pengguna.

- Coins **dimiliki oleh Labuda**, bukan oleh pengguna.
- Pengguna hanya memiliki **hak untuk menggunakan coins di dalam platform Labuda**.
- Labuda tidak memiliki utang (payable liability) kepada pengguna atas saldo coins mereka.
- Apabila pengguna menghapus akun atau meninggal, **Labuda tidak memiliki kewajiban membayar** coins yang tersisa.

---

## 2. What Coins Are NOT

| Dilarang | Alasan |
|---------|--------|
| Coins bukan uang / saldo finansial | Tidak ada nilai moneter tetap |
| Coins tidak dapat ditarik ke rekening bank | Tidak ada jalur withdrawal |
| Coins tidak dapat ditransfer ke pengguna lain | Tidak ada jalur transfer |
| Coins tidak dapat ditukar dengan uang tunai | Tidak ada jalur cash redemption |
| Coins tidak dapat digunakan di luar platform Labuda | Hak penggunaan terbatas |
| Coins tidak dapat dibeli oleh pengguna | Tidak ada jalur coin purchase |
| Coins tidak dapat diberikan oleh admin secara manual | Tidak ada admin grant |

---

## 3. Earn Rules

**Formula utama:** Rp1.000 nilai transaksi = 1 coin

**Sumber earn saat ini (V1):**
- Transaksi selesai (order.completed): `finalPaidAmount / 1000` coins diberikan kepada pembeli

**Pembatasan earn:**
- Nilai transaksi minimum: Rp10.000 (tidak ada coins untuk transaksi sangat kecil)
- Batas harian: 10.000 coins per pengguna per hari

**Sumber earn masa depan (PARKED — belum diimplementasi):**
- Seller registration / seller activation: Rp1.000 = 1 coin (formula sama)
- Annual seller subscription renewal: Rp1.000 = 1 coin (formula sama)

> Jangan implementasikan seller earn sources sebelum transaction-based coins berjalan dengan benar.

---

## 4. Allowed Spend Contexts

Coins hanya dapat digunakan pada konteks transaksi berikut:

| Konteks | Status |
|---------|--------|
| Listing non-negotiable checkout | ✅ AKTIF |
| Listing negotiated order / checkout | ✅ AKTIF |
| Shipping fee (termasuk dalam order total) | ✅ AKTIF (sudah termasuk dalam base perhitungan) |
| Auction buy-now checkout | ✅ AKTIF |
| Auction bid-win claim / payment | ✅ AKTIF (owner canonical 2026-06-16) |

**Coins TIDAK DAPAT digunakan untuk:**
- Penarikan (withdrawal)
- Subscription biaya seller (belum diputuskan)
- Diskon terpisah dari order / promotion campaign

---

## 5. Spend Rules (Backend Authority)

Backend adalah **satu-satunya otoritas** atas semua keputusan coins:

| Aturan | Detail |
|--------|--------|
| **Maksimal 20% dari nilai order** | `maxCoinsAllowed = orderValue / 5` |
| **Commission safety** | Coins tidak boleh mengurangi pembayaran di bawah jumlah komisi |
| **Balance cap** | Coins yang digunakan tidak boleh melebihi saldo aktif pengguna |
| **Mobile intent only** | Mobile hanya mengirim flag `use_coins: true/false` — backend menentukan jumlah aktual |
| **Atomicity** | Coins digunakan dalam transaksi DB yang sama dengan pembuatan order |

`OrderValueForCoins = subtotal + shipping_fee - discount`

---

## 6. Earn Formula Detail

```
coins_earned = final_paid_amount / 1000   (integer division)
```

Contoh: Order Rp150.000 → 150 coins

---

## 7. Idempotency

- **Hard guard:** UNIQUE index `idx_coins_transactions_unique_reference` pada `(user_id, reference_type, reference_id)`
- **Pre-check:** `FindEarnByReference` fast path sebelum insert
- **INSERT-FIRST pattern:** Attempt insert → PostgreSQL 23505 = idempotent success

---

## 8. Dual-Table Design

| Tabel | Fungsi |
|-------|--------|
| `coins_transactions` | Append-only audit ledger (earn / spend / refund) |
| `user_coin_balance` | Single aggregate row per user; atomic spend guard (`UPDATE WHERE balance >= amount`) |

Balance di `user_coin_balance` selalu sinkron dengan ledger melalui setiap write di service.

---

## 9. Refund

Semua refund coins mengalir melalui **satu entry point**:
`coins.refund_required` outbox event → `CoinsRefundRequiredHandler`

Lihat [COINS_REFUND_ARCHITECTURE.md](COINS_REFUND_ARCHITECTURE.md) untuk detail.

---

## 10. Enforcement Rules (System-Level)

- Coins TIDAK boleh diperlakukan sebagai uang atau saldo finansial
- Coins TIDAK boleh memiliki jalur withdrawal, transfer, atau cash redemption
- Coins HANYA dapat digunakan dalam konteks transaksi yang terdaftar di atas
- Semua penggunaan coins harus tercatat dalam sistem (audit trail wajib)
- Coins TIDAK boleh menjadi sumber kebenaran harga (pricing source of truth)
- Backend adalah otoritas tunggal untuk jumlah coins yang digunakan


