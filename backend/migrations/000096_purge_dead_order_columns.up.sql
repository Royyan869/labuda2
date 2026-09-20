-- 000096 PURGE DEAD ORDER COLUMNS — ORDER-SCOPE-B (SCHEMA ↔ CONSUMER PARITY)
--
-- Proven before this migration: every production `orders` SQL statement was
-- EXPLAINed against a freshly migrated database (migrations 000001–000095), and
-- every identifier was classified. These six columns are dead:
--
--   orders.shipping_destination — never written by any Go code or migration.
--       Its two readers (OrderRepository.GetByShippingQuoteID and
--       GetBlockingOrderByShippingQuoteID) scanned it into the address-snapshot
--       field, so a shipping-quote lookup silently lost the buyer's address.
--       Readers now use the canonical orders.address_snapshot (the immutable
--       creation snapshot exposed as Order.ShippingDestination).
--
--   orders.refunded_amount — never written (defaulted to 0 forever). Readers now
--       use the canonical refund domain: the sum of gateway-succeeded refunds
--       (COALESCE(refunded_product_amount, final_refund_amount) +
--       COALESCE(refunded_shipping_amount, 0)). Consumers updated: admin order
--       detail, order_summaries projection (all three projection queries) and
--       the admin order list; the finance verifier stopped loading the column.
--
--   orders.destination_address — superseded address column: never written and
--       never read (the address snapshot lives in orders.address_snapshot).
--
--   orders.discount_code / orders.discount_type / orders.discount_value —
--       never written and never read. The discount authority is the pricing
--       token snapshot (pricing_tokens.discount_*), never the order row.
--       discount_type_enum stays: discounts.type still uses it.
--
-- No constraint or index depends on these columns (verified against a freshly
-- migrated database). orders_check, which referenced escrow_amount, was already
-- removed when 000093 dropped that column.
--
-- 000001 is intentionally NOT edited: it is the canonical baseline replay
-- history, and 000093/000095/000096 record the deliberate purges in order.
-- Fresh replay determinism is preserved: 000001 creates the columns (with their
-- DEAD COLUMN notes), then the purge migrations remove them.

ALTER TABLE orders DROP COLUMN IF EXISTS shipping_destination;
ALTER TABLE orders DROP COLUMN IF EXISTS refunded_amount;
ALTER TABLE orders DROP COLUMN IF EXISTS destination_address;
ALTER TABLE orders DROP COLUMN IF EXISTS discount_code;
ALTER TABLE orders DROP COLUMN IF EXISTS discount_type;
ALTER TABLE orders DROP COLUMN IF EXISTS discount_value;
