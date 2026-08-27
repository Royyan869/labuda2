# Manage Preferences

> **Status:** STABLE
> **Domain:** Preference

## Purpose

Memberi Registered User kontrol atas perilaku aplikasi pribadinya — terutama **preferensi notifikasi** (kategori notifikasi yang ingin/tidak ingin diterima).

## Actors

- **Registered User** — pemilik preferensi.
- **System** — menyimpan preferensi dan menerapkannya saat distribusi notifikasi.

## Preconditions

- User sudah `active` dan sign-in.

## Main Flow

1. User membuka layar Notification Settings.
2. Sistem menampilkan toggle untuk kategori notifikasi:
   - **General** (push enabled — toggle utama).
   - **Transactions & Orders** (commerce).
   - **Communication** (chat).
   - **Security** (non-toggleable — selalu menyala).
   - **Promotions** (marketing).
3. User mengubah toggle.
4. User menekan "Simpan".
5. Sistem menyimpan preferensi dan menerapkan filter pada distribusi notifikasi.

## Failure / Rejection Cases

- Mencoba mematikan kategori non-toggleable (Security, commerce-critical) → ditolak.

## State Changes

- Preferensi user ter-update; distribusi notifikasi mengikutinya.

## Cross-Domain Relations

- **Notification (system):** tujuan dari flow ini adalah memengaruhi distribusi notifikasi yang dikirim oleh domain lain (Order, Chat, Dispute, Social, Moderation).
- **Manage Profile** vs **Manage Preferences**: profile menyimpan identitas; preferences menyimpan kontrol behavior. Dua flow terpisah.

## Business Rules

- **Default preferensi untuk akun baru:**
  - Push notification **aktif**.
  - Notifikasi order **aktif**.
  - Notifikasi chat **aktif**.
  - Security alert **aktif** (non-toggleable).
  - Marketing **non-aktif** (opt-in).

## Forbidden Behaviors

- Sistem tidak boleh mematikan kategori **commerce-critical** (mis. order paid, order shipped) berdasarkan preferensi user — komunikasi transaksional tidak boleh hilang.
- Sistem tidak boleh menampilkan toggle untuk kategori yang tidak terhubung ke event emitter aktif.
- Mobile tidak boleh mengklaim "preferensi tersimpan" jika persistence belum ditegakkan.

## Notes

- Preferensi tema, language, data sharing, dan privasi yang lebih luas belum termasuk dalam flow ini — akan ditambahkan saat tersedia.
