-- 000114 down: restore the pre-authority shape.
--
-- The old model cannot represent system/anonymized notifications at all, so
-- those rows are purged before NOT NULL is restored (development-only
-- migration: no production data exists at the time this authority landed).

DROP TRIGGER IF EXISTS trg_notifications_demote_deleted_actor ON users;
DROP FUNCTION IF EXISTS notifications_demote_deleted_actor();

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_actor_not_system_caller;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_actor_kind_shape;

DELETE FROM notifications WHERE actor_id IS NULL;

DROP INDEX IF EXISTS uniq_notification_event;

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_actor_id_fkey;
ALTER TABLE notifications ALTER COLUMN actor_id SET NOT NULL;
ALTER TABLE notifications ADD CONSTRAINT notifications_actor_id_fkey
    FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE UNIQUE INDEX uniq_notification_event
    ON notifications (recipient_id, actor_id, type, entity_id);

ALTER TABLE notifications DROP COLUMN IF EXISTS actor_key;
ALTER TABLE notifications DROP COLUMN IF EXISTS actor_display;
ALTER TABLE notifications DROP COLUMN IF EXISTS actor_kind;

DROP TYPE IF EXISTS notification_actor_kind_enum;
