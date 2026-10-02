-- Per-item video processing state (processing → ready / failed).
-- A video row is born processing at create time; the media-readiness worker
-- flips it once the Lambda poster frame exists (ready) or past the timeout
-- (failed). Images are born ready. Existing rows default ready: every file
-- already in S3 predates this state machine, and failing closed to
-- processing would wrongly dim the whole catalog.
ALTER TABLE content_media ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ready';
ALTER TABLE comment_media ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'ready';
ALTER TABLE content_media ADD CONSTRAINT content_media_status_vocab_chk CHECK (status IN ('processing','ready','failed'));
ALTER TABLE comment_media ADD CONSTRAINT comment_media_status_vocab_chk CHECK (status IN ('processing','ready','failed'));
-- products.media_urls is jsonb per-item: legacy items without a status key
-- read as ready (mediaref.NormalizeStatus). No backfill: absent = ready.
