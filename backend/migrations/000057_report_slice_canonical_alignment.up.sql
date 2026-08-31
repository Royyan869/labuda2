-- ============================================================
-- 000057_report_slice_canonical_alignment.up.sql
-- SLICE 2 — CANONICAL REPORT RUNTIME SCHEMA ALIGNMENT
--
-- Aligns the `reports` table created by 000055 (preliminary shape:
-- target_type/target_id/reason) to the LOCKED canonical Report model
-- from "LABUDA — CANONICAL MODERATION SPECIFICATION v1" §6:
--
--   id, reporter_id, subject_type, subject_id, reason_code,
--   reason_note, evidence_snapshot, created_at
--
-- Changes:
--   1. Recreate `reports` with canonical column names:
--        subject_type (moderation_target_type_enum)
--        subject_id   (uuid)
--        reason_code  (report_reason_code_enum — locked taxonomy)
--        reason_note  (text, optional)
--        evidence_snapshot (jsonb, optional minimal immutable snapshot)
--   2. Add report_reason_code_enum (taxonomy locked by Owner).
--   3. Add immutability trigger (reports are historical intake records;
--      no UPDATE path may alter reporter/subject/reason/evidence/created_at).
--   4. Add duplicate-protection unique index:
--        (reporter_id, subject_type, subject_id)
--      Race-safe final guard: same reporter + same subject => one report.
--      Different reporter + same subject => still valid (different key row).
--
-- The old table is empty (verified: dev and test DBs have 0 rows), so
-- recreation loses nothing. Labuda-from-zero: no production data.
--
-- Case correlation (case_id) remains nullable and untouched — Report →
-- Case correlation is NOT in Slice 2 scope.
-- ============================================================

-- ── Canonical report reason taxonomy (Owner-locked) ──────────────
CREATE TYPE report_reason_code_enum AS ENUM (
    'scam_or_fraud',
    'prohibited_content',
    'harassment_or_abuse',
    'impersonation',
    'misleading_information',
    'commerce_violation',
    'other'
);

-- ── Recreate reports with canonical shape ───────────────────────
DROP TABLE IF EXISTS reports;

CREATE TABLE reports (
    id                uuid DEFAULT gen_random_uuid() NOT NULL,
    reporter_id       uuid NOT NULL,
    subject_type      moderation_target_type_enum NOT NULL,
    subject_id        uuid NOT NULL,
    reason_code       report_reason_code_enum NOT NULL,
    reason_note       text,
    evidence_snapshot jsonb,
    case_id           uuid,
    created_at        timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE reports ADD CONSTRAINT reports_pkey PRIMARY KEY (id);
ALTER TABLE reports ADD CONSTRAINT reports_reporter_id_fkey
    FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE reports ADD CONSTRAINT reports_case_id_fkey
    FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE SET NULL;

-- Polymorphic subject: PostgreSQL cannot FK subject_id to five tables.
-- Application-layer target existence validation is REQUIRED (canonical spec §9).

-- ── Immutability guard ──────────────────────────────────────────
-- Report is an immutable historical intake record. No UPDATE path may
-- alter reporter_id, subject_type, subject_id, reason_code, reason_note,
-- evidence_snapshot, or created_at (canonical spec §6).
CREATE OR REPLACE FUNCTION prevent_reports_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'reports rows are immutable (historical intake records)';
END;
$$;

CREATE TRIGGER trg_reports_immutable
    BEFORE UPDATE ON reports
    FOR EACH ROW
    EXECUTE FUNCTION prevent_reports_update();

-- ── Duplicate protection (race-safe, DB-level final guard) ──────
-- Same reporter + same subject => rejected by this unique index,
-- even under concurrent inserts. Different reporter => different key row,
-- so multiple users may report the same subject (canonical spec §11).
CREATE UNIQUE INDEX uniq_reports_one_per_reporter_subject
    ON public.reports USING btree (reporter_id, subject_type, subject_id);

-- ── Indexes ─────────────────────────────────────────────────────
CREATE INDEX idx_reports_reporter ON public.reports USING btree (reporter_id, created_at DESC);
CREATE INDEX idx_reports_subject ON public.reports USING btree (subject_type, subject_id, created_at DESC);
CREATE INDEX idx_reports_case_id ON public.reports USING btree (case_id) WHERE (case_id IS NOT NULL);

-- ============================================================
-- END OF 000057
-- ============================================================
