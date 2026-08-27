-- ============================================================
-- 000001_canonical_schema.down.sql
-- Canonical schema revert.
--
-- WARNING: Destructive. Only run on dev/test DB.
-- Drops ALL application tables and enum types.
-- Functionally equivalent to DROP DATABASE + CREATE DATABASE,
-- but schema-scoped so the database itself survives.
--
-- Safe: CASCADE handles FK ordering; two-pass (tables then types)
-- prevents type-still-in-use errors. No half-dropped state.
-- ============================================================

-- Pass 1: Drop all tables (CASCADE resolves FK dependencies)
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'public'
    ) LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END $$;

-- Pass 2: Drop all enum types (tables gone; no type-in-use errors)
DO $$ DECLARE
    r RECORD;
BEGIN
    FOR r IN (
        SELECT typname
        FROM pg_type t
        WHERE t.typtype = 'e'
          AND t.typnamespace = (SELECT oid FROM pg_namespace WHERE nspname = 'public')
    ) LOOP
        EXECUTE 'DROP TYPE IF EXISTS ' || quote_ident(r.typname) || ' CASCADE';
    END LOOP;
END $$;
