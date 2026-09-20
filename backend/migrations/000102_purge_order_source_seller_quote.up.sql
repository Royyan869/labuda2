-- ============================================================
-- 000102 PURGE DEAD order_source_enum VALUE 'seller_quote' — ORDER-A3-A
--
-- Canonical OrderSourceType (backend/internal/commerce/order/entity/
-- order_source_type.go) permits exactly three values:
--
--     for_sale | auction | negotiation
--
-- 'seller_quote' was retained when 000047 recreated order_source_enum, but no
-- production producer can write it:
--   - order creation copies the pricing token's sale_surface_type_enum
--     (for_sale | auction | negotiation) into orders.source_type
--   - the fixed-price checkout path rejects any source that is not for_sale
--   - no Go constant, DTO, worker, migration, or fixture produces it
--   - orders.source_type is the ONLY consumer of this enum type
--
-- 'seller_quote' is therefore obsolete schema residue: a dangling value that
-- invites the rejected per-order "seller quote" source design back. It is NOT
-- the auctions.seller_quote_provided flag (a distinct, live concept).
--
-- FAIL-SAFE: this migration refuses to run — and rewrites NO data — if any live
-- orders row still holds 'seller_quote'. If that guard fires, the value is
-- historical data and narrowing needs an Owner/business decision; do NOT delete
-- the guard to force it through.
--
-- 000001 is the immutable baseline snapshot and 000047 records the enum's
-- history; both are intentionally left untouched.
-- ============================================================

DO $$
DECLARE
    stale_count bigint;
BEGIN
    SELECT COUNT(*) INTO stale_count
    FROM orders
    WHERE source_type::text = 'seller_quote';

    IF stale_count > 0 THEN
        RAISE EXCEPTION
            'order_source_enum.seller_quote is still used by % live orders row(s); refusing to narrow the enum. Resolve the historical data before applying 000102.',
            stale_count
            USING ERRCODE = 'check_violation';
    END IF;
END
$$;

CREATE TYPE order_source_enum_new AS ENUM ('for_sale', 'auction', 'negotiation');

ALTER TABLE orders
    ALTER COLUMN source_type TYPE order_source_enum_new
    USING source_type::text::order_source_enum_new;

DROP TYPE IF EXISTS order_source_enum;
ALTER TYPE order_source_enum_new RENAME TO order_source_enum;
