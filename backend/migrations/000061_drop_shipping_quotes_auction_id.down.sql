-- IRREVERSIBLE: This migration drops the legacy auction_id column.
-- The canonical identity is source_type + source_id.
-- Down migration restores the column for rollback safety only.

ALTER TABLE shipping_quotes ADD COLUMN auction_id uuid;
