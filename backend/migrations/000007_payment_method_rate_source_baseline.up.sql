-- PASS_19A: Midtrans public fee baseline + rate-source verification metadata.
--
-- PASS_18V seeded payment_methods with illustrative placeholder rates. This
-- pass (a) records, per method, whether its rate is an unverified public
-- Midtrans pricing snapshot, an owner-confirmed merchant-contract rate, or a
-- manual admin override, and (b) adds dana/convenience_store coverage, which
-- PASS_18V did not seed.
--
-- Owner policy (PASS_19A addendum): only bank_transfer, qris, dana,
-- convenience_store, and credit_card (card payment) are active initially.
-- ShopeePay, SPayLater, Kredivo, and Akulaku PayLater are explicitly
-- forbidden and are never seeded here — see entity.AllowedMidtransChannels,
-- which no longer contains their channel codes at all, so no method row can
-- ever reference them.
--
-- Public Midtrans pricing is checked externally, not through a merchant
-- dashboard/contract, and can change or differ from Labuda's real merchant
-- terms once a Midtrans contract exists (see rate_source_note per row).

ALTER TABLE payment_methods
    ADD COLUMN rate_source text NOT NULL DEFAULT 'public_baseline',
    ADD COLUMN rate_source_note text,
    ADD COLUMN merchant_verified_at timestamptz;

ALTER TABLE payment_methods
    ADD CONSTRAINT payment_methods_rate_source_check
        CHECK (rate_source IN ('public_baseline', 'merchant_verified', 'manual_override'));

-- Label the PASS_18V placeholder rows as what they actually are: an
-- unverified public-pricing snapshot, not a Midtrans merchant contract rate.
-- credit_card (card payment) stays enabled per the PASS_19A addendum — card
-- payment is allowed; only PayLater/installment products are forbidden.
UPDATE payment_methods
SET rate_source = 'public_baseline',
    rate_source_note = 'Public Midtrans pricing checked externally on 2026-07-06. Not Labuda''s merchant-contract rate.'
WHERE method_code IN ('bank_transfer', 'qris', 'credit_card');

-- New public-baseline methods (PASS_19A coverage gap fix, addendum scope).
INSERT INTO payment_methods
    (method_code, display_name, enabled, fee_type, flat_amount_rupiah, percent_bps, min_fee_rupiah, max_fee_rupiah,
     midtrans_channels, sort_order, rate_source, rate_source_note)
VALUES
    ('dana', 'DANA', true, 'percent', 0, 150, NULL, NULL,
        ARRAY['dana'], 25,
        'public_baseline',
        'DANA''s exact public rate was not in the 2026-07-06 checked Midtrans snapshot; seeded at 1.5% as a same-category e-wallet placeholder (matching GoPay/ShopeePay-tier public pricing). Needs explicit verification before being treated as reliable, even as a baseline.'),
    ('convenience_store', 'Indomaret / Alfamart / Alfamidi / DAN+DAN', true, 'flat', 5000, 0, NULL, NULL,
        ARRAY['alfamart', 'indomaret'], 40,
        'public_baseline',
        'Public Midtrans pricing checked externally on 2026-07-06. Not Labuda''s merchant-contract rate. Alfamidi and DAN+DAN are served over the same Alfamart Group retail network and are billed through the alfamart Midtrans channel code — there is no separate Midtrans channel code for them.');
