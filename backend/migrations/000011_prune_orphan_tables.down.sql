-- Rollback for 000011: recreate the six pruned tables exactly as they
-- existed in 000001_canonical_schema. All were confirmed empty of
-- application writes at prune time, so this rollback restores structure
-- only, not data.

CREATE TABLE IF NOT EXISTS actors (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    actor_type text NOT NULL,
    actor_data jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE actors ADD CONSTRAINT actors_pkey PRIMARY KEY (id);
ALTER TABLE actors ADD CONSTRAINT actors_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS bnr_classifications (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    classification text NOT NULL,
    reason text,
    classified_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone
);
ALTER TABLE bnr_classifications ADD CONSTRAINT bnr_classifications_pkey PRIMARY KEY (id);
ALTER TABLE bnr_classifications ADD CONSTRAINT bnr_classifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS financial_reconciliations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    wallet_available_balance bigint NOT NULL,
    wallet_held_balance bigint NOT NULL,
    wallet_total bigint NOT NULL,
    finance_total bigint NOT NULL,
    difference bigint NOT NULL,
    status text NOT NULL,
    breakdown jsonb,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE financial_reconciliations ADD CONSTRAINT financial_reconciliations_pkey PRIMARY KEY (id);
ALTER TABLE financial_reconciliations ADD CONSTRAINT financial_reconciliations_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE financial_reconciliations ADD CONSTRAINT financial_reconciliations_status_check CHECK ((status = ANY (ARRAY['matched'::text, 'mismatch'::text])));

CREATE TABLE IF NOT EXISTS search_results (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    query text NOT NULL,
    results jsonb NOT NULL,
    total_count integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE search_results ADD CONSTRAINT search_results_pkey PRIMARY KEY (id);
ALTER TABLE search_results ADD CONSTRAINT search_results_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS ticket_escalations (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    ticket_id uuid NOT NULL,
    escalated_by uuid NOT NULL,
    from_level ticket_escalation_enum NOT NULL,
    to_level ticket_escalation_enum NOT NULL,
    reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
ALTER TABLE ticket_escalations ADD CONSTRAINT ticket_escalations_pkey PRIMARY KEY (id);
ALTER TABLE ticket_escalations ADD CONSTRAINT ticket_escalations_escalated_by_fkey FOREIGN KEY (escalated_by) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE ticket_escalations ADD CONSTRAINT ticket_escalations_ticket_id_fkey FOREIGN KEY (ticket_id) REFERENCES support_tickets(id) ON DELETE CASCADE;

CREATE TABLE IF NOT EXISTS user_online_status (
    user_id uuid NOT NULL,
    is_online boolean DEFAULT false NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    current_room_id uuid
);
ALTER TABLE user_online_status ADD CONSTRAINT user_online_status_pkey PRIMARY KEY (user_id);
ALTER TABLE user_online_status ADD CONSTRAINT user_online_status_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
