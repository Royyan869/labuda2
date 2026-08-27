-- ============================================================
-- 000028_bank_account_primary_invariant_hardening.down.sql
--
-- Drops the primary-bank-account invariant index only.
-- The repair update is intentionally not reverted.
-- ============================================================

DROP INDEX IF EXISTS public.idx_bank_accounts_user_active_default_unique;
