-- REC-6 Slice 1: Add refund reason for gateway success arriving after order
-- entered a terminal state (expired, cancelled, etc.) that prevents normal
-- payment finalization. This reason is used by the canonical refund-intent
-- authority created in this slice.
ALTER TYPE refund_reason_enum ADD VALUE IF NOT EXISTS 'gateway_captured_after_order_invalid';
