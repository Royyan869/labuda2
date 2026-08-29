-- ============================================================
-- 000051_discount_targeting_convergence.up.sql
-- Converge discounts schema to canonical seller discount model.
--
-- This migration was rewritten because the original version
-- attempted to ADD min_purchase (which already exists) and
-- failed. The 000048_discount_canonical_convergence was also
-- broken (dropped min_purchase, which is canonical) and had a
-- version collision with 000048_product_selling_surface_exclusivity.
--
-- CHANGES:
--   1. Remove free_shipping from discount_type_enum
--   2. Rename scope → applies_to (column + enum)
--   3. Drop rejected columns: target_mode, valid_from, max_usage_per_user, max_discount
--   4. Drop discount_targets table
--   5. Fix idx_discounts_is_active (was referencing dropped valid_from)
-- ============================================================

-- 1. Remove free_shipping from discount_type_enum
--    (destructive recreate required for PostgreSQL enums)
CREATE TYPE discount_type_enum_new AS ENUM ('percentage', 'flat_amount');

-- Alter discounts.type column to use new enum type
ALTER TABLE discounts
    ALTER COLUMN type DROP DEFAULT,
    ALTER COLUMN type TYPE discount_type_enum_new
    USING type::text::discount_type_enum_new,
    ALTER COLUMN type SET DEFAULT 'percentage'::discount_type_enum_new;

-- Alter orders.discount_type column (DEAD but column still references the enum)
ALTER TABLE orders
    ALTER COLUMN discount_type DROP DEFAULT,
    ALTER COLUMN discount_type TYPE discount_type_enum_new
    USING discount_type::text::discount_type_enum_new,
    ALTER COLUMN discount_type SET DEFAULT 'percentage'::discount_type_enum_new;

DROP TYPE discount_type_enum;
ALTER TYPE discount_type_enum_new RENAME TO discount_type_enum;

-- 2. Rename scope → applies_to
--    The code expects 'applies_to' column name.
ALTER TABLE discounts RENAME COLUMN scope TO applies_to;

-- Rename the enum type too (optional but keeps naming consistent)
ALTER TYPE discount_scope_enum RENAME TO discount_applies_to_enum;

-- 3. Drop rejected columns
ALTER TABLE discounts DROP COLUMN IF EXISTS target_mode;
ALTER TABLE discounts DROP COLUMN IF EXISTS valid_from;
ALTER TABLE discounts DROP COLUMN IF EXISTS max_usage_per_user;
ALTER TABLE discounts DROP COLUMN IF EXISTS max_discount;

-- 4. Drop discount_targets table (specific-item targeting is rejected)
DROP TABLE IF EXISTS discount_targets;

-- 5. Fix idx_discounts_is_active
--    The old index referenced valid_from which was just dropped.
DROP INDEX IF EXISTS idx_discounts_is_active;
CREATE INDEX idx_discounts_is_active ON public.discounts USING btree (is_active, valid_until) WHERE (is_active = true);

-- 6. Ensure min_purchase is canonical
--    min_purchase already exists in the schema. Verify NOT NULL + DEFAULT 0.
ALTER TABLE discounts ALTER COLUMN min_purchase SET NOT NULL;
ALTER TABLE discounts ALTER COLUMN min_purchase SET DEFAULT 0;
