-- 000040: S2C2 canonical refund economics rebase.
-- Add refunded_product_amount, refunded_shipping_amount, coins_refunded_amount to refunds table.
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS refunded_product_amount bigint;
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS refunded_shipping_amount bigint;
ALTER TABLE refunds ADD COLUMN IF NOT EXISTS coins_refunded_amount bigint;
