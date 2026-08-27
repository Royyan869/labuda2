-- Migration 000034: Chat message resource occurrences
--
-- Stores chat message resource occurrence metadata.
--
-- DESIGN (Scope 3B2 locked):
-- - message_id is the PK (one occurrence per message at most)
-- - actor_id and room_id derive from chat_messages (no duplication)
-- - Exactly one typed source FK must be non-null (FK presence IS the type authority)
-- - direct_commerce_insert_chat permits FPS and Auction only
-- - fallback_snapshot is server-built; client never supplies preview data

-- Step 1: Operation enum
CREATE TYPE chat_resource_occurrence_operation_enum AS ENUM (
    'share_to_chat',
    'direct_commerce_insert_chat'
);

-- Step 2: Occurrence table
CREATE TABLE chat_message_resource_occurrences (
    message_id uuid PRIMARY KEY REFERENCES chat_messages(id) ON DELETE CASCADE,

    operation chat_resource_occurrence_operation_enum NOT NULL,

    -- Exactly one typed source FK
    profile_source_id           uuid REFERENCES users(id) ON DELETE RESTRICT,
    content_source_id           uuid REFERENCES contents(id) ON DELETE RESTRICT,
    fixed_price_sale_source_id  uuid REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
    auction_source_id           uuid REFERENCES auctions(id) ON DELETE RESTRICT,

    -- Server-built immutable display fallback
    fallback_snapshot jsonb NOT NULL,

    created_at timestamp with time zone NOT NULL DEFAULT now(),

    -- Exactly-one-source invariant
    CONSTRAINT chat_occurrence_exactly_one_source CHECK (
        (CASE WHEN profile_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN content_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN fixed_price_sale_source_id IS NOT NULL THEN 1 ELSE 0 END +
         CASE WHEN auction_source_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    ),

    -- direct_commerce_insert_chat: FPS and Auction only
    CONSTRAINT chat_occurrence_valid_operation CHECK (
        (operation = 'direct_commerce_insert_chat' AND
            (fixed_price_sale_source_id IS NOT NULL OR auction_source_id IS NOT NULL))
        OR
        (operation = 'share_to_chat')
    )
);

-- Step 3: Partial indexes for source-type lookups
CREATE INDEX idx_chat_occurrence_profile_src
    ON chat_message_resource_occurrences (profile_source_id, created_at DESC)
    WHERE profile_source_id IS NOT NULL;

CREATE INDEX idx_chat_occurrence_content_src
    ON chat_message_resource_occurrences (content_source_id, created_at DESC)
    WHERE content_source_id IS NOT NULL;

CREATE INDEX idx_chat_occurrence_fps_src
    ON chat_message_resource_occurrences (fixed_price_sale_source_id, created_at DESC)
    WHERE fixed_price_sale_source_id IS NOT NULL;

CREATE INDEX idx_chat_occurrence_auction_src
    ON chat_message_resource_occurrences (auction_source_id, created_at DESC)
    WHERE auction_source_id IS NOT NULL;
