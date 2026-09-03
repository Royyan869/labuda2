-- Rollback: restore legacy shipping snapshot columns
ALTER TABLE pricing_tokens ADD COLUMN IF NOT EXISTS shipping_estimated_days text;
ALTER TABLE pricing_tokens ADD COLUMN IF NOT EXISTS shipping_expedition_name text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipping_estimated_days text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipping_expedition_name text;
ALTER TABLE shipping_city_overrides ADD COLUMN IF NOT EXISTS price bigint;
