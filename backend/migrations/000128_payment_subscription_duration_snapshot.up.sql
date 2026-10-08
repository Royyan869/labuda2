-- 000128_payment_subscription_duration_snapshot
--
-- OWNER DECISION: seller subscription duration is a purchased term and MUST be
-- snapshotted on the payment record at initiation, exactly like gross_amount
-- and service_fee_amount. A later seller_subscription_configs.duration_days
-- change must not alter an already-purchased entitlement.
--
-- The payments row is the canonical purchase snapshot; no second duration
-- authority is introduced. The subscription entitlement is later written from
-- this snapshot into seller_subscriptions.duration_days.

ALTER TABLE payments
    ADD COLUMN subscription_duration_days integer;

ALTER TABLE payments
    ADD CONSTRAINT payments_subscription_duration_positive
    CHECK (subscription_duration_days IS NULL OR subscription_duration_days > 0);

-- Backfill PENDING subscription payments created before this snapshot existed
-- with the currently-active duration so an in-flight purchase can still settle.
-- Settled/terminal subscription payments already carry their entitlement length
-- on seller_subscriptions.duration_days.
UPDATE payments
SET subscription_duration_days = (
    SELECT duration_days
    FROM seller_subscription_configs
    WHERE enabled = true
    ORDER BY created_at DESC
    LIMIT 1
)
WHERE reference_type = 'subscription'
  AND status = 'pending'
  AND subscription_duration_days IS NULL;

COMMENT ON COLUMN payments.payment_method_code IS
    'Canonical payment method selected by the buyer before this payment was created. Set by every flow that carries a payment-method fee: order, billing (promote balance top-up), and seller subscription.';

COMMENT ON COLUMN payments.subscription_duration_days IS
    'Purchased seller-subscription entitlement length in days, snapshotted at initiation. NON-NULL only for reference_type = subscription. A later config change never alters an already-purchased entitlement.';
