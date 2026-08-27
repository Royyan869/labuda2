-- ============================================================
-- 000047_for_sale_vocabulary_convergence.up.sql
-- Stage C: converge database vocabulary to canonical For Sale naming.
--
-- Canonical vocabulary:
--   table            fixed_price_sales   -> for_sales
--   status enum      fixed_price_sale_status_enum -> for_sale_status_enum
--   FK columns       fixed_price_sale_id / fixed_price_sale_source_id
--                    -> for_sale_id / for_sale_source_id
--   enum values      'fixed_price_sale' -> 'for_sale'
--                    'listing' (as the For Sale selling surface) -> 'for_sale'
--   wire discriminators persisted in text columns -> 'for_sale'
--
-- NOT renamed (per doctrine): products / product_id, auctions / auction_*,
-- order_items.product_id, financial/ledger/escrow tables.
--
-- All discriminator tables are empty at apply time (verified against the
-- live dev DB: 0 rows in every table touched by the backfill statements).
-- Backfills are therefore deterministic no-ops that keep the migration
-- correct if data ever exists during a replay.
--
-- The enum values are converged via destructive type recreation (create
-- new type -> alter columns -> drop old type). This avoids PG < 12
-- `ALTER TYPE ... RENAME VALUE` and keeps the migration fully
-- transactional (no enum value additions, so the executor runs it in
-- a single transaction).
-- ============================================================

-- ------------------------------------------------------------
-- 1. Drop the cross-table trigger BEFORE the table rename (the
--    trigger function body references fixed_price_sales by name).
-- ------------------------------------------------------------
DROP TRIGGER IF EXISTS trg_fixed_price_sales_single_active_channel ON fixed_price_sales;
DROP TRIGGER IF EXISTS trg_auctions_single_active_channel ON auctions;

-- ------------------------------------------------------------
-- 2. Rename the status enum for the For Sale table.
--    Drop the partial unique index whose predicate references the
--    status enum BEFORE the type swap; recreate it after (section 6).
-- ------------------------------------------------------------
DROP INDEX IF EXISTS uniq_active_fixed_price_sale_per_product;

CREATE TYPE for_sale_status_enum AS ENUM ('draft', 'active', 'sold', 'withdrawn');

ALTER TABLE fixed_price_sales
    ALTER COLUMN status DROP DEFAULT,
    ALTER COLUMN status TYPE for_sale_status_enum
    USING status::text::for_sale_status_enum,
    ALTER COLUMN status SET DEFAULT 'draft'::for_sale_status_enum;

DROP TYPE IF EXISTS fixed_price_sale_status_enum;

-- ------------------------------------------------------------
-- 3. Recreate the enum types whose values change. Destructive
--    recreate: new type -> alter each consumer column -> drop old.
-- ------------------------------------------------------------

-- 3a. order_source_enum: drop dead 'listing', rename 'fixed_price_sale' -> 'for_sale'
CREATE TYPE order_source_enum_new AS ENUM ('for_sale', 'seller_quote', 'auction', 'negotiation');

ALTER TABLE orders
    ALTER COLUMN source_type TYPE order_source_enum_new
    USING source_type::text::order_source_enum_new;

DROP TYPE IF EXISTS order_source_enum;
ALTER TYPE order_source_enum_new RENAME TO order_source_enum;

-- 3b. sale_surface_type_enum: 'fixed_price_sale' -> 'for_sale'
--     consumers: pricing_tokens.source_type, shipping_quotes.source_type
CREATE TYPE sale_surface_type_enum_new AS ENUM ('for_sale', 'auction', 'negotiation');

ALTER TABLE pricing_tokens
    ALTER COLUMN source_type TYPE sale_surface_type_enum_new
    USING source_type::text::sale_surface_type_enum_new;

ALTER TABLE shipping_quotes
    ALTER COLUMN source_type TYPE sale_surface_type_enum_new
    USING source_type::text::sale_surface_type_enum_new;

DROP TYPE IF EXISTS sale_surface_type_enum;
ALTER TYPE sale_surface_type_enum_new RENAME TO sale_surface_type_enum;

