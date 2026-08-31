-- ============================================================
-- 000055_canonical_moderation_foundation.up.sql
-- SLICE 1 — CANONICAL MODERATION SCHEMA FOUNDATION
--
-- Establishes the canonical moderation entity set:
--   reports → cases → decisions → enforcements
--   warnings  (Decision provenance)
--   appeals   (Decision provenance)
--
-- Canonical target types: content, comment, for_sale, auction, user.
-- chat_message is intentionally NOT part of the canonical scope.
--
-- These tables REPLACE the rejected GovernanceCase super-entity
-- (moderation_cases). Drop of the legacy schema happens in
-- migration 000056 after this foundation is established.
--
-- Per Audit 4 (docs/audits/moderation/REPORT_CASE_AUDIT_4_SCHEMA_FOUNDATION.md):
--   - partial unique index enforces "one active Case per subject"
--   - Decision is append-only / immutable (no UPDATE path)
--   - Enforcement is a durable execution record, NOT an outbox event
--   - Warning/Appeal gain Decision provenance
-- ============================================================

-- ── Canonical moderation enums ─────────────────────────────────

-- Canonical moderation target. chat_message is EXCLUDED (out of v1 scope).
CREATE TYPE moderation_target_type_enum AS ENUM (
    'content',
    'comment',
    'for_sale',
    'auction',
    'user'
);

-- Canonical Case lifecycle: open → resolved.
-- Decision and Enforcement are NEVER represented as Case status.
CREATE TYPE case_status_enum AS ENUM (
    'open',
    'resolved'
);

-- Canonical Decision outcome (Specification §5).
CREATE TYPE decision_outcome_enum AS ENUM (
    'no_violation',
    'violation'
);

-- Canonical Enforcement lifecycle (pending → processing → succeeded/failed).
CREATE TYPE enforcement_status_enum AS ENUM (
    'pending',
    'processing',
    'succeeded',
    'failed'
);

-- ── reports ───────────────────────────────────────────────────

CREATE TABLE reports (
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    reporter_id uuid NOT NULL,
    target_type moderation_target_type_enum NOT NULL,
    target_id   uuid NOT NULL,
    reason      text NOT NULL,
    created_at  timestamp with time zone DEFAULT now() NOT NULL
);

-- Polymorphic target: PostgreSQL cannot FK to five different tables.
-- Application-layer validation is REQUIRED (same pattern as legacy
-- ResourceExists). The CHECK guarantees the target_type is canonical.
ALTER TABLE reports ADD CONSTRAINT reports_pkey PRIMARY KEY (id);
ALTER TABLE reports ADD CONSTRAINT reports_reporter_id_fkey FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE reports ADD CONSTRAINT reports_reason_not_blank CHECK (length(btrim(reason)) > 0);

CREATE INDEX idx_reports_reporter ON public.reports USING btree (reporter_id, created_at DESC);
CREATE INDEX idx_reports_target ON public.reports USING btree (target_type, target_id, created_at DESC);

-- ── cases ─────────────────────────────────────────────────────

