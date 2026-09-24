-- 000108_comment_media
-- Enables foto+video support for comments (Komentar) — 5 media max (4 image + 1 video)
-- Symmetric to content_media but scoped to comments.
-- Uses same S3 presign flow: POST /media/upload-url (folder images/videos)
-- Stores storage_key (raw S3 key) + read_url (CDN-resolved) for fail-open display.

DO $$
BEGIN
    CREATE TYPE comment_media_type_enum AS ENUM ('image', 'video');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS comment_media (
    id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    comment_id uuid NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    storage_key text NOT NULL CHECK (char_length(storage_key) > 0),
    media_url text NOT NULL CHECK (char_length(media_url) > 0),
    media_type comment_media_type_enum NOT NULL,
    position int NOT NULL CHECK (position >= 0 AND position < 5),
    byte_size bigint CHECK (byte_size > 0),
    created_at timestamptz DEFAULT now() NOT NULL,
    UNIQUE(comment_id, position)
);

CREATE INDEX IF NOT EXISTS idx_comment_media_comment_id ON comment_media(comment_id, position);
CREATE INDEX IF NOT EXISTS idx_comment_media_storage_key ON comment_media(storage_key);
