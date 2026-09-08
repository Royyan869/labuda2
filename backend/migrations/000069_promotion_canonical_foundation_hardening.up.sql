-- 000069_promotion_canonical_foundation_hardening
-- Corrects uniqueness authority for canonical promotion lifecycle reuse
-- and adds creation idempotency mapping authority.

DROP INDEX IF EXISTS promotions_identity_unique;

CREATE UNIQUE INDEX IF NOT EXISTS promotions_non_terminal_identity_unique
    ON promotions (seller_id, target_type, target_id, kind)
    WHERE status NOT IN ('finalized', 'cancelled', 'failed');

CREATE TABLE IF NOT EXISTS promotion_creation_idempotency (
    idempotency_key text PRIMARY KEY,
    promotion_id uuid NOT NULL REFERENCES promotions(id),
    seller_id uuid NOT NULL,
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    kind text NOT NULL,
    budget bigint NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
