-- ============================================================
-- 000119_address_tags_over_purpose.down.sql
--
-- Restores the pre-tags single-purpose column. The restored value is the
-- FIRST canonical tag (shipping before sender), so a dual-tagged address
-- loses its sender role on rollback — this is a rollback of the design,
-- not a preservation of it.
-- ============================================================

ALTER TABLE addresses DROP CONSTRAINT addresses_tags_known;
ALTER TABLE addresses DROP CONSTRAINT addresses_tags_not_empty;
DROP INDEX IF EXISTS idx_addresses_tags;

ALTER TABLE addresses ADD COLUMN purpose text;

UPDATE addresses
SET purpose = tags[1]
WHERE cardinality(tags) > 0;

ALTER TABLE addresses ALTER COLUMN purpose SET NOT NULL;
ALTER TABLE addresses DROP COLUMN tags;
