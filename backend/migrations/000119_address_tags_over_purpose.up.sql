-- ============================================================
-- 000019_address_tags_over_purpose.up.sql
--
-- CANONICAL TRUTH: one address book per ACCOUNT, not per role.
--
-- purpose (one role per address, forcing a second book and a second
-- primary) is replaced by tags (a set of roles per address). One address
-- may be both a shipping destination and a sender origin. Promotion scope
-- will attach further tags later without another schema change.
--
-- The account-wide single primary invariant introduced by
-- 000017_primary_address_invariant_hardening is unchanged: still exactly
-- one is_primary row per user.
-- ============================================================

-- 1. Add the canonical column, seeded from the legacy single purpose.
ALTER TABLE addresses
    ADD COLUMN tags text[] NOT NULL DEFAULT '{}';

UPDATE addresses
SET tags = ARRAY[purpose]
WHERE cardinality(tags) = 0;

-- 2. Every address must declare at least one usage.
ALTER TABLE addresses
    ADD CONSTRAINT addresses_tags_not_empty CHECK (cardinality(tags) > 0);

-- 3. Drop the obsolete single-purpose authority. No compatibility column.
ALTER TABLE addresses
    DROP COLUMN purpose;

-- 4. Tag lookup index (list-by-tag and primary-by-tag both filter on it).
CREATE INDEX idx_addresses_tags
    ON addresses USING btree (tags);

-- 5. Guard the tag vocabulary at the database layer.
ALTER TABLE addresses
    ADD CONSTRAINT addresses_tags_known
    CHECK (tags <@ ARRAY['shipping', 'sender']::text[]);