-- 3c. negotiation_resource_enum: drop 'listing', 'fixed_price_sale' -> 'for_sale'
--     consumer: negotiation_sessions.resource_type
CREATE TYPE negotiation_resource_enum_new AS ENUM ('for_sale', 'auction');

ALTER TABLE negotiation_sessions
    ALTER COLUMN resource_type TYPE negotiation_resource_enum_new
    USING resource_type::text::negotiation_resource_enum_new;

DROP TYPE IF EXISTS negotiation_resource_enum;
ALTER TYPE negotiation_resource_enum_new RENAME TO negotiation_resource_enum;

-- 3d. moderation_resource_enum: 'fixed_price_sale' -> 'for_sale'
--     consumer: moderation_cases.resource_type
CREATE TYPE moderation_resource_enum_new AS ENUM ('content', 'comment', 'for_sale', 'auction', 'user', 'chat_message');

ALTER TABLE moderation_cases
    ALTER COLUMN resource_type TYPE moderation_resource_enum_new
    USING resource_type::text::moderation_resource_enum_new;

DROP TYPE IF EXISTS moderation_resource_enum;
ALTER TYPE moderation_resource_enum_new RENAME TO moderation_resource_enum;

-- 3e. discount_scope_enum: 'listing' -> 'for_sale'
--     consumer: discounts.scope
CREATE TYPE discount_scope_enum_new AS ENUM ('for_sale', 'auction', 'both');

ALTER TABLE discounts
    ALTER COLUMN scope TYPE discount_scope_enum_new
    USING scope::text::discount_scope_enum_new;

DROP TYPE IF EXISTS discount_scope_enum;
ALTER TYPE discount_scope_enum_new RENAME TO discount_scope_enum;

-- 3f. chat_commerce_reference_target_type_enum: 'fixed_price_sale' -> 'for_sale'
--     consumer: chat_commerce_references.target_type
CREATE TYPE chat_commerce_reference_target_type_enum_new AS ENUM ('for_sale', 'auction');

ALTER TABLE chat_commerce_references
    ALTER COLUMN target_type TYPE chat_commerce_reference_target_type_enum_new
    USING target_type::text::chat_commerce_reference_target_type_enum_new;

DROP TYPE IF EXISTS chat_commerce_reference_target_type_enum;
ALTER TYPE chat_commerce_reference_target_type_enum_new RENAME TO chat_commerce_reference_target_type_enum;

-- ------------------------------------------------------------
-- 4. Rename the FK columns on the four consumer tables.
--    The CHECK constraints that reference these columns must be
--    dropped first and recreated after (PG does not rewrite
--    constraint expressions on column rename).
-- ------------------------------------------------------------

-- 4a. negotiation_sessions.fixed_price_sale_id -> for_sale_id
ALTER TABLE negotiation_sessions RENAME COLUMN fixed_price_sale_id TO for_sale_id;
ALTER INDEX idx_negotiation_sessions_fixed_price_sale_id RENAME TO idx_negotiation_sessions_for_sale_id;

-- 4b. comment_commerce_references.fixed_price_sale_id -> for_sale_id
ALTER TABLE comment_commerce_references DROP CONSTRAINT IF EXISTS comment_commerce_reference_exactly_one_source;
ALTER TABLE comment_commerce_references RENAME COLUMN fixed_price_sale_id TO for_sale_id;
DROP INDEX IF EXISTS idx_comment_commerce_ref_fps;

-- 4c. content_resource_occurrences.fixed_price_sale_source_id -> for_sale_source_id
ALTER TABLE content_resource_occurrences
    DROP CONSTRAINT IF EXISTS content_resource_occurrence_exactly_one_source;
ALTER TABLE content_resource_occurrences
    DROP CONSTRAINT IF EXISTS content_resource_occurrence_operation_compatibility;
ALTER TABLE content_resource_occurrences RENAME COLUMN fixed_price_sale_source_id TO for_sale_source_id;
DROP INDEX IF EXISTS idx_content_resource_occurrences_fps_source;

-- 4d. chat_message_resource_occurrences.fixed_price_sale_source_id -> for_sale_source_id
ALTER TABLE chat_message_resource_occurrences
    DROP CONSTRAINT IF EXISTS chat_occurrence_exactly_one_source;
