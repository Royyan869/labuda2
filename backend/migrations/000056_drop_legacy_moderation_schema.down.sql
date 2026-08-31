-- ============================================================
-- 000056_drop_legacy_moderation_schema.down.sql
-- Reverse of 000056: restore the rejected legacy moderation schema.
--
-- WARNING: This restores the rejected GovernanceCase architecture.
-- It exists only so the migration chain is reversible for tooling;
-- the canonical direction treats moderation_cases as dead.
-- ============================================================

CREATE TYPE moderation_resource_enum AS ENUM (
    'content',
    'comment',
    'for_sale',
    'auction',
    'user',
    'chat_message'
);

CREATE TYPE moderation_status_enum AS ENUM (
    'pending',
    'approved',
    'rejected',
    'removed',
    'enforced'
);

CREATE TABLE moderation_cases (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    resource_type moderation_resource_enum NOT NULL,
    resource_id uuid NOT NULL,
    status moderation_status_enum DEFAULT 'pending'::moderation_status_enum NOT NULL,
    reported_by uuid NOT NULL,
    reviewed_by uuid,
    reason text NOT NULL,
    decision_note text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);

ALTER TABLE moderation_cases ADD CONSTRAINT moderation_cases_pkey PRIMARY KEY (id);

CREATE UNIQUE INDEX idx_moderation_cases_one_report_per_user ON public.moderation_cases USING btree (reported_by, resource_type, resource_id);
CREATE INDEX idx_moderation_cases_reported_by ON public.moderation_cases USING btree (reported_by);
CREATE INDEX idx_moderation_cases_resource ON public.moderation_cases USING btree (resource_type, resource_id);
CREATE INDEX idx_moderation_pending ON public.moderation_cases USING btree (status, created_at) WHERE (status = 'pending'::moderation_status_enum);
CREATE INDEX idx_moderation_reporter ON public.moderation_cases USING btree (reported_by, created_at DESC);
CREATE INDEX idx_moderation_resource ON public.moderation_cases USING btree (resource_type, resource_id, created_at DESC);
CREATE INDEX idx_moderation_reviewer ON public.moderation_cases USING btree (reviewed_by) WHERE (reviewed_by IS NOT NULL);
