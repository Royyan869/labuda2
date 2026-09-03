-- 000062_auction_settlement_canonicalization.down.sql
--
-- Reverses 000062. This is a rollback-safety migration, not a supported
-- runtime path: 'expired_bnr' is reintroduced as an enum value and the
-- settlement_deadline column is restored as nullable.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. Restore buyer_bnr_strikes (legacy table shape from 000001)
-- ---------------------------------------------------------------------------
CREATE TABLE buyer_bnr_strikes (
    id           uuid DEFAULT gen_random_uuid() NOT NULL,
    buyer_id     uuid NOT NULL,
    auction_id   uuid NOT NULL,
    struck_at    timestamp with time zone DEFAULT now(),
    decayed_at   timestamp with time zone,
    appeal_id    uuid,
    admin_reset  boolean DEFAULT false NOT NULL,
    CONSTRAINT buyer_bnr_strikes_pkey PRIMARY KEY (id),
    CONSTRAINT buyer_bnr_strikes_auction_id_key UNIQUE (auction_id),
    CONSTRAINT buyer_bnr_strikes_auction_id_fkey FOREIGN KEY (auction_id) REFERENCES auctions(id),
    CONSTRAINT buyer_bnr_strikes_buyer_id_fkey FOREIGN KEY (buyer_id) REFERENCES users(id)
);

CREATE INDEX idx_buyer_bnr_strikes_buyer_active
    ON buyer_bnr_strikes (buyer_id, struck_at)
    WHERE ((decayed_at IS NULL) AND (admin_reset = false));

-- ---------------------------------------------------------------------------
-- 2. Restore auctions.settlement_deadline
-- ---------------------------------------------------------------------------
ALTER TABLE auctions ADD COLUMN settlement_deadline timestamp with time zone;

-- ---------------------------------------------------------------------------
-- 3. Rebuild auction_status_enum WITH 'expired_bnr' (restoring legacy shape)
-- ---------------------------------------------------------------------------
-- Drop dependents that reference the CURRENT (canonical) enum type.
ALTER TABLE auctions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE auctions DROP CONSTRAINT auction_order_consistency;
DROP INDEX IF EXISTS uniq_active_auction_per_product;

CREATE TYPE auction_status_enum_legacy AS ENUM (
    'draft', 'scheduled', 'active', 'waiting_settlement', 'expired_bnr', 'ended', 'cancelled'
);

ALTER TABLE auctions
    ALTER COLUMN status TYPE auction_status_enum_legacy
    USING (status::text::auction_status_enum_legacy);

DROP TYPE auction_status_enum;
ALTER TYPE auction_status_enum_legacy RENAME TO auction_status_enum;

-- Restore the legacy default, order-consistency CHECK (order only on ended),
-- and the live-auction partial unique index.
ALTER TABLE auctions ALTER COLUMN status SET DEFAULT 'draft'::auction_status_enum;

ALTER TABLE auctions ADD CONSTRAINT auction_order_consistency CHECK (
    (order_id IS NULL) OR (status = 'ended'::auction_status_enum)
);

CREATE UNIQUE INDEX uniq_active_auction_per_product
    ON public.auctions USING btree (product_id)
    WHERE (status = ANY (ARRAY['draft'::auction_status_enum, 'scheduled'::auction_status_enum, 'active'::auction_status_enum, 'waiting_settlement'::auction_status_enum]));

-- ---------------------------------------------------------------------------
-- 4. Drop auctions canonicalization columns
-- ---------------------------------------------------------------------------
ALTER TABLE auctions
    DROP COLUMN IF EXISTS seller_quote_provided,
    DROP COLUMN IF EXISTS seller_action_required,
    DROP COLUMN IF EXISTS shipping_resolved_at;

-- ---------------------------------------------------------------------------
-- 5. Drop commerce restrictions/violations (restoring legacy state)
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS commerce_restrictions;
DROP TABLE IF EXISTS commerce_violations;
DROP FUNCTION IF EXISTS prevent_commerce_violations_mutation();

COMMIT;
