-- 000128_payment_subscription_duration_snapshot (down)

ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_subscription_duration_positive;
ALTER TABLE payments DROP COLUMN IF EXISTS subscription_duration_days;

COMMENT ON COLUMN payments.payment_method_code IS
    'Canonical payment method selected by the buyer before this payment was created. NULL for non-order payments (billing/subscription), which are out of scope for the method-based fee model.';