ALTER TABLE chat_message_resource_occurrences
    DROP CONSTRAINT IF EXISTS chat_occurrence_valid_operation;
ALTER TABLE chat_message_resource_occurrences RENAME COLUMN fixed_price_sale_source_id TO for_sale_source_id;
DROP INDEX IF EXISTS idx_chat_occurrence_fps_src;

-- ------------------------------------------------------------
-- 5. Rename the For Sale table.
--    The table's own constraints/indexes carry the old name and are
--    renamed explicitly; the FK constraints pointing AT the table
--    from the four consumer tables are recreated after the rename.
-- ------------------------------------------------------------
ALTER TABLE fixed_price_sales RENAME TO for_sales;

-- Table-owned constraints / indexes
ALTER INDEX fixed_price_sales_pkey RENAME TO for_sales_pkey;
ALTER INDEX idx_fixed_price_sales_product_id RENAME TO idx_for_sales_product_id;
ALTER INDEX idx_fixed_price_sales_seller_id RENAME TO idx_for_sales_seller_id;
ALTER INDEX idx_fixed_price_sales_status RENAME TO idx_for_sales_status;

ALTER TABLE for_sales RENAME CONSTRAINT fixed_price_sales_product_id_fkey TO for_sales_product_id_fkey;
ALTER TABLE for_sales RENAME CONSTRAINT fixed_price_sales_seller_id_fkey TO for_sales_seller_id_fkey;
ALTER TABLE for_sales RENAME CONSTRAINT fixed_price_sales_price_per_unit_check TO for_sales_price_per_unit_check;
ALTER TABLE for_sales RENAME CONSTRAINT fixed_price_sales_quantity_available_nonnegative TO for_sales_quantity_available_nonnegative;

-- FK constraints on consumer tables referencing the (renamed) table
ALTER TABLE negotiation_sessions RENAME CONSTRAINT fk_negotiation_sessions_fixed_price_sale TO fk_negotiation_sessions_for_sale;
ALTER TABLE comment_commerce_references
    RENAME CONSTRAINT comment_commerce_references_fixed_price_sale_id_fkey TO comment_commerce_references_for_sale_id_fkey;
ALTER TABLE content_resource_occurrences
    RENAME CONSTRAINT content_resource_occurrences_fixed_price_sale_source_id_fkey TO content_resource_occurrences_for_sale_source_id_fkey;
ALTER TABLE chat_message_resource_occurrences
    RENAME CONSTRAINT chat_message_resource_occurrenc_fixed_price_sale_source_id_fkey TO chat_message_resource_occurrences_for_sale_source_id_fkey;

-- ------------------------------------------------------------
-- 6. Recreate the dropped partial indexes and CHECK constraints
--    with canonical column names.
-- ------------------------------------------------------------
CREATE UNIQUE INDEX uniq_active_for_sale_per_product
    ON public.for_sales USING btree (product_id)
    WHERE (status = ANY (ARRAY['draft'::for_sale_status_enum, 'active'::for_sale_status_enum]));

CREATE INDEX idx_comment_commerce_ref_for_sale
    ON comment_commerce_references(for_sale_id)
    WHERE for_sale_id IS NOT NULL;

CREATE INDEX idx_content_resource_occurrences_for_sale_source
    ON content_resource_occurrences (for_sale_source_id, created_at DESC)
    WHERE for_sale_source_id IS NOT NULL;

CREATE INDEX idx_chat_occurrence_for_sale_src
    ON chat_message_resource_occurrences (for_sale_source_id, created_at DESC)
    WHERE for_sale_source_id IS NOT NULL;

ALTER TABLE comment_commerce_references
    ADD CONSTRAINT comment_commerce_reference_exactly_one_source
    CHECK (
        (for_sale_id IS NOT NULL AND auction_id IS NULL)
        OR
        (for_sale_id IS NULL AND auction_id IS NOT NULL)
    );

ALTER TABLE content_resource_occurrences
    ADD CONSTRAINT content_resource_occurrence_exactly_one_source
    CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN for_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    );

