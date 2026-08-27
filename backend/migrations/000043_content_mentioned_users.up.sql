CREATE TABLE content_mentioned_users (
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (content_id, user_id)
);

CREATE INDEX content_mentioned_users_user_id_idx ON content_mentioned_users (user_id);
