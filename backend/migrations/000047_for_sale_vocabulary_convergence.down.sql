-- ============================================================
-- 000047_for_sale_vocabulary_convergence.down.sql
-- Stage C DOWN: restore the pre-000047 fixed-price vocabulary.
--
-- Inverse of the UP migration. Restores:
--   table            for_sales -> fixed_price_sales
--   status enum      for_sale_status_enum -> fixed_price_sale_status_enum
--   FK columns       for_sale_id / for_sale_source_id
--                    -> fixed_price_sale_id / fixed_price_sale_source_id
--   enum values      'for_sale' -> 'fixed_price_sale' (or 'listing' where
--                    that was the pre-existing value: order_source_enum,
--                    negotiation_resource_enum, discount_scope_enum,
--                    saved_items/discount_targets CHECK + backfill)
--   trigger          trg_for_sales_single_active_channel ->
--                    trg_fixed_price_sales_single_active_channel
--
-- The orphaned listing_* enum types are NOT restored by DOWN: they were
-- dead before 000047 (dropped table in 000010), and DOWN restores the
-- schema to the state required for replay of 000047, not to the full
-- pre-000010 state.
-- ============================================================

-- ------------------------------------------------------------
-- 1. Drop the cross-table trigger BEFORE the table rename.
-- ------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_for_sales_single_active_channel ON for_sales;
DROP TRIGGER IF EXISTS trg_auctions_single_active_channel ON auctions;

-- ------------------------------------------------------------
-- 2. Recreate the For Sale status enum with the old name.
--    Drop the partial unique index whose predicate references the
--    status enum BEFORE the type swap; recreate it after (section 6).
-- ------------------------------------------------------------
DROP INDEX IF EXISTS uniq_active_for_sale_per_product;

CREATE TYPE fixed_price_sale_status_enum AS ENUM ('draft', 'active', 'sold', 'withdrawn');

ALTER TABLE for_sales
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE fixed_price_sale_status_enum
    USING status::text::fixed_price_sale_status_enum,
    ALTER COLUMN status SET DEFAULT 'draft'::fixed_price_sale_status_enum;

DROP TYPE IF EXISTS for_sale_status_enum;

-- ------------------------------------------------------------
-- 3. Recreate the enum types with the old values.
-- ------------------------------------------------------------

-- 3a. order_source_enum: 'for_sale' -> 'fixed_price_sale', restore 'listing'
CREATE TYPE order_source_enum_old AS ENUM ('listing', 'seller_quote', 'auction', 'negotiation', 'fixed_price_sale');

ALTER TABLE orders
    ALTER COLUMN source_type TYPE order_source_enum_old
    USING source_type::text::order_source_enum_old;

DROP TYPE IF EXISTS order_source_enum;
ALTER TYPE order_source_enum_old RENAME TO order_source_enum;

-- 3b. sale_surface_type_enum: 'for_sale' -> 'fixed_price_sale'
CREATE TYPE sale_surface_type_enum_old AS ENUM ('fixed_price_sale', 'auction', 'negotiation');

ALTER TABLE pricing_tokens
    ALTER COLUMN source_type TYPE sale_surface_type_enum_old
    USING source_type::text::sale_surface_type_enum_old;

ALTER TABLE shipping_quotes
    ALTER COLUMN source_type TYPE sale_surface_type_enum_old
    USING source_type::text::sale_surface_type_enum_old;

DROP TYPE IF EXISTS sale_surface_type_enum;
ALTER TYPE sale_surface_type_enum_old RENAME TO sale_surface_type_enum;

-- 3c. negotiation_resource_enum: 'for_sale' -> 'fixed_price_sale', restore 'listing'
CREATE TYPE negotiation_resource_enum_old AS ENUM ('listing', 'auction', 'fixed_price_sale');

ALTER TABLE negotiation_sessions
    ALTER COLUMN resource_type TYPE negotiation_resource_enum_old
    USING resource_type::text::negotiation_resource_enum_old;

DROP TYPE IF EXISTS negotiation_resource_enum;
ALTER TYPE negotiation_resource_enum_old RENAME TO negotiation_resource_enum;

-- 3d. moderation_resource_enum: 'for_sale' -> 'fixed_price_sale'
CREATE TYPE moderation_resource_enum_old AS ENUM ('content', 'comment', 'fixed_price_sale', 'auction', 'user', 'chat_message');

ALTER TABLE moderation_cases
    ALTER COLUMN resource_type TYPE moderation_resource_enum_old
    USING resource_type::text::moderation_resource_enum_old;

DROP TYPE IF EXISTS moderation_resource_enum;
ALTER TYPE moderation_resource_enum_old RENAME TO moderation_resource_enum;

-- 3e. discount_scope_enum: 'for_sale' -> 'listing'
CREATE TYPE discount_scope_enum_old AS ENUM ('listing', 'auction', 'both');

