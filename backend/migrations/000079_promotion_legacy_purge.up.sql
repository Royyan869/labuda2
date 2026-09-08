-- 000079_promotion_legacy_purge
--
-- PURGE FORBIDDEN DURATION AUTHORITY (§28, §29)
--
-- Drops legacy promotion duration-entitlement tables that are forbidden as
-- financial authority: purchase hours → validity window → consumed_duration_hours
-- → wall-clock expiry → duration exhausted.
--
-- Canonical authority is promotion_contracts + promotion_contract_targets +
-- promotion_delivery_tickets + promotion_qualified_impressions + financial ledger.
-- These legacy tables have zero production callers after canonical queue/pacing/finalization
-- convergence (see dependencies.go feed injector now canonical-only).
--
-- Forward-only. No data migration: no production data exists under the forbidden model
-- that must be preserved. The ledger remains the only financial truth.

DROP TABLE IF EXISTS promotion_events CASCADE;
DROP TABLE IF EXISTS promotion_instances CASCADE;
DROP TABLE IF EXISTS promotion_ownerships CASCADE;
DROP TABLE IF EXISTS promotion_packages CASCADE;

-- Legacy duration worker state: nothing to drop (worker is application-only).
-- Legacy indexes are dropped with tables.

-- Note: competing `promotions` aggregate (canonical_promotion_* analytics) is NOT dropped here;
-- its lifecycle usage (Create/Activate/ListMy) is being deprecated in app layer first,
-- while canonical_promotion_delivery_events remains as projection until migrated to contract_id.
