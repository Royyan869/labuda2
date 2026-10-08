-- 000125_product_view_events
--
-- CANONICAL PRODUCT VIEW AUTHORITY.
--
-- 1 Product View = one successful open of a product DETAIL page by a viewer
-- entitled to see the product. This table is the SINGLE canonical record for
-- Product View. It is an immutable, append-only event log. There is NO mutable
-- view_count counter on products / for_sales / auctions.
--
-- Identity: the view attaches to the canonical PRODUCT (products.id), never to
-- a For Sale or Auction row. Both selling surfaces resolve to the same Product
-- identity (products.selling_surface = 'for_sale' | 'auction'), so relist or
-- selling-surface reuse never mints a second view authority.
--
-- Viewer: viewer_user_id is the authenticated viewer, or NULL for an anonymous
-- viewer. Seller self-views and admin/moderator views are excluded at the
-- producer and are never written here.
--
-- Dedup: none. Every valid open is a distinct row (no per-user / per-day /
-- per-session uniqueness). Seller Analytics later aggregates
-- COUNT(*) GROUP BY product_id.
--
-- viewed_at is the server clock (DB now()), never a client timestamp.
--
-- The pre-refactor `listing_views` table is dead and must stay dead. This is
-- NOT a revival of that schema.

CREATE TABLE IF NOT EXISTS product_view_events (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    viewer_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    viewed_at timestamptz NOT NULL DEFAULT now()
);

-- Seller Analytics aggregate access path: COUNT per product over time.
CREATE INDEX IF NOT EXISTS product_view_events_product_id_time_idx
    ON product_view_events (product_id, viewed_at DESC);
