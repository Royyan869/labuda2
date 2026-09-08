-- 000064_promotion_financial_foundation
--
-- PROMOTION PHASE 1 - FINANCIAL FOUNDATION
-- Canonical account types:
--   PROMOTE_BALANCE      = seller-owned, non-withdrawable platform usage
--                          balance (user-scoped: one per seller).
--   PROMOTION_ALLOCATION = per-promotion allocation account
--                          (user + contract/holder scoped: one per
--                          seller x promotion contract).
--
-- Design rules honored:
--   - No second ledger. Allocation lives INSIDE financial_accounts.
--   - Ordinary user accounts keep their existing (user_id, account_type)
--     partial-unique invariant; PROMOTION_ALLOCATION rows are excluded from
--     that index and instead unique on (user_id, account_type, holder_id).
--   - The balance >= 0 CHECK already on financial_accounts makes negative
--     PROMOTE_BALANCE / PROMOTION_ALLOCATION impossible at the DB level.

-- 1. Holder scope columns (nullable; only PROMOTION_ALLOCATION rows use them).
ALTER TABLE financial_accounts ADD COLUMN holder_type text;
ALTER TABLE financial_accounts ADD COLUMN holder_id uuid;

-- 2. Scope CHECK: either a well-formed PROMOTION_ALLOCATION row
--    (user + holder_type + holder_id all present) or a plain account
--    (no holder columns at all). No hybrid rows.
ALTER TABLE financial_accounts ADD CONSTRAINT financial_accounts_promotion_scope_check CHECK (
    (account_type = 'PROMOTION_ALLOCATION'
        AND user_id IS NOT NULL
        AND holder_type IS NOT NULL
        AND holder_id IS NOT NULL)
    OR
    (account_type <> 'PROMOTION_ALLOCATION'
        AND holder_type IS NULL
        AND holder_id IS NULL)
);

-- 3. Preserve ordinary user-account uniqueness, excluding allocation rows.
DROP INDEX IF EXISTS uniq_financial_accounts_user_account_type;
CREATE UNIQUE INDEX uniq_financial_accounts_user_account_type
    ON financial_accounts (user_id, account_type)
    WHERE user_id IS NOT NULL AND account_type <> 'PROMOTION_ALLOCATION';

-- 4. Per-contract allocation uniqueness: one PROMOTION_ALLOCATION account per
--    (seller, promotion contract).
CREATE UNIQUE INDEX uniq_financial_accounts_promotion_allocation_holder
    ON financial_accounts (user_id, account_type, holder_id)
    WHERE user_id IS NOT NULL
      AND account_type = 'PROMOTION_ALLOCATION'
      AND holder_id IS NOT NULL;

-- 5. Platform config foundation for promotion.
--    Promotion delivery is DISABLED by default; seeds are inert until a later
--    phase enables delivery. Values are provisional defaults (admin editable).
INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
VALUES
    ('promotion_cpm', 7500, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
    ('promotion_min_daily_budget', 10000, NULL, NULL, EXTRACT(epoch FROM now())::bigint),
    ('promotion_delivery_enabled', NULL, 'disabled', NULL, EXTRACT(epoch FROM now())::bigint)
ON CONFLICT (key) DO NOTHING;
