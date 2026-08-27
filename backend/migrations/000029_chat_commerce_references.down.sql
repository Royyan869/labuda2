-- ============================================================
-- 000029_chat_commerce_references.down.sql
-- Reverts chat commerce references.
-- ============================================================

DROP TABLE IF EXISTS chat_commerce_references;

DO $$
BEGIN
    DROP TYPE IF EXISTS chat_commerce_reference_target_type_enum;
EXCEPTION
    WHEN dependent_objects_still_exist THEN NULL;
END $$;
