-- PASS: shipping-quote persistence and lifecycle hardening.
-- This migration is fail-closed: if legacy rows still rely on nullable expiry
-- or nullable canonical-context dimensions, the migration aborts instead of
-- silently backfilling authority data.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM shipping_quotes
        WHERE expires_at IS NULL
    ) THEN
        RAISE EXCEPTION 'shipping_quotes contains NULL expires_at values; refusing to enforce NOT NULL';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM shipping_quotes
        WHERE product_id IS NULL
           OR source_type IS NULL
           OR source_id IS NULL
    ) THEN
        RAISE EXCEPTION 'shipping_quotes contains NULL canonical-context values; refusing to enforce partial uniqueness';
    END IF;
END
$$;

ALTER TABLE shipping_quotes
    ADD COLUMN superseded_at timestamp with time zone,
    ADD COLUMN superseded_by_id uuid;

ALTER TABLE shipping_quotes
    ADD CONSTRAINT shipping_quotes_superseded_by_id_fkey
    FOREIGN KEY (superseded_by_id) REFERENCES shipping_quotes(id) ON DELETE SET NULL
    DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE shipping_quotes
    ALTER COLUMN product_id SET NOT NULL,
    ALTER COLUMN source_type SET NOT NULL,
    ALTER COLUMN source_id SET NOT NULL,
    ALTER COLUMN expires_at SET NOT NULL;

ALTER TABLE shipping_quotes
    ADD CONSTRAINT shipping_quotes_reactivation_count_nonnegative_check CHECK ((reactivation_count >= 0)),
    ADD CONSTRAINT shipping_quotes_max_reuse_nonnegative_check CHECK ((max_reuse >= 0)),
    ADD CONSTRAINT shipping_quotes_reactivation_count_within_reuse_check CHECK ((reactivation_count <= max_reuse)),
    ADD CONSTRAINT shipping_quotes_active_used_at_check CHECK (((status <> 'ACTIVE'::shipping_quote_status_enum) OR (used_at IS NULL))),
    ADD CONSTRAINT shipping_quotes_used_used_at_check CHECK (((status <> 'USED'::shipping_quote_status_enum) OR (used_at IS NOT NULL)));

CREATE INDEX idx_shipping_quotes_superseded_by_id
    ON public.shipping_quotes USING btree (superseded_by_id)
    WHERE (superseded_by_id IS NOT NULL);

CREATE INDEX idx_shipping_quotes_context_lookup
    ON public.shipping_quotes USING btree (chat_id, product_id, source_type, source_id, seller_id, buyer_id, created_at DESC);

CREATE INDEX idx_shipping_quotes_current_active_lookup
    ON public.shipping_quotes USING btree (chat_id, product_id, source_type, source_id, seller_id, buyer_id, created_at DESC)
    WHERE ((status = 'ACTIVE'::shipping_quote_status_enum) AND (superseded_at IS NULL));

CREATE UNIQUE INDEX uq_shipping_quotes_current_active_context
    ON public.shipping_quotes USING btree (chat_id, product_id, source_type, source_id, seller_id, buyer_id)
    WHERE ((status = 'ACTIVE'::shipping_quote_status_enum) AND (superseded_at IS NULL));
