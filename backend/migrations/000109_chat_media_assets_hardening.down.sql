-- 000109 down: best-effort revert (cannot drop PK if referenced, so DROP IF EXISTS)

ALTER TABLE chat_messages DROP COLUMN IF EXISTS has_media;

DROP INDEX IF EXISTS idx_chat_message_media_assets_msg_sort;
ALTER TABLE chat_message_media_assets DROP CONSTRAINT IF EXISTS chat_message_media_assets_asset_fkey;
ALTER TABLE chat_message_media_assets DROP CONSTRAINT IF EXISTS chat_message_media_assets_msg_fkey;
ALTER TABLE chat_message_media_assets DROP CONSTRAINT IF EXISTS chat_message_media_assets_pkey;

DROP INDEX IF EXISTS idx_chat_media_assets_storage_key;
DROP INDEX IF EXISTS idx_chat_media_assets_room_uploader;
ALTER TABLE chat_media_assets DROP CONSTRAINT IF EXISTS chat_media_assets_expires_chk;
ALTER TABLE chat_media_assets DROP CONSTRAINT IF EXISTS chat_media_assets_uploader_id_fkey;
ALTER TABLE chat_media_assets DROP CONSTRAINT IF EXISTS chat_media_assets_room_id_fkey;
-- PK last
ALTER TABLE chat_media_assets DROP CONSTRAINT IF EXISTS chat_media_assets_pkey;
