-- IRREVERSIBLE: This migration purges the dead listing_shipping_options table.
-- All listing/auction shipping links now use product_shipping_options exclusively.
-- Recreating the legacy table would reintroduce rejected authority.
-- There is no safe down-migration path.
-- This file exists only to satisfy the migration framework.
-- No-op: the purge cannot and should not be reversed.
SELECT 1;
