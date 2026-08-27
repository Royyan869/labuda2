-- Hard purge of rejected seller shipping fields.
-- The canonical model keeps seller-only notes on shipping_options.internal_purpose
-- and removes legacy expedition / estimated-days columns from the shipping tables.

ALTER TABLE shipping_city_overrides
    DROP COLUMN IF EXISTS estimated_days;

ALTER TABLE shipping_coverages
    DROP COLUMN IF EXISTS estimated_days;

ALTER TABLE shipping_options
    DROP COLUMN IF EXISTS expedition_name;
