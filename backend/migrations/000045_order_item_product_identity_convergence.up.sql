-- 000045_order_item_product_identity_convergence.up.sql
--
-- Stage 5 (identity convergence): order_items.product_id is ALWAYS products.id.
--
-- Under the agreed Model B the stable physical-item identity is products.id and
-- the selling surface is separately recorded on orders.source_type +
-- orders.source_id. Two writers previously disagreed about the order_items
-- namespace:
--   - FPS / negotiation orders stored fixed_price_sales.id
--   - auction orders stored products.id
--
-- This migration converges historical FPS / negotiation rows onto products.id,
-- then hardens the contract so the column can never hold a selling-surface ID
-- again: NOT NULL + a real FK to products(id), matching the FPS/Auction FK
-- posture (ON DELETE RESTRICT; a product is never hard-deleted while a
-- selling surface references it).
--
-- From-zero project: no production data exists. Any row that cannot be
-- resolved (NULL product_id, or product_id pointing at nothing) aborts the
-- migration loudly instead of silently weakening the contract.

UPDATE order_items oi
SET product_id = fps.product_id
FROM fixed_price_sales fps
WHERE fps.id = oi.product_id;

ALTER TABLE order_items
    ALTER COLUMN product_id SET NOT NULL;

ALTER TABLE order_items
    ADD CONSTRAINT order_items_product_id_fkey
        FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE RESTRICT;