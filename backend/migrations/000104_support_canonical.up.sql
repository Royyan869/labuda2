-- CANONICAL SUPPORT CONVERGENCE (000104)
--
-- This migration removes the historical competing Support designs and locks the
-- single canonical model:
--
-- 1. ONE TICKET = ONE SUPPORT CONVERSATION.
--    Support rooms have exactly one human participant (the ticket owner,
--    participant_a). The agent side is authorized by the Support domain at
--    request time, so participant_b is NULL for support rooms. Non-support
--    rooms keep the existing NOT NULL invariant via a partial CHECK.
--
-- 2. The previous pair-uniqueness constraint is replaced by a partial unique
--    index that applies only to non-support rooms, so many support rooms may
--    coexist for the same user (one per ticket).
--
-- 3. subject/description become first-class support_tickets columns instead of
--    being smuggled inside the metadata JSON blob (single representation).
--
-- 4. The dead support_admins workload/queue table is purged. Canonical
--    assignment lives on support_tickets.assigned_admin_id.

-- ---------------------------------------------------------------------------
-- 1 & 2. Bounded support-room participant model
-- ---------------------------------------------------------------------------
ALTER TABLE chat_rooms ALTER COLUMN participant_b DROP NOT NULL;

ALTER TABLE chat_rooms
    ADD CONSTRAINT chat_rooms_non_support_participant_b
    CHECK (room_type = 'support' OR participant_b IS NOT NULL);

ALTER TABLE chat_rooms
    DROP CONSTRAINT chat_rooms_participant_a_participant_b_room_type_key;

CREATE UNIQUE INDEX chat_rooms_participant_pair_key
    ON chat_rooms (participant_a, participant_b, room_type)
    WHERE room_type <> 'support';

-- ---------------------------------------------------------------------------
-- 3. First-class support ticket subject/description
-- ---------------------------------------------------------------------------
ALTER TABLE support_tickets ADD COLUMN subject text;
ALTER TABLE support_tickets ADD COLUMN description text;

-- ---------------------------------------------------------------------------
-- 4. Purge the dead support_admins assignment pool
-- ---------------------------------------------------------------------------
DROP TABLE IF EXISTS support_admins;
