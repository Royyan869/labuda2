-- Restore content_mentioned_users for canonical Content mention persistence.
-- Migration 000043 originally created this table; 000052 dropped it in error.
-- This forward-only migration recreates the canonical relation.
CREATE TABLE IF NOT EXISTS content_mentioned_users (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (content_id, user_id)
);

CREATE INDEX IF NOT EXISTS content_mentioned_users_user_id_idx ON content_mentioned_users (user_id);
CREATE INDEX IF NOT EXISTS content_mentioned_users_content_id_idx ON content_mentioned_users (content_id);
