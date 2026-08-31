-- ============================================================
-- 000056_drop_legacy_moderation_schema.up.sql
-- SLICE 1 — REMOVE REJECTED GOVERNANCECASE SCHEMA
--
-- Drops the rejected super-entity moderation_cases and its old enums.
-- Canonical replacement (reports/cases/decisions/enforcements) was
-- established in 000055.
--
-- EVIDENCE (Audit 4):
--   - moderation_cases mixes Report + Case + Decision in one row
--   - moderation_status_enum ('pending','approved','rejected','removed',
--     'enforced') is rejected canonical vocabulary
--   - moderation_resource_enum contains 'chat_message' (out of v1 scope)
--   - no foreign keys reference moderation_cases (verified)
--   - dev DB has 0 rows in moderation_cases/appeals/user_warnings
--     (verified 2026-08-30) — no data migration required
-- ============================================================

-- No inbound FKs exist (verified: zero "REFERENCES moderation_cases"
-- across backend/migrations). Drop the table directly.
DROP TABLE IF EXISTS moderation_cases;

-- Old enums are now unused (only moderation_cases consumed them).
DROP TYPE IF EXISTS moderation_status_enum;
DROP TYPE IF EXISTS moderation_resource_enum;
