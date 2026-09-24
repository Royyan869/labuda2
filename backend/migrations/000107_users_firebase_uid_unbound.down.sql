-- 000107_users_firebase_uid_unbound.down.sql
--
-- Restoring NOT NULL fails while any unbound account row (firebase_uid IS NULL)
-- still exists. That is intentional: reverting this migration requires an
-- explicit decision about those rows, never a silent backfill with a
-- fabricated UID.

ALTER TABLE users ALTER COLUMN firebase_uid SET NOT NULL;
