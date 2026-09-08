-- 000067_promotion_exposure_attempt_identity
--
-- PROMOTION PHASE 4E-2 — CANONICAL EXPOSURE ATTEMPT IDENTITY
--
-- Adds the canonical server-side idempotency authority for promotion exposure
-- (viewport impression) attempts. The identity is client-generated, retried
-- safely, and persisted here with a database-level uniqueness authority.
--
-- Money rule (Model A) is unchanged: exposure attempts move NO money. Only
-- a server-validated Qualified Impression (Phase 3, migration 000066) may
-- charge PROMOTION_ALLOCATION -> PLATFORM_REVENUE. This migration touches
-- analytics only.
--
-- Semantics:
--   * exposure_attempt_id is NULL for legacy rows and for click events.
--     PostgreSQL partial unique indexes admit multiple NULLs, so legacy
--     append-only click analytics behavior is preserved.
--   * A non-NULL exposure_attempt_id is globally unique per logical exposure
--     attempt. Concurrent duplicate ingestion of the same identity collapses
--     to one row (23505 -> load existing -> idempotent success or explicit
--     conflict on incompatible immutable binding).
--   * Immutable binding (viewer, instance, surface, event_type) is enforced
--     at the application boundary; the database enforces identity uniqueness.
--
-- Forward-only. Reversible by dropping the column + index.

ALTER TABLE promotion_events
    ADD COLUMN IF NOT EXISTS exposure_attempt_id uuid;

CREATE UNIQUE INDEX IF NOT EXISTS promotion_events_exposure_attempt_id_unique
    ON promotion_events (exposure_attempt_id)
    WHERE exposure_attempt_id IS NOT NULL;
