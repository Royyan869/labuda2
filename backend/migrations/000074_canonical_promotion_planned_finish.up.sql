-- 000074_canonical_promotion_planned_finish
--
-- Canonical planned-finish boundary for the promotion aggregate.
--
-- Semantics (single meaning only):
--   planned_finish_at = the planned lifecycle/delivery boundary computed by
--   the ONE canonical calculation authority (first activation):
--       planned_finish_at = activated_at + duration_seconds
--   It is NULL when the promotion carries no duration (no planned finish),
--   and it is written EXACTLY ONCE together with activated_at by the
--   activation transaction (DB clock via repository GetDBTime). It is never
--   recomputed and never updated after first write: the repository persists
--   it with COALESCE(planned_finish_at, $new), mirroring activated_at, so
--   the DB is the immutability guard.
--
-- planned_finish_at is a lifecycle/delivery boundary only. It is NOT
-- financial consumption: no billing, ledger, or allocation movement is
-- triggered by it in this slice.

ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS planned_finish_at timestamptz;

-- planned_finish_at may only exist on a promotion that has been activated:
-- the boundary derives from the activation anchor, so it can never appear on
-- a never-activated row.
ALTER TABLE promotions
    ADD CONSTRAINT promotions_planned_finish_requires_activation
    CHECK (planned_finish_at IS NULL OR activated_at IS NOT NULL);

-- The canonical calculation is activated_at + duration with a non-negative
-- duration, so the boundary can never precede the activation anchor.
ALTER TABLE promotions
    ADD CONSTRAINT promotions_planned_finish_not_before_activation
    CHECK (planned_finish_at IS NULL OR planned_finish_at >= activated_at);

-- The canonical state graph gains 'finalizing' (lifecycle completion in
-- progress, the single path into the terminal 'finalized'). The status
-- allowlist (000070) is recreated to admit it.
ALTER TABLE promotions
    DROP CONSTRAINT IF EXISTS promotions_status_valid;

ALTER TABLE promotions
    ADD CONSTRAINT promotions_status_valid
    CHECK (status IN ('created', 'funded', 'eligible', 'active', 'paused', 'finalizing', 'finalized', 'cancelled', 'failed'));