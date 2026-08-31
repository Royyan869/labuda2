-- ============================================================
-- 000057_report_slice_canonical_alignment.down.sql
-- Reverse of 000057: restore the preliminary 000055 reports shape.
-- ============================================================

DROP TRIGGER IF EXISTS trg_reports_immutable ON reports;
DROP FUNCTION IF EXISTS prevent_reports_update();
DROP INDEX IF EXISTS uniq_reports_one_per_reporter_subject;
DROP INDEX IF EXISTS idx_reports_reporter;
DROP INDEX IF EXISTS idx_reports_subject;
DROP INDEX IF EXISTS idx_reports_case_id;

DROP TABLE IF EXISTS reports;

-- Restore the preliminary 000055 foundation shape.
CREATE TABLE reports (
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    reporter_id uuid NOT NULL,
    target_type moderation_target_type_enum NOT NULL,
    target_id   uuid NOT NULL,
    reason      text NOT NULL,
    created_at  timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE reports ADD CONSTRAINT reports_pkey PRIMARY KEY (id);
ALTER TABLE reports ADD CONSTRAINT reports_reporter_id_fkey
    FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE reports ADD CONSTRAINT reports_reason_not_blank
    CHECK (length(btrim(reason)) > 0);

CREATE INDEX idx_reports_reporter ON public.reports USING btree (reporter_id, created_at DESC);
CREATE INDEX idx_reports_target ON public.reports USING btree (target_type, target_id, created_at DESC);

ALTER TABLE reports ADD COLUMN case_id uuid;
ALTER TABLE reports ADD CONSTRAINT reports_case_id_fkey
    FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE SET NULL;
CREATE INDEX idx_reports_case_id ON public.reports USING btree (case_id) WHERE (case_id IS NOT NULL);

DROP TYPE IF EXISTS report_reason_code_enum;

-- ============================================================
-- END OF 000057 DOWN
-- ============================================================
