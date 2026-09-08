-- 000067_promotion_exposure_attempt_identity (down)

DROP INDEX IF EXISTS promotion_events_exposure_attempt_id_unique;
ALTER TABLE promotion_events
    DROP COLUMN IF EXISTS exposure_attempt_id;
