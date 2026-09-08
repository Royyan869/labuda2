-- 000081_promotion_competing_aggregate_purge
--
-- HARD CONVERGENCE: drop competing promotions aggregate after analytics has
-- converged to promotion_contracts (000080).
--
-- Drops:
--   promotions
--   promotion_funding
--   promotion_creation_idempotency
-- and related indexes/triggers. Historical data under the competing model is
-- not migrated (ledger is the only financial truth; no production data must be preserved).

DROP TABLE IF EXISTS promotion_funding CASCADE;
DROP TABLE IF EXISTS promotion_creation_idempotency CASCADE;
DROP TABLE IF EXISTS promotions CASCADE;

-- Canonical delivery event FK already points to promotion_contracts after 000080
-- No further action needed for canonical_promotion_delivery_events
