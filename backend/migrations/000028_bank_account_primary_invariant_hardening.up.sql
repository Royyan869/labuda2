-- ============================================================
-- 000028_bank_account_primary_invariant_hardening.up.sql
--
-- Enforces the canonical primary bank-account invariant:
-- - every seller with active bank accounts has at most one active default
-- - every seller with active bank accounts and no active default is repaired
--   deterministically to the oldest active account (UUID tie-breaker)
-- - soft-deleted accounts do not participate in the uniqueness constraint
-- ============================================================

-- Repair sellers that have active bank accounts but no active default.
WITH missing_default_users AS (
    SELECT user_id
    FROM public.bank_accounts
    WHERE deleted_at IS NULL
    GROUP BY user_id
    HAVING COUNT(*) FILTER (WHERE is_default = true) = 0
),
missing_default_candidates AS (
    SELECT DISTINCT ON (b.user_id)
        b.id
    FROM public.bank_accounts b
    JOIN missing_default_users u ON u.user_id = b.user_id
    WHERE b.deleted_at IS NULL
    ORDER BY b.user_id, b.created_at ASC, b.id ASC
)
UPDATE public.bank_accounts b
SET is_default = true,
    updated_at = NOW()
FROM missing_default_candidates c
WHERE b.id = c.id;

-- Normalize sellers that currently have more than one active default.
WITH multi_default_users AS (
    SELECT user_id
    FROM public.bank_accounts
    WHERE deleted_at IS NULL
      AND is_default = true
    GROUP BY user_id
    HAVING COUNT(*) > 1
),
default_keeper AS (
    SELECT DISTINCT ON (b.user_id)
        b.id,
        b.user_id
    FROM public.bank_accounts b
    JOIN multi_default_users u ON u.user_id = b.user_id
    WHERE b.deleted_at IS NULL
      AND b.is_default = true
    ORDER BY b.user_id, b.created_at ASC, b.id ASC
)
UPDATE public.bank_accounts b
SET is_default = false,
    updated_at = NOW()
WHERE b.deleted_at IS NULL
  AND b.is_default = true
  AND b.user_id IN (SELECT user_id FROM multi_default_users)
  AND NOT EXISTS (
      SELECT 1
      FROM default_keeper k
      WHERE k.id = b.id
  );

-- Enforce the invariant at the database layer for active bank accounts only.
CREATE UNIQUE INDEX idx_bank_accounts_user_active_default_unique
    ON public.bank_accounts USING btree (user_id)
    WHERE (is_default = true AND deleted_at IS NULL);
