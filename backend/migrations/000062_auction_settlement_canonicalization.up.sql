-- 000062_auction_settlement_canonicalization.up.sql
--
-- AUCTION SETTLEMENT CANONICALIZATION (PHASE 1)
--
-- Authority: business truth + canonical implementation plan
-- (docs/audits/CANONICAL_IMPLEMENTATION_PLAN_AUCTION_SETTLEMENT_WINNER_SHIPPING.md).
--
-- 1. New commerce_violations + commerce_restrictions tables replace
--    buyer_bnr_strikes as the violation/restriction authority.
-- 2. auctions gains shipping_resolved_at, seller_action_required,
--    seller_quote_provided; drops settlement_deadline (deadline is derived:
--    end_at + 24h for shipping, shipping_resolved_at + 24h for payment).
-- 3. auction_status_enum is rebuilt without 'expired_bnr'; existing
--    expired_bnr rows are migrated to 'draft' with settlement state cleared.
-- 4. buyer_bnr_strikes is dropped (history is preserved in commerce_violations).

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Commerce violation authority (immutable, append-only)
-- ---------------------------------------------------------------------------
CREATE TABLE commerce_violations (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id),
    violation_type text NOT NULL,
    source_type    text NOT NULL,
    source_id      uuid NOT NULL,
    reason         text,
    metadata       jsonb,
    created_at     timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_commerce_violations_user ON commerce_violations (user_id, created_at DESC);
CREATE INDEX idx_commerce_violations_source ON commerce_violations (source_type, source_id);

-- Immutability guard: violations are append-only history.
CREATE OR REPLACE FUNCTION prevent_commerce_violations_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'commerce_violations rows are immutable (append-only history)';
END;
$$;

CREATE TRIGGER trg_commerce_violations_immutable
    BEFORE UPDATE OR DELETE ON commerce_violations
    FOR EACH ROW
    EXECUTE FUNCTION prevent_commerce_violations_mutation();

-- ---------------------------------------------------------------------------
-- 2. Commerce restriction authority (one active row per user, EXTEND stacking)
-- ---------------------------------------------------------------------------
-- One row per user is enforced by UNIQUE(user_id), which also provides the
-- index for the only access pattern (lookup by user_id — see
-- GetRestrictionForUpdate). "Is the restriction currently active?" is
-- evaluated in application code against commerce_restrictions.restricted_until;
-- PostgreSQL forbids time-dependent predicates (e.g. WHERE restricted_until >
-- NOW()) in partial indexes because NOW() is not IMMUTABLE, and no query
-- filters by restricted_until, so no additional index is justified.
CREATE TABLE commerce_restrictions (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            uuid NOT NULL REFERENCES users(id) UNIQUE,
    violation_count    integer NOT NULL DEFAULT 1,
    restricted_until   timestamptz NOT NULL,
    last_violation_id  uuid NOT NULL REFERENCES commerce_violations(id),
    created_at         timestamptz NOT NULL DEFAULT NOW(),
    updated_at         timestamptz NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- 3. Auctions: settlement canonicalization columns
-- ---------------------------------------------------------------------------
ALTER TABLE auctions
    ADD COLUMN shipping_resolved_at    timestamptz,
    ADD COLUMN seller_action_required  boolean NOT NULL DEFAULT FALSE,
    ADD COLUMN seller_quote_provided   boolean NOT NULL DEFAULT FALSE;

-- ---------------------------------------------------------------------------
-- 4. Rebuild auction_status_enum WITHOUT 'expired_bnr'
-- ---------------------------------------------------------------------------
-- PostgreSQL cannot DROP VALUE from an enum in place. Rebuild the type,
-- migrate data, swap the column (dropping dependents that reference the old
-- type first), then drop the old type and restore the dependents.

-- 4a. Drop dependents that reference the OLD enum type.
ALTER TABLE auctions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE auctions DROP CONSTRAINT auction_order_consistency;
DROP INDEX IF EXISTS uniq_active_auction_per_product;

-- 4b. Data migration first: expired_bnr -> draft with settlement state cleared.
--     An auction returning to DRAFT must carry no current settlement context.
UPDATE auctions
SET status = 'draft',
    order_id = NULL,
    settlement_deadline = NULL,
    current_winner_id = NULL,
    current_bid = NULL,
    shipping_resolved_at = NULL,
    updated_at = NOW()
WHERE status = 'expired_bnr';

-- 4c. Create the canonical enum type.
CREATE TYPE auction_status_enum_new AS ENUM (
    'draft', 'scheduled', 'active', 'waiting_settlement', 'ended', 'cancelled'
);

-- 4d. Swap the column to the new type. The USING cast is safe because no
--     'expired_bnr' rows remain after the data migration above.
ALTER TABLE auctions
    ALTER COLUMN status TYPE auction_status_enum_new
    USING (status::text::auction_status_enum_new);

-- 4e. Rename the type to the canonical name and drop the old enum.
DROP TYPE auction_status_enum;
ALTER TYPE auction_status_enum_new RENAME TO auction_status_enum;

-- 4f. Restore the column default and the order-consistency invariant.
--     A bid-win auction now STAYS in waiting_settlement while its order is
--     bound (until payment success), so the CHECK is relaxed: an order_id is
--     allowed when status is 'ended' OR 'waiting_settlement' (the settlement
--     state). Payment expiry returns the auction to DRAFT and clears order_id
--     in the same transaction.
ALTER TABLE auctions ALTER COLUMN status SET DEFAULT 'draft'::auction_status_enum;

ALTER TABLE auctions ADD CONSTRAINT auction_order_consistency CHECK (
    (order_id IS NULL) OR (status = 'ended'::auction_status_enum) OR (status = 'waiting_settlement'::auction_status_enum)
);

-- Restore the live-auction-per-product partial unique index (unchanged scope:
-- draft/scheduled/active/waiting_settlement are live surfaces).
CREATE UNIQUE INDEX uniq_active_auction_per_product
    ON public.auctions USING btree (product_id)
    WHERE (status = ANY (ARRAY['draft'::auction_status_enum, 'scheduled'::auction_status_enum, 'active'::auction_status_enum, 'waiting_settlement'::auction_status_enum]));

-- ---------------------------------------------------------------------------
-- 5. Drop the stored settlement deadline. Deadline authority is derived:
--    shipping deadline  = auctions.end_at + 24h
--    payment deadline   = shipping_resolved_at + 24h
-- ---------------------------------------------------------------------------
ALTER TABLE auctions DROP COLUMN settlement_deadline;

-- ---------------------------------------------------------------------------
-- 6. Drop obsolete buyer_bnr_strikes (replaced by commerce_violations +
--    commerce_restrictions). No decay, no admin reset, no permanent ban.
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS buyer_bnr_strikes;

COMMIT;
