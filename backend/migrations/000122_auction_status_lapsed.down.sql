-- PostgreSQL cannot drop a value from an enum type. The 'lapsed' value is
-- inert once the up-migration is reverted: no code path reads it, and
-- 000001 remains the authority for a clean from-zero bootstrap.
SELECT 1;
