-- ============================================================
-- 000027_chat_media_reply_authority.down.sql
-- Reverts the forward-only chat media/reply authority migration.
-- ============================================================

DROP TABLE IF EXISTS chat_message_media_assets;
DROP TABLE IF EXISTS chat_media_assets;

ALTER TABLE chat_messages
    DROP COLUMN IF EXISTS reply_preview_json,
    DROP COLUMN IF EXISTS reply_to_message_id;

DO $$
BEGIN
    DROP TYPE IF EXISTS chat_media_asset_type_enum;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END $$;

DO $$
BEGIN
    DROP TYPE IF EXISTS chat_media_asset_status_enum;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END $$;
