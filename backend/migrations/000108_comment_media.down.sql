-- 000108_comment_media down

DROP INDEX IF EXISTS idx_comment_media_storage_key;
DROP INDEX IF EXISTS idx_comment_media_comment_id;
DROP TABLE IF EXISTS comment_media;
DROP TYPE IF EXISTS comment_media_type_enum;
