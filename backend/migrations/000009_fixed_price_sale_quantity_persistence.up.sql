-- PASS_19E: persist real fixed-price sale quantity.
--
-- Root cause: fixed_price_sales had no quantity column at all.
-- FixedPriceSaleRepositoryImpl derived QuantityAvailable purely from status
-- (active=>1, sold/withdrawn=>0) on every Create/Update/scan, silently
-- discarding whatever real quantity the entity (constructor, ReduceQuantity,
-- RestoreQuantity) computed. Multi-quantity fixed-price listings could never
-- actually persist a partial reservation.
--
-- Owner decision: multi-quantity fixed-price selling is a real, supported
-- feature (most koi listings are unique quantity=1, but sellers with
-- multiple units of the same product must be able to list quantity > 1).
-- This migration adds real persistence; the entity/API already modeled
-- quantity correctly and needed no changes.
ALTER TABLE fixed_price_sales
    ADD COLUMN quantity_available integer DEFAULT 1 NOT NULL;

ALTER TABLE fixed_price_sales
    ADD CONSTRAINT fixed_price_sales_quantity_available_nonnegative
        CHECK (quantity_available >= 0);

-- Backfill (from-zero: only test/dev data exists). No real historical
-- quantity is recoverable from a status-only model, so this uses the same
-- status-derived mapping the repository used to compute on the fly:
-- sold/withdrawn implies quantity is exhausted; anything else defaults to
-- the DEFAULT 1 already applied by the ADD COLUMN above.
UPDATE fixed_price_sales
SET quantity_available = 0
WHERE status IN ('sold', 'withdrawn');
