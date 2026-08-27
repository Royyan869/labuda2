-- Rollback of 000002_negotiation_schema_alignment (local rollback only).
--
-- NOTE: 'fixed_price_sale' added to negotiation_resource_enum is NOT removed
-- here. PostgreSQL does not support dropping a single enum value without
-- recreating the type; leaving the label behind is harmless (unused values
-- are not enforced) and matches this being a local-only rollback path.

DROP TABLE IF EXISTS negotiation_price_history;

ALTER TABLE negotiation_sessions DROP CONSTRAINT IF EXISTS negotiation_sessions_order_id_key;
ALTER TABLE negotiation_sessions DROP CONSTRAINT IF EXISTS negotiation_sessions_chat_room_id_fkey;
ALTER TABLE negotiation_sessions DROP CONSTRAINT IF EXISTS negotiation_sessions_order_id_fkey;

DROP INDEX IF EXISTS idx_negotiation_sessions_chat_room_id;
DROP INDEX IF EXISTS idx_negotiation_sessions_order_id;

ALTER TABLE negotiation_sessions
    DROP COLUMN IF EXISTS chat_room_id,
    DROP COLUMN IF EXISTS order_id,
    DROP COLUMN IF EXISTS accepted_price,
    DROP COLUMN IF EXISTS accepted_at,
    DROP COLUMN IF EXISTS proposal_sequence;