ALTER TABLE content_resource_occurrences
    ADD CONSTRAINT content_resource_occurrence_operation_compatibility
    CHECK (
        operation = 'share_to_feed'
        OR (
            operation = 'direct_commerce_insert_content'
            AND (for_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL)
        )
    );

ALTER TABLE chat_message_resource_occurrences
    ADD CONSTRAINT chat_occurrence_exactly_one_source
    CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN for_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    );

ALTER TABLE chat_message_resource_occurrences
    ADD CONSTRAINT chat_occurrence_valid_operation
    CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (for_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    );

-- ------------------------------------------------------------
-- 7. Recreate the cross-table single-active-channel trigger with
--    canonical table names (rule 9 invariant preserved).
-- ------------------------------------------------------------
CREATE OR REPLACE FUNCTION enforce_single_active_sale_channel_per_product()
RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'for_sales' THEN
        IF NEW.status IN ('draft', 'active') THEN
            IF EXISTS (
                SELECT 1 FROM auctions
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
            ) THEN
                RAISE EXCEPTION 'product % already has an active auction; cannot activate a for_sale for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'auctions' THEN
        IF NEW.status IN ('draft', 'scheduled', 'active', 'waiting_settlement') THEN
            IF EXISTS (
                SELECT 1 FROM for_sales
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'active')
            ) THEN
                RAISE EXCEPTION 'product % already has an active for_sale; cannot activate an auction for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_for_sales_single_active_channel
    BEFORE INSERT OR UPDATE ON for_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

CREATE TRIGGER trg_auctions_single_active_channel
    BEFORE INSERT OR UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

-- ------------------------------------------------------------
-- 8. Persisted text/array discriminator backfills. All tables are
--    empty at apply time; the UPDATEs are deterministic no-ops that
--    keep replay correct if data exists.
-- ------------------------------------------------------------

-- 8a. saved_items.target_type CHECK + backfill
ALTER TABLE saved_items DROP CONSTRAINT IF EXISTS saved_items_target_type_check;
UPDATE saved_items SET target_type = 'for_sale' WHERE target_type = 'listing';
ALTER TABLE saved_items
    ADD CONSTRAINT saved_items_target_type_check
    CHECK ((target_type = ANY (ARRAY['for_sale'::text, 'auction'::text])));

-- 8b. discount_targets.target_type CHECK + backfill
ALTER TABLE discount_targets DROP CONSTRAINT IF EXISTS discount_targets_target_type_check;
UPDATE discount_targets SET target_type = 'for_sale' WHERE target_type = 'listing';
ALTER TABLE discount_targets
    ADD CONSTRAINT discount_targets_target_type_check
    CHECK ((target_type = ANY (ARRAY['for_sale'::text, 'auction'::text])));

-- 8c. promotion_instances / promotion_events (plain text)
UPDATE promotion_instances SET target_type = 'for_sale' WHERE target_type IN ('listing', 'fixed_price_sale');
UPDATE promotion_events SET target_type = 'for_sale' WHERE target_type IN ('listing', 'fixed_price_sale');
UPDATE promotion_events SET surface = 'for_sale' WHERE surface IN ('listing', 'fixed_price_sale');

-- 8d. promotion_packages.allowed_target_types (text[])
UPDATE promotion_packages
SET allowed_target_types = array_replace(allowed_target_types, 'listing', 'for_sale')
WHERE 'listing' = ANY (allowed_target_types);

UPDATE promotion_packages
SET allowed_target_types = array_replace(allowed_target_types, 'fixed_price_sale', 'for_sale')
WHERE 'fixed_price_sale' = ANY (allowed_target_types);

-- 8e. content/chat occurrence operation enums and comment share targetType
--     live in wire/text columns handled by application code; the persisted
--     enums are covered above. No further backfill required.

-- ------------------------------------------------------------
-- 9. Drop orphaned listing_* enum types (dead since the listings
--    table was dropped in 000010). Verified: zero tables reference them.
-- ------------------------------------------------------------
DROP TYPE IF EXISTS listing_status_enum;
DROP TYPE IF EXISTS listing_type_enum;
DROP TYPE IF EXISTS listing_visibility_enum;
DROP TYPE IF EXISTS listing_origin_enum;
