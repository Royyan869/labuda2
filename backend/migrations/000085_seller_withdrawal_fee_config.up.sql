-- 000085_seller_withdrawal_fee_config
-- SCOPE-CFG-02: canonical admin-configurable seller withdrawal fee.
-- Canonical key: seller_withdrawal_fee_rupiah (Rupiah integer, Rp0 allowed).
-- Runtime consumer: ConfigService.GetSellerWithdrawalFee →
--   WithdrawService.RequestWithdrawal (snapshot at request time) →
--   settlement net_payout = amount - fee (RecordWithdrawalComplete).
-- Owner truth: default baseline Rp5.000, admin-configurable, Rp0 allowed,
-- no hardcoded runtime authority.

-- Seed the canonical key on fresh and already-migrated databases.
INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
VALUES ('seller_withdrawal_fee_rupiah', 5000, NULL, NULL, (EXTRACT(epoch FROM now()))::bigint)
ON CONFLICT (key) DO NOTHING;