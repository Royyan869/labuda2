-- RECONSTRUCT ESCROW AUTHORITY (Owner-locked canonical model):
--
--   unpaid order            => NO escrow
--   gateway settlement      => exactly one escrow row, status='holding'
--   escrows                 => SOLE authority for escrow existence/amount/state
--
-- orders.escrow_status was an independent persisted projection seeded to
-- 'holding' at order creation (before any payment). It competed with the
-- canonical escrows row, poisoned every escrow_status='holding' filter, and
-- produced systemic false-positive reconciliation alerts for unpaid orders.
-- PURGED. All consumers now read the escrows table (live) — never a
-- second persisted representation.
--
-- order_summaries.escrow_status mirrored the same projection into the read
-- model; no functional consumer required it. PURGED with the source column.

ALTER TABLE orders DROP COLUMN IF EXISTS escrow_status;
ALTER TABLE order_summaries DROP COLUMN IF EXISTS escrow_status;
