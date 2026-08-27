# Admin Bootstrap Guide

**Status: STABLE**
Last reviewed: 2026-06-22

How to create the first admin user in a fresh Labuda environment, and how to manage admin capabilities afterward.

---

## Concepts

Labuda uses a capability-based permission system (`user_capabilities` table). Admin access requires two things:

1. `role = 'admin'` in the `users` table — controls coarse admin endpoint access
2. One or more capability rows in `user_capabilities` — controls which admin pages and actions are available

The admin panel reads capabilities from `GET /api/v1/admin/me` after Firebase login.

**Active capability** = a row in `user_capabilities` where `revoked_at IS NULL`.

---

## First Admin — SQL Bootstrap Required

The first admin cannot be created through the admin panel. The panel requires the `governance.capability.assign` capability to grant capabilities via API, and no admin exists yet. Bootstrap via direct SQL.

### Step 1 — Create the Firebase user

The target person must sign in to the admin panel at `http://localhost:5173`, click **Login**, and authenticate with their Firebase account (Google or email/password). This creates a row in the `users` table via the `/auth/firebase` endpoint.

> **Do this before running any SQL.** The users row must exist first.

### Step 2 — Find the user UUID

```sql
SELECT id, email, username, role
FROM users
WHERE email = 'admin@example.com';
```

Note the `id` value. Replace `<USER_UUID>` with it in subsequent steps.

### Step 3 — Set admin role

```sql
UPDATE users
SET role = 'admin', updated_at = NOW()
WHERE id = '<USER_UUID>';
```

### Step 4 — Grant capabilities

For a super-admin with full access (owner / tech lead):

```sql
DO $$
DECLARE
  target_id UUID := '<USER_UUID>';
  caps TEXT[] := ARRAY[
    'finance.withdraw.read',
    'finance.withdraw.review',
    'finance.dispute.resolve',
    'finance.refund.gateway.initiate',
    'governance.dashboard.view',
    'governance.alert.read',
    'governance.alert.resolve',
    'governance.user.read',
    'governance.user.suspend',
    'governance.user.ban',
    'governance.user.activate',
    'governance.user.unban',
    'governance.role.assign',
    'governance.capability.assign',
    'governance.audit.read',
    'governance.bnr.reset',
    'moderation.case.read',
    'moderation.content.view',
    'moderation.content.remove',
    'moderation.case.resolve',
    'moderation.appeal.review',
    'promotion.external_product.review',
    'promotion.package.manage',
    'promotion.campaign.view',
    'promotion.campaign.stop',
    'seller.verification.review',
    'seller.subscription.recover',
    'order.read',
    'config.view',
    'config.update.general',
    'config.update.financial',
    'support.ticket.read',
    'support.ticket.respond',
    'support.ticket.claim',
    'support.ticket.resolve',
    'support.ticket.escalate',
    'support.admin.assign',
    'support.admin.read'
  ];
  cap TEXT;
BEGIN
  FOREACH cap IN ARRAY caps LOOP
    INSERT INTO user_capabilities (user_id, capability)
    SELECT target_id, cap
    WHERE NOT EXISTS (
      SELECT 1 FROM user_capabilities
      WHERE user_id = target_id
      AND capability = cap
      AND resource_id IS NULL
      AND revoked_at IS NULL
    );
  END LOOP;
END $$;
```

This script is idempotent — safe to run multiple times.

### Step 5 — Verify

Refresh the admin panel. The full sidebar should appear. To verify in SQL:

```sql
SELECT capability, granted_at
FROM user_capabilities
WHERE user_id = '<USER_UUID>'
AND revoked_at IS NULL
ORDER BY capability;
```

You should see all 38 capabilities listed.

---

## Subsequent Admins — Admin Panel

Once the first admin has `governance.capability.assign` and `governance.role.assign`, all future admins can be set up through the admin panel:

1. Target person logs in to the admin panel (creates their `users` row via Firebase)
2. Find their UUID: **Users** → search by email → open detail → copy UUID from URL
3. **Users → User Detail → Set Role → `admin`**
4. **Users → User Detail → Capabilities → Grant** each required capability

Or via the REST API (requires a valid admin token):

```bash
# Set role
PUT /api/v1/admin/users/<TARGET_UUID>/role
Content-Type: application/json
{"role": "admin"}

# Grant a capability
POST /api/v1/admin/users/<TARGET_UUID>/capabilities
Content-Type: application/json
{"capability": "governance.user.read"}
```

---

## Recommended Capability Sets

Grant only what the operator needs. These match the presets in `backend/internal/platform/capability/bootstrap_service.go`.

### Super Admin — Owner / Tech Lead
All 38 capabilities. Required for owner test.

### Finance Reviewer
```
finance.withdraw.read
finance.withdraw.review
finance.dispute.resolve
```

### Governance Basic
```
governance.audit.read
governance.user.read
governance.user.suspend
governance.role.assign
```

### Moderation Operator
```
moderation.case.read
moderation.content.view
moderation.content.remove
moderation.case.resolve
moderation.appeal.review
```

### Seller Verification Reviewer
```
seller.verification.review
governance.user.read
```

### Config Manager
```
config.view
config.update.general
config.update.financial
```

### Support Admin
```
support.ticket.read
support.ticket.respond
support.ticket.claim
support.ticket.resolve
support.ticket.escalate
support.admin.assign
support.admin.read
```

---

## Revoking a Capability

Revocation sets `revoked_at` — rows are preserved for the audit trail. Do not delete rows.

Via admin panel: **Users → User Detail → Capabilities → Revoke**.

Via API:
```bash
DELETE /api/v1/admin/users/<TARGET_UUID>/capabilities/<capability>
# Example:
DELETE /api/v1/admin/users/<UUID>/capabilities/config.update.financial
```

Via SQL (emergency only):
```sql
UPDATE user_capabilities
SET revoked_at = NOW()
WHERE user_id = '<USER_UUID>'
AND capability = '<capability>'
AND revoked_at IS NULL;
```

---

## Warnings

- **One Firebase account per person.** Do not share admin credentials.
- **`governance.capability.assign` is the highest-privilege capability.** It allows granting any other capability. Only the owner / tech lead should hold it.
- **`config.update.financial` has no approval gate.** Changes to commission rates and withdrawal limits take effect immediately. Restrict this to owner / tech lead only.
- **Never delete rows from `user_capabilities`.** Revoke instead — the audit trail depends on row preservation.
- **Self-escalation is blocked.** An admin cannot grant admin role to themselves via the API. Bootstrap must be done via SQL.
- **`DEV_MOCK_FIREBASE_AUTH=true` bypasses Firebase token validation.** Never enable this in staging or production.
