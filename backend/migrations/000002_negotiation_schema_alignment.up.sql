-- 000002_negotiation_schema_alignment
--
-- Root cause: NegotiationRepositoryImpl (backend/internal/commerce/negotiation/
-- infrastructure/repository/negotiation_repository_impl.go) and the order
-- creation / fixed-price-sale-sold flows that depend on it reference
-- negotiation_sessions.chat_room_id, .order_id, .accepted_price, .accepted_at,
-- .proposal_sequence, and a negotiation_price_history table. None of these
-- ever existed in 000001_canonical_schema — they were added to the repository
-- during a "NEGOTIATION -> CHAT UNIFICATION" / "PRICE SECURITY HARDENING" pass
-- without a matching migration. This is current, load-bearing business logic
-- (chat linkage, order-settlement idempotency, order_creation_service price
-- override + duplicate-settlement guard, price audit trail) — a schema
-- omission, not repository residue. Aligning schema to match.
--
-- Also: negotiation_resource_enum only defines 'listing'/'auction' (from a
-- removed pre-refactor domain model). The negotiation service now exclusively
-- uses NegotiationResourceFixedPriceSale ("fixed_price_sale") and explicitly
-- rejects any other resource type (validateFixedPriceSaleAndGetSeller). Every
-- session insert fails on the enum cast without this value.
ALTER TYPE negotiation_resource_enum ADD VALUE IF NOT EXISTS 'fixed_price_sale';

ALTER TABLE negotiation_sessions
    ADD COLUMN chat_room_id uuid,
    ADD COLUMN order_id uuid,
    ADD COLUMN accepted_price bigint,
    ADD COLUMN accepted_at timestamp with time zone,
    ADD COLUMN proposal_sequence bigint NOT NULL DEFAULT 0;

ALTER TABLE negotiation_sessions
    ADD CONSTRAINT negotiation_sessions_chat_room_id_fkey
        FOREIGN KEY (chat_room_id) REFERENCES chat_rooms(id) ON DELETE SET NULL;

ALTER TABLE negotiation_sessions
    ADD CONSTRAINT negotiation_sessions_order_id_fkey
        FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE SET NULL;

-- One negotiation session settles at most one order (order_creation_service
-- sets session.OrderID once, at settlement, to prevent duplicate order
-- creation from the same accepted negotiation).
ALTER TABLE negotiation_sessions
    ADD CONSTRAINT negotiation_sessions_order_id_key UNIQUE (order_id);

CREATE INDEX idx_negotiation_sessions_chat_room_id
    ON negotiation_sessions USING btree (chat_room_id) WHERE chat_room_id IS NOT NULL;

CREATE INDEX idx_negotiation_sessions_order_id
    ON negotiation_sessions USING btree (order_id) WHERE order_id IS NOT NULL;

CREATE TABLE negotiation_price_history (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    proposal_sequence bigint NOT NULL,
    old_price bigint,
    new_price bigint NOT NULL,
    changed_by_user_id uuid NOT NULL,
    change_reason text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE negotiation_price_history ADD CONSTRAINT negotiation_price_history_pkey PRIMARY KEY (id);

ALTER TABLE negotiation_price_history
    ADD CONSTRAINT negotiation_price_history_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES negotiation_sessions(id) ON DELETE CASCADE;

ALTER TABLE negotiation_price_history
    ADD CONSTRAINT negotiation_price_history_changed_by_user_id_fkey
        FOREIGN KEY (changed_by_user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX idx_negotiation_price_history_session_id
    ON negotiation_price_history USING btree (session_id);

CREATE INDEX idx_negotiation_price_history_changed_by_user_id
    ON negotiation_price_history USING btree (changed_by_user_id);
