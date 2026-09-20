-- 000100_refunds_reviewed_by_not_system_caller
--
-- Canonical refund attribution. refunds.reviewed_by is the users.id of the
-- human admin who made the refund decision; it is NULL when the refund was
-- produced automatically by the platform (system_refund with no human
-- reviewer).
--
-- 00000000-0000-0000-0000-000000000001 is the system-caller sentinel used for
-- authorization/audit only. Migration 000086 permanently removed it as a human
-- identity (users_reserved_system_caller_id). It must therefore never be stored
-- as refunds.reviewed_by — doing so violates refunds_reviewed_by_fkey because no
-- users row can carry that id.
--
-- This constraint encodes that invariant at the schema layer, mirroring 000086.

ALTER TABLE refunds
    ADD CONSTRAINT refunds_reviewed_by_not_system_caller
    CHECK (reviewed_by IS NULL OR reviewed_by <> '00000000-0000-0000-0000-000000000001');
