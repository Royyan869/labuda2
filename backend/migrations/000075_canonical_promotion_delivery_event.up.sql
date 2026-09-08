-- 000075_canonical_promotion_delivery_event
--
-- Canonical delivery measurement: one immutable observation row per time a
-- canonical promotion card was actually INCLUDED in a feed response served to
-- a viewer.
--
-- Truthfulness contract (single meaning only):
--   event_type = 'included'  → the promotion card was placed in the feed
--                              response the server returned (after slot policy
--                              and interleave). This is a SERVER-SIDE
--                              observation; it is NOT an 'impression' — a
--                              client-visible impression requires an explicit
--                              client acknowledgement, which does not exist
--                              for canonical promotions yet.
--
-- Attribution: every row resolves to the CANONICAL promotion id (FK to
-- promotions). The legacy promotion_instances model is never written by
-- canonical measurement, and no legacy lifecycle authority is entered.
--
-- Duplicate model: each row is one observation with its own primary key.
-- Repeated inclusion of the same promotion across different pages/refreshes
-- is a real, intended delivery (no global-unique semantics invented here).
-- The only proven in-page duplicate vector (the same promotion card placed
-- more than once in ONE response) is prevented at the measurement authority
-- (page-level dedup by promotion id) — see DeliveryMeasurementService.
--
-- server_occurred_at is the DB clock (server time authority); client
-- timestamps are never trusted.

CREATE TABLE IF NOT EXISTS canonical_promotion_delivery_events (
    id uuid PRIMARY KEY,
    promotion_id uuid NOT NULL REFERENCES promotions(id),
    target_type text NOT NULL,
    target_id uuid NOT NULL,
    viewer_id uuid NOT NULL,
    event_type text NOT NULL,
    server_occurred_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT canonical_delivery_event_promotion_required CHECK (promotion_id <> '00000000-0000-0000-0000-000000000000'),
    CONSTRAINT canonical_delivery_event_target_required CHECK (target_type <> ''),
    CONSTRAINT canonical_delivery_event_viewer_required CHECK (viewer_id <> '00000000-0000-0000-0000-000000000000'),
    CONSTRAINT canonical_delivery_event_type_valid CHECK (event_type IN ('included'))
);

CREATE INDEX IF NOT EXISTS canonical_delivery_events_promotion_time_idx
    ON canonical_promotion_delivery_events (promotion_id, server_occurred_at);

CREATE INDEX IF NOT EXISTS canonical_delivery_events_viewer_time_idx
    ON canonical_promotion_delivery_events (viewer_id, server_occurred_at);