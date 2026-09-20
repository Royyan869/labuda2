-- Reverse of 000090: restore the (dead) chat_read_states.last_read_message_id
-- column and its foreign key.

ALTER TABLE chat_read_states
    ADD COLUMN IF NOT EXISTS last_read_message_id uuid;

ALTER TABLE chat_read_states
    ADD CONSTRAINT chat_read_states_last_read_message_id_fkey
    FOREIGN KEY (last_read_message_id) REFERENCES chat_messages(id) ON DELETE SET NULL;
