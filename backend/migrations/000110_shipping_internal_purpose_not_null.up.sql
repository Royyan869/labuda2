-- 000110_shipping_internal_purpose_not_null
-- BUSINESS TRUTH: shipping_options.internal_purpose is the seller-private
-- note ("kantong besar", "untuk 1 ekor", "untuk 10 ekor", ...). It exists on
-- every canonical shipping option — an empty note is an empty string, never
-- a missing value. Legacy rows created before application code wrote the
-- column carry NULL, which crashes repository scans ("cannot scan NULL into
-- *string") and takes down every seller shipping surface (options list,
-- create/edit for sale, auction setup) with a 500.
-- This migration materializes legacy NULLs to '' and enforces the invariant
-- at the schema level. The unique index already treats NULL as '' via
-- COALESCE, so this change does not alter uniqueness semantics.

UPDATE shipping_options
SET internal_purpose = ''
WHERE internal_purpose IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'shipping_options'
          AND column_name = 'internal_purpose'
          AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE shipping_options
            ALTER COLUMN internal_purpose SET NOT NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'shipping_options'
          AND column_name = 'internal_purpose'
          AND column_default IS NOT NULL
    ) THEN
        ALTER TABLE shipping_options
            ALTER COLUMN internal_purpose SET DEFAULT '';
    END IF;
END $$;
