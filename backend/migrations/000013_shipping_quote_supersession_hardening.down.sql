DROP INDEX IF EXISTS uq_shipping_quotes_current_active_context;

DROP INDEX IF EXISTS idx_shipping_quotes_current_active_lookup;
DROP INDEX IF EXISTS idx_shipping_quotes_context_lookup;
DROP INDEX IF EXISTS idx_shipping_quotes_superseded_by_id;

ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_used_used_at_check;
ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_active_used_at_check;
ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_reactivation_count_within_reuse_check;
ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_max_reuse_nonnegative_check;
ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_reactivation_count_nonnegative_check;

ALTER TABLE shipping_quotes
    DROP CONSTRAINT IF EXISTS shipping_quotes_superseded_by_id_fkey;

ALTER TABLE shipping_quotes
    ALTER COLUMN expires_at DROP NOT NULL,
    ALTER COLUMN source_id DROP NOT NULL,
    ALTER COLUMN source_type DROP NOT NULL,
    ALTER COLUMN product_id DROP NOT NULL;

ALTER TABLE shipping_quotes
    DROP COLUMN IF EXISTS superseded_by_id,
    DROP COLUMN IF EXISTS superseded_at;
