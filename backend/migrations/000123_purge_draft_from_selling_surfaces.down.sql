-- 000123_purge_draft_from_selling_surfaces.down.sql
--
-- Inverse of the UP migration: restores the pre-000123 vocabulary, where both
-- selling surfaces could be 'draft' (auctions born 'draft', for_sales born
-- 'draft').
--
-- The ROW mapping is NOT reversible: a 'lapsed' auction or a 'withdrawn'
-- for_sale is indistinguishable from a genuine one, so rows stay where the UP
-- migration put them. Only the vocabulary, defaults, and index predicates are
-- restored.

-- ---------------------------------------------------------------------------
-- 1. Auctions: 'draft' returns
-- ---------------------------------------------------------------------------
ALTER TABLE auctions ALTER COLUMN status DROP DEFAULT;
ALTER TABLE auctions DROP CONSTRAINT IF EXISTS auction_order_consistency;
DROP INDEX IF EXISTS uniq_active_auction_per_product;

CREATE TYPE auction_status_enum_old AS ENUM (
    'draft', 'scheduled', 'active', 'waiting_settlement', 'ended', 'cancelled', 'lapsed'
);

ALTER TABLE auctions
    ALTER COLUMN status TYPE auction_status_enum_old
    USING (status::text::auction_status_enum_old);

DROP TYPE auction_status_enum;
ALTER TYPE auction_status_enum_old RENAME TO auction_status_enum;

ALTER TABLE auctions ALTER COLUMN status SET DEFAULT 'draft'::auction_status_enum;

ALTER TABLE auctions ADD CONSTRAINT auction_order_consistency CHECK (
    (order_id IS NULL)
    OR (status = 'ended'::auction_status_enum)
    OR (status = 'waiting_settlement'::auction_status_enum)
);

CREATE UNIQUE INDEX uniq_active_auction_per_product
    ON public.auctions USING btree (product_id)
    WHERE (status = ANY (ARRAY[
        'draft'::auction_status_enum,
        'scheduled'::auction_status_enum,
        'active'::auction_status_enum,
        'waiting_settlement'::auction_status_enum
    ]));

-- ---------------------------------------------------------------------------
-- 2. For Sales: 'draft' returns
-- ---------------------------------------------------------------------------
ALTER TABLE for_sales ALTER COLUMN status DROP DEFAULT;
DROP INDEX IF EXISTS uniq_active_for_sale_per_product;

CREATE TYPE for_sale_status_enum_old AS ENUM ('draft', 'active', 'sold', 'withdrawn');

ALTER TABLE for_sales
    ALTER COLUMN status TYPE for_sale_status_enum_old
    USING (status::text::for_sale_status_enum_old);

DROP TYPE for_sale_status_enum;
ALTER TYPE for_sale_status_enum_old RENAME TO for_sale_status_enum;

ALTER TABLE for_sales ALTER COLUMN status SET DEFAULT 'draft'::for_sale_status_enum;

CREATE UNIQUE INDEX uniq_active_for_sale_per_product
    ON public.for_sales USING btree (product_id)
    WHERE (status = ANY (ARRAY[
        'draft'::for_sale_status_enum,
        'active'::for_sale_status_enum
    ]));
