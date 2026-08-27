-- Migration 000032: Actor-scoped chat message idempotency
--
-- REPAIR: P1 Security — global idempotency_key UNIQUE allowed cross-actor
-- message leakage. Actor A's message could be returned to Actor B when
-- both used the same opaque idempotency key.
--
-- CHANGES:
-- 1. Drop global UNIQUE(idempotency_key) — the unsafe authority (irreversible)
-- 2. Add command_fingerprint NOT NULL DEFAULT '' — server-computed SHA-256 of
--    normalized send-message command, used for replay validation.
--    DEFAULT '' is temporary; 000033 hardens this to a CHECK constraint.
-- 3. Add UNIQUE(sender_id, idempotency_key) — actor-scoped, no
--    cross-actor collision possible

-- Step 1: Remove global unsafe authority (IRREVERSIBLE)
ALTER TABLE chat_messages DROP CONSTRAINT IF EXISTS chat_messages_idempotency_key_key;

-- Step 2: Add command fingerprint column
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS command_fingerprint text NOT NULL DEFAULT '';

-- Step 3: Add actor-scoped unique constraint
ALTER TABLE chat_messages ADD CONSTRAINT chat_messages_sender_idempotency_key
    UNIQUE (sender_id, idempotency_key);

-- Step 4: Index for the actor-scoped lookup used on every SendMessage
CREATE INDEX IF NOT EXISTS idx_chat_messages_sender_idempotency
    ON chat_messages (sender_id, idempotency_key);
