-- 000069_promotion_canonical_foundation_hardening down

DROP TABLE IF EXISTS promotion_creation_idempotency;

DROP INDEX IF EXISTS promotions_non_terminal_identity_unique;

CREATE UNIQUE INDEX IF NOT EXISTS promotions_identity_unique
    ON promotions (seller_id, target_type, target_id, kind);
