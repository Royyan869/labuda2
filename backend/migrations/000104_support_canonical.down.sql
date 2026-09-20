-- Down migration for 000104_support_canonical.
--
-- NOTE: reverting the participant model is destructive for support rooms that
-- have no agent participant. They are removed before restoring NOT NULL.

DROP TABLE IF EXISTS support_admins;

ALTER TABLE support_tickets DROP COLUMN IF EXISTS description;
ALTER TABLE support_tickets DROP COLUMN IF EXISTS subject;

DELETE FROM chat_rooms WHERE room_type = 'support' AND participant_b IS NULL;

DROP INDEX IF EXISTS chat_rooms_participant_pair_key;

ALTER TABLE chat_rooms
    ADD CONSTRAINT chat_rooms_participant_a_participant_b_room_type_key
    UNIQUE (participant_a, participant_b, room_type);

ALTER TABLE chat_rooms
    DROP CONSTRAINT IF EXISTS chat_rooms_non_support_participant_b;

ALTER TABLE chat_rooms ALTER COLUMN participant_b SET NOT NULL;
