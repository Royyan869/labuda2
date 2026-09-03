-- 000059_add_gateway_reference_no.up.sql
-- PAYOUT-01B: Add a dedicated column for the Midtrans-assigned reference_no.
--
-- The Midtrans Iris status-query API uses GET /payouts/{reference_no},
-- NOT GET /payouts/{external_id}. The reference_no is returned by the
-- gateway in the payout creation response and is needed for:
--   1. Status queries (reconciliation)
--   2. Gateway-side idempotency correlation
--   3. Audit/debug traceability
--
-- The column is nullable because pre-existing SUBMITTED/PROCESSING
-- withdrawals may not have a reference_no yet (gateway not yet called).
-- It is populated atomically in UpdateForSubmission when the gateway
-- creation response is received.

ALTER TABLE withdrawals ADD COLUMN gateway_reference_no text DEFAULT ''::text NOT NULL;

-- Index for future reconciliation queries: "find all SUBMITTED withdrawals
-- with a known gateway reference number, ordered by submission time."
CREATE INDEX idx_withdrawals_gateway_reference_no
    ON withdrawals (gateway_reference_no)
    WHERE gateway_reference_no != '';
