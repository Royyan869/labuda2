-- Migration 000032 down: IRREVERSIBLE.
--
-- Restoring the global UNIQUE(idempotency_key) constraint would resurrect
-- the cross-actor message-leakage security defect (P1).
--
-- The canonical down path is a no-op. To undo actor-scoped idempotency,
-- deploy a new forward migration.

SELECT 1;
