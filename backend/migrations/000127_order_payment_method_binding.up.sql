-- 000127 ORDER PAYMENT METHOD BINDING — PHASE 2 FOLLOW-UP
--
-- Business invariant: the buyer selects a payment method BEFORE "Buat Pesanan",
-- and that exact method identity is part of the checkout decision and must be
-- the method used to pay the order.
--
-- The previous design bound only the resulting FEE onto the order
-- (orders.service_fee_amount). That is insufficient: two different methods with
-- the same fee (including two zero-fee methods) are indistinguishable, and a
-- bound zero-fee method is indistinguishable from an order with no bound method
-- (auction-claim orders). Payment creation could therefore use a different
-- method than the one the buyer selected.
--
-- This column is the canonical binding of the selected method identity:
--   NULL  = no method bound yet (auction-claim / post-order paths that select a
--           method at payment time; the first payment binds it).
--   value = the exact method_code the buyer selected at checkout.
--
-- The money snapshots (service_fee_amount, total_payable_amount) remain the
-- backend fee authority; this column adds the identity, not a second fee.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS payment_method_code text
    REFERENCES payment_methods(method_code);