ALTER TABLE discounts
    ALTER COLUMN scope TYPE discount_scope_enum_old
    USING scope::text::discount_scope_enum_old;

DROP TYPE IF EXISTS discount_scope_enum;
ALTER TYPE discount_scope_enum_old RENAME TO discount_scope_enum;

-- 3f. chat_commerce_reference_target_type_enum: 'for_sale' -> 'fixed_price_sale'
CREATE TYPE chat_commerce_reference_target_type_enum_old AS ENUM ('fixed_price_sale', 'auction');

ALTER TABLE chat_commerce_references
    ALTER COLUMN target_type TYPE chat_commerce_reference_target_type_enum_old
    USING target_type::text::chat_commerce_reference_target_type_enum_old;

DROP TYPE IF EXISTS chat_commerce_reference_target_type_enum;
ALTER TYPE chat_commerce_reference_target_type_enum_old RENAME TO chat_commerce_reference_target_type_enum;

-- ------------------------------------------------------------
-- 4. Drop canonical CHECK constraints and partial indexes, then
--    rename FK columns back.
-- ------------------------------------------------------------

-- 4a. negotiation_sessions.for_sale_id -> fixed_price_sale_id
ALTER TABLE negotiation_sessions RENAME COLUMN for_sale_id TO fixed_price_sale_id;
ALTER INDEX idx_negotiation_sessions_for_sale_id RENAME TO idx_negotiation_sessions_fixed_price_sale_id;

-- 4b. comment_commerce_references.for_sale_id -> fixed_price_sale_id
ALTER TABLE comment_commerce_references DROP CONSTRAINT IF EXISTS comment_commerce_reference_exactly_one_source;
ALTER TABLE comment_commerce_references RENAME COLUMN for_sale_id TO fixed_price_sale_id;
DROP INDEX IF EXISTS idx_comment_commerce_ref_for_sale;

-- 4c. content_resource_occurrences.for_sale_source_id -> fixed_price_sale_source_id
ALTER TABLE content_resource_occurrences
    DROP CONSTRAINT IF EXISTS content_resource_occurrence_exactly_one_source;
ALTER TABLE content_resource_occurrences
    DROP CONSTRAINT IF EXISTS content_resource_occurrence_operation_compatibility;
ALTER TABLE content_resource_occurrences RENAME COLUMN for_sale_source_id TO fixed_price_sale_source_id;
DROP INDEX IF EXISTS idx_content_resource_occurrences_for_sale_source;

-- 4d. chat_message_resource_occurrences.for_sale_source_id -> fixed_price_sale_source_id
ALTER TABLE chat_message_resource_occurrences
    DROP CONSTRAINT IF EXISTS chat_occurrence_exactly_one_source;
ALTER TABLE chat_message_resource_occurrences
    DROP CONSTRAINT IF EXISTS chat_occurrence_valid_operation;
ALTER TABLE chat_message_resource_occurrences RENAME COLUMN for_sale_source_id TO fixed_price_sale_source_id;
DROP INDEX IF EXISTS idx_chat_occurrence_for_sale_src;

-- ------------------------------------------------------------
-- 5. Rename the table back.
-- ------------------------------------------------------------
ALTER TABLE for_sales RENAME TO fixed_price_sales;

ALTER INDEX for_sales_pkey RENAME TO fixed_price_sales_pkey;
ALTER INDEX idx_for_sales_product_id RENAME TO idx_fixed_price_sales_product_id;
ALTER INDEX idx_for_sales_seller_id RENAME TO idx_fixed_price_sales_seller_id;
ALTER INDEX idx_for_sales_status RENAME TO idx_fixed_price_sales_status;

ALTER TABLE fixed_price_sales RENAME CONSTRAINT for_sales_product_id_fkey TO fixed_price_sales_product_id_fkey;
ALTER TABLE fixed_price_sales RENAME CONSTRAINT for_sales_seller_id_fkey TO fixed_price_sales_seller_id_fkey;
ALTER TABLE fixed_price_sales RENAME CONSTRAINT for_sales_price_per_unit_check TO fixed_price_sales_price_per_unit_check;
ALTER TABLE fixed_price_sales RENAME CONSTRAINT for_sales_quantity_available_nonnegative TO fixed_price_sales_quantity_available_nonnegative;

ALTER TABLE negotiation_sessions RENAME CONSTRAINT fk_negotiation_sessions_for_sale TO fk_negotiation_sessions_fixed_price_sale;
ALTER TABLE comment_commerce_references
    RENAME CONSTRAINT comment_commerce_references_for_sale_id_fkey TO comment_commerce_references_fixed_price_sale_id_fkey;
ALTER TABLE content_resource_occurrences
    RENAME CONSTRAINT content_resource_occurrences_for_sale_source_id_fkey TO content_resource_occurrences_fixed_price_sale_source_id_fkey;
