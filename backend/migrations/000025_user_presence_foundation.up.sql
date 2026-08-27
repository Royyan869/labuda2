CREATE TABLE user_presence (
    user_id uuid PRIMARY KEY,
    last_seen_at timestamp with time zone NULL,
    updated_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT user_presence_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
