-- 000065_promotion_contract_canonical
--
-- PROMOTION PHASE 2 - CANONICAL PROMOTION CONTRACT
--
-- The promotion contract is the lifecycle authority for one Promotion:
--
--   kind          internal | external
--   status        prepared | active | paused | finalizing | finalized
--   budget_rupiah immutable seller-selected budget (integer Rupiah)
--   cpm_rupiah    immutable pricing snapshot taken at creation
--   planned_start / planned_finish   pacing boundary (NOT purchased time)
--   allocation_account_id  -> financial_accounts PROMOTION_ALLOCATION row
--                            (seller + contract holder scoped, migration 000064)
--   paused_at     start of the current EXPLICIT seller pause (DB time)
--
-- Lifecycle truth (LABUDA_PROMOTION_DOMAIN_CANONICAL_CONTRACT.md):
--   - Duration is a pacing/planned-completion boundary, NOT a time wallet.
--     There is NO total_duration_hours / validity_window_hours /
--     consumed_duration_hours / expired-because-time-ran-out anywhere here.
--   - Only an explicit seller pause shifts planned_finish:
--       new_planned_finish = old_planned_finish + explicit_pause_duration
--   - Seller concurrency is enforced STRUCTURALLY: at most one non-finalized
--     internal AND at most one non-finalized external contract per seller.
--     non-finalized = prepared | active | paused | finalizing (finalized frees
--     the slot). Partial unique indexes are the final authority.
--   - Money stays in the immutable ledger: budget is moved by
--     FinanceService.RecordPromotionAllocation; release happens through
--     FinanceService.RecordPromotionAllocationRelease.

-- 1. Promote Balance top-up becomes a first-class canonical billing type.
--    A verified top-up payment MUST credit PROMOTE_BALANCE (funding), never
--    PLATFORM_REVENUE. The value is added to the enum here; it may only be
--    USED in a later migration (PostgreSQL enum caveat, see 000008).
ALTER TYPE billing_type_enum ADD VALUE IF NOT EXISTS 'promote_balance_top_up';

-- 2. Contract kind + status enums.
CREATE TYPE promotion_contract_kind_enum AS ENUM (
    'internal',
    'external'
);

CREATE TYPE promotion_contract_status_enum AS ENUM (
    'prepared',
    'active',
    'paused',
    'finalizing',
    'finalized'
);

-- 3. Canonical contract table.
CREATE TABLE promotion_contracts (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    seller_id uuid NOT NULL,
    kind promotion_contract_kind_enum NOT NULL,
    status promotion_contract_status_enum NOT NULL DEFAULT 'prepared',
    budget_rupiah bigint NOT NULL,
    cpm_rupiah bigint NOT NULL,
    planned_start timestamp with time zone NOT NULL,
    planned_finish timestamp with time zone NOT NULL,
    allocation_account_id uuid NOT NULL,
    paused_at timestamp with time zone,
    finalized_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE promotion_contracts ADD CONSTRAINT promotion_contracts_pkey PRIMARY KEY (id);

ALTER TABLE promotion_contracts
    ADD CONSTRAINT promotion_contracts_seller_id_fkey
    FOREIGN KEY (seller_id) REFERENCES users(id);

ALTER TABLE promotion_contracts
    ADD CONSTRAINT promotion_contracts_allocation_account_id_fkey
    FOREIGN KEY (allocation_account_id) REFERENCES financial_accounts(id);

ALTER TABLE promotion_contracts
    ADD CONSTRAINT promotion_contracts_budget_positive CHECK (budget_rupiah > 0);

ALTER TABLE promotion_contracts
    ADD CONSTRAINT promotion_contracts_cpm_positive CHECK (cpm_rupiah > 0);

-- planned_finish is the pacing boundary; it must always lie after planned_start.
ALTER TABLE promotion_contracts
    ADD CONSTRAINT promotion_contracts_finish_after_start CHECK (planned_finish > planned_start);

-- 4. SELLER CONCURRENCY - DB final authority (contract 16).
-- At most one non-finalized INTERNAL promotion per seller.
CREATE UNIQUE INDEX ux_promotion_contracts_one_nonfinalized_internal_per_seller
    ON promotion_contracts (seller_id)
    WHERE kind = 'internal'
      AND status IN ('prepared', 'active', 'paused', 'finalizing');

-- At most one non-finalized EXTERNAL promotion per seller.
-- Internal and external are independent: they may coexist.
CREATE UNIQUE INDEX ux_promotion_contracts_one_nonfinalized_external_per_seller
    ON promotion_contracts (seller_id)
    WHERE kind = 'external'
      AND status IN ('prepared', 'active', 'paused', 'finalizing');

CREATE INDEX idx_promotion_contracts_seller_id
    ON promotion_contracts USING btree (seller_id);

CREATE INDEX idx_promotion_contracts_status
    ON promotion_contracts USING btree (status);
