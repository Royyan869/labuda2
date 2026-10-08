-- ============================================================
-- 000130_canonical_geography_master.up.sql
--
-- CANONICAL GEOGRAPHY AUTHORITY (owner locked):
--   Labuda has exactly ONE Geography Master. Address, Shipping,
--   Recipient snapshots, Promotion and every other consumer reference
--   this table; none of them may define geographic truth of their own.
--
-- The retired 000083 table was city-level only (7 rows). It is replaced
-- here by a single self-referential hierarchy carrying the full
-- Province -> Regency/City -> District -> Village tree plus the village
-- postal code. Identity is the normalized BPS code with dots removed
-- (province 2, regency 4, district 6, village 10 digits), the same code
-- shape the address book and shipping coverage already persist.
--
-- Parent/child integrity is enforced by a self-referencing FK; the level
-- of a row is constrained to the canonical vocabulary and a non-province
-- row must carry a parent.
--
-- The table is populated by the canonical Geography seeder
-- (internal/platform/geography) which is invoked by cmd/migrate and by
-- the test bootstrap. No application code may insert geographic truth
-- ad hoc.
-- ============================================================

DROP TABLE IF EXISTS canonical_geographies CASCADE;

CREATE TABLE canonical_geographies (
    code text PRIMARY KEY,
    level text NOT NULL,
    name text NOT NULL,
    parent_code text REFERENCES canonical_geographies(code) ON DELETE RESTRICT,
    postal_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT canonical_geographies_code_not_empty CHECK (code <> ''),
    CONSTRAINT canonical_geographies_name_not_empty CHECK (name <> ''),
    CONSTRAINT canonical_geographies_level_vocabulary
        CHECK (level IN ('province', 'regency', 'district', 'village')),
    CONSTRAINT canonical_geographies_parent_by_level
        CHECK (
            (level = 'province' AND parent_code IS NULL)
            OR (level <> 'province' AND parent_code IS NOT NULL)
        )
);

CREATE INDEX idx_canonical_geographies_parent ON canonical_geographies(parent_code);
CREATE INDEX idx_canonical_geographies_level ON canonical_geographies(level);
