-- ============================================================
-- 000098 CANONICAL WEBHOOK NOTIFICATION IDENTITY (down)
--
-- Restores the pre-REC-3 shape: event_id is the uniqueness authority again
-- and notification_key is removed.
--
-- NOTE: restoring UNIQUE(event_id) is only possible while no transaction has
-- more than one stored notification. Once the up migration has allowed a
-- transaction to accumulate several rows (its whole purpose), this down
-- migration FAILS LOUDLY on the unique index instead of silently discarding
-- the extra events. That is deliberate: collapsing distinct notifications back
-- into one row would recreate the exact money-signal loss REC-3 fixed.
-- ============================================================

DROP INDEX IF EXISTS payment_webhook_events_notification_key_key;
DROP INDEX IF EXISTS idx_payment_webhook_events_event_id;

ALTER TABLE payment_webhook_events
    ADD CONSTRAINT payment_webhook_events_event_id_key UNIQUE (event_id);

ALTER TABLE payment_webhook_events DROP COLUMN notification_key;

COMMENT ON COLUMN payment_webhook_events.event_id IS NULL;
