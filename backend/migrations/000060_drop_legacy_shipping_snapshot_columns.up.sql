-- SHIPPING-03A: Drop legacy shipping snapshot columns
-- These columns have been dead since the original schema: always NULL, no active
-- producer or consumer remains in application code.
--
-- Verified dead by:
--   - SHIPPING-02 forensic audit
--   - SHIPPING-03A full residue sweep
--
-- Columns removed:
--   orders.shipping_expedition_name
--   orders.shipping_estimated_days
--   pricing_tokens.shipping_expedition_name
--   pricing_tokens.shipping_estimated_days

ALTER TABLE orders DROP COLUMN IF EXISTS shipping_expedition_name;
ALTER TABLE orders DROP COLUMN IF EXISTS shipping_estimated_days;
ALTER TABLE pricing_tokens DROP COLUMN IF EXISTS shipping_expedition_name;
ALTER TABLE pricing_tokens DROP COLUMN IF EXISTS shipping_estimated_days;
ALTER TABLE shipping_city_overrides DROP COLUMN IF EXISTS price;
