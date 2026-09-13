-- 000086_capability_authority_integrity (down)
--
-- Reverses the authority-integrity constraints.
--
-- The deleted reserved-UUID user row and the purged duplicate grants are NOT
-- restored: they were data-integrity defects, and the down migration only needs
-- to remove the enforcement so the schema matches the previous revision.

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_reserved_system_caller_id;

DROP INDEX IF EXISTS user_capabilities_unique_active_capability;
