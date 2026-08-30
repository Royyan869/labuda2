-- 000054_drop_dead_chat_commerce_references.up.sql
--
-- Drop the dead chat_commerce_references table and its enum.
--
-- EVIDENCE:
-- - Zero production Go code reads or writes chat_commerce_references
-- - Zero mobile code imports ChatCommerceReferenceDto
-- - The chat domain stores commerce context in chat_rooms.context_json
-- - Object references are stored in chat_messages.attachment_json
-- - The entity ChatCommerceReference was never constructed in production

DROP TABLE IF EXISTS chat_commerce_references;
DROP TYPE IF EXISTS chat_commerce_reference_target_type_enum;
