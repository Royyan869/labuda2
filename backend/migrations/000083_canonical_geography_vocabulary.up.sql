-- 000083_canonical_geography_vocabulary
-- Canonical geographic vocabulary authority for promotion targeting
-- Separate from addresses (viewer instance) — this is the master registry
-- city_id is canonical Kabupaten/Kota identifier (BPS normalized)

CREATE TABLE IF NOT EXISTS canonical_geographies (
    city_id text PRIMARY KEY,
    city_name text NOT NULL,
    province_id text NOT NULL,
    province_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (city_id <> ''),
    CHECK (province_id <> ''),
    CHECK (city_name <> '')
);

CREATE INDEX IF NOT EXISTS idx_canonical_geographies_province ON canonical_geographies(province_id);

-- Seed minimal canonical cities used in tests and common regions
-- 32 = West Java, 31 = Jakarta, 51 = Bali, 35 = East Java
INSERT INTO canonical_geographies (city_id, city_name, province_id, province_name) VALUES
    ('3204', 'Kabupaten Bandung', '32', 'Jawa Barat'),
    ('3171', 'Kota Jakarta Selatan', '31', 'DKI Jakarta'),
    ('3172', 'Kota Jakarta Timur', '31', 'DKI Jakarta'),
    ('3173', 'Kota Jakarta Pusat', '31', 'DKI Jakarta'),
    ('5103', 'Kabupaten Gianyar', '51', 'Bali'),
    ('3501', 'Kabupaten Pacitan', '35', 'Jawa Timur'),
    ('3502', 'Kabupaten Ponorogo', '35', 'Jawa Timur')
ON CONFLICT (city_id) DO NOTHING;
