-- Reverse 000049: Remove permanent exclusivity triggers

DROP TRIGGER IF EXISTS trg_products_selling_surface_immutability ON products;
DROP FUNCTION IF EXISTS enforce_selling_surface_immutability();

DROP TRIGGER IF EXISTS trg_for_sales_permanent_exclusivity ON for_sales;
DROP TRIGGER IF EXISTS trg_auctions_permanent_exclusivity ON auctions;
DROP FUNCTION IF EXISTS enforce_permanent_selling_surface_exclusivity();

-- Restore the old active-only trigger from 000047
CREATE OR REPLACE FUNCTION enforce_single_active_sale_channel_per_product()
RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'for_sales' THEN
        IF NEW.status IN ('draft', 'active') THEN
            IF EXISTS (
                SELECT 1 FROM auctions
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
            ) THEN
                RAISE EXCEPTION 'product % already has an active auction; cannot activate a for_sale for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'auctions' THEN
        IF NEW.status IN ('draft', 'scheduled', 'active', 'waiting_settlement') THEN
            IF EXISTS (
                SELECT 1 FROM for_sales
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'active')
            ) THEN
                RAISE EXCEPTION 'product % already has an active for_sale; cannot activate an auction for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_for_sales_single_active_channel
    BEFORE INSERT OR UPDATE ON for_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

CREATE TRIGGER trg_auctions_single_active_channel
    BEFORE INSERT OR UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();
