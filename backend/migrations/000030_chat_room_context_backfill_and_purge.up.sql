-- ============================================================
-- 000030_chat_room_context_backfill_and_purge.up.sql
-- Backfill legacy chat room context into immutable commerce references,
-- then remove room-level context columns.
--
-- Legacy room context stored a chat-specific preview payload. The backfill
-- normalizes both camelCase and snake_case keys into the canonical
-- chat_commerce_references.display_snapshot format.
-- ============================================================

WITH raw_context_source AS (
    SELECT
        cr.id AS room_id,
        CASE
            WHEN cr.context_set_by IS NOT NULL
             AND cr.context_set_by IN (cr.participant_a, cr.participant_b)
                THEN cr.context_set_by
            ELSE cr.participant_a
        END AS creator_id,
        cr.updated_at AS created_at,
        cr.participant_a,
        cr.participant_b,
        COALESCE(cr.context_json->>'target_type', cr.context_json->>'targetType') AS target_type_text,
        COALESCE(cr.context_json->>'target_id', cr.context_json->>'targetId') AS target_id_text,
        CASE
            WHEN jsonb_typeof(cr.context_json->'preview') = 'object' THEN cr.context_json->'preview'
            ELSE '{}'::jsonb
        END AS legacy_preview
    FROM chat_rooms cr
    WHERE cr.context_json IS NOT NULL
),
context_source AS (
    SELECT
        room_id,
        creator_id,
        created_at,
        participant_a,
        participant_b,
        target_type_text,
        CASE
            WHEN target_id_text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
                THEN target_id_text::uuid
        END AS target_id,
        legacy_preview
    FROM raw_context_source
    WHERE target_type_text IN ('fixed_price_sale', 'auction')
)
INSERT INTO chat_commerce_references (
    id,
    room_id,
    target_type,
    target_id,
    creator_id,
    display_snapshot,
    created_at
)
SELECT
    gen_random_uuid(),
    s.room_id,
    'fixed_price_sale'::chat_commerce_reference_target_type_enum,
    s.target_id,
    s.creator_id,
    jsonb_strip_nulls(jsonb_build_object(
        'title', COALESCE(s.legacy_preview->>'title', ''),
        'image_url', COALESCE(s.legacy_preview->>'image_url', s.legacy_preview->>'imageUrl'),
        'display_value',
            CASE
                WHEN COALESCE(s.legacy_preview->>'display_value', s.legacy_preview->>'displayValue') ~ '^-?[0-9]+$'
                    THEN COALESCE(s.legacy_preview->>'display_value', s.legacy_preview->>'displayValue')::bigint
            END,
        'seller_id', COALESCE(s.legacy_preview->>'seller_id', s.legacy_preview->>'sellerId'),
        'seller_name', COALESCE(s.legacy_preview->>'seller_name', s.legacy_preview->>'sellerName'),
        'is_available',
            COALESCE(
                (s.legacy_preview->>'is_available')::boolean,
                (s.legacy_preview->>'isAvailable')::boolean,
                true
            ),
        'is_sold',
            COALESCE(
                (s.legacy_preview->>'is_sold')::boolean,
                (s.legacy_preview->>'isSold')::boolean,
                false
            ),
        'is_closed',
            COALESCE(
                (s.legacy_preview->>'is_closed')::boolean,
                (s.legacy_preview->>'isClosed')::boolean,
                false
            ),
        'is_deleted',
            COALESCE(
                (s.legacy_preview->>'is_deleted')::boolean,
                (s.legacy_preview->>'isDeleted')::boolean,
                false
            )
    )),
    s.created_at
FROM context_source s
JOIN fixed_price_sales fps
    ON fps.id = s.target_id
   AND fps.seller_id IN (s.participant_a, s.participant_b)
WHERE s.target_id IS NOT NULL
ON CONFLICT (room_id, target_type, target_id) DO NOTHING;

WITH raw_context_source AS (
    SELECT
        cr.id AS room_id,
        CASE
            WHEN cr.context_set_by IS NOT NULL
             AND cr.context_set_by IN (cr.participant_a, cr.participant_b)
                THEN cr.context_set_by
            ELSE cr.participant_a
        END AS creator_id,
        cr.updated_at AS created_at,
        cr.participant_a,
        cr.participant_b,
        COALESCE(cr.context_json->>'target_type', cr.context_json->>'targetType') AS target_type_text,
        COALESCE(cr.context_json->>'target_id', cr.context_json->>'targetId') AS target_id_text,
        CASE
            WHEN jsonb_typeof(cr.context_json->'preview') = 'object' THEN cr.context_json->'preview'
            ELSE '{}'::jsonb
        END AS legacy_preview
    FROM chat_rooms cr
    WHERE cr.context_json IS NOT NULL
),
context_source AS (
    SELECT
        room_id,
        creator_id,
        created_at,
        participant_a,
        participant_b,
        target_type_text,
        CASE
            WHEN target_id_text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'
                THEN target_id_text::uuid
        END AS target_id,
        legacy_preview
    FROM raw_context_source
    WHERE target_type_text IN ('fixed_price_sale', 'auction')
)
INSERT INTO chat_commerce_references (
    id,
    room_id,
    target_type,
    target_id,
    creator_id,
    display_snapshot,
    created_at
)
SELECT
    gen_random_uuid(),
    s.room_id,
    'auction'::chat_commerce_reference_target_type_enum,
    s.target_id,
    s.creator_id,
    jsonb_strip_nulls(jsonb_build_object(
        'title', COALESCE(s.legacy_preview->>'title', ''),
        'image_url', COALESCE(s.legacy_preview->>'image_url', s.legacy_preview->>'imageUrl'),
        'display_value',
            CASE
                WHEN COALESCE(s.legacy_preview->>'display_value', s.legacy_preview->>'displayValue') ~ '^-?[0-9]+$'
                    THEN COALESCE(s.legacy_preview->>'display_value', s.legacy_preview->>'displayValue')::bigint
            END,
        'seller_id', COALESCE(s.legacy_preview->>'seller_id', s.legacy_preview->>'sellerId'),
        'seller_name', COALESCE(s.legacy_preview->>'seller_name', s.legacy_preview->>'sellerName'),
        'is_available',
            COALESCE(
                (s.legacy_preview->>'is_available')::boolean,
                (s.legacy_preview->>'isAvailable')::boolean,
                true
            ),
        'is_sold',
            COALESCE(
                (s.legacy_preview->>'is_sold')::boolean,
                (s.legacy_preview->>'isSold')::boolean,
                false
            ),
        'is_closed',
            COALESCE(
                (s.legacy_preview->>'is_closed')::boolean,
                (s.legacy_preview->>'isClosed')::boolean,
                false
            ),
        'is_deleted',
            COALESCE(
                (s.legacy_preview->>'is_deleted')::boolean,
                (s.legacy_preview->>'isDeleted')::boolean,
                false
            )
    )),
    s.created_at
FROM context_source s
JOIN auctions a
    ON a.id = s.target_id
   AND a.seller_id IN (s.participant_a, s.participant_b)
WHERE s.target_id IS NOT NULL
ON CONFLICT (room_id, target_type, target_id) DO NOTHING;

ALTER TABLE chat_rooms
    DROP CONSTRAINT IF EXISTS chat_rooms_context_set_by_fkey;

ALTER TABLE chat_rooms
    DROP COLUMN IF EXISTS context_json;

ALTER TABLE chat_rooms
    DROP COLUMN IF EXISTS context_set_by;
