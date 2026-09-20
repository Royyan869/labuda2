-- Down migration: restore auction_settlement_type_enum for chain integrity only.
-- WARNING: This type is OBSOLETE. Canonical authority for auction settlement
-- type is the pricing-token snapshot + entity.AuctionSettlementType; the order
-- does not persist settlement metadata.
-- Restored only so `migrate down` reproduces the historical 000001 snapshot.

CREATE TYPE auction_settlement_type_enum AS ENUM (
    'standard',
    'buy_now'
);
