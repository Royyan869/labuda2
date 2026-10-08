-- 000129_purge_legacy_promotion_funding (down)
--
-- Intentionally irreversible: promotion_funding is dead schema that references
-- the purged `promotions` aggregate. Recreating it would resurrect a table with
-- no writer and a dangling foreign key. Do not restore it.
SELECT 1;
