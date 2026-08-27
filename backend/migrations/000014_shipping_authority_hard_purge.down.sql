ALTER TABLE shipping_options
    ADD COLUMN IF NOT EXISTS expedition_name text;

ALTER TABLE shipping_coverages
    ADD COLUMN IF NOT EXISTS estimated_days text;

ALTER TABLE shipping_city_overrides
    ADD COLUMN IF NOT EXISTS estimated_days text;
