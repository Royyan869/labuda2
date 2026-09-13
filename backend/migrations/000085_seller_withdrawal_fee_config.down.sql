-- 000085_seller_withdrawal_fee_config (down)
-- Rollback: remove the canonical seller withdrawal fee config key.

DELETE FROM platform_configs WHERE key = 'seller_withdrawal_fee_rupiah';