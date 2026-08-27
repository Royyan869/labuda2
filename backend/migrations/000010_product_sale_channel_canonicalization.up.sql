-- PASS_21C: canonicalize Product/Listing(FixedPriceSale)/Auction as siblings.
--
-- Drops all remaining schema residue from the rejected pre-refactor design
-- where Auction was created FROM a Listing and Listing was the generic sale
-- parent (Product/Listing/Auction unified into one `listings` table).
-- PASS_21A/21B already proved zero Go code in backend/internal or
-- backend/cmd reads or writes any of the columns/tables below. Verified
-- directly against the live dev DB before writing this migration: every
-- affected table/column is empty (0 rows), so this is a pure schema drop,
-- not a data migration.
--
-- shipping_quotes.listing_id is a distinct, higher-severity finding: it is
-- declared NOT NULL with no default, no trigger, and no FK, while the one
-- and only Go INSERT path (ShippingQuoteRepositoryImpl.Create) never
-- supplies a value for it. This means CreateShippingQuote has been
-- structurally broken (guaranteed NOT NULL violation) since the moment this
-- column was set NOT NULL — confirmed live: `shipping_quotes` has 0 rows in
-- the dev DB, i.e. this path has likely never successfully executed. This
-- migration fixes that live bug by dropping the dead column.

-- ============================================================
-- Drop dead listing_id columns (all confirmed zero rows, zero
-- read/write Go code referencing them)
-- ============================================================

ALTER TABLE auctions DROP CONSTRAINT IF EXISTS auctions_listing_id_fkey;
DROP INDEX IF EXISTS idx_auctions_listing_id;
ALTER TABLE auctions DROP COLUMN IF EXISTS listing_id;

DROP INDEX IF EXISTS idx_pricing_tokens_listing_id;
ALTER TABLE pricing_tokens DROP COLUMN IF EXISTS listing_id;

ALTER TABLE order_items DROP CONSTRAINT IF EXISTS order_items_listing_id_fkey;
DROP INDEX IF EXISTS idx_order_items_listing_id;
ALTER TABLE order_items DROP COLUMN IF EXISTS listing_id;

-- shipping_quotes.listing_id: NOT NULL, no FK, no writer — fixes a live
-- structural bug (see header comment) as a side effect of the cleanup.
DROP INDEX IF EXISTS idx_shipping_quotes_listing_id;
ALTER TABLE shipping_quotes DROP COLUMN IF EXISTS listing_id;

-- ============================================================
-- Drop whole tables that exist only to support the old listing_id
-- design and have zero Go code reading/writing them at all.
-- ============================================================

DROP TABLE IF EXISTS listing_shipping_options;
DROP TABLE IF EXISTS listing_views;

-- ============================================================
-- Drop the legacy unified Product+Listing+Auction parent table.
-- Confirmed zero INSERT/UPDATE/DELETE/SELECT against it anywhere in
-- backend/internal or backend/cmd (enforced going forward by
-- internal/platform/schemaguard.TestNoProductionCodeReadsLegacyListingsTable).
-- All FKs referencing it (auctions, listing_shipping_options,
-- listing_views, order_items) were already dropped above.
-- ============================================================

DROP TABLE IF EXISTS listings;

-- ============================================================
-- Rule 9 (canonical architecture): one Product may have only ONE
-- active selling channel at a time — either an active/pending Listing
-- (fixed_price_sales) or an active/pending Auction, never both.
--
-- Same-table partial unique indexes already prevent two active rows
-- within a single table (uniq_active_auction_per_product,
-- uniq_active_fixed_price_sale_per_product). Nothing previously
-- enforced this ACROSS the two tables. Both create flows currently
-- always mint a brand-new Product per channel, so this trigger is not
-- expected to fire on any reachable path today — it exists to make a
-- future "attach a second channel to an existing product" feature
-- fail loudly at the database instead of silently violating rule 9.
-- ============================================================

CREATE OR REPLACE FUNCTION enforce_single_active_sale_channel_per_product()
RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'fixed_price_sales' THEN
        IF NEW.status IN ('draft', 'active') THEN
            IF EXISTS (
                SELECT 1 FROM auctions
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'scheduled', 'active', 'waiting_settlement')
            ) THEN
                RAISE EXCEPTION 'product % already has an active auction; cannot activate a fixed-price sale for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    ELSIF TG_TABLE_NAME = 'auctions' THEN
        IF NEW.status IN ('draft', 'scheduled', 'active', 'waiting_settlement') THEN
            IF EXISTS (
                SELECT 1 FROM fixed_price_sales
                WHERE product_id = NEW.product_id
                  AND status IN ('draft', 'active')
            ) THEN
                RAISE EXCEPTION 'product % already has an active fixed-price sale; cannot activate an auction for the same product (rule 9: one active selling channel per product)', NEW.product_id
                    USING ERRCODE = 'check_violation';
            END IF;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_fixed_price_sales_single_active_channel
    BEFORE INSERT OR UPDATE ON fixed_price_sales
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();

CREATE TRIGGER trg_auctions_single_active_channel
    BEFORE INSERT OR UPDATE ON auctions
    FOR EACH ROW EXECUTE FUNCTION enforce_single_active_sale_channel_per_product();
