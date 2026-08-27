-- Migration 000042: Enforce at most one active alert per group_key
--
-- The alert service already deduplicates by group_key at the application
-- level, but that check is read-then-write and can race when more than one
-- worker instance runs concurrently. This partial unique index closes that
-- gap at the database level: no two active (open/active) alerts may share a
-- group_key. Resolved/false_positive rows are excluded so the daily
-- stuck-detection rules can keep creating a fresh alert each day.

-- Repair any pre-existing duplicates before adding the constraint: keep the
-- newest alert per group_key and resolve the rest.
WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY group_key
            ORDER BY created_at DESC, id ASC
        ) AS rn
    FROM system_alerts
    WHERE group_key IS NOT NULL
      AND status IN ('active', 'open')
)
UPDATE system_alerts sa
SET status = 'resolved',
    resolved_at = NOW(),
    updated_at = NOW()
FROM ranked r
WHERE sa.id = r.id
  AND r.rn > 1;

CREATE UNIQUE INDEX idx_system_alerts_active_group_key
    ON system_alerts (group_key)
    WHERE group_key IS NOT NULL
      AND status IN ('active', 'open');
