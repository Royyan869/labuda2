-- Migration 000033: Chat message fingerprint hardening
--
-- Hardens the command_fingerprint column added by 000032:
-- 1. Purges dev-only rows with empty fingerprints (zero production data)
-- 2. Removes the temporary DEFAULT '' — every INSERT must supply a real fingerprint
-- 3. Adds CHECK (command_fingerprint <> '') — empty fingerprints rejected at DB level
--
-- After this migration, every chat_messages row carries a non-empty canonical
-- SHA-256 fingerprint. No sentinel, no bypass, no legacy exception.

-- Step 1: Purge dependent rows first (FK safety)
DELETE FROM chat_message_media_assets
 WHERE message_id IN (SELECT id FROM chat_messages WHERE command_fingerprint = '');

UPDATE chat_read_states
   SET last_read_message_id = NULL
 WHERE last_read_message_id IN (SELECT id FROM chat_messages WHERE command_fingerprint = '');

-- Step 2: Delete rows that cannot be canonicalized (zero production data)
DELETE FROM chat_messages WHERE command_fingerprint = '';

-- Step 3: Remove the temporary DEFAULT — new inserts must supply fingerprint explicitly
ALTER TABLE chat_messages ALTER COLUMN command_fingerprint DROP DEFAULT;

-- Step 4: Enforce non-empty fingerprint at the DB level
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_command_fingerprint_not_empty
    CHECK (command_fingerprint <> '');
