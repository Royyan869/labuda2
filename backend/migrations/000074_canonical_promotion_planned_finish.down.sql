-- 000074_canonical_promotion_planned_finish down

-- Restore the pre-slice status allowlist (without 'finalizing').
ALTER TABLE promotions
    DROP CONSTRAINT IF EXISTS promotions_status_valid;

ALTER TABLE promotions
    ADD CONSTRAINT promotions_status_valid
    CHECK (status IN ('created', 'funded', 'eligible', 'active', 'paused', 'finalized', 'cancelled', 'failed'));

ALTER TABLE promotions
    DROP CONSTRAINT IF EXISTS promotions_planned_finish_requires_activation;

ALTER TABLE promotions
    DROP CONSTRAINT IF EXISTS promotions_planned_finish_not_before_activation;

ALTER TABLE promotions
    DROP COLUMN IF EXISTS planned_finish_at;