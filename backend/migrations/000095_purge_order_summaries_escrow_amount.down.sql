-- Down for 000095: restore obsolete alias (for chain integrity only)
ALTER TABLE order_summaries RENAME COLUMN total_before_coins_amount TO escrow_amount;
-- service_fee_amount and total_payable_amount retained (harmless additive columns)
