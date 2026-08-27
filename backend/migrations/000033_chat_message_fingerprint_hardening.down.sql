-- Migration 000033 down: IRREVERSIBLE.
--
-- The purged rows cannot be recovered. Restoring DEFAULT '' would re-open
-- the door to canonical messages without fingerprints.
--
-- The canonical down path is a no-op. Deploy a new forward migration to
-- change fingerprint semantics.

SELECT 1;
