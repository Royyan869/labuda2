-- Down migration: restore dropped tables and columns
-- WARNING: These are restored for migration chain integrity only.
-- The canonical data source is financial_accounts (ledger), NOT these tables.

-- Restore orders columns
ALTER TABLE orders ADD COLUMN IF NOT EXISTS coin_discount_amount bigint DEFAULT 0 NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS coins_used bigint DEFAULT 0 NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_amount bigint DEFAULT 0 NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS escrow_amount bigint DEFAULT 0 NOT NULL;

-- Restore account_balances table
CREATE TABLE IF NOT EXISTS account_balances (
    id uuid NOT NULL,
    user_id uuid,
    account_type text NOT NULL,
    balance bigint NOT NULL,
    currency text DEFAULT 'IDR'::text NOT NULL,
    updated_at timestamp with time zone NOT NULL
);

ALTER TABLE account_balances ADD CONSTRAINT account_balances_pkey PRIMARY KEY (id);
CREATE INDEX IF NOT EXISTS idx_account_balances_user_id ON public.account_balances USING btree (user_id);
