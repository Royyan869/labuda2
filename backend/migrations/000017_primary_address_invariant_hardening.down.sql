-- ============================================================
-- 000017_primary_address_invariant_hardening.down.sql
--
-- Forward-only data repair is intentionally not reversed.
-- The rollback only removes the unique index added by the up migration.
-- ============================================================

DROP INDEX IF EXISTS public.idx_addresses_user_active_primary_unique;
