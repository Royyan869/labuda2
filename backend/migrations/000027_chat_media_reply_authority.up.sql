-- ============================================================
-- 000027_chat_media_reply_authority.up.sql
-- Forward-only chat media/reply authority extracted from 000001.
-- Safe on databases that already applied the old 000001 addition.
-- ============================================================

DO $$
BEGIN
    CREATE TYPE chat_media_asset_status_enum AS ENUM (
        'pending',
        'finalized',
        'deleted'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE chat_media_asset_type_enum AS ENUM (
        'image',
        'video'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE chat_messages
    ADD COLUMN IF NOT EXISTS reply_to_message_id uuid;

ALTER TABLE chat_messages
    ADD COLUMN IF NOT EXISTS reply_preview_json jsonb;

CREATE TABLE IF NOT EXISTS chat_media_assets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    uploader_id uuid NOT NULL,
    media_type chat_media_asset_type_enum NOT NULL,
    content_type text NOT NULL,
    storage_key text NOT NULL,
    thumbnail_storage_key text,
    byte_size bigint NOT NULL,
    width integer,
    height integer,
    duration_ms integer,
    status chat_media_asset_status_enum NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finalized_at timestamp with time zone,
    deleted_at timestamp with time zone,
    deleted_by uuid,
    deletion_reason text
);

CREATE TABLE IF NOT EXISTS chat_message_media_assets (
    message_id uuid NOT NULL,
    media_asset_id uuid NOT NULL,
    sort_order integer NOT NULL
);
