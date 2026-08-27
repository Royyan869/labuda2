-- 000045_order_item_product_identity_convergence.down.sql

ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS order_items_product_id_fkey;

ALTER TABLE order_items
    ALTER COLUMN product_id DROP NOT NULL;