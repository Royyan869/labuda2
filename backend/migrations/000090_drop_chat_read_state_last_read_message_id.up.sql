-- 000090: Drop the dead chat_read_states.last_read_message_id column.
--
-- Unread is a TIMESTAMP projection: chat_read_states.last_read_at compared
-- against chat_messages.created_at. The message-id cursor was never written or
-- read by any Go code — the only reference outside the schema was migration
-- 000033 nulling it during fingerprint hardening. It is dead schema residue
-- and is removed so the read-state table carries exactly one read cursor.

ALTER TABLE chat_read_states
    DROP CONSTRAINT IF EXISTS chat_read_states_last_read_message_id_fkey;

ALTER TABLE chat_read_states
    DROP COLUMN IF EXISTS last_read_message_id;
