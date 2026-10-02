-- Restore the 000001 shape: partial unique index on (user_id, device_id).
-- NOTE: reverting reintroduces the 42P10 failure for the repository's plain
-- `ON CONFLICT (user_id, device_id)` upsert — kept only for rollback parity.

DROP INDEX IF EXISTS idx_fcm_tokens_user_device;

CREATE UNIQUE INDEX idx_fcm_tokens_user_device
    ON public.fcm_tokens USING btree (user_id, device_id)
    WHERE (device_id IS NOT NULL);
