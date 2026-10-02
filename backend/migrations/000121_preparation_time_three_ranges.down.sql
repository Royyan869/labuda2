-- Down migration: restore the four-value preparation_time_enum
-- (immediate | short | medium | long) for chain integrity only.
--
-- Reverse mapping (coarse — the old vocabulary is coarser than the 3 ranges):
--   1_3_days → short    (1-2 days, closest to the 1-3 day range)
--   4_7_days → medium   (3-5 days, closest to the 4-7 day range)
--   8_15_days → long    (7+ days, the old longest bucket)
--
-- WARNING: immediate can never be recreated — a down migration returns rows to
-- the pre-000121 four-value shape only.

CREATE TYPE preparation_time_enum_restore AS ENUM (
    'immediate',
    'short',
    'medium',
    'long'
);

ALTER TABLE products
    ALTER COLUMN preparation_time TYPE preparation_time_enum_restore
    USING (
        CASE preparation_time::text
            WHEN '4_7_days'  THEN 'medium'
            WHEN '8_15_days' THEN 'long'
            ELSE 'short'  -- 1_3_days (and defensive default)
        END
    )::preparation_time_enum_restore;

-- NULL snapshot must stay NULL.
ALTER TABLE orders
    ALTER COLUMN preparation_time_snapshot TYPE preparation_time_enum_restore
    USING (
        CASE
            WHEN preparation_time_snapshot IS NULL THEN NULL
            WHEN preparation_time_snapshot::text = '4_7_days'  THEN 'medium'
            WHEN preparation_time_snapshot::text = '8_15_days' THEN 'long'
            ELSE 'short'  -- 1_3_days (and defensive default)
        END
    )::preparation_time_enum_restore;

DROP TYPE IF EXISTS preparation_time_enum;
ALTER TYPE preparation_time_enum_restore RENAME TO preparation_time_enum;
