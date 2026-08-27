DELETE FROM payment_methods WHERE method_code IN ('dana', 'convenience_store');

ALTER TABLE payment_methods DROP CONSTRAINT IF EXISTS payment_methods_rate_source_check;

ALTER TABLE payment_methods
    DROP COLUMN IF EXISTS merchant_verified_at,
    DROP COLUMN IF EXISTS rate_source_note,
    DROP COLUMN IF EXISTS rate_source;
