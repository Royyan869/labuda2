-- 000086_capability_authority_integrity
--
-- Canonical admin authority integrity. Three defects are closed here:
--
-- 1. ACTIVE GRANT UNIQUENESS
--    The old constraint UNIQUE (user_id, capability, resource_id) does not
--    constrain anything while resource_id IS NULL (NULLs are distinct in a
--    unique index), so the same active grant could be inserted repeatedly.
--    resource_id is unused in the canonical model (every row is NULL), so the
--    canonical grant identity is (user_id, capability) with one ACTIVE row.
--    Revoked rows are history and may repeat freely.
--
-- 2. RESERVED SYSTEM-CALLER UUID
--    00000000-0000-0000-0000-000000000001 is the sentinel for system-initiated
--    operations. It must never be a human identity: any users row carrying it
--    would inherit internal-job treatment. A seed artifact had assigned it to
--    buyer@test.local. The sentinel row is removed and the invariant is
--    enforced for all future writes.
--
-- 3. DUPLICATE ACTIVE GRANTS
--    Purged before the unique index is created (keep the earliest grant, which
--    is the one a re-grant would have skipped).

-- ---------------------------------------------------------------------------
-- 1. Purge duplicate active grants, keeping the earliest row per identity.
-- ---------------------------------------------------------------------------
DELETE FROM user_capabilities dupe
USING user_capabilities keep
WHERE dupe.user_id = keep.user_id
  AND dupe.capability = keep.capability
  AND dupe.resource_id IS NULL
  AND keep.resource_id IS NULL
  AND dupe.revoked_at IS NULL
  AND keep.revoked_at IS NULL
  AND dupe.granted_at > keep.granted_at;

-- Deterministic tie-break when granted_at collides (the seeder writes all rows
-- with the same NOW() inside one statement).
DELETE FROM user_capabilities dupe
USING user_capabilities keep
WHERE dupe.user_id = keep.user_id
  AND dupe.capability = keep.capability
  AND dupe.resource_id IS NULL
  AND keep.resource_id IS NULL
  AND dupe.revoked_at IS NULL
  AND keep.revoked_at IS NULL
  AND dupe.id > keep.id;

-- ---------------------------------------------------------------------------
-- 2. Enforce one ACTIVE grant per (user, capability) when unscoped.
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX user_capabilities_unique_active_capability
    ON user_capabilities (user_id, capability)
    WHERE revoked_at IS NULL AND resource_id IS NULL;

-- ---------------------------------------------------------------------------
-- 3. Reserved system-caller UUID is not a human identity.
-- ---------------------------------------------------------------------------
-- Any capability grant attributed to the sentinel loses its attribution (the
-- FK is ON DELETE SET NULL); audit rows keep their own column and are handled
-- by the actor FK below.
DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

ALTER TABLE users
    ADD CONSTRAINT users_reserved_system_caller_id
    CHECK (id <> '00000000-0000-0000-0000-000000000001');
