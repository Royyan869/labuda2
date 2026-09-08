-- 000076_canonical_promotion_impression_ack
--
-- Canonical client impression acknowledgement.
--
-- The canonical delivery measurement vocabulary now distinguishes two
-- truthful server-side facts:
--
--   'included'   → the server placed the canonical card into a returned feed
--                  response (existing semantics; every included row now also
--                  carries a server-issued exposure_id token returned to the
--                  client with that card).
--   'impression' → the client EXPLICITLY acknowledged that the canonical card
--                  was actually exposed/rendered, by echoing the issued
--                  exposure_id back to the canonical acknowledgement
--                  endpoint. Never created by the server on its own.
--
-- exposure_id is the canonical exposure identity:
--   * issued ONLY on 'included' rows (a card that actually entered a
--     response), server-generated UUID;
--   * echoed verbatim on the 'impression' row that acknowledges it.
--
-- Idempotency boundary (DB-enforced): one impression per issued exposure.
--   canonical_delivery_events_impression_exposure_unique is a partial unique
--   index on (exposure_id) WHERE event_type = 'impression' — a retried /
--   re-sent acknowledgement hits the unique violation and maps to an
--   idempotent success. Included tokens are also unique among themselves
--   (partial unique index), so a client can never be handed two deliveries
--   sharing one token.
--
-- Existing 'included' rows keep exposure_id NULL: they were issued before the
-- exposure-token feature and honestly carry no token (a client cannot
-- acknowledge a delivery that never presented one). Old rows are preserved.

ALTER TABLE canonical_promotion_delivery_events
    ADD COLUMN IF NOT EXISTS exposure_id uuid;

-- Vocabulary now admits both truthful observation types.
ALTER TABLE canonical_promotion_delivery_events
    DROP CONSTRAINT IF EXISTS canonical_delivery_event_type_valid;

ALTER TABLE canonical_promotion_delivery_events
    ADD CONSTRAINT canonical_delivery_event_type_valid
    CHECK (event_type IN ('included', 'impression'));

-- Exposure tokens are unique among issued inclusions.
CREATE UNIQUE INDEX IF NOT EXISTS canonical_delivery_events_included_exposure_unique
    ON canonical_promotion_delivery_events (exposure_id)
    WHERE event_type = 'included';

-- THE acknowledgement idempotency boundary: at most one impression per
-- issued exposure, regardless of client retries / HTTP repeats.
CREATE UNIQUE INDEX IF NOT EXISTS canonical_delivery_events_impression_exposure_unique
    ON canonical_promotion_delivery_events (exposure_id)
    WHERE event_type = 'impression';