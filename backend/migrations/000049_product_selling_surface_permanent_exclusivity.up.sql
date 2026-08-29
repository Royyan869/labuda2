-- 000049: Permanent selling-surface exclusivity trigger
--
-- RULE (LOCKED, NON-NEGOTIABLE):
--   A Product may belong to EXACTLY ONE selling surface TYPE (for_sale OR auction)
--   for its entire lifecycle. Once claimed, selling_surface is IMMUTABLE.
--
-- This replaces the weaker trigger from 000047 which only checked active statuses.
-- The new trigger enforces:
--   1. A for_sales row can only reference a Product with selling_surface = 'for_sale'
--   2. An auctions row can only reference a Product with selling_surface = 'auction'
--   3. selling_surface can only transition from NULL → non-NULL, never back

-- Step 1: Drop the old active-only trigger and function
DROP TRIGGER IF EXISTS trg_for_sales_single_active_channel ON for_sales;
DROP TRIGGER IF EXISTS trg_auctions_single_active_channel ON auctions;
DROP FUNCTION IF EXISTS enforce_single_active_sale_channel_per_product();

-- Step 2: Create the permanent exclusivity function.
-- This enforces cross-type exclusion: a Product claimed as 'for_sale' can NEVER
-- have an Auction row, and a Product claimed as 'auction' can NEVER have a ForSale row.
-- Products with NULL selling_surface (unclaimed) are allowed in either table —
-- the application layer (ClaimSellingSurface) is responsible for setting selling_surface
-- atomically during creation. The database prevents the cross-type bypass.
CREATE OR REPLACE FUNCTION enforce_permanent_selling_surface_exclusivity()
RETURNS trigger AS $$
DECLARE
    product_surface text;
BEGIN
    -- Get the product's selling_surface
    SELECT selling_surface INTO product_surface
    FROM products
    WHERE id = NEW.product_id;

    IF TG_TABLE_NAME = 'for_sales' THEN
        -- REJECT if Product is claimed as 'auction' (cross-type)
        IF product_surface = 'auction' THEN
            RAISE EXCEPTION
                'product % cannot be used for for_sale: permanently claimed as auction',
                NEW.product_id
                USING ERRCODE = 'check_violation';
        END IF;
    ELSIF TG_TABLE_NAME = 'auctions' THEN
        -- REJECT if Product is claimed as 'for_sale' (cross-type)
        IF product_surface = 'for_sale' THEN
            RAISE EXCEPTION
                'product % cannot be used for auction: permanently claimed as for_sale',
                NEW.product_id
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Step 3: Create triggers on for_sales and auctions
CREATE TRIGGER trg_for_sales_permanent_exclusivity
    BEFORE INSERT OR UPDATE ON for_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_permanent_selling_surface_exclusivity();

CREATE TRIGGER trg_auctions_permanent_exclusivity
    BEFORE INSERT OR UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION enforce_permanent_selling_surface_exclusivity();

-- Step 4: Create a trigger on products to prevent selling_surface from reverting to NULL
-- or changing type after being claimed.
CREATE OR REPLACE FUNCTION enforce_selling_surface_immutability()
RETURNS trigger AS $$
BEGIN
    -- If the old value was non-NULL (claimed), the new value must be identical.
    IF OLD.selling_surface IS NOT NULL AND NEW.selling_surface IS DISTINCT FROM OLD.selling_surface THEN
        RAISE EXCEPTION
            'selling_surface is immutable once claimed: cannot change from % to %',
            OLD.selling_surface, NEW.selling_surface
            USING ERRCODE = 'check_violation';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_products_selling_surface_immutability
    BEFORE UPDATE OF selling_surface ON products
    FOR EACH ROW EXECUTE FUNCTION enforce_selling_surface_immutability();
