-- FCM TOKEN ON CONFLICT 42P10 — the fix.
--
-- The repository upsert (fcm_token_repository.Insert) targets
-- `ON CONFLICT (user_id, device_id)`. The only supporting index was the
-- PARTIAL unique index from 000001:
--   CREATE UNIQUE INDEX ... (user_id, device_id) WHERE (device_id IS NOT NULL)
-- Postgres cannot infer a partial index for an index-predicate-free conflict
-- target, so every token registration failed with 42P10
-- ("there is no unique or exclusion constraint matching the ON CONFLICT
-- specification") and the device never received push notifications.
--
-- device_id became NOT NULL in 000026, so the WHERE clause of the partial
-- index no longer excludes anything: a FULL unique index is the same
-- guarantee without breaking inference. Any pre-hardening duplicate
-- (user_id, device_id) rows would fail this index build — those were
-- normalized in 000026 (device_id = token) and duplicates cannot survive
-- the deactivation path in the repository insert.

DROP INDEX IF EXISTS idx_fcm_tokens_user_device;

CREATE UNIQUE INDEX idx_fcm_tokens_user_device
    ON public.fcm_tokens USING btree (user_id, device_id);
