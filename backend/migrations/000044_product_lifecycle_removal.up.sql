-- 000044_product_lifecycle_removal.up.sql
--
-- Model B (Commerce Product Lifecycle, Stage 3): Product carries no selling
-- lifecycle.
--
-- products.status was a duplicated availability mirror of the selling
-- surface (fixed_price_sales.status / auctions.status): written by the FPS
-- pipeline via derivedProductStatus AND by the auction flow with raw
-- strings, consumed only by two redundant catalog predicates
-- (p.status = 'available' while fps.status = 'active' was already the
-- authority) and one order-release gate. products.sold_at was write-only.
--
-- Availability is now derived from the active selling surface only.
-- From-zero project; no data backfill required.

DROP INDEX IF EXISTS idx_products_status;

ALTER TABLE products DROP COLUMN IF EXISTS status;
ALTER TABLE products DROP COLUMN IF EXISTS sold_at;

DROP TYPE IF EXISTS product_status_enum;