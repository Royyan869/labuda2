-- 000080_promotion_analytics_contract_convergence
--
-- ANALYTICS AGGREGATE CONVERGENCE — contract_id authority
--
-- Moves canonical_promotion_delivery_events from competing promotions aggregate
-- (FK promotions.id) to canonical promotion_contracts authority.
-- After this, analytics identity = contract_id (promotion_contracts).
--
-- Steps:
-- 1. Drop FK to promotions (canonical_promotion_delivery_events_promotion_id_fkey)
-- 2. Rename column promotion_id -> contract_id
-- 3. Re-add FK to promotion_contracts (DEFERRABLE, not strict for old test rows)
-- 4. Recreate indexes keyed by contract_id
-- 5. Update constraints that referenced promotion_id

-- Drop FK to competing aggregate if exists
ALTER TABLE canonical_promotion_delivery_events DROP CONSTRAINT IF EXISTS canonical_promotion_delivery_events_promotion_id_fkey;

-- Drop old indexes on promotion_id
DROP INDEX IF EXISTS canonical_delivery_events_promotion_time_idx;

-- Rename column
ALTER TABLE canonical_promotion_delivery_events RENAME COLUMN promotion_id TO contract_id;

-- Rename/drop constraints that reference old name
ALTER TABLE canonical_promotion_delivery_events DROP CONSTRAINT IF EXISTS canonical_delivery_event_promotion_required;
ALTER TABLE canonical_promotion_delivery_events ADD CONSTRAINT canonical_delivery_event_contract_required CHECK (contract_id <> '00000000-0000-0000-0000-000000000000');

-- Recreate FK to promotion_contracts (not valid for old orphan rows — use NOT VALID initially then validate)
ALTER TABLE canonical_promotion_delivery_events
    ADD CONSTRAINT canonical_delivery_events_contract_id_fkey
    FOREIGN KEY (contract_id) REFERENCES promotion_contracts(id) NOT VALID;

-- Validate FK on future rows (existing orphan rows remain — clean DB has none)
-- Do not validate existing rows now; allow convergence to proceed
-- ALTER TABLE ... VALIDATE CONSTRAINT will be done lazily when old promotions rows are gone

-- Recreate index on contract_id
CREATE INDEX IF NOT EXISTS canonical_delivery_events_contract_time_idx
    ON canonical_promotion_delivery_events (contract_id, server_occurred_at);

-- Viewer index remains
-- Ensure exposure indexes still valid (they are on exposure_id only)
