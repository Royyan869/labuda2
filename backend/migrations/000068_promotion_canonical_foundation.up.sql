-- 000068_promotion_canonical_foundation
-- Forward-only canonical Promotion aggregate foundation.

CREATE TABLE IF NOT EXISTS promotions (
    id uuid PRIMARY KEY,
    seller_id uuid NOT NULL REFERENCES users(id),
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    kind text NOT NULL,
    budget bigint NOT NULL,
    duration_seconds bigint,
    status text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promotions_budget_non_negative CHECK (budget >= 0),
    CONSTRAINT promotions_target_required CHECK (target_type <> ''),
    CONSTRAINT promotions_kind_required CHECK (kind <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS promotions_identity_unique
    ON promotions (seller_id, target_type, target_id, kind);