CREATE TABLE cases (
    id          uuid DEFAULT gen_random_uuid() NOT NULL,
    subject_type moderation_target_type_enum NOT NULL,
    subject_id  uuid NOT NULL,
    status      case_status_enum DEFAULT 'open'::case_status_enum NOT NULL,
    created_at  timestamp with time zone DEFAULT now() NOT NULL,
    closed_at   timestamp with time zone,
    updated_at  timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE cases ADD CONSTRAINT cases_pkey PRIMARY KEY (id);
ALTER TABLE cases ADD CONSTRAINT cases_subject_id_not_null CHECK (subject_id IS NOT NULL);

-- ONE ACTIVE CASE PER SUBJECT.
-- DB-enforced invariant (not application-only). A second 'open' Case for
-- the same subject is rejected by this partial unique index.
CREATE UNIQUE INDEX uniq_active_case_per_subject
    ON public.cases USING btree (subject_type, subject_id)
    WHERE (status = 'open'::case_status_enum);

CREATE INDEX idx_cases_subject ON public.cases USING btree (subject_type, subject_id, created_at DESC);
CREATE INDEX idx_cases_status ON public.cases USING btree (status, created_at) WHERE (status = 'open'::case_status_enum);

-- Case → Report relationship. reports.case_id is nullable: a Report may be
-- created before correlation to a Case (canonical Report → Case correlation
-- happens on intake; one Case aggregates many Reports).
ALTER TABLE reports ADD COLUMN case_id uuid;
ALTER TABLE reports ADD CONSTRAINT reports_case_id_fkey FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE SET NULL;
CREATE INDEX idx_reports_case_id ON public.reports USING btree (case_id) WHERE (case_id IS NOT NULL);

-- ── decisions ─────────────────────────────────────────────────

CREATE TABLE decisions (
    id            uuid DEFAULT gen_random_uuid() NOT NULL,
    case_id       uuid NOT NULL,
    decided_by    uuid NOT NULL,
    outcome       decision_outcome_enum NOT NULL,
    decision_note text,
    created_at    timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE decisions ADD CONSTRAINT decisions_pkey PRIMARY KEY (id);
ALTER TABLE decisions ADD CONSTRAINT decisions_case_id_fkey FOREIGN KEY (case_id) REFERENCES cases(id) ON DELETE CASCADE;
ALTER TABLE decisions ADD CONSTRAINT decisions_decided_by_fkey FOREIGN KEY (decided_by) REFERENCES users(id);

CREATE INDEX idx_decisions_case ON public.decisions USING btree (case_id, created_at DESC);

-- IMMUTABLE DECISION GUARD.
-- Decision is a historical, append-only record. UPDATE is rejected at the
-- database level; a new Decision is created instead (e.g. Appeal produces
-- Decision #2, never mutating Decision #1).
CREATE OR REPLACE FUNCTION prevent_decisions_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'decisions rows are immutable (append-only governance history)';
END;
$$;

CREATE TRIGGER trg_decisions_immutable
    BEFORE UPDATE ON decisions
    FOR EACH ROW
    EXECUTE FUNCTION prevent_decisions_update();

-- ── enforcements ──────────────────────────────────────────────

CREATE TABLE enforcements (
    id             uuid DEFAULT gen_random_uuid() NOT NULL,
    decision_id    uuid NOT NULL,
    target_type    moderation_target_type_enum NOT NULL,
    target_id      uuid NOT NULL,
    status         enforcement_status_enum DEFAULT 'pending'::enforcement_status_enum NOT NULL,
    attempt_count  integer DEFAULT 0 NOT NULL,
    requested_at   timestamp with time zone DEFAULT now() NOT NULL,
    started_at     timestamp with time zone,
    finished_at    timestamp with time zone,
    last_error     text,
    next_attempt_at timestamp with time zone,
    created_at     timestamp with time zone DEFAULT now() NOT NULL,
    updated_at     timestamp with time zone DEFAULT now() NOT NULL
);

ALTER TABLE enforcements ADD CONSTRAINT enforcements_pkey PRIMARY KEY (id);
ALTER TABLE enforcements ADD CONSTRAINT enforcements_decision_id_fkey FOREIGN KEY (decision_id) REFERENCES decisions(id) ON DELETE CASCADE;
ALTER TABLE enforcements ADD CONSTRAINT enforcements_attempt_count_nonneg CHECK (attempt_count >= 0);

CREATE INDEX idx_enforcements_decision ON public.enforcements USING btree (decision_id);
CREATE INDEX idx_enforcements_status ON public.enforcements USING btree (status, next_attempt_at) WHERE (status IN ('pending'::enforcement_status_enum, 'processing'::enforcement_status_enum));
CREATE INDEX idx_enforcements_target ON public.enforcements USING btree (target_type, target_id);

-- Idempotent execution/retry identity: one Enforcement per (Decision, target, action)
-- is naturally unique by PK; the unique (decision_id, target_type, target_id)
-- prevents duplicate Enforcement rows for the same consequence.
ALTER TABLE enforcements ADD CONSTRAINT enforcements_decision_target_unique UNIQUE (decision_id, target_type, target_id);

-- ── user_warnings: Decision provenance ────────────────────────

-- Canonical: Warning MUST originate from a Decision (Decision → Warning).
-- The standalone admin warning path is removed in a later runtime slice;
-- this schema makes Decision provenance structurally mandatory.
ALTER TABLE user_warnings ADD COLUMN decision_id uuid;
ALTER TABLE user_warnings ADD CONSTRAINT user_warnings_decision_id_fkey FOREIGN KEY (decision_id) REFERENCES decisions(id);
ALTER TABLE user_warnings ADD CONSTRAINT user_warnings_decision_id_required CHECK (decision_id IS NOT NULL);
ALTER TABLE user_warnings ADD CONSTRAINT user_warnings_decision_unique UNIQUE (decision_id, user_id);

-- ── appeals: Decision provenance ──────────────────────────────

-- Canonical: Appeal → Decision (NOT Report, NOT Case).
-- The legacy column appeals.report_id (which actually stored a CaseID) is
-- dropped; a new decision_id FK replaces it.
ALTER TABLE appeals DROP COLUMN IF EXISTS report_id;
ALTER TABLE appeals ADD COLUMN decision_id uuid;
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_fkey FOREIGN KEY (decision_id) REFERENCES decisions(id);
ALTER TABLE appeals ADD CONSTRAINT appeals_decision_id_required CHECK (decision_id IS NOT NULL);
ALTER TABLE appeals ADD CONSTRAINT appeals_status_check CHECK (status IN ('pending', 'approved', 'rejected'));

CREATE INDEX idx_appeals_decision_id ON public.appeals USING btree (decision_id);

-- ============================================================
-- END OF 000055
-- ============================================================
