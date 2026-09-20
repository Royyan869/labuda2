-- ============================================================
-- 000103 PURGE LEGACY preparation_time_enum VALUES — ORDER-A3-B
--
-- Canonical preparation time (backend/internal/commerce/forsale/entity/
-- preparation_time.go + product/entity/product_validation.go) is exactly:
--
--     immediate | short | medium | long
--
-- The enum created in 000001 also carried a legacy set:
--
--     same_day | 1_2_days | 3_5_days | 6_10_days | 11_15_days | more_than_15_days
--
-- The legacy values have NO production producer:
--   - for_sale/auction HTTP handlers bind preparation_time with
--     `oneof=immediate short medium long`
--   - product.ValidatePreparationTime rejects everything else
--   - the only writers are the HTTP DTOs and cmd/feed_external_fixture (canonical)
--   - no seed, worker, admin, or migration writes a legacy value
--
-- Effective consumers of this enum (verified against the live schema):
--   products.preparation_time        (NOT NULL)  — canonical seller declaration
--   orders.preparation_time_snapshot (NULLABLE)  — creation-time freeze from Product
-- auctions.preparation_time was dropped by 000046; listings (and its column) by
-- 000010. Neither is a live consumer.
--
-- SEMANTIC SAFETY: order.preparationTimeToDays maps only the four canonical
-- values (immediate=1, short=2, medium=5, long=7) and its default branch (2) is a
-- defensive fallback for empty/unknown input — it is NOT a legacy-value
-- dependency. Narrowing the enum cannot change any produced value's meaning.
--
-- FAIL-SAFE: this migration refuses to run — and rewrites NO data — if any live
-- products or orders row still holds a legacy value. If that guard fires the
-- value is historical data and narrowing needs an Owner/business decision; do
-- NOT convert/delete rows or delete the guard to force it through.
--
-- 000001 is the immutable baseline snapshot; it is intentionally left untouched.
-- ============================================================

DO $$
DECLARE
    legacy_products bigint;
    legacy_orders   bigint;
BEGIN
    SELECT COUNT(*) INTO legacy_products
    FROM products
    WHERE preparation_time::text IN
        ('same_day', '1_2_days', '3_5_days', '6_10_days', '11_15_days', 'more_than_15_days');

    SELECT COUNT(*) INTO legacy_orders
    FROM orders
    WHERE preparation_time_snapshot::text IN
        ('same_day', '1_2_days', '3_5_days', '6_10_days', '11_15_days', 'more_than_15_days');

    IF legacy_products + legacy_orders > 0 THEN
        RAISE EXCEPTION
            'preparation_time_enum legacy values still used by % products row(s) and % orders row(s); refusing to narrow the enum. Resolve the historical data before applying 000103.',
            legacy_products, legacy_orders
            USING ERRCODE = 'check_violation';
    END IF;
END
$$;

CREATE TYPE preparation_time_enum_new AS ENUM ('immediate', 'short', 'medium', 'long');

ALTER TABLE products
    ALTER COLUMN preparation_time TYPE preparation_time_enum_new
    USING preparation_time::text::preparation_time_enum_new;

ALTER TABLE orders
    ALTER COLUMN preparation_time_snapshot TYPE preparation_time_enum_new
    USING preparation_time_snapshot::text::preparation_time_enum_new;

DROP TYPE IF EXISTS preparation_time_enum;
ALTER TYPE preparation_time_enum_new RENAME TO preparation_time_enum;
