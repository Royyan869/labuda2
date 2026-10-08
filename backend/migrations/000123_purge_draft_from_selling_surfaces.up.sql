-- 000123_purge_draft_from_selling_surfaces.up.sql
--
-- Owner decision (Oct 2026): CREATING A SELLING SURFACE IS PUBLISHING IT.
-- There is no draft stage anywhere in the Auction or For Sale lifecycle:
--
--   auctions.status   born 'scheduled' (create = publish)
--   for_sales.status  born 'active'    (create = publish)
--
-- PostgreSQL cannot DROP a value from an enum in place. Both enum types are
-- therefore rebuilt destructively - the proven pattern from 000047/000062:
-- drop the dependents that reference the old type (column defaults, the
-- order-consistency CHECK, the live-surface partial unique indexes), migrate
-- the data, swap the column, drop the old type, restore the dependents with
-- the canonical vocabulary.
--
-- Legacy row mapping (these rows can never be read as live surfaces):
--   auctions.status   'draft' -> 'lapsed'    (never went live; hidden from every
--                                             viewer surface; relistable - 000122)
--   for_sales.status  'draft' -> 'withdrawn' (never published; stock untouched)
--
-- Out of scope (different lifecycle): external_products.review_status and the
-- pricing/promotion draft vocabulary are untouched.

-- ---------------------------------------------------------------------------
-- 1. Auctions: 'draft' is purged (born 'scheduled')
-- ---------------------------------------------------------------------------

-- 1a. Drop the dependents that reference the OLD enum type.
ALTER TABLE auctions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE auctions DROP CONSTRAINT IF EXISTS auction_order_consistency;
DROP INDEX IF EXISTS uniq_active_auction_per_product;

-- 1b. Data first: a draft auction never went live. 'lapsed' is the canonical
--     never-live/relistable state; 'cancelled' stays reserved for
--     seller-initiated and moderation/admin outcomes (see 000122).
UPDATE auctions
SET status = 'lapsed',
    updated_at = NOW()
WHERE status = 'draft';

-- 1c. Rebuild the enum without 'draft'; 'lapsed' (000122) stays.
CREATE TYPE auction_status_enum_new AS ENUM (
    'scheduled', 'active', 'waiting_settlement', 'ended', 'cancelled', 'lapsed'
);

ALTER TABLE auctions
    ALTER COLUMN status TYPE auction_status_enum_new
    USING (status::text::auction_status_enum_new);

DROP TYPE auction_status_enum;
ALTER TYPE auction_status_enum_new RENAME TO auction_status_enum;

-- 1d. Restore the default to the born state and the order-consistency rule.
ALTER TABLE auctions ALTER COLUMN status SET DEFAULT 'scheduled'::auction_status_enum;

ALTER TABLE auctions ADD CONSTRAINT auction_order_consistency CHECK (
    (order_id IS NULL)
    OR (status = 'ended'::auction_status_enum)
    OR (status = 'waiting_settlement'::auction_status_enum)
);

-- 1e. Restore the one-live-auction-per-product index without 'draft'
--     (scheduled / active / waiting_settlement are the live surfaces).
CREATE UNIQUE INDEX uniq_active_auction_per_product
    ON public.auctions USING btree (product_id)
    WHERE (status = ANY (ARRAY[
        'scheduled'::auction_status_enum,
        'active'::auction_status_enum,
        'waiting_settlement'::auction_status_enum
    ]));

-- ---------------------------------------------------------------------------
-- 2. For Sales: 'draft' is purged (born 'active')
-- ---------------------------------------------------------------------------

-- 2a. Drop the dependents that reference the OLD enum type.
ALTER TABLE for_sales ALTER COLUMN status DROP DEFAULT;
DROP INDEX IF EXISTS uniq_active_for_sale_per_product;

-- 2b. Data first: a draft for_sale was never published. 'withdrawn' is the
--     canonical not-live state (nothing is exposed, no stock is claimed).
UPDATE for_sales
SET status = 'withdrawn',
    updated_at = NOW()
WHERE status = 'draft';

-- 2c. Rebuild the enum without 'draft'.
CREATE TYPE for_sale_status_enum_new AS ENUM ('active', 'sold', 'withdrawn');

ALTER TABLE for_sales
    ALTER COLUMN status TYPE for_sale_status_enum_new
    USING (status::text::for_sale_status_enum_new);

DROP TYPE for_sale_status_enum;
ALTER TYPE for_sale_status_enum_new RENAME TO for_sale_status_enum;

-- 2d. Restore the default to the born state.
ALTER TABLE for_sales ALTER COLUMN status SET DEFAULT 'active'::for_sale_status_enum;

-- 2e. Restore the one-active-for-sale-per-product index without 'draft'.
CREATE UNIQUE INDEX uniq_active_for_sale_per_product
    ON public.for_sales USING btree (product_id)
    WHERE (status = ANY (ARRAY['active'::for_sale_status_enum]));
