-- 000105_purge_billing_duration_package_columns
--
-- PURGE Design A residue from billing_transactions.
--
-- The event_date, unlock_date, and unlocked_at columns were part of the
-- obsolete Promotion Duration-Package model (Design A). They carried
-- duration-entitlement state (activation time, unlock time, unlock date)
-- that is no longer relevant under the canonical CPM impression-based
-- Promotion model (Design B).
--
-- No current domain writes or reads these columns for business logic.
-- The BillingTransaction entity and repository no longer map them.
--
-- Forward-only. The billing_transactions table retains all other columns.

ALTER TABLE billing_transactions DROP COLUMN IF EXISTS event_date;
ALTER TABLE billing_transactions DROP COLUMN IF EXISTS unlock_date;
ALTER TABLE billing_transactions DROP COLUMN IF EXISTS unlocked_at;
