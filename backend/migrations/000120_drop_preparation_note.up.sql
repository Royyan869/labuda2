-- 000120 DROP PREPARATION NOTE — ORDER-A3-C
--
-- Owner decision: "catatan persiapan" is a killed design. The seller-authored
-- note was removed end-to-end — both create screens, the Product write/read
-- paths, the order snapshot, the chat auction projection, and every wire DTO.
-- The columns are dropped so the obsolete field cannot quietly reappear through
-- a migration, fixture, or projection.
--
-- preparation_time (the readiness expectation that buyers actually need) stays.
-- `auctions.preparation_note` was already dropped by 000046; the legacy
-- `listings` table was dropped by 000010.

ALTER TABLE products DROP COLUMN IF EXISTS preparation_note;
ALTER TABLE orders DROP COLUMN IF EXISTS preparation_note_snapshot;
