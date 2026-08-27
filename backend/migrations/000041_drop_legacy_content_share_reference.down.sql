-- Migration 000041 down: restore contents.share_reference column

ALTER TABLE contents
    ADD COLUMN IF NOT EXISTS share_reference jsonb;
