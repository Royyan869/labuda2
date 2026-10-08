-- ============================================================
-- 000124_address_single_book_convergence.up.sql
--
-- CANONICAL ADDRESS MODEL (owner locked):
--   One account -> one address book (0..N addresses).
--   Exactly one primary when >=1 active address, zero when none.
--   No shipping/sender roles, no purpose, no tags.
--   Label/nickname is recognition-only.
--   Every product uses the account's primary address; there is no
--   product-level origin address (farm_address_id is obsolete).
--
-- This migration converges databases that still carry the rejected
-- role/purpose/tag architecture and the product-level origin pointer.
-- It is idempotent so a fresh baseline (which already omits these)
-- and an existing database converge to the same shape.
-- ============================================================

-- 1. Drop the rejected tag vocabulary (added by the retired 000119).
ALTER TABLE addresses DROP CONSTRAINT IF EXISTS addresses_tags_known;
ALTER TABLE addresses DROP CONSTRAINT IF EXISTS addresses_tags_not_empty;
DROP INDEX IF EXISTS idx_addresses_tags;
ALTER TABLE addresses DROP COLUMN IF EXISTS tags;

-- 2. Drop the older single-purpose column and its residue, if any.
DROP INDEX IF EXISTS idx_addresses_purpose;
ALTER TABLE addresses DROP CONSTRAINT IF EXISTS addresses_purpose_check;
ALTER TABLE addresses DROP COLUMN IF EXISTS purpose;

-- 3. Drop the product-level origin address authority. Every product now
--    resolves its origin from the account's primary address.
ALTER TABLE products DROP CONSTRAINT IF EXISTS products_farm_address_id_fkey;
ALTER TABLE products DROP COLUMN IF EXISTS farm_address_id;

-- The legacy `listings` table (which also carried farm_address_id) was
-- already dropped by migration 000010 on any database past that point. Guard
-- on its existence so this migration is safe whether or not it is still
-- present (ALTER TABLE on a missing table errors even with IF EXISTS).
DO $$
BEGIN
    IF to_regclass('public.listings') IS NOT NULL THEN
        ALTER TABLE listings DROP CONSTRAINT IF EXISTS listings_farm_address_id_fkey;
        ALTER TABLE listings DROP COLUMN IF EXISTS farm_address_id;
    END IF;
END $$;
