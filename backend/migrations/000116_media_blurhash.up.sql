-- BlurHash placeholders for instant media paint (Mastodon-style).
-- Client already computes the hash at upload; this column persists it so
-- EVERY viewer gets the blur first, not just the uploader. Nullable:
-- legacy rows render the static mat fallback. No backfill: hashes arrive
-- with new uploads; old rows degrade gracefully by design.
ALTER TABLE content_media ADD COLUMN IF NOT EXISTS blurhash TEXT;
ALTER TABLE comment_media ADD COLUMN IF NOT EXISTS blurhash TEXT;
