-- SCOPE 3 FOLLOW-UP: retire the dormant for_sale_type column.
--
-- The ForSaleType concept was purged from the Go domain in commit c9ab974:
-- no code reads or writes this column anymore (entity comment in
-- for_sale.go confirms "column is no longer read"). The column remains in
-- the schema only as a resurrection breadcrumb — agents may mistake it for
-- an authority and rebuild the concept on top of it.
--
-- Owner decision (2026-09-25): drop it.
-- Safe because: zero application references (verified by residue sweep);
-- guard test migration_authority_guard_test.go pins .env.example shape, not
-- schema; for_sale list/detail/read paths were already re-hydrated from
-- products JOIN and do not select this column.

ALTER TABLE for_sales DROP COLUMN IF EXISTS for_sale_type;
