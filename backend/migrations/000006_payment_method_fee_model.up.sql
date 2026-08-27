-- PASS_18V: Payment method / buyer payment fee model.
--
-- Replaces the flat Rp3.000 buyer service fee (backend/internal/commerce/checkoutfee,
-- removed in this pass) with a canonical, backend-owned payment method table.
-- The buyer selects a method_code before payment creation; the backend looks
-- up the fee formula here and computes the buyer payment fee itself.

CREATE TABLE payment_methods (
    method_code text PRIMARY KEY,
    display_name text NOT NULL,
    enabled boolean DEFAULT true NOT NULL,
    fee_type text NOT NULL,
    flat_amount_rupiah bigint DEFAULT 0 NOT NULL,
    percent_bps integer DEFAULT 0 NOT NULL,
    min_fee_rupiah bigint,
    max_fee_rupiah bigint,
    midtrans_channels text[] DEFAULT ARRAY[]::text[] NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT payment_methods_fee_type_check
        CHECK (fee_type IN ('flat', 'percent', 'percent_plus_flat')),
    CONSTRAINT payment_methods_flat_amount_nonnegative CHECK (flat_amount_rupiah >= 0),
    CONSTRAINT payment_methods_percent_bps_nonnegative CHECK (percent_bps >= 0),
    CONSTRAINT payment_methods_min_fee_nonnegative CHECK (min_fee_rupiah IS NULL OR min_fee_rupiah >= 0),
    CONSTRAINT payment_methods_max_fee_nonnegative CHECK (max_fee_rupiah IS NULL OR max_fee_rupiah >= 0)
);

-- Illustrative canonical seed rates (Rupiah integer, no minor unit).
-- These are placeholders pending a real Midtrans merchant fee contract
-- (see PASS_18V report, "remaining debt") and can be edited by an admin
-- follow-up pass without a schema change.
INSERT INTO payment_methods
    (method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah, midtrans_channels, sort_order)
VALUES
    ('bank_transfer', 'Transfer Bank (Virtual Account)', true, 'flat', 4000, 0, NULL, NULL,
        ARRAY['bca_va', 'bni_va', 'bri_va', 'permata_va', 'other_va'], 10),
    ('qris', 'QRIS', true, 'percent', 0, 70, 500, NULL,
        ARRAY['other_qris'], 20),
    ('credit_card', 'Kartu Kredit/Debit', true, 'percent_plus_flat', 2000, 290, NULL, NULL,
        ARRAY['credit_card'], 30);

ALTER TABLE payments ADD COLUMN payment_method_code text REFERENCES payment_methods(method_code);

COMMENT ON COLUMN payments.payment_method_code IS
    'Canonical payment method selected by the buyer before this payment was created. NULL for non-order payments (billing/subscription), which are out of scope for the method-based fee model.';
