-- Stage 8: Product content authority convergence
-- Auctions no longer maintain their own title/description/preparation fields.
-- Product is the sole canonical authority for all product identity/content.
-- Drop dead media tables (fixed_price_sale_media, auction_media).

BEGIN;

-- ============================================================================
-- AUCTION CONTENT AUTHORITY CONVERGENCE
-- ============================================================================

-- Drop auctions.title, auctions.description (duplicate authority → Product)
ALTER TABLE auctions DROP COLUMN IF EXISTS title;
ALTER TABLE auctions DROP COLUMN IF EXISTS description;

-- Drop auctions.preparation_time, auctions.preparation_note (zombie fields)
ALTER TABLE auctions DROP COLUMN IF EXISTS preparation_time;
ALTER TABLE auctions DROP COLUMN IF EXISTS preparation_note;

-- ============================================================================
-- DEAD MEDIA TABLES
-- ============================================================================

-- fixed_price_sale_media: zero production writers, one stale reader (chat)
-- auction_media: zero production writers, one stale reader (chat)
-- Both are frozen snapshots from migration 000023, never updated.
-- Canonical media authority is products.media_urls.

DROP TABLE IF EXISTS fixed_price_sale_media;
DROP TABLE IF EXISTS auction_media;

COMMIT;
