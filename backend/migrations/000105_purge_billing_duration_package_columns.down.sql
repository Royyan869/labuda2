-- 000105_purge_billing_duration_package_columns (DOWN)
--
-- Restore the legacy duration-package columns for rollback safety.

ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS event_date timestamptz;
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS unlock_date timestamptz;
ALTER TABLE billing_transactions ADD COLUMN IF NOT EXISTS unlocked_at timestamptz;
