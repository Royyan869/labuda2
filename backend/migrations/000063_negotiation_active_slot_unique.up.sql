-- 000063_negotiation_active_slot_unique
--
-- N8-C: Structural concurrency authority for negotiation start.
--
-- Canonical invariant (buyer × for_sale slot):
--   There MUST NOT be more than one row satisfying
--     status IN ('active','accepted') AND order_id IS NULL
--   for the same (resource_type, for_sale_id, buyer_id).
--
-- Occupies slot:
--   active                → yes
--   accepted + order_id NULL   → yes
-- Excluded:
--   accepted + order_id NOT NULL → no (settled)
--   cancelled / expired            → no
--
-- Predicate intentionally includes BOTH active and accepted
-- and explicitly excludes settled via order_id IS NULL.
-- Database is final authority; application lookup is defense-in-depth.
CREATE UNIQUE INDEX ux_negotiation_one_active_per_buyer_for_sale
    ON negotiation_sessions (resource_type, for_sale_id, buyer_id)
    WHERE status IN ('active', 'accepted') AND order_id IS NULL;
