-- 000078_promotion_contract_target_queue
--
-- PROMOTION PHASE — ROLLING TARGET QUEUE (CANONICAL §12)
--
-- Internal Promotion subjects: For Sale, Auction.
-- External Promotion subjects: Event, Business.
--
-- A promotion contract may maintain a rolling queue of UP TO 10 configured
-- targets. Delivery resolves one currently effective target; unavailable /
-- ineligible targets are skipped without duplicating Commerce lifecycle
-- authority inside Promotion. Position order is the queue order.
--
-- This table is the queue authority. Single-target columns are NOT added to
-- promotion_contracts — the queue is the only target model. Legacy
-- promotion_instances / promotions.target_id single-target designs remain
-- untouched here (purge is a later scope); this table is additive and
-- forward-only.
--
-- Constraints:
--   - position 0..9 (max 10 entries per contract enforced by UNIQUE(contract_id, position) + app check COUNT(*) < 10)
--   - UNIQUE(contract_id, target_id) — a target appears at most once per queue
--   - status tracking is NOT duplicated: operability is deferred to canonical
--     For Sale / Auction authority at delivery/qualification time.
--   - Added as append-only; removal is DELETE, reordering is transactional
--     position rewrite (no gaps required — resolver is ORDER BY position).

CREATE TABLE IF NOT EXISTS promotion_contract_targets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id uuid NOT NULL REFERENCES promotion_contracts(id) ON DELETE CASCADE,
    target_type promotion_target_type_enum NOT NULL,
    target_id uuid NOT NULL,
    position int NOT NULL,
    added_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promotion_contract_targets_position_range CHECK (position >= 0 AND position < 10),
    CONSTRAINT promotion_contract_targets_unique_target UNIQUE (contract_id, target_id),
    CONSTRAINT promotion_contract_targets_unique_position UNIQUE (contract_id, position)
);

CREATE INDEX IF NOT EXISTS idx_promotion_contract_targets_contract_position
    ON promotion_contract_targets (contract_id, position);

CREATE INDEX IF NOT EXISTS idx_promotion_contract_targets_target
    ON promotion_contract_targets (target_type, target_id);
