-- 000080_promotion_analytics_contract_convergence down (forward-only convergence; down is best-effort revert)

ALTER TABLE canonical_promotion_delivery_events DROP CONSTRAINT IF EXISTS canonical_delivery_events_contract_id_fkey;
ALTER TABLE canonical_promotion_delivery_events DROP CONSTRAINT IF EXISTS canonical_delivery_event_contract_required;
DROP INDEX IF EXISTS canonical_delivery_events_contract_time_idx;
ALTER TABLE canonical_promotion_delivery_events RENAME COLUMN contract_id TO promotion_id;
ALTER TABLE canonical_promotion_delivery_events ADD CONSTRAINT canonical_delivery_event_promotion_required CHECK (promotion_id <> '00000000-0000-0000-0000-000000000000');
ALTER TABLE canonical_promotion_delivery_events ADD CONSTRAINT canonical_promotion_delivery_events_promotion_id_fkey FOREIGN KEY (promotion_id) REFERENCES promotions(id);
CREATE INDEX IF NOT EXISTS canonical_delivery_events_promotion_time_idx ON canonical_promotion_delivery_events (promotion_id, server_occurred_at);
