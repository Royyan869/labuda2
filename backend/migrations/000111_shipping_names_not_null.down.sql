-- 000111 down: restore nullable shape. Written '' stays '' — no NULL resurrection.

ALTER TABLE shipping_city_overrides ALTER COLUMN city_name DROP DEFAULT;
ALTER TABLE shipping_city_overrides ALTER COLUMN city_name DROP NOT NULL;
ALTER TABLE shipping_coverages ALTER COLUMN province_name DROP DEFAULT;
ALTER TABLE shipping_coverages ALTER COLUMN province_name DROP NOT NULL;
