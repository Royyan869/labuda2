-- 000114_notification_actor_authority
-- ============================================================
-- BUSINESS TRUTH
-- A notification is caused by exactly one of:
--   1. a real human user (buyer, seller, admin),
--   2. the system (timer/worker/gateway; there is no human),
--   3. a human whose identity is hidden from the recipient by block policy.
--
-- The old schema collapsed all three into actor_id uuid NOT NULL REFERENCES
-- users(id): "no human actor" had to be written as uuid.Nil, which can never
-- satisfy the FK. Every system/anonymized notification therefore failed with
-- SQLSTATE 23503 and the outbox retried the same deterministic failure until
-- dead_letter. Three meanings, one unpersistable sentinel — a modeling
-- defect, not a missing guard.
--
-- CANONICAL MODEL
--   actor_kind = 'user'       → actor_id = users.id (visible identity)
--   actor_kind = 'system'     → actor_id IS NULL (no human actor)
--   actor_kind = 'anonymized' → actor_id IS NULL, actor_display = role label
--
-- INVARIANTS ENFORCED HERE
--   * notifications_actor_kind_shape: kind 'user' ⇔ actor_id IS NOT NULL.
--   * notifications_actor_not_system_caller: audit.SystemCallerID
--     (00000000-0000-0000-0000-000000000001) can never be a persisted human
--     identity — mirrors 000086 (users_reserved_system_caller_id) and 000100
--     (refunds_reviewed_by_not_system_caller). It must never be stored here.
--   * actor FK is ON DELETE SET NULL: deleting an actor must not erase the
--     recipient's history. The trigger demotes such rows to 'system' first so
--     the shape check above stays true.
--
-- DEDUP
-- The old unique key (recipient_id, actor_id, type, entity_id) loses meaning
-- once actor_id can be NULL (PostgreSQL treats NULLs as distinct), which would
-- silently break idempotency for system/anonymized notifications. actor_key
-- (STORED generated, COALESCE(actor_id, zero)) restores exactly the old dedup
-- semantics for user actors and gives system/anonymized notifications one
-- stable key.

CREATE TYPE notification_actor_kind_enum AS ENUM ('user', 'system', 'anonymized');

ALTER TABLE notifications ADD COLUMN actor_kind notification_actor_kind_enum;

UPDATE notifications
SET actor_kind = 'user';

ALTER TABLE notifications ALTER COLUMN actor_kind SET NOT NULL;

ALTER TABLE notifications ADD COLUMN actor_display text NOT NULL DEFAULT '';

-- Role label backfill for rows written while anonymized actors were still
-- smuggled through data.actor_display (they could not persist before this
-- migration, so this is defensive for development databases only).
UPDATE notifications
SET actor_display = COALESCE(data ->> 'actor_display', '')
WHERE actor_display = '' AND data ? 'actor_display';

ALTER TABLE notifications ADD COLUMN actor_key uuid
    GENERATED ALWAYS AS (COALESCE(actor_id, '00000000-0000-0000-0000-000000000000'::uuid)) STORED;

ALTER TABLE notifications ALTER COLUMN actor_id DROP NOT NULL;

ALTER TABLE notifications DROP CONSTRAINT notifications_actor_id_fkey;
ALTER TABLE notifications ADD CONSTRAINT notifications_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE SET NULL;

DROP INDEX uniq_notification_event;
CREATE UNIQUE INDEX uniq_notification_event
    ON notifications (recipient_id, actor_key, type, entity_id);

ALTER TABLE notifications ADD CONSTRAINT notifications_actor_kind_shape
    CHECK ((actor_kind = 'user') = (actor_id IS NOT NULL));

ALTER TABLE notifications ADD CONSTRAINT notifications_actor_not_system_caller
    CHECK (actor_id IS NULL OR actor_id <> '00000000-0000-0000-0000-000000000001');

-- Hard-deleting a user removes the identity, not the recipient's history.
-- BEFORE DELETE runs ahead of the FK's SET NULL, so both columns move in one
-- statement and the shape check never observes an inconsistent row.
CREATE FUNCTION notifications_demote_deleted_actor() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE notifications
    SET actor_kind = 'system', actor_id = NULL, actor_display = ''
    WHERE actor_id = OLD.id AND actor_kind = 'user';
    RETURN OLD;
END $$;

CREATE TRIGGER trg_notifications_demote_deleted_actor
    BEFORE DELETE ON users
    FOR EACH ROW EXECUTE FUNCTION notifications_demote_deleted_actor();
