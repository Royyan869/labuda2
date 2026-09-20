-- Down migration: restore order_source_enum with the historical 'seller_quote'
-- value for chain integrity only (the 000047 shape: for_sale, seller_quote,
-- auction, negotiation).
--
-- WARNING: 'seller_quote' is OBSOLETE — no production producer can create it.
-- Canonical OrderSourceType is for_sale | auction | negotiation. Restored only
-- so `migrate down` reproduces the pre-000102 state.

CREATE TYPE order_source_enum_restore AS ENUM ('for_sale', 'seller_quote', 'auction', 'negotiation');

ALTER TABLE orders
    ALTER COLUMN source_type TYPE order_source_enum_restore
    USING source_type::text::order_source_enum_restore;

DROP TYPE IF EXISTS order_source_enum;
ALTER TYPE order_source_enum_restore RENAME TO order_source_enum;
