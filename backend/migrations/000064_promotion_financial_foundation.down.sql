-- 000064_promotion_financial_foundation (down)
DROP INDEX IF EXISTS uniq_financial_accounts_promotion_allocation_holder;
ALTER TABLE financial_accounts DROP CONSTRAINT IF EXISTS financial_accounts_promotion_scope_check;

DROP INDEX IF EXISTS uniq_financial_accounts_user_account_type;
CREATE UNIQUE INDEX uniq_financial_accounts_user_account_type
    ON public.financial_accounts USING btree (user_id, account_type)
    WHERE (user_id IS NOT NULL);

ALTER TABLE financial_accounts DROP COLUMN IF EXISTS holder_type;
ALTER TABLE financial_accounts DROP COLUMN IF EXISTS holder_id;

DELETE FROM platform_configs WHERE key IN (
    'promotion_cpm',
    'promotion_min_daily_budget',
    'promotion_delivery_enabled'
);
