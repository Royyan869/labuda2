-- 000044_product_lifecycle_removal.down.sql
--
-- Restores the removed Product lifecycle columns. From-zero project; backfill
-- is not attempted (columns return with their creation defaults).

CREATE TYPE product_status_enum AS ENUM (
    'draft',
    'available',
    'sold',
    'withdrawn'
);

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS status product_status_enum DEFAULT 'draft'::product_status_enum NOT NULL;

ALTER TABLE products
    ADD COLUMN IF NOT EXISTS sold_at timestamp with time zone;

CREATE INDEX IF NOT EXISTS idx_products_status ON public.products USING btree (status);