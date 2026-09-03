-- SHIPPING-03C: Drop legacy auction_id from shipping_quotes
-- auction_id was a redundant field: source_type + source_id already provides
-- canonical auction identity (source_type='auction', source_id=auction.id).
-- No validation, query, or authorization reads auction_id for business logic.
-- The field was nullable and only written by NewAuctionShippingQuote (now removed).
--
-- Verified by SHIPPING-03C audit:
--   - Repository queries filter on source_type/source_id, never auction_id
--   - ValidateQuoteForCheckout validates source_type/source_id
--   - GetItemReference() fallback to AuctionID was dead code
--   - Mobile ignores auction_id entirely

ALTER TABLE shipping_quotes DROP COLUMN IF EXISTS auction_id;
