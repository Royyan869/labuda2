-- 000059_add_gateway_reference_no.down.sql
-- PAYOUT-01B: Drop the gateway_reference_no column added for Iris contract correction.

DROP INDEX IF EXISTS idx_withdrawals_gateway_reference_no;
ALTER TABLE withdrawals DROP COLUMN IF EXISTS gateway_reference_no;
