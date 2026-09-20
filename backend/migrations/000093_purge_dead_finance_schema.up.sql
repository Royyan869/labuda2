-- ============================================================
-- 000093 PURGE DEAD FINANCE SCHEMA
--
-- Targets (all proven OBSOLETE in Finance Root Truth Audit):
--
-- 1. account_balances TABLE
--    - Projection of financial_accounts with ZERO readers
--    - Projection worker writes to it but outbox event
--      "ledger.transaction.completed" is never emitted
--    - GetSystemAccountBalances / GetUserAccountBalances
--      have zero callers in production code
--    - Admin finance handler reads directly from
--      financial_accounts, NOT from account_balances
--    - Canonical: financial_accounts.balance
--
-- 2. orders.escrow_amount COLUMN
--    - NOT written by production INSERT (order_repository.go)
--    - Defaults to 0
--    - Canonical: total_before_coins_amount (PD + S)
--    - Comments: "DEAD COLUMN... NEVER a financial authority"
--    - Projection worker already reads
--      o.total_before_coins_amount, not o.escrow_amount
--
-- 3. orders.discount_amount COLUMN
--    - NOT written by production INSERT
--    - Defaults to 0
--    - Canonical: pricing token snapshot
--    - Comments: "DEAD COLUMN... NEVER a financial authority"
--
-- 4. orders.coins_used COLUMN
--    - NOT written by production INSERT
--    - Defaults to 0
--    - Canonical: coins domain (user_coin_balance, coins_transactions)
--    - Comments: "DEAD COLUMNS... never read by production"
--
-- 5. orders.coin_discount_amount COLUMN
--    - NOT written by production INSERT
--    - Defaults to 0
--    - Canonical: coins domain
--    - Comments: "DEAD COLUMNS... never read by production"
--
-- NOTE: financial_reconciliations was already purged by 000011.
-- ============================================================

-- 1. DROP account_balances projection table
DROP TABLE IF EXISTS account_balances;

-- 2-5. DROP dead financial columns from orders
-- All four default to 0 and are never written by production code.
ALTER TABLE orders DROP COLUMN IF EXISTS escrow_amount;
ALTER TABLE orders DROP COLUMN IF EXISTS discount_amount;
ALTER TABLE orders DROP COLUMN IF EXISTS coins_used;
ALTER TABLE orders DROP COLUMN IF EXISTS coin_discount_amount;
