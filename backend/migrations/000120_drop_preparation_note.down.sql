-- Down migration: restore the historical columns.
--
-- The concept stays dead — no producer, consumer, DTO, or projection exists for
-- them anymore; this is only a rollback path for migration tooling.

ALTER TABLE products ADD COLUMN IF NOT EXISTS preparation_note text;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS preparation_note_snapshot text;
