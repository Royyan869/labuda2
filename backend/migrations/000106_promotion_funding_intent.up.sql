-- 000106_promotion_funding_intent
--
-- PROMOTION FUNDING INTENT — snapshot funding calculation and payment obligation.
--
-- When PreviewFunding shows shortage > 0, a funding intent row is created
-- documenting the exact promotion params and shortage amount. A billing
-- transaction of TypePromoteBalanceTopUp is then created with amount =
-- exact shortage and target_id = this intent's id.
--
-- The intent status is DERIVED from the linked billing transaction status:
--   no billing yet        → pending
--   billing.status=paid   → paid (funding complete)
--   billing.status=failed → failed
--
-- This table is NOT:
--   - a new payment engine;
--   - a new balance;
--   - a new account type;
--   - a promotion contract;
--   - a reservation of PROMOTE_BALANCE;
--   - a binding to a specific future promotion.
--
-- It is a snapshot that documents the shortage calculation and payment
-- obligation. The actual financial authority remains:
--   PROMOTE_BALANCE (ledger) → PROMOTION_ALLOCATION (ledger)
--
-- After settlement, the exact payment amount is credited to PROMOTE_BALANCE
-- through the existing canonical settlement path. The resulting balance is
-- reusable — it is NOT tied to this FundingIntent.
--
-- AUTHORITY: this table is informational only. The ledger is the financial
-- truth. This table does not authorize any money movement.

CREATE TABLE promotion_funding_intents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    seller_id uuid NOT NULL,
    kind text NOT NULL,
    budget_rupiah bigint NOT NULL,
    duration_days bigint NOT NULL,
    city_ids text[],
    shortage_amount bigint NOT NULL,
    billing_transaction_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE promotion_funding_intents ADD CONSTRAINT promotion_funding_intents_pkey PRIMARY KEY (id);

-- FK to users: seller_id must reference a valid user.
-- Funding intent is informational, but referential integrity is still authoritative.
ALTER TABLE promotion_funding_intents
    ADD CONSTRAINT promotion_funding_intents_seller_id_fkey
    FOREIGN KEY (seller_id) REFERENCES users(id);

ALTER TABLE promotion_funding_intents
    ADD CONSTRAINT promotion_funding_intents_shortage_positive CHECK (shortage_amount > 0);

ALTER TABLE promotion_funding_intents
    ADD CONSTRAINT promotion_funding_intents_budget_positive CHECK (budget_rupiah > 0);

ALTER TABLE promotion_funding_intents
    ADD CONSTRAINT promotion_funding_intents_duration_positive CHECK (duration_days > 0);

-- RACE-SAFE IDEMPOTENCY: exactly one active (non-failed) intent per seller + params.
-- This prevents concurrent duplicate billing obligations via INSERT ON CONFLICT.
-- Only failed intents are superseded — the unique index excludes rows where the
-- linked billing has status 'failed' (checked at application layer, not partial index,
-- because billing status lives in a different table).
--
-- The application layer uses INSERT ... ON CONFLICT DO NOTHING RETURNING id to
-- atomically handle concurrent duplicate requests. If the INSERT conflicts,
-- the existing row is returned without creating a duplicate billing.
CREATE UNIQUE INDEX ux_promotion_funding_intents_seller_params
    ON promotion_funding_intents (seller_id, kind, budget_rupiah, duration_days);
