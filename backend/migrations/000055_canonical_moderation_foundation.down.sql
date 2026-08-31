-- ============================================================
-- 000055_canonical_moderation_foundation.down.sql
-- Reverse of 000055: remove the canonical moderation foundation.
-- Restores user_warnings and appeals to their pre-000055 shape.
-- ============================================================

-- appeals: restore legacy report_id column (empty), drop decision_id
ALTER TABLE appeals DROP CONSTRAINT IF EXISTS appeals_status_check;
ALTER TABLE appeals DROP CONSTRAINT IF EXISTS appeals_decision_id_required;
ALTER TABLE appeals DROP CONSTRAINT IF EXISTS appeals_decision_id_fkey;
DROP INDEX IF EXISTS idx_appeals_decision_id;
ALTER TABLE appeals DROP COLUMN IF EXISTS decision_id;
ALTER TABLE appeals ADD COLUMN report_id uuid;

-- user_warnings: drop decision provenance
ALTER TABLE user_warnings DROP CONSTRAINT IF EXISTS user_warnings_decision_unique;
ALTER TABLE user_warnings DROP CONSTRAINT IF EXISTS user_warnings_decision_id_required;
ALTER TABLE user_warnings DROP CONSTRAINT IF EXISTS user_warnings_decision_id_fkey;
ALTER TABLE user_warnings DROP COLUMN IF EXISTS decision_id;

-- enforcements
DROP TABLE IF EXISTS enforcements;

-- decisions (trigger + function dropped with table via CASCADE)
DROP TABLE IF EXISTS decisions;
DROP FUNCTION IF EXISTS prevent_decisions_update();

-- reports (case_id FK dropped with table)
DROP TABLE IF EXISTS reports;

-- cases
DROP TABLE IF EXISTS cases;

-- enums
DROP TYPE IF EXISTS enforcement_status_enum;
DROP TYPE IF EXISTS decision_outcome_enum;
DROP TYPE IF EXISTS case_status_enum;
DROP TYPE IF EXISTS moderation_target_type_enum;
