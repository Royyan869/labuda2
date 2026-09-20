-- Down migration: restore the historical ten-value preparation_time_enum for
-- chain integrity only (the 000001 shape).
--
-- WARNING: the six legacy values (same_day, 1_2_days, 3_5_days, 6_10_days,
-- 11_15_days, more_than_15_days) are OBSOLETE — no production producer can
-- create them. Restored only so `migrate down` reproduces the pre-000103 state.

CREATE TYPE preparation_time_enum_restore AS ENUM (
    'same_day',
    '1_2_days',
    '3_5_days',
    '6_10_days',
    '11_15_days',
    'more_than_15_days',
    'immediate',
    'short',
    'medium',
    'long'
);

ALTER TABLE products
    ALTER COLUMN preparation_time TYPE preparation_time_enum_restore
    USING preparation_time::text::preparation_time_enum_restore;

ALTER TABLE orders
    ALTER COLUMN preparation_time_snapshot TYPE preparation_time_enum_restore
    USING preparation_time_snapshot::text::preparation_time_enum_restore;

DROP TYPE IF EXISTS preparation_time_enum;
ALTER TYPE preparation_time_enum_restore RENAME TO preparation_time_enum;
