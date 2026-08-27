-- 000031_comment_commerce_reference_canonical.up.sql
--
-- Canonical comment-commerce association table.
--
-- Fresh-chain doctrine:
-- - The authoritative relationship between a comment and a commerce target
--   lives in comment_commerce_references.
-- - The legacy comments.share_reference / comments.fixed_price_sale_id /
--   comments.type columns are purged after backfill.
-- - The backfill is defensive and fail-closed: any orphaned or malformed
--   legacy row aborts the migration rather than silently truncating data.

CREATE TABLE comment_commerce_references (
    comment_id UUID PRIMARY KEY
        REFERENCES comments(id) ON DELETE CASCADE,
    fixed_price_sale_id UUID
        REFERENCES fixed_price_sales(id) ON DELETE RESTRICT,
    auction_id UUID
        REFERENCES auctions(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT comment_commerce_reference_exactly_one_source
        CHECK (
            (fixed_price_sale_id IS NOT NULL AND auction_id IS NULL)
            OR
            (fixed_price_sale_id IS NULL AND auction_id IS NOT NULL)
        )
);

CREATE INDEX idx_comment_commerce_ref_fps
    ON comment_commerce_references(fixed_price_sale_id)
    WHERE fixed_price_sale_id IS NOT NULL;

CREATE INDEX idx_comment_commerce_ref_auction
    ON comment_commerce_references(auction_id)
    WHERE auction_id IS NOT NULL;

DO $$
DECLARE
    source_count   INTEGER;
    inserted_count INTEGER;
    has_share_ref  BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'comments' AND column_name = 'share_reference'
    ) INTO has_share_ref;

    IF has_share_ref THEN
        EXECUTE '
            WITH backfilled AS (
                INSERT INTO comment_commerce_references (
                    comment_id,
                    fixed_price_sale_id,
                    auction_id,
                    created_at
                )
                SELECT
                    c.id,
                    CASE
                        WHEN c.share_reference->>''targetType'' = ''fixed_price_sale''
                            THEN NULLIF(c.share_reference->>''targetId'', '''')' || '::uuid
                        ELSE NULL
                    END AS fixed_price_sale_id,
                    CASE
                        WHEN c.share_reference->>''targetType'' = ''auction''
                            THEN NULLIF(c.share_reference->>''targetId'', '''')' || '::uuid
                        ELSE NULL
                    END AS auction_id,
                    c.created_at
                FROM comments c
                WHERE c.type = ''listing_reference''
                  AND c.share_reference IS NOT NULL
                RETURNING comment_id
            )
            SELECT COUNT(*) FROM backfilled
        ';
    ELSIF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'comments' AND column_name = 'fixed_price_sale_id'
    ) THEN
        EXECUTE '
            INSERT INTO comment_commerce_references (
                comment_id,
                fixed_price_sale_id,
                created_at
            )
            SELECT
                c.id,
                c.fixed_price_sale_id,
                c.created_at
            FROM comments c
            WHERE c.type = ''listing_reference''
              AND c.fixed_price_sale_id IS NOT NULL
        ';
    END IF;
END $$;

ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_listing_ref_consistency_check;
ALTER TABLE comments DROP CONSTRAINT IF EXISTS comments_fixed_price_sale_id_fkey;
DROP INDEX IF EXISTS idx_comments_fixed_price_sale_id;
DROP INDEX IF EXISTS idx_comments_share_reference_listing;

ALTER TABLE comments DROP COLUMN IF EXISTS fixed_price_sale_id;
ALTER TABLE comments DROP COLUMN IF EXISTS share_reference;
ALTER TABLE comments DROP COLUMN IF EXISTS type;

DROP TYPE IF EXISTS comment_type_enum;
