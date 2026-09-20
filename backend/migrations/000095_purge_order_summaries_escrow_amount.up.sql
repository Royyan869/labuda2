-- 000095 PURGE ORDER_SUMMARIES ESCROW_AMOUNT — CANONICAL CONVERGENCE FIN-R01E
-- order_summaries.escrow_amount is obsolete projection alias of orders.total_before_coins_amount (PD+S).
-- Projection previously aliased o.total_before_coins_amount AS escrow_amount for read model.
-- Canonical persisted order financial base is orders.total_before_coins_amount.
-- pricing_tokens.escrow_amount is preserved (pricing intent snapshot, distinct authority).
-- This migration renames the read-model column to canonical name and adds missing canonical columns.
-- No data loss: escrow_amount value is already PD+S, rename preserves it.
-- Service/columns service_fee_amount and total_payable_amount were referenced by projection code but missing from 000001 schema.

-- 1. Rename obsolete alias to canonical column name
ALTER TABLE order_summaries RENAME COLUMN escrow_amount TO total_before_coins_amount;

-- 2. Add missing canonical projection columns if not exists (for display, not financial authority)
ALTER TABLE order_summaries ADD COLUMN IF NOT EXISTS service_fee_amount bigint DEFAULT 0 NOT NULL;
ALTER TABLE order_summaries ADD COLUMN IF NOT EXISTS total_payable_amount bigint DEFAULT 0 NOT NULL;
