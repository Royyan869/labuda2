-- 000048: Product selling-surface exclusivity invariant
--
-- RULE: A Product may belong to EXACTLY ONE selling surface (for_sale OR auction).
-- The selling_surface column on products is the single source of truth for this
-- ownership. Both ForSale and Auction creation paths MUST set this column
-- atomically within the same transaction that creates the selling surface.
--
-- This prevents the rejected design where a Product has both a ForSale and an
-- Auction simultaneously.

-- 1. Add the selling_surface column (nullable initially for backfill)
ALTER TABLE products
    ADD COLUMN selling_surface text;

-- 2. Backfill from existing data
--    Products with active/draft ForSales → 'for_sale'
--    Products with active/draft/scheduled/waiting Auctions → 'auction'
--    Products with both (invalid) → NULL (needs manual resolution)
--    Products with neither → NULL

UPDATE products p
SET selling_surface = 'for_sale'
WHERE EXISTS (
    SELECT 1 FROM for_sales fs
    WHERE fs.product_id = p.id
    AND fs.status IN ('draft', 'active')
)
AND NOT EXISTS (
    SELECT 1 FROM auctions a
    WHERE a.product_id = p.id
    AND a.status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
);

UPDATE products p
SET selling_surface = 'auction'
WHERE EXISTS (
    SELECT 1 FROM auctions a
    WHERE a.product_id = p.id
    AND a.status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
)
AND NOT EXISTS (
    SELECT 1 FROM for_sales fs
    WHERE fs.product_id = p.id
    AND fs.status IN ('draft', 'active')
);

-- Products with both surfaces are INVALID (rejected coexistence design).
-- Since this is a from-zero project with no production data, set them to NULL.
-- The new permanent-exclusivity trigger (000049) will require proper claiming
-- before any surface insert.

-- Products with neither surface remain NULL (available for attachment)

-- 3. Add CHECK constraint to enforce valid values
ALTER TABLE products
    ADD CONSTRAINT products_selling_surface_check
    CHECK (selling_surface IN ('for_sale', 'auction') OR selling_surface IS NULL);

-- 4. Add comment for documentation
COMMENT ON COLUMN products.selling_surface IS
    'Canonical selling-surface ownership. NULL = unattached. ';

