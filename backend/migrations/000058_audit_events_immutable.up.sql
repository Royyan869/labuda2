-- IMMUTABLE AUDIT EVENTS GUARD.
-- audit_events is an append-only governance audit trail. UPDATE and DELETE
-- are rejected at the database level; the application repository only exposes
-- INSERT. This trigger provides defense-in-depth.
CREATE OR REPLACE FUNCTION prevent_audit_events_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        RAISE EXCEPTION 'audit_events rows are immutable (append-only audit trail)';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'audit_events rows are immutable (append-only audit trail)';
    END IF;
    RETURN NULL;
END;
$$;

CREATE TRIGGER trg_audit_events_immutable
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION prevent_audit_events_mutation();
