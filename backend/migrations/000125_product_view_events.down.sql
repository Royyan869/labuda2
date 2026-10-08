-- 000125_product_view_events (down)
--
-- Removes the canonical Product View authority. The pre-refactor
-- `listing_views`/`listings` schema is intentionally NOT recreated.

DROP INDEX IF EXISTS product_view_events_product_id_time_idx;
DROP TABLE IF EXISTS product_view_events;
