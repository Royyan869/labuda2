-- Down migration for 000112.
--
-- NOTE: this restores the COLUMN SHAPE only. The ForSaleType concept is
-- purged from the domain (commit c9ab974) — recreating the column does not
-- resurrect the concept, and historical per-row type values are NOT
-- recoverable (dropping a column discards its data by design).
--
-- If a rollback is ever executed here, the application keeps working: no
-- code reads or writes for_sale_type.

ALTER TABLE for_sales ADD COLUMN IF NOT EXISTS for_sale_type text;
