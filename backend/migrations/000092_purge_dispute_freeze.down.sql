-- Rollback for 000092: recreate dispute_freezes exactly as it existed in
-- 000001_canonical_schema. The table had no application writes at purge time,
-- so this rollback restores structure only, not data.

CREATE TABLE IF NOT EXISTS dispute_freezes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    dispute_id uuid NOT NULL,
    order_id uuid NOT NULL,
    seller_id uuid NOT NULL,
    frozen_amount bigint NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at bigint NOT NULL,
    updated_at bigint NOT NULL,
    CONSTRAINT dispute_freezes_pkey PRIMARY KEY (id),
    CONSTRAINT dispute_freezes_dispute_id_key UNIQUE (dispute_id),
    CONSTRAINT dispute_freezes_dispute_id_fkey FOREIGN KEY (dispute_id) REFERENCES disputes(id),
    CONSTRAINT dispute_freezes_order_id_fkey FOREIGN KEY (order_id) REFERENCES orders(id),
    CONSTRAINT dispute_freezes_frozen_amount_check CHECK ((frozen_amount > 0)),
    CONSTRAINT dispute_freezes_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'released'::character varying])::text[])))
);

CREATE INDEX IF NOT EXISTS idx_dispute_freezes_dispute_id ON public.dispute_freezes USING btree (dispute_id);
CREATE INDEX IF NOT EXISTS idx_dispute_freezes_order_id ON public.dispute_freezes USING btree (order_id);
CREATE INDEX IF NOT EXISTS idx_dispute_freezes_seller_status ON public.dispute_freezes USING btree (seller_id, status);
