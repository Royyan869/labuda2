-- Rollback for Stage 9: Product content authority convergence

BEGIN;

-- Recreate dead media tables (minimal schema)
CREATE TABLE IF NOT EXISTS fixed_price_sale_media (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    fixed_price_sale_id uuid NOT NULL,
    media_url text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE IF NOT EXISTS auction_media (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    auction_id uuid NOT NULL,
    media_url text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

-- Recreate auction content columns
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT '';
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS preparation_time preparation_time_enum NOT NULL DEFAULT 'immediate';
ALTER TABLE auctions ADD COLUMN IF NOT EXISTS preparation_note text;

COMMIT;
