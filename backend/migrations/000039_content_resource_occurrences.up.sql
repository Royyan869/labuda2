-- Migration 000039: Content resource occurrences
--
-- Canonical immutable content occurrence authority.
--
-- DESIGN:
-- - content_id is the primary key (one occurrence per content row)
-- - actor_id is server-derived from the authenticated creator
-- - Exactly one typed source FK must be non-null
-- - share_to_feed permits profile/content/fixed_price_sale/auction
-- - direct_commerce_insert_content permits fixed_price_sale/auction only
-- - content_source_id cannot equal content_id
-- - UPDATE is rejected by trigger; parent content DELETE still cascades

CREATE TYPE content_resource_occurrence_operation_enum AS ENUM (
    'share_to_feed',
    'direct_commerce_insert_content'
);

CREATE TABLE content_resource_occurrences (
    content_id uuid PRIMARY KEY REFERENCES contents(id) ON DELETE CASCADE,
    actor_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

    operation content_resource_occurrence_operation_enum NOT NULL,

    profile_source_id           uuid REFERENCES users(id) ON DELETE RESTRICT,
    content_source_id           uuid REFERENCES contents(id) ON DELETE RESTRICT,
    fixed_price_sale_source_id  uuid REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
    auction_source_id           uuid REFERENCES auctions(id) ON DELETE RESTRICT,

    created_at timestamp with time zone NOT NULL DEFAULT now(),

    CONSTRAINT content_resource_occurrence_exactly_one_source CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    ),

    CONSTRAINT content_resource_occurrence_operation_compatibility CHECK (
        operation = 'share_to_feed'
        OR (
            operation = 'direct_commerce_insert_content'
            AND (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL)
        )
    ),

    CONSTRAINT content_resource_occurrence_no_self_reference CHECK (
        content_source_id IS NULL OR content_source_id <> content_id
    )
);

CREATE INDEX idx_content_resource_occurrences_profile_source
    ON content_resource_occurrences (profile_source_id, created_at DESC)
    WHERE profile_source_id IS NOT NULL;

CREATE INDEX idx_content_resource_occurrences_content_source
    ON content_resource_occurrences (content_source_id, created_at DESC)
    WHERE content_source_id IS NOT NULL;

CREATE INDEX idx_content_resource_occurrences_fps_source
    ON content_resource_occurrences (fixed_price_sale_source_id, created_at DESC)
    WHERE fixed_price_sale_source_id IS NOT NULL;

CREATE INDEX idx_content_resource_occurrences_auction_source
    ON content_resource_occurrences (auction_source_id, created_at DESC)
    WHERE auction_source_id IS NOT NULL;

CREATE OR REPLACE FUNCTION prevent_content_resource_occurrences_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'content_resource_occurrences rows are immutable';
END;
$$;

CREATE TRIGGER trg_content_resource_occurrences_immutable
BEFORE UPDATE ON content_resource_occurrences
FOR EACH ROW
EXECUTE FUNCTION prevent_content_resource_occurrences_update();
