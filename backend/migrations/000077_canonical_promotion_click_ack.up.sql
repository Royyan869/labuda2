-- 000077_canonical_promotion_click_ack
--
-- Canonical client click acknowledgement.
--
-- The canonical delivery measurement vocabulary now admits a third truthful
-- client-explicit observation:
--
--   'included'   → the server placed the canonical card into a returned feed
--                  response (existing semantics; carries the issued exposure_id).
--   'impression' → the client EXPLICITLY acknowledged that the canonical card
--                  was rendered, by echoing the issued exposure_id (existing
--                  semantics).
--   'click'      → the client EXPLICITLY acknowledged an explicit user tap on
--                  the canonical card, echoing the issued exposure_id. Never
--                  created by the server on its own; never inferred from
--                  inclusion, selection, or navigation.
--
-- The click row echoes the SAME exposure identity the client received with
-- the card. The promotion identity is NOT accepted from the client: the
-- acknowledgement authority resolves the canonical promotion from the issued
-- exposure event, so an arbitrary {promotion_id, event_type} claim can never
-- fabricate a click.
--
-- Idempotency boundary (DB-enforced): one click per issued exposure.
--   canonical_delivery_events_click_exposure_unique is a partial unique index
--   on (exposure_id) WHERE event_type = 'click' — a retried / re-sent click
--   acknowledgement hits the unique violation and maps to an idempotent
--   success, exactly like the impression boundary. A viewer may legitimately
--   tap after acknowledging an impression (or without acknowledging one): the
--   impression and click boundaries are independent, so both a click AND an
--   impression can exist for the same exposure — each at most once.

-- Vocabulary now admits the third truthful client-explicit observation type.
ALTER TABLE canonical_promotion_delivery_events
    DROP CONSTRAINT IF EXISTS canonical_delivery_event_type_valid;

ALTER TABLE canonical_promotion_delivery_events
    ADD CONSTRAINT canonical_delivery_event_type_valid
    CHECK (event_type IN ('included', 'impression', 'click'));

-- THE click acknowledgement idempotency boundary: at most one click per
-- issued exposure, regardless of client retries / HTTP repeats.
CREATE UNIQUE INDEX IF NOT EXISTS canonical_delivery_events_click_exposure_unique
    ON canonical_promotion_delivery_events (exposure_id)
    WHERE event_type = 'click';
