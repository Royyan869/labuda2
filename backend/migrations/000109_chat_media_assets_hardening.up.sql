-- 000109_chat_media_assets_hardening
-- Hardens chat_media_assets + chat_message_media_assets for foto+video attach.
-- Existing tables created via CREATE TABLE IF NOT EXISTS in 000027 lacked PK/FK/indexes.
-- This migration adds canonical constraints so chat media becomes queryable & cascade-safe.

-- Chat media assets: add PK/FK/indexes idempotently
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_media_assets_pkey') THEN
        ALTER TABLE chat_media_assets ADD PRIMARY KEY (id);
    END IF;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE chat_media_assets ADD CONSTRAINT chat_media_assets_room_id_fkey FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE chat_media_assets ADD CONSTRAINT chat_media_assets_uploader_id_fkey FOREIGN KEY (uploader_id) REFERENCES users(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_chat_media_assets_room_uploader ON chat_media_assets(room_id, uploader_id, status);
CREATE INDEX IF NOT EXISTS idx_chat_media_assets_storage_key ON chat_media_assets(storage_key);

-- Message<->media link table hardening
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chat_message_media_assets_pkey') THEN
        ALTER TABLE chat_message_media_assets ADD PRIMARY KEY (message_id, media_asset_id);
    END IF;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE chat_message_media_assets ADD CONSTRAINT chat_message_media_assets_msg_fkey FOREIGN KEY (message_id) REFERENCES chat_messages(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    ALTER TABLE chat_message_media_assets ADD CONSTRAINT chat_message_media_assets_asset_fkey FOREIGN KEY (media_asset_id) REFERENCES chat_media_assets(id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_chat_message_media_assets_msg_sort ON chat_message_media_assets(message_id, sort_order);

-- Convenience flag for has_media queries (fast list rendering)
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS has_media boolean DEFAULT false NOT NULL;

-- Ensure expires_at sanity (pending assets expire after 24h by convention)
DO $$
BEGIN
    ALTER TABLE chat_media_assets ADD CONSTRAINT chat_media_assets_expires_chk CHECK (expires_at > created_at);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
