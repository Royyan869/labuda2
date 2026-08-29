-- Reverse 000048: Remove selling_surface enforcement

ALTER TABLE products DROP CONSTRAINT IF EXISTS products_selling_surface_check;
ALTER TABLE products DROP COLUMN IF EXISTS selling_surface;
