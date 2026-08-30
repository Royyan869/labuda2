-- 000054_drop_dead_chat_commerce_references.down.sql
-- Rollback: recreate the chat_commerce_references table and enum

CREATE TYPE chat_commerce_reference_target_type_enum AS ENUM (
    'for_sale',
    'auction'
);

CREATE TABLE IF NOT EXISTS chat_commerce_references (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    room_id uuid NOT NULL,
    target_type chat_commerce_reference_target_type_enum NOT NULL,
    target_id uuid NOT NULL,
    creator_id uuid NOT NULL,
    display_snapshot jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chat_commerce_references_pkey PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_chat_commerce_references_room_target
    ON chat_commerce_references (room_id, target_type, target_id);

CREATE INDEX IF NOT EXISTS idx_chat_commerce_references_room_created_at
    ON chat_commerce_references (room_id, created_at DESC, id DESC);

ALTER TABLE chat_commerce_references
    ADD CONSTRAINT chat_commerce_references_room_id_fkey
    FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE;

ALTER TABLE chat_commerce_references
    ADD CONSTRAINT chat_commerce_references_creator_id_fkey
    FOREIGN KEY (creator_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE chat_commerce_references
    ADD CONSTRAINT chat_commerce_references_target_id_check
    CHECK (target_id IS NOT NULL);
