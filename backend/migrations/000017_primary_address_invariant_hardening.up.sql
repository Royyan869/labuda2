-- ============================================================
-- 000017_primary_address_invariant_hardening.up.sql
--
-- Enforces the canonical primary-address invariant:
-- - every user with active addresses has at most one active primary
-- - every user with active addresses and no active primary is repaired
--   deterministically to the oldest active address (UUID tie-breaker)
-- - soft-deleted / unavailable addresses do not participate in the
--   uniqueness constraint
-- ============================================================

-- Repair users that have active addresses but no active primary.
WITH missing_primary_users AS (
    SELECT user_id
    FROM public.addresses
    WHERE is_available_for_checkout = true
    GROUP BY user_id
    HAVING COUNT(*) FILTER (WHERE is_primary = true) = 0
),
missing_primary_candidates AS (
    SELECT DISTINCT ON (a.user_id)
        a.id
    FROM public.addresses a
    JOIN missing_primary_users u ON u.user_id = a.user_id
    WHERE a.is_available_for_checkout = true
    ORDER BY a.user_id, a.created_at ASC, a.id ASC
)
UPDATE public.addresses a
SET is_primary = true,
    updated_at = NOW()
FROM missing_primary_candidates c
WHERE a.id = c.id;

-- Normalize users that currently have more than one active primary.
WITH multi_primary_users AS (
    SELECT user_id
    FROM public.addresses
    WHERE is_available_for_checkout = true
      AND is_primary = true
    GROUP BY user_id
    HAVING COUNT(*) > 1
),
primary_keeper AS (
    SELECT DISTINCT ON (a.user_id)
        a.id,
        a.user_id
    FROM public.addresses a
    JOIN multi_primary_users u ON u.user_id = a.user_id
    WHERE a.is_available_for_checkout = true
      AND a.is_primary = true
    ORDER BY a.user_id, a.created_at ASC, a.id ASC
)
UPDATE public.addresses a
SET is_primary = false,
    updated_at = NOW()
WHERE a.is_available_for_checkout = true
  AND a.is_primary = true
  AND a.user_id IN (SELECT user_id FROM multi_primary_users)
  AND NOT EXISTS (
      SELECT 1
      FROM primary_keeper k
      WHERE k.id = a.id
  );

-- Enforce the invariant at the database layer for active addresses only.
CREATE UNIQUE INDEX idx_addresses_user_active_primary_unique
    ON public.addresses USING btree (user_id)
    WHERE (is_primary = true AND is_available_for_checkout = true);
