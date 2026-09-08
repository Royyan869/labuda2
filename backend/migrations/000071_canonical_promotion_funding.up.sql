-- 000071_canonical_promotion_funding
-- Forward-only canonical initial funding authority.

CREATE TABLE promotion_funding (
    id uuid PRIMARY KEY,
    promotion_id uuid NOT NULL REFERENCES promotions(id),
    funding_key text NOT NULL,
    amount bigint NOT NULL CHECK (amount > 0),
    allocation_account_id uuid REFERENCES financial_accounts(id),
    status text NOT NULL CHECK (status IN ('pending', 'funded', 'failed')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    funded_at timestamptz,
    UNIQUE (promotion_id),
    UNIQUE (funding_key)
);

CREATE INDEX promotion_funding_allocation_account_idx
    ON promotion_funding (allocation_account_id);
