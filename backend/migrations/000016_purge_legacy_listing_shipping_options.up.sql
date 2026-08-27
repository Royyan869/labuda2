-- Purge dead listing_shipping_options table.
-- All listing/auction shipping links now use product_shipping_options exclusively.
-- No backend or mobile code references this table.
-- Verified at: 000001_canonical_schema lines 929-933 (table), 1890 (PK), 2343-2344 (FKs).
DROP TABLE IF EXISTS listing_shipping_options;
