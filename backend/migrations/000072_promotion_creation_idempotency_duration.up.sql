-- 000072_promotion_creation_idempotency_duration
-- Fixes canonical creation idempotency gap: duration is immutable input.
-- Stores duration_seconds in idempotency record so mismatched duration is deterministic error.

ALTER TABLE promotion_creation_idempotency
    ADD COLUMN IF NOT EXISTS duration_seconds bigint;

ALTER TABLE promotion_creation_idempotency
    ADD CONSTRAINT promotion_creation_idempotency_duration_non_negative
    CHECK (duration_seconds IS NULL OR duration_seconds > 0);