ALTER TABLE chat_message_resource_occurrences
    RENAME CONSTRAINT chat_message_resource_occurrences_for_sale_source_id_fkey TO chat_message_resource_occurrenc_fixed_price_sale_source_id_fkey;

-- ------------------------------------------------------------
-- 6. Recreate old partial indexes and CHECK constraints.
-- ------------------------------------------------------------
CREATE UNIQUE INDEX uniq_active_fixed_price_sale_per_product
    ON public.fixed_price_sales USING btree (product_id)
    WHERE (status = ANY (ARRAY['draft'::fixed_price_sale_status_enum, 'active'::fixed_price_sale_status_enum]));

CREATE INDEX idx_comment_commerce_ref_fps
    ON comment_commerce_references(fixed_price_sale_id)
    WHERE fixed_price_sale_id IS NOT NULL;

CREATE INDEX idx_content_resource_occurrences_fps_source
    ON content_resource_occurrences (fixed_price_sale_source_id, created_at DESC)
    WHERE fixed_price_sale_source_id IS NOT NULL;

CREATE INDEX idx_chat_occurrence_fps_src
    ON chat_message_resource_occurrences (fixed_price_sale_source_id, created_at DESC)
    WHERE fixed_price_sale_source_id IS NOT NULL;

ALTER TABLE comment_commerce_references
    ADD CONSTRAINT comment_commerce_reference_exactly_one_source
    CHECK (
        (fixed_price_sale_id IS NOT NULL AND auction_id IS NULL)
        OR
        (fixed_price_sale_id IS NULL AND auction_id IS NOT NULL)
    );

ALTER TABLE content_resource_occurrences
    ADD CONSTRAINT content_resource_occurrence_exactly_one_source
    CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    );

ALTER TABLE content_resource_occurrences
    ADD CONSTRAINT content_resource_occurrence_operation_compatibility
    CHECK (
        operation = 'share_to_feed'
        OR (
            operation = 'direct_commerce_insert_content'
            AND (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL)
        )
    );

ALTER TABLE chat_message_resource_occurrences
    ADD CONSTRAINT chat_occurrence_exactly_one_source
    CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    );

ALTER TABLE chat_message_resource_occurrences
    ADD CONSTRAINT chat_occurrence_valid_operation
    CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    );

-- ------------------------------------------------------------
-- 7. Recreate the trigger with the old table name.
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION enforce_single_active_sale_channel_per_product()
RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'fixed_price_sales' THEN
        IF NEW.status IN ('draft', 'active') THEN
            IF EXISTS (
                SELECT 1 FROM auctions
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
            ) THEN
                RAISE EXCEPTION 'product % already has an active auction; cannot activate a fixed-price sale for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'auctions' THEN
        IF NEW.status IN ('draft', 'scheduled', 'active', 'waiting_settlement') THEN
            IF EXISTS (
                SELECT 1 FROM fixed_price_sales
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'active')
            ) THEN
                RAISE EXCEPTION 'product % already has an active fixed-price sale; cannot activate an auction for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_fixed_price_sales_single_active_channel
    BEFORE INSERT OR UPDATE ON fixed_price_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

CREATE TRIGGER trg_auctions_single_active_channel
    BEFORE INSERT OR UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

-- ------------------------------------------------------------
-- 8. Restore persisted discriminator backfills.
-- ------------------------------------------------------------

-- 8a. saved_items.target_type CHECK + backfill
ALTER TABLE saved_items DROP CONSTRAINT IF EXISTS saved_items_target_type_check;
UPDATE saved_items SET target_type = 'listing' WHERE target_type = 'for_sale';
ALTER TABLE saved_items
    ADD CONSTRAINT saved_items_target_type_check
    CHECK ((target_type = ANY (ARRAY['listing'::text, 'auction'::text])));

-- 8b. discount_targets.target_type CHECK + backfill
ALTER TABLE discount_targets DROP CONSTRAINT IF EXISTS discount_targets_target_type_check;
UPDATE discount_targets SET target_type = 'listing' WHERE target_type = 'for_sale';
ALTER TABLE discount_targets
    ADD CONSTRAINT discount_targets_target_type_check
    CHECK ((target_type = ANY (ARRAY['listing'::text, 'auction'::text])));

-- 8c. promotion_instances / promotion_events (plain text)
UPDATE promotion_instances SET target_type = 'fixed_price_sale' WHERE target_type = 'for_sale';
UPDATE promotion_events SET target_type = 'fixed_price_sale' WHERE target_type = 'for_sale';
UPDATE promotion_events SET surface = 'fixed_price_sale' WHERE surface = 'for_sale';

-- 8d. promotion_packages.allowed_target_types (text[])
UPDATE promotion_packages
SET allowed_target_types = array_replace(allowed_target_types, 'for_sale', 'fixed_price_sale')
WHERE 'for_sale' = ANY (allowed_target_types);
