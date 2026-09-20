-- ============================================================
-- 000089 PAYMENT METHOD DISPLAY ORDER CLEANUP
--
-- Tujuan: Rapikan urutan tampilan payment method pada surface
-- pembayaran Labuda agar Convenience Store berada paling bawah
-- dan seluruh surface menampilkan urutan canonical.
--
-- Ini HANYA perubahan sort_order (display ordering).
-- JANGAN mengubah: code, channel, fee, business semantics.
--
-- Authority: payment_methods.sort_order via
--   payment_method_repository.go:48 ListEnabled ORDER BY sort_order ASC
--   and ListAll ORDER BY sort_order ASC — sole canonical ordering.
--
-- Target canonical order:
--   1 bank_transfer       10
--   2 qris                20
--   3 credit_card         30
--   4 dana                40
--   5 gopay               50
--   6 ovo                 60
--   7 shopeepay           70
--   8 convenience_store   80  (wajib paling bawah)
--
-- Alfamart/Indomaret/Alfamidi/DAN+DAN tetap via convenience_store
-- single method, tidak di-split. Wallet tetap terpisah.
-- ============================================================

UPDATE payment_methods SET sort_order = 10 WHERE method_code = 'bank_transfer';
UPDATE payment_methods SET sort_order = 20 WHERE method_code = 'qris';
UPDATE payment_methods SET sort_order = 30 WHERE method_code = 'credit_card';
UPDATE payment_methods SET sort_order = 40 WHERE method_code = 'dana';
UPDATE payment_methods SET sort_order = 50 WHERE method_code = 'gopay';
UPDATE payment_methods SET sort_order = 60 WHERE method_code = 'ovo';
UPDATE payment_methods SET sort_order = 70 WHERE method_code = 'shopeepay';
UPDATE payment_methods SET sort_order = 80 WHERE method_code = 'convenience_store';
