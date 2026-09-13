-- 000084_for_sale_commission_canonical
-- SCOPE-CFG-01: converge seller transaction commission to a single authority.
-- Canonical runtime key: for_sale_commission_percent
--   consumed by ConfigService.GetForSaleCommission → pricing token →
--   for-sale / negotiation transaction pricing.
-- Obsolete inert duplicate: listing_commission_percent
--   had an Admin edit surface but no runtime consumer and is purged.
-- No alias, no dual-read, no dual-write: exactly one commission authority remains.

-- Ensure the canonical key exists on databases migrated before the 000001 seed
-- correction (no-op on fresh databases, where 000001 now seeds it directly).
INSERT INTO platform_configs (key, value_numeric, value_text, updated_by, updated_at)
VALUES ('for_sale_commission_percent', 4, NULL, NULL, (EXTRACT(epoch FROM now()))::bigint)
ON CONFLICT (key) DO NOTHING;

-- Purge the obsolete duplicate authority.
DELETE FROM platform_configs WHERE key = 'listing_commission_percent';