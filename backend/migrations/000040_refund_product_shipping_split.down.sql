ALTER TABLE refunds DROP COLUMN IF EXISTS refunded_product_amount;
ALTER TABLE refunds DROP COLUMN IF EXISTS refunded_shipping_amount;
ALTER TABLE refunds DROP COLUMN IF EXISTS coins_refunded_amount;
