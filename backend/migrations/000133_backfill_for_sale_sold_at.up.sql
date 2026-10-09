-- Backfill missing for_sales.sold_at transition timestamps.
--
-- Older sold rows were created without SoldAt even though the lifecycle
-- transition had already occurred. updated_at is the closest canonical
-- transition signal available for those rows.
--
-- Going forward, ForSale.ReduceQuantity stamps SoldAt atomically with the
-- active -> sold transition.

UPDATE for_sales
SET sold_at = updated_at
WHERE status = 'sold' AND sold_at IS NULL;
