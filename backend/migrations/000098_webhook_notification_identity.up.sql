-- ============================================================
-- 000098 CANONICAL WEBHOOK NOTIFICATION IDENTITY
--
-- WHY THIS EXISTS (REC-3 root defect)
-- -----------------------------------
-- payment_webhook_events.event_id held the Midtrans transaction_id and was
-- UNIQUE. Midtrans sends SEVERAL notifications for one gateway transaction as
-- its status advances (pending → settlement, settlement → deny on reversal,
-- …), all sharing that transaction_id. The first notification therefore
-- occupied the only row for that transaction and every later one hit
-- ON CONFLICT DO NOTHING and was discarded as an "idempotent duplicate".
--
-- Proven consequence (disposable Postgres): pending then settlement left the
-- payment pending forever, with NO event row and NO durable failure record —
-- the money signal was silently thrown away.
--
-- THE CONTRACT FACT
-- -----------------
-- The Midtrans payment-notification payload carries NO provider-assigned
-- notification id (the documented fields are transaction_time,
-- transaction_status, transaction_id, status_message, status_code,
-- signature_key, payment_type, order_id, merchant_id, gross_amount,
-- fraud_status, currency plus method-specific extras). Midtrans' own guidance
-- is to deduplicate on the merchant order reference, and it warns that several
-- notifications per transaction are possible and may arrive out of order.
-- A notification's identity must therefore be DERIVED FROM ITS CONTENT.
--
-- THE CANONICAL IDENTITY
-- ----------------------
--   notification_key = md5(transaction_id | transaction_status | status_code |
--                          fraud_status | gross_amount | payment_type |
--                          order_id | refund_key | refund_amount |
--                          refund_chargeback_id)      joined by 0x1F
--
--   * identical redelivery        → identical key → idempotent
--   * different status transition → different key → PRESERVED
--   * different refund            → different key → PRESERVED
--
-- Included: every field that can distinguish one signal from another.
-- Excluded on purpose:
--   transaction_time / status_message → timestamps and free text that do not
--       distinguish a signal and could vary between redeliveries of the SAME
--       notification, which would break idempotency;
--   merchant_id / currency → constant per merchant and transaction;
--   signature_key → derived from order_id + status_code + gross_amount, so it
--       carries no additional distinguishing information.
--
-- Go MIRROR (LOCKSTEP REQUIRED)
-- -----------------------------
-- pkg/midtrans.NotificationIdentity builds the SAME string and hashes it with
-- md5. The field ORDER and the SEPARATOR in this migration MUST stay identical
-- to that function: the backfill below is only deterministic because both
-- sides agree. md5 is used because it is built into PostgreSQL — pgcrypto is
-- NOT created anywhere in this migration chain, so digest() cannot be relied
-- on. Uniqueness/dedup is the only job here; notification authenticity remains
-- signature verification, which is a separate control.
--
-- ROLE CHANGE (no column is rewritten)
-- ------------------------------------
--   event_id         → the Midtrans transaction reference. It is NO LONGER the
--                      uniqueness authority, so the UNIQUE constraint is
--                      dropped and replaced by a plain index. Existing values
--                      stay factually correct (they were always the gateway
--                      transaction id), which is why no historical row has to
--                      be rewritten or reinterpreted.
--   notification_key → the ONE canonical identity authority for idempotency.
--
-- BACKFILL SAFETY
-- ---------------
-- Every existing row was written from the same marshalled notification struct,
-- so the derived key is recomputable from the stored payload. Because
-- event_id was UNIQUE and equals payload->>'transaction_id', existing rows
-- have pairwise distinct transaction_ids and therefore pairwise distinct keys;
-- if that invariant were ever violated the unique index below FAILS LOUDLY
-- instead of silently merging two different events.
-- ============================================================

ALTER TABLE payment_webhook_events ADD COLUMN notification_key text;

UPDATE payment_webhook_events
SET notification_key = md5(
        COALESCE(payload ->> 'transaction_id', '')       || E'\x1f' ||
        COALESCE(payload ->> 'transaction_status', '')   || E'\x1f' ||
        COALESCE(payload ->> 'status_code', '')          || E'\x1f' ||
        COALESCE(payload ->> 'fraud_status', '')         || E'\x1f' ||
        COALESCE(payload ->> 'gross_amount', '')         || E'\x1f' ||
        COALESCE(payload ->> 'payment_type', '')         || E'\x1f' ||
        COALESCE(payload ->> 'order_id', '')             || E'\x1f' ||
        COALESCE(payload ->> 'refund_key', '')           || E'\x1f' ||
        COALESCE(payload ->> 'refund_amount', '')        || E'\x1f' ||
        COALESCE(payload ->> 'refund_chargeback_id', '')
    )
WHERE notification_key IS NULL;

ALTER TABLE payment_webhook_events ALTER COLUMN notification_key SET NOT NULL;

-- Demote the gateway transaction reference from identity to reference.
ALTER TABLE payment_webhook_events DROP CONSTRAINT payment_webhook_events_event_id_key;
CREATE INDEX idx_payment_webhook_events_event_id ON payment_webhook_events (event_id);

-- The one canonical idempotency authority.
CREATE UNIQUE INDEX payment_webhook_events_notification_key_key
    ON payment_webhook_events (notification_key);

COMMENT ON COLUMN payment_webhook_events.notification_key IS
    'Canonical identity of ONE Midtrans notification, derived from its signal-defining fields (md5 of the 0x1F-joined tuple defined in migration 000098). Uniqueness authority: identical redelivery dedups, a different status/refund is a distinct event. Mirror: pkg/midtrans.NotificationIdentity.';

COMMENT ON COLUMN payment_webhook_events.event_id IS
    'Midtrans gateway TRANSACTION reference (transaction_id). NOT unique: one transaction produces several notifications (status transitions), each stored as its own row keyed by notification_key.';
