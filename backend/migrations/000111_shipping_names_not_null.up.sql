-- 000111_shipping_names_not_null
-- Same bug class as 000110, closed proactively: shipping_coverages.province_name
-- and shipping_city_overrides.city_name are nullable text columns scanned into
-- non-nullable Go strings. Any NULL row crashes the seller shipping reads with
-- "cannot scan NULL into *string" (500 on every seller shipping surface).
-- BUSINESS TRUTH: a destination/city row always carries its display name — an
-- unnamed row is an empty string, never a missing value. Display-only columns;
-- materializing legacy NULLs to '' is lossless for the UI.

UPDATE shipping_coverages SET province_name = '' WHERE province_name IS NULL;
UPDATE shipping_city_overrides SET city_name = '' WHERE city_name IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'shipping_coverages'
          AND column_name = 'province_name'
          AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE shipping_coverages
            ALTER COLUMN province_name SET DEFAULT '';
        ALTER TABLE shipping_coverages
            ALTER COLUMN province_name SET NOT NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'shipping_city_overrides'
          AND column_name = 'city_name'
          AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE shipping_city_overrides
            ALTER COLUMN city_name SET DEFAULT '';
        ALTER TABLE shipping_city_overrides
            ALTER COLUMN city_name SET NOT NULL;
    END IF;
END $$;
