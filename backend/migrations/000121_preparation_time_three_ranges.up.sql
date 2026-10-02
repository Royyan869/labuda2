-- ============================================================
-- 000121 PREPARATION TIME → 3 RANGES — owner decision 2026-10-02
--
-- Canonical preparation time is now exactly THREE ranges
-- (forsale/entity/preparation_time.go + product/entity/product_validation.go):
--
--     1_3_days | 4_7_days | 8_15_days      (default: 1_3_days)
--
-- Business truth: preparation time is WHEN THE SELLER CAN SHIP AFTER CHECKOUT
-- (koi sometimes need karantina before transport). It tells the buyer the
-- maximum time (upper bound) the seller needs; order.ReadyToShipBy maps
-- 1_3_days=3, 4_7_days=7, 8_15_days=15 (unknown → 3).
--
-- Consumers (unchanged since 000103):
--   products.preparation_time        (NOT NULL) — canonical seller declaration
--   orders.preparation_time_snapshot (NULLABLE) — creation-time freeze
-- auctions.preparation_time was dropped by 000046; listings by 000010.
--
-- Data mapping never SHORTENS an existing promise:
--   immediate (0d), short (1-2d) → 1_3_days   (fits within 3)
--   medium (3-5d)                → 4_7_days   (fits within 7)
--   long (7+d, uncapped)         → 8_15_days  (longest available range)
--
-- FAIL-SAFE: refuses to run — and rewrites NO data — if any live products or
-- orders row holds a value outside the four pre-000121 canonical values.
-- Historical data needs an Owner/business decision; do NOT convert/delete rows
-- or delete the guard to force it through.
--
-- 000001 is the immutable baseline snapshot; it is intentionally left untouched.
-- ============================================================

DO $$
DECLARE
    unknown_products bigint;
    unknown_orders   bigint;
BEGIN
    SELECT COUNT(*) INTO unknown_products
    FROM products
    WHERE preparation_time::text NOT IN ('immediate', 'short', 'medium', 'long');

    SELECT COUNT(*) INTO unknown_orders
    FROM orders
    WHERE preparation_time_snapshot IS NOT NULL
      AND preparation_time_snapshot::text NOT IN ('immediate', 'short', 'medium', 'long');

    IF unknown_products + unknown_orders > 0 THEN
        RAISE EXCEPTION
            'preparation_time_enum holds % products row(s) and % orders row(s) with values outside immediate/short/medium/long; refusing to re-range the enum. Resolve the historical data before applying 000121.',
            unknown_products, unknown_orders
            USING ERRCODE = 'check_violation';
    END IF;
END
$$;

CREATE TYPE preparation_time_enum_new AS ENUM ('1_3_days', '4_7_days', '8_15_days');

ALTER TABLE products
    ALTER COLUMN preparation_time TYPE preparation_time_enum_new
    USING (
        CASE preparation_time::text
            WHEN 'medium' THEN '4_7_days'
            WHEN 'long'   THEN '8_15_days'
            ELSE '1_3_days'  -- immediate, short (and defensive default)
        END
    )::preparation_time_enum_new;

-- NULL snapshot must stay NULL (do not coerce into the default range).
ALTER TABLE orders
    ALTER COLUMN preparation_time_snapshot TYPE preparation_time_enum_new
    USING (
        CASE
            WHEN preparation_time_snapshot IS NULL THEN NULL
            WHEN preparation_time_snapshot::text = 'medium' THEN '4_7_days'
            WHEN preparation_time_snapshot::text = 'long'   THEN '8_15_days'
            ELSE '1_3_days'  -- immediate, short (and defensive default)
        END
    )::preparation_time_enum_new;

DROP TYPE IF EXISTS preparation_time_enum;
ALTER TYPE preparation_time_enum_new RENAME TO preparation_time_enum;
