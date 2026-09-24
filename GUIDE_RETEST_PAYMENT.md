# GUIDE_RETEST_PAYMENT.md — Retest Matrix Mesin Payment (Sandbox)

Checklist retest manual **per metode pembayaran**. Konteks: semua metode lewat **satu mesin** (Midtrans Snap di dalam `PaymentWebviewScreen`); nomor VA/QR/kode retail **ditampilkan oleh halaman Snap**, bukan dibuat oleh app Labuda. Perbaikan yang harus diverifikasi ikut bekerja: (1) webview auto-close pada `/payment/finish`, (2) handoff deep-link wallet + banner panduan, (3) dialog aktivasi seller pertama.

## Persiapan
- [ ] Backend jalan dengan Midtrans **sandbox** (`MIDTRANS_ENV=sandbox`), `FRONTEND_URL` terisi (agar `callbacks.finish` aktif).
- [ ] App terhubung ke backend sandbox; siapkan 2 akun: **pembeli** (order) dan **seller baru** (aktivasi langganan).
- [ ] Emulator/fisik dengan app wallet sandbox yang dipakai Midtrans (GoPay/ovo/dana sandbox app bila tersedia).

## Legenda hasil
✅ = sesuai harapan · ⚠️ = jalan tapi perlu perbaikan lanjutan · ❌ = rusak

## A. Virtual Account (metode `bank_transfer`, kanal bca_va/bni_va/bri_va/permata_va/other_va)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| A1 | Pilih VA → Bayar → webview terbuka halaman Snap | Halaman Snap tampil, **pilih bank dulu** (belum ada no. VA — normal) | |
| A2 | Pilih bank (mis. BCA) | No. VA **muncul di halaman Snap** | |
| A3 | Biarkan webview terbuka, bayar via simulator m-banking | Tidak ada auto-close paksa (by design); settle terdeteksi lewat polling/workers | |
| A4 | Bayar → kembali ke Snap → tekan selesai | Redirect `/payment/finish` → **webview close sendiri** | |
| A5 | Aktivasi seller (akun baru): settle VA | Dialog "Memproses pembayaran" berubah sukses *"Selamat! Anda sekarang penjual"*; jika melewati window polling, tutup dialog → **"Cek status pembayaran"** → konfirmasi manual; juga terdeteksi otomatis saat app resume | |

## B. QRIS (metode `qris`, kanal other_qris)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| B1 | Pilih QRIS → Bayar | QR tampil di Snap | |
| B2 | Bayar pakai app lain / simulator QRIS sandbox | Snap mendeteksi → redirect finish → webview close | |
| B3 | Settle terdeteksi app (order status / seller aktif) | Berubah tanpa restart app | |

## C. Wallet — GoPay/OVO/DANA/ShopeePay (metode gopay/ovo/dana/shopeepay)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| C1 | Pilih wallet → Bayar | **Banner panduan** tampil di atas webview | |
| C2 | Klik tombol buka app wallet di Snap | App wallet **terbuka via deep-link** (bukan ERR_UNKNOWN_URL_SCHEME) | |
| C3 | Device **tanpa** app wallet | `intent://` jatuh ke browser fallback / store, tanpa error toast | |
| C4 | Selesaikan bayar di wallet → kembali ke Labuda | Settle terdeteksi via polling/workers | |

## D. Kartu Kredit/Debit (metode `credit_card`, kanal credit_card)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| D1 | Pilih kartu → isi kartu test sandbox | Form kartu tampil | |
| D2 | Kartu 3DS (test card 3DS-enabled) | Challenge 3DS tampil dan selesai di dalam Snap | |
| D3 | Sukses → redirect finish | **Webview close sendiri** (kasus yang Anda laporkan kemarin — harus kini close, bukan halaman error) | |

## E. Retail — Alfamart/Indomaret (metode `convenience_store`, kanal alfamart/indomaret)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| E1 | Pilih retail → Bayar | Kode pembayaran tampil di Snap | |
| E2 | Bayar simulasi retail sandbox | Settle terdeteksi (discovery/reconciliation worker) | |

## F. Lintas-mesin (regression umum)
| # | Langkah | Harapan | Hasil |
|---|---|---|---|
| F1 | Mulai bayar metode A (pending), ganti metode B, initiate lagi | Payment lama **di-reuse** (snapshot immutable) — cek response `payment_id` sama | |
| F2 | Biarkan payment expire (24h — atau minta backend pendekkan di env test) | Expiry tertangani, tidak ada settle setelah expire | |
| F3 | Tutup webview manual tanpa bayar → initiate ulang | Snap URL sama (reuse), tidak ada dobel row payment | |
| F4 | Kill app saat pending → buka lagi | State seller/order konsisten setelah settle (workers bekerja tanpa app) | |

## Catatan
- ❌ pada C2 adalah **bug P1** — seharusnya sudah diperbaiki (deep-link handoff + banner); kalau masih gagal, laporkan.
- Setelah 60 detik polling: dialog kini **selalu** punya tombol "Cek status pembayaran" + loop re-entry (Batch 2 selesai) — gunakan itu; ⚠️ hanya jika tombol tidak muncul.
- Simpan screenshot setiap anomali beserta `order_id`/`payment_number` untuk trace ke log backend (`payment_webhook_notifications`, log discovery worker).
