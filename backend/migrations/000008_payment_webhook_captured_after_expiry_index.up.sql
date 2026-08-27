-- PASS_19B: split out of 000005_payment_webhook_captured_after_expiry.
--
-- This index's WHERE clause references the 'captured_after_expiry' enum
-- value added by migration 000005. Postgres requires a newly added enum
-- value to be committed in a prior transaction before it can be used, so
-- this must run as its own migration, after 000005 has already committed.
CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_captured_after_expiry
    ON payment_webhook_events USING btree (status, received_at DESC)
    WHERE (status = 'captured_after_expiry'::payment_webhook_status_enum);
