-- Client-provisional video metadata for instant rich paint (duration badge,
-- aspect placeholders). Server canonicalizes via the video worker later;
-- NULL = unknown (legacy rows, images without dims). No backfill: new
-- uploads carry the values, old rows degrade gracefully by design.
ALTER TABLE content_media ADD COLUMN IF NOT EXISTS duration_ms INT;
ALTER TABLE content_media ADD COLUMN IF NOT EXISTS width INT;
ALTER TABLE content_media ADD COLUMN IF NOT EXISTS height INT;
ALTER TABLE comment_media ADD COLUMN IF NOT EXISTS duration_ms INT;
ALTER TABLE comment_media ADD COLUMN IF NOT EXISTS width INT;
ALTER TABLE comment_media ADD COLUMN IF NOT EXISTS height INT;
