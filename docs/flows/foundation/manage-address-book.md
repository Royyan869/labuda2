# Manage Address Book

> **Status:** STABLE
> **Domain:** Address Book

## Purpose

Mengelola alamat-alamat yang dimiliki Registered User: untuk menerima barang (Buyer) dan untuk mengirim barang (Seller). Address Book adalah sumber data **siapa** dan **di mana** untuk Order.

## Actors

- **Registered User** — pemilik address book.
- **Buyer** — pemakai address purpose `shipping` saat checkout.
- **Seller** — pemilik address purpose `sender` (alamat asal pengiriman).
- **System** — memvalidasi field, menjaga konsistensi primary address, dan menyediakan snapshot saat order dibuat.

## Preconditions

- User sudah `active` dan sign-in.
- Untuk membuat alamat ber-purpose `sender`: User sudah memiliki Seller capability ([Become Seller](./become-seller.md)).

## Main Flow

1. User membuka layar Address Book.
2. User memilih **Tambah Alamat** atau memilih alamat existing untuk diedit.
3. User mengisi field:
 - **Nama penerima** (atau nama pengirim untuk purpose sender).
 - **Nomor telepon**.
 - **Provinsi, Kota, Kecamatan, Kelurahan/Desa** (dipilih dari data wilayah).
 - **Alamat jalan** (detail).
 - **Kode pos** (opsional).
 - **Koordinat** (opsional, lat/long).
 - **Label/nickname** (opsional, mis. "Rumah", "Kantor").
 - **Purpose**: `shipping` (default) atau `sender` (hanya jika User Seller).
4. User menyimpan.
5. Sistem memvalidasi field dan menyimpan address baru / perubahan.
6. User dapat menetapkan satu alamat sebagai **primary** (default checkout).

## Alternate Flows

- **Setel Primary**: User menandai sebuah alamat sebagai primary; sistem memastikan hanya **satu** primary per user (alamat lama dilepas).
- **Soft delete**: User menghapus alamat — alamat tidak muncul lagi di pilihan checkout, tetapi record-nya tetap tersimpan untuk integritas riwayat order.

## Failure / Rejection Cases

- **Nama penerima kosong** → ditolak.
- **Nomor telepon < 10 atau > 15 karakter** → ditolak.
- **ProvinceID atau CityID kosong** → ditolak.
- **Alamat jalan kosong** → ditolak.
- **User non-Seller mencoba membuat purpose `sender`** → ditolak dengan error semantic-spesifik (sender requires seller role).
- **User mencoba menghapus alamat yang sedang primary** → diperbolehkan secara teknis, tetapi berikutnya User harus memilih primary baru sebelum checkout.

## State Changes

- **Address (entity)**: tidak ada → terisi, atau terisi → ter-update.
- **Primary flag**: jika di-set, primary lama menjadi non-primary.
- **`is_available_for_checkout`**: aktif (true) saat dibuat → false saat soft-delete.

## Financial Impact

Flow ini tidak memiliki dampak finansial. Tidak menyentuh Wallet/Escrow/Coins.

> Address tidak menyimpan saldo, tidak menyimpan biaya pengiriman, dan tidak terkait dengan pembayaran. Pengaruh address terhadap **biaya pengiriman** dilakukan oleh domain Shipping di flow Checkout/Shipping, bukan di sini.

## Notifications

- Tidak ada notifikasi user-visible default pada flow ini.

## Cross-Domain Relations

- **Order**: order menyimpan **snapshot** dari address (recipient name, phone, lokasi, jalan, postal code) — bukan referensi. Edit / hapus alamat tidak mengubah riwayat order.
- **Shipping**: address purpose `shipping` dipakai untuk menghitung biaya pengiriman dan validasi coverage; address purpose `sender` dipakai untuk menentukan titik asal pengiriman seller.

## Business Rules

- **Primary address**: hanya **satu** primary per user, ditegakkan dengan transaksi (primary lama dilepas saat primary baru di-set).
- **Soft delete via flag** `is_available_for_checkout = false` — alamat tidak hilang dari DB, hanya tidak dapat dipakai untuk checkout baru.
- **Snapshot ke Order**: saat Order dibuat, sistem **menyalin** field address ke Order; bukan referensi. Konsekuensi: edit alamat tidak mengubah Order yang sudah ada.
- **Purpose**:
 - `shipping`: alamat penerima — siapa pun (Buyer/Registered User) boleh memiliki.
 - `sender`: alamat pengirim — hanya Seller. Ditolak jika User belum Seller.
- **Format kode pos**: opsional; tidak divalidasi formatnya. Mobile memberi panduan format.
- **Format nomor telepon**: 10–15 karakter; mobile menerapkan regex `^\+?[0-9]{10,15}$`.

## Forbidden Behaviors

- Sistem tidak boleh menerima address baru tanpa minimal: nama penerima, nomor telepon, provinsi, kota, alamat jalan.
- Sistem tidak boleh memungkinkan User non-Seller membuat alamat purpose `sender`.
- Sistem tidak boleh memiliki dua alamat primary aktif untuk satu user.
- Sistem tidak boleh menghapus address secara hard-delete saat ada riwayat order atau order aktif yang merujuk pada snapshot-nya. Soft delete adalah jalur yang benar.
- Sistem tidak boleh mengubah snapshot address yang sudah tertanam di Order ketika address sumber diedit/dihapus — riwayat Order harus immutable terhadap perubahan address book.

## Notes

- Address Book tidak menyimpan informasi pajak / NPWP / data finansial lain — pemisahan ini disengaja.
