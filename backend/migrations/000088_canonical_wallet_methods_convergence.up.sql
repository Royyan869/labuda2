-- ============================================================
-- 000088 ADDITIVE WALLET METHODS — OVO / DANA / GOPAY / SHOPEEPAY
--
-- Phase 2D: ADDITIVE migration. Existing buyer payment methods are
-- REQUIRED and MUST remain. This migration ONLY adds missing wallet
-- methods; it does NOT replace, purge, disable, or alter existing
-- methods.
--
-- Factual existing before this migration (000006 + 000007): 5 rows
--   bank_transfer       (Virtual Account: bca_va/bni_va/bri_va/permata_va/other_va) flat 4000
--   qris                (other_qris) percent 70 min 500
--   credit_card         (credit_card) percent_plus_flat 2000+290
--   dana                (dana) percent 150  — existing, do not duplicate
--   convenience_store   (alfamart/indomaret; Alfamidi/DAN+DAN via alfamart) flat 5000
--
-- Required wallets (locked business truth): OVO, DANA, GoPay, ShopeePay
--   DANA already exists → KEEP (do not insert, do not update)
--   Missing → ADD: gopay, ovo, shopeepay
--
-- Target after this migration: 5 existing KEEP + 3 ADD = 8 rows
--   bank_transfer       KEEP
--   qris                KEEP
--   credit_card         KEEP
--   dana                KEEP (existing)
--   convenience_store   KEEP
--   gopay               ADD
--   ovo                 ADD
--   shopeepay           ADD
--
-- Fee truth for new wallets:
--   ovo/gopay/shopeepay are seeded as public_baseline placeholders
--   at the same e-wallet tier as DANA (percent 150bps = 1.5%).
--   No merchant_verified rate exists — all three require explicit
--   owner verification against the Midtrans merchant contract before
--   being treated as reliable, even as a baseline.
--   See OWNER DECISION REQUIRED in Phase 2 report. Do not alter
--   existing fees, channels, enabled state, or sort_order in this
--   migration.
-- ============================================================

-- ADD missing wallet methods only (dana already exists — do not touch).
INSERT INTO payment_methods
    (method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah, midtrans_channels, sort_order, rate_source, rate_source_note)
VALUES
    ('gopay', 'GoPay', true, 'percent', 0, 150, NULL, NULL,
        ARRAY['gopay'], 10,
        'public_baseline',
        'Public Midtrans pricing placeholder at 1.5% (150 bps) — same e-wallet tier as DANA. Not Labuda''s merchant-contract rate. Requires explicit owner verification.'),
    ('ovo', 'OVO', true, 'percent', 0, 150, NULL, NULL,
        ARRAY['ovo'], 20,
        'public_baseline',
        'Public Midtrans pricing placeholder at 1.5% (150 bps) — same e-wallet tier as DANA. Not Labuda''s merchant-contract rate. Requires explicit owner verification.'),
    ('shopeepay', 'ShopeePay', true, 'percent', 0, 150, NULL, NULL,
        ARRAY['shopeepay'], 30,
        'public_baseline',
        'Public Midtrans pricing placeholder at 1.5% (150 bps) — same e-wallet tier as DANA. Not Labuda''s merchant-contract rate. ShopeePay e-wallet only; SPayLater is a separate forbidden channel. Requires explicit owner verification.')
ON CONFLICT (method_code) DO NOTHING;

-- Existing methods (bank_transfer, qris, credit_card, dana, convenience_store)
-- are intentionally untouched: no DELETE, no UPDATE, no SET NULL, no mapping.
-- sort_order for dana remains 25 from 000007; no UPDATE needed.
