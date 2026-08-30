-- Drop content_mentioned_users: structurally dead table.
-- Created in migration 000043 but no Go application code ever writes to or reads from it.
-- The mobile CreateContentDto accepted mentioned_user_ids but the backend silently dropped them.
DROP TABLE IF EXISTS content_mentioned_users;
