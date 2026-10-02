ALTER TABLE comment_media DROP CONSTRAINT IF EXISTS comment_media_status_vocab_chk;
ALTER TABLE content_media DROP CONSTRAINT IF EXISTS content_media_status_vocab_chk;
ALTER TABLE comment_media DROP COLUMN IF EXISTS status;
ALTER TABLE content_media DROP COLUMN IF EXISTS status;
