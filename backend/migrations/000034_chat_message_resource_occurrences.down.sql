-- Migration 000034 down: IRREVERSIBLE.
--
-- Dropping the occurrence table would lose the canonical resource-reference
-- authority for chat messages. There is no legacy fallback to restore.
--
-- The canonical down path is a no-op. Deploy a new forward migration to
-- change occurrence semantics.

SELECT 1;
