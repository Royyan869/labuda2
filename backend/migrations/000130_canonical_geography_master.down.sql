-- ============================================================
-- 000130_canonical_geography_master.down.sql
--
-- Reverses the canonical Geography hierarchy back to the retired
-- city-level vocabulary table from 000083. Data is not restored.
-- ============================================================

DROP TABLE IF EXISTS canonical_geographies CASCADE;

CREATE TABLE canonical_geographies (
    city_id text PRIMARY KEY,
    city_name text NOT NULL,
    province_id text NOT NULL,
    province_name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (city_id <> ''),
    CHECK (province_id <> ''),
    CHECK (city_name <> '')
);

CREATE INDEX idx_canonical_geographies_province ON canonical_geographies(province_id);
