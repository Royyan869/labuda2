-- 000073_canonical_promotion_activation down

ALTER TABLE promotions
    DROP CONSTRAINT IF EXISTS promotions_active_requires_activated_at;

ALTER TABLE promotions
    DROP COLUMN IF EXISTS activated_at;
