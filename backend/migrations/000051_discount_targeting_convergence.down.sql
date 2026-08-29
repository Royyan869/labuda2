-- ============================================================
-- 000051_discount_targeting_convergence.down.sql
-- Rollback discount schema convergence.
--
-- NOTE: This rollback restores the OLD schema which is NOT
-- canonical. It exists only for migration chain integrity.
-- ============================================================

-- 1. Restore free_shipping in discount_type_enum
CREATE TYPE discount_type_enum_old AS ENUM ('percentage', 'flat_amount', 'free_shipping');

ALTER TABLE discounts
    ALTER COLUMN type DROP DEFAULT,
    ALTER COLUMN type TYPE discount_type_enum_old
    USING type::text::discount_type_enum_old,
    ALTER COLUMN type SET DEFAULT 'percentage'::discount_type_enum_old;

ALTER TABLE orders
    ALTER COLUMN discount_type DROP DEFAULT,
    ALTER COLUMN discount_type TYPE discount_type_enum_old
    USING discount_type::text::discount_type_enum_old,
    ALTER COLUMN discount_type SET DEFAULT 'percentage'::discount_type_enum_old;

DROP TYPE discount_type_enum;
ALTER TYPE discount_type_enum_old RENAME TO discount_type_enum;

-- 2. Rename applies_to → scope
ALTER TABLE discounts RENAME COLUMN applies_to TO scope;

-- Restore original enum name
ALTER TYPE discount_applies_to_enum RENAME TO discount_scope_enum;

-- 3. Restore rejected columns
ALTER TABLE discounts ADD COLUMN target_mode text DEFAULT 'seller_wide'::text NOT NULL;
ALTER TABLE discounts ADD COLUMN valid_from timestamp with time zone NOT NULL DEFAULT '1970-01-01T00:00:00Z'::timestamptz;
ALTER TABLE discounts ADD COLUMN max_usage_per_user integer DEFAULT 0 NOT NULL;
ALTER TABLE discounts ADD COLUMN max_discount numeric;

-- 4. Restore discount_targets table
CREATE TABLE discount_targets (
    id uuid DEFAULT gen_random_uuid() NOT NULL PRIMARY KEY,
    discount_id uuid NOT NULL REFERENCES discounts(id) ON DELETE CASCADE,
    for_sale_id uuid,
    auction_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

-- 5. Restore old index
DROP INDEX IF EXISTS idx_discounts_is_active;
CREATE INDEX idx_discounts_is_active ON public.discounts USING btree (is_active, valid_from, valid_until) WHERE (is_active = true);
