-- NEGOTIATION MESSAGES PURGE (Z1) — DOWN
--
-- Recreates negotiation_messages with the exact shape it had in
-- 000001_canonical_schema.up.sql (including all constraints 000001 added).
-- Provided only for local development tooling; the canonical design no
-- longer includes this table, so the down migration must never be applied
-- on a shared environment.
CREATE TABLE IF NOT EXISTS negotiation_messages (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    session_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    price bigint NOT NULL,
    note text NOT NULL,
    created_at bigint NOT NULL,
    idempotency_key text NOT NULL
);

ALTER TABLE negotiation_messages ADD CONSTRAINT negotiation_messages_pkey PRIMARY KEY (id);

CREATE INDEX idx_negotiation_messages_session_id
    ON public.negotiation_messages USING btree (session_id, created_at);

ALTER TABLE negotiation_messages
    ADD CONSTRAINT negotiation_messages_sender_id_fkey
    FOREIGN KEY (sender_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE negotiation_messages
    ADD CONSTRAINT negotiation_messages_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES negotiation_sessions(id) ON DELETE CASCADE;

ALTER TABLE negotiation_messages
    ADD CONSTRAINT negotiation_messages_price_check CHECK ((price > 0));
