-- 000076_canonical_promotion_impression_ack down

DROP INDEX IF EXISTS canonical_delivery_events_impression_exposure_unique;
DROP INDEX IF EXISTS canonical_delivery_events_included_exposure_unique;

ALTER TABLE canonical_promotion_delivery_events
    DROP CONSTRAINT IF EXISTS canonical_delivery_event_type_valid;

ALTER TABLE canonical_promotion_delivery_events
    ADD CONSTRAINT canonical_delivery_event_type_valid
    CHECK (event_type IN ('included'));

ALTER TABLE canonical_promotion_delivery_events
    DROP COLUMN IF EXISTS exposure_id;