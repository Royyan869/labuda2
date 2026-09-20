-- Down migration: restore the purged order columns for migration-chain
-- integrity only. They are NOT canonical sources and MUST NOT be revived:
--   shipping_destination / destination_address → orders.address_snapshot
--   refunded_amount                            → refunds domain (gateway-succeeded sum)
--   discount_code / discount_type / discount_value → pricing_tokens snapshot

ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipping_destination jsonb;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS refunded_amount bigint DEFAULT 0 NOT NULL;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS destination_address jsonb;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_code text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_type discount_type_enum DEFAULT 'percentage'::discount_type_enum;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_value numeric;
