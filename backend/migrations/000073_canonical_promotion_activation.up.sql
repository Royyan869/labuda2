-- 000073_canonical_promotion_activation
--
-- Canonical lifecycle start anchor for the promotion aggregate.
--
-- Semantics (single meaning only):
--   activated_at = the instant the canonical activation authority moved the
--   promotion out of the pre-active state into 'active' (funded -> active,
--   or the matrix-preserved eligible -> active). It is NULL before
--   activation and is written EXACTLY ONCE by the activation transaction
--   (DB clock via repository GetDBTime). It is never recomputed from
--   created_at / funded_at and never updated after first write: the
--   repository persists it with COALESCE(activated_at, $new) so the DB is
--   the immutability guard.
--
-- duration_seconds keeps its existing meaning (planned pacing window
-- attribute). No expiry/finalization/pacing processing is introduced here.

ALTER TABLE promotions
    ADD COLUMN IF NOT EXISTS activated_at timestamptz;

-- A promotion that is 'active' must carry its lifecycle anchor. The single
-- activation authority writes status and anchor in one UPDATE, so there is
-- no transient window. Generic (non-activation) writes that attempt to make
-- a row 'active' without an anchor fail loudly instead of silently creating
-- an anchor-less active state.
ALTER TABLE promotions
    ADD CONSTRAINT promotions_active_requires_activated_at
    CHECK (status <> 'active' OR activated_at IS NOT NULL);
