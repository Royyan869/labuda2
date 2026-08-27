ALTER TABLE fixed_price_sales DROP CONSTRAINT IF EXISTS fixed_price_sales_quantity_available_nonnegative;
ALTER TABLE fixed_price_sales DROP COLUMN IF EXISTS quantity_available;
