-- 000070_promotion_canonical_constraints

ALTER TABLE promotions
    ADD CONSTRAINT promotions_status_valid
    CHECK (status IN ('created', 'funded', 'eligible', 'active', 'paused', 'finalized', 'cancelled', 'failed'));

ALTER TABLE promotions
    ADD CONSTRAINT promotions_duration_non_negative
    CHECK (duration_seconds IS NULL OR duration_seconds > 0);

CREATE INDEX IF NOT EXISTS promotion_creation_idempotency_promotion_id_idx
    ON promotion_creation_idempotency (promotion_id);
