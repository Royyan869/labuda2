-- 000126_seller_reputation_identity
-- ============================================================
-- BUSINESS TRUTH
-- The canonical seller identity across commerce is users.id. Every commerce
-- table (orders, order_ratings, refunds, products, auctions, ...) references
-- users(id), and seller_profiles.user_id carries that same identity.
-- seller_profiles.id is ONLY the seller-profile surrogate primary key.
--
-- The original schema encoded the wrong identity: seller_reputation_state
-- referenced seller_profiles(id). The reputation worker therefore aggregated
-- orders / order_ratings / refunds with the profile surrogate key, which never
-- matches rows keyed by users.id — so every rolling metric was always zero and
-- tiers never promoted.
--
-- CONVERGENCE
--   seller_reputation_state.seller_id → users(id)
-- No dual identity, no compatibility column, no alias, no fallback lookup.
--
-- seller_reputation_state is a derived cache (fully overwritten nightly by
-- SellerReputationRecomputeWorker, and on boot). Any pre-existing rows are
-- keyed by the old wrong identity and carry no durable authority, so they are
-- cleared before the FK converges; the recompute repopulates canonical keys.

DELETE FROM seller_reputation_state;

ALTER TABLE seller_reputation_state
    DROP CONSTRAINT IF EXISTS seller_reputation_state_seller_id_fkey;

ALTER TABLE seller_reputation_state
    ADD CONSTRAINT seller_reputation_state_seller_id_fkey
    FOREIGN KEY (seller_id) REFERENCES users(id) ON DELETE CASCADE;
