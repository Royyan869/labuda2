-- 000126_seller_reputation_identity (down)
-- Restore the original (buggy) FK target. The reputation state rows are derived
-- cache and are cleared first so the restored FK is always satisfiable.

DELETE FROM seller_reputation_state;

ALTER TABLE seller_reputation_state
    DROP CONSTRAINT IF EXISTS seller_reputation_state_seller_id_fkey;

ALTER TABLE seller_reputation_state
    ADD CONSTRAINT seller_reputation_state_seller_id_fkey
    FOREIGN KEY (seller_id) REFERENCES seller_profiles(id);
