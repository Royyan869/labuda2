-- 000082_promotion_contract_geographies
-- Canonical geographic targeting for Promotion Contract
-- Arbitrary set of City/Kabupaten/Kota — empty = nationwide/unrestricted
-- Uses normalized city_id vocabulary from addresses (province_id/city_id)

CREATE TABLE IF NOT EXISTS promotion_contract_geographies (
    contract_id uuid NOT NULL REFERENCES promotion_contracts(id) ON DELETE CASCADE,
    city_id text NOT NULL,
    city_name text NOT NULL,
    province_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contract_id, city_id),
    CHECK (city_id <> ''),
    CHECK (province_id <> '')
);

CREATE INDEX IF NOT EXISTS idx_promotion_contract_geographies_contract ON promotion_contract_geographies(contract_id);
CREATE INDEX IF NOT EXISTS idx_promotion_contract_geographies_city ON promotion_contract_geographies(city_id);
