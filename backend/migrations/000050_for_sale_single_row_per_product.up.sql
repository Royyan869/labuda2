CREATE OR REPLACE FUNCTION enforce_single_for_sale_row_per_product()
RETURNS trigger AS $$
DECLARE
    product_surface text;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.product_id::text, 0));

    SELECT selling_surface INTO product_surface
    FROM products
    WHERE id = NEW.product_id;

    IF EXISTS (
        SELECT 1
        FROM for_sales
        WHERE product_id = NEW.product_id
          AND id <> NEW.id
    ) THEN
        RAISE EXCEPTION
            'product % already has a for_sale row; historical rows are immutable and cannot be replaced',
            NEW.product_id
            USING ERRCODE = 'check_violation';
    END IF;

    IF product_surface IS NULL THEN
        UPDATE products
        SET selling_surface = CASE WHEN TG_TABLE_NAME = 'for_sales' THEN 'for_sale' ELSE 'auction' END,
            updated_at = NOW()
        WHERE id = NEW.product_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_for_sales_single_row_per_product
    BEFORE INSERT OR UPDATE OF product_id ON for_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_single_for_sale_row_per_product();
