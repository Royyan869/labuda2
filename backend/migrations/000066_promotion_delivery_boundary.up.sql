-- 000066_promotion_delivery_boundary
--
-- PROMOTION PHASE 3 - DELIVERY TICKET + QUALIFIED IMPRESSION
--
-- Canonical delivery boundary between the Promotion Contract (lifecycle
-- authority, migration 000065) and immutable financial consumption
-- (FinanceService ledger, migration 000064):
--
--   Promotion Contract
--       ↓
--   Delivery Ticket          (server-side delivery authorization, NO money)
--       ↓
--   server-side qualification
--       ↓
--   Qualified Impression     (immutable billable fact, UNIQUE per ticket)
--       ↓
--   PROMOTION_ALLOCATION debit + PLATFORM_REVENUE credit
--       (via FinanceService.RecordQualifiedImpression only)
--
-- LOCKED MONEY RULE (Model A): ticket issuance performs NO ledger
-- transaction, NO allocation reservation, NO balance mutation. Money moves
-- only when a Qualified Impression is server-validated.
--
-- Invariants enforced structurally here:
--   - one Delivery Ticket  -> at most one Qualified Impression
--     (UNIQUE promotion_qualified_impressions.ticket_id; DB is final
--     authority, application pre-checks are not)
--   - per-contract Qualified Impression sequence N is unique
--     (UNIQUE (contract_id, sequence_n)) so concurrent qualifications of
--     the same contract cannot collide on N
--   - charge_rupiah >= 0 (the zero-charge case from Phase 1 cumulative CPM
--     arithmetic is a legitimate QI with no money movement)
--   - expires_at > issued_at (tickets always carry a server-derived expiry)
--
-- promotion_events (analytics projection, legacy domain) is NOT touched:
-- it is never a billing authority. No code path may charge money from a
-- promotion event impression.

-- 1. Target vocabulary (mirrors the canonical promotion target types:
--    for_sale | auction | external_product).
CREATE TYPE promotion_target_type_enum AS ENUM (
    'for_sale',
    'auction',
    'external_product'
);

-- 2. Ticket lifecycle: issued -> consumed, and invalidated on contract
--    finalization (a finalized contract must never let an old ticket charge).
CREATE TYPE promotion_ticket_status_enum AS ENUM (
    'issued',
    'consumed',
    'invalidated'
);

-- 3. Delivery ticket: server-side delivery authorization. The viewer binding
--    makes seller self-delivery structurally detectable at qualification.
CREATE TABLE promotion_delivery_tickets (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    contract_id uuid NOT NULL,
    target_type promotion_target_type_enum NOT NULL,
    target_id uuid NOT NULL,
    viewer_id uuid NOT NULL,
    status promotion_ticket_status_enum NOT NULL DEFAULT 'issued',
    issued_at timestamp with time zone NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT promotion_delivery_tickets_pkey PRIMARY KEY (id),
    CONSTRAINT promotion_delivery_tickets_expires_after_issued
        CHECK (expires_at > issued_at)
);

ALTER TABLE promotion_delivery_tickets
    ADD CONSTRAINT promotion_delivery_tickets_contract_fkey
    FOREIGN KEY (contract_id) REFERENCES promotion_contracts(id);

ALTER TABLE promotion_delivery_tickets
    ADD CONSTRAINT promotion_delivery_tickets_viewer_fkey
    FOREIGN KEY (viewer_id) REFERENCES users(id);

CREATE INDEX idx_promotion_delivery_tickets_contract_id
    ON promotion_delivery_tickets USING btree (contract_id);

CREATE INDEX idx_promotion_delivery_tickets_status
    ON promotion_delivery_tickets USING btree (status);

-- 4. Qualified Impression: immutable billable financial fact.
CREATE TABLE promotion_qualified_impressions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    ticket_id uuid NOT NULL,
    contract_id uuid NOT NULL,
    allocation_account_id uuid NOT NULL,
    target_type promotion_target_type_enum NOT NULL,
    target_id uuid NOT NULL,
    sequence_n bigint NOT NULL,
    charge_rupiah bigint NOT NULL,
    server_occurred_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT promotion_qualified_impressions_pkey PRIMARY KEY (id),
    CONSTRAINT promotion_qualified_impressions_ticket_unique UNIQUE (ticket_id),
    CONSTRAINT promotion_qualified_impressions_contract_sequence_unique
        UNIQUE (contract_id, sequence_n),
    CONSTRAINT promotion_qualified_impressions_charge_non_negative
        CHECK (charge_rupiah >= 0),
    CONSTRAINT promotion_qualified_impressions_sequence_positive
        CHECK (sequence_n > 0)
);

ALTER TABLE promotion_qualified_impressions
    ADD CONSTRAINT promotion_qualified_impressions_ticket_fkey
    FOREIGN KEY (ticket_id) REFERENCES promotion_delivery_tickets(id);

ALTER TABLE promotion_qualified_impressions
    ADD CONSTRAINT promotion_qualified_impressions_contract_fkey
    FOREIGN KEY (contract_id) REFERENCES promotion_contracts(id);

ALTER TABLE promotion_qualified_impressions
    ADD CONSTRAINT promotion_qualified_impressions_allocation_fkey
    FOREIGN KEY (allocation_account_id) REFERENCES financial_accounts(id);

CREATE INDEX idx_promotion_qualified_impressions_contract_id
    ON promotion_qualified_impressions USING btree (contract_id);