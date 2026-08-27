# 🔒 DATABASE IDENTITY LOCK - EXECUTION GUIDE

## 🚨 PRECONDITION: START DATABASE

The PostgreSQL database must be running first. Choose ONE method:

### METHOD 1: Start Docker (Recommended)
```bash
cd d:/Project/labuda
docker-compose up -d postgres
```

### METHOD 2: Start Docker Desktop
1. Open Docker Desktop application
2. Wait for PostgreSQL container to start

### METHOD 3: Start PostgreSQL directly (if installed locally)
```bash
# Windows: Start PostgreSQL service
net start postgresql-x64-16
```

---

## 📋 AUTOMATED EXECUTION

Once the database is running, execute the automated Go script:

```bash
cd d:/Project/labuda/backend
go run scripts/lock_identity.go
```

This will:
1. ✅ Inspect current database state
2. ✅ Normalize emails (lowercase, trim)
3. ✅ Apply constraints (if safe)
4. ✅ Show verification results

---

## 🛠️ MANUAL EXECUTION (Alternative)

If you prefer using psql directly:

```bash
# Connect to database
cd d:/Project/labuda/backend
psql -h localhost -p 5432 -U labuda -d labuda

# Run the SQL script
\i scripts/lock_identity.sql
```

---

## 📊 EXPECTED OUTPUT

The script will output:

```
=== TASK 1: INSPECT CURRENT STATE ===

1A. DUPLICATE EMAIL CHECK:
   ✅ No duplicate emails found

1B. DUPLICATE FIREBASE_UID CHECK:
   ✅ No duplicate firebase_uid found

1C. NULL OR EMPTY EMAIL CHECK:
   ✅ No null or empty emails found

TOTAL USERS: 123

=== TASK 2: CLEAN DATA ===

2A. NORMALIZING EMAILS (lowercase, trim)...
   ✅ Normalized 5 email(s)

=== TASK 3: APPLY CONSTRAINTS ===

3A. SETTING EMAIL TO NOT NULL...
   ✅ Email column set to NOT NULL

3B. ADDING UNIQUE CONSTRAINT ON EMAIL...
   ✅ UNIQUE constraint on email added

3C. CHECKING FIREBASE_UID UNIQUE INDEX...
   ✅ UNIQUE index on firebase_uid already exists

=== FINAL OUTPUT ===

DUPLICATE EMAIL FOUND: NO ✅
DUPLICATE FIREBASE_UID FOUND: NO ✅
NULL EMAIL FOUND: NO ✅

CLEANUP DONE: YES ✅

EMAIL UNIQUE ENFORCED: YES ✅
FIREBASE_UID UNIQUE ENFORCED: YES ✅
EMAIL NOT NULL ENFORCED: YES ✅

DUPLICATE INSERT BLOCKED: YES ✅
```

---

## ⚠️ IF DATA ISSUES ARE FOUND

If the script finds duplicates or null emails, it will skip applying constraints.

### RESOLUTION OPTIONS:

#### Option 1: Delete problematic users
```sql
-- Delete users with duplicate emails (keep oldest)
DELETE FROM users
WHERE id NOT IN (
    SELECT MIN(id)
    FROM users
    GROUP BY email
    HAVING email IS NOT NULL AND email != ''
);

-- Delete users with null emails
DELETE FROM users WHERE email IS NULL OR email = '';
```

#### Option 2: Manually review and merge
```sql
-- View duplicate emails with details
SELECT
    email,
    COUNT(*) AS cnt,
    ARRAY_AGG(id ORDER BY created_at) AS user_ids,
    ARRAY_AGG(firebase_uid ORDER BY created_at) AS firebase_uids,
    ARRAY_AGG(created_at ORDER BY created_at) AS created_dates
FROM users
WHERE email IS NOT NULL AND email != ''
GROUP BY email
HAVING COUNT(*) > 1;
```

After cleanup, run the script again to apply constraints.

---

## 🔍 VERIFICATION TESTS

After successful execution, verify constraints work:

```sql
-- Test 1: Try duplicate email (should FAIL)
INSERT INTO users (id, email, firebase_uid)
VALUES (gen_random_uuid(), 'test@dup.com', 'test1');

INSERT INTO users (id, email, firebase_uid)
VALUES (gen_random_uuid(), 'test@dup.com', 'test2');
-- Expected: ERROR: duplicate key value violates unique constraint "users_email_unique"

-- Test 2: Try duplicate firebase_uid (should FAIL)
INSERT INTO users (id, email, firebase_uid)
VALUES (gen_random_uuid(), 'a@test.com', 'uid123');

INSERT INTO users (id, email, firebase_uid)
VALUES (gen_random_uuid(), 'b@test.com', 'uid123');
-- Expected: ERROR: duplicate key value violates unique constraint "users_firebase_uid_key"

-- Test 3: Try NULL email (should FAIL)
INSERT INTO users (id, firebase_uid)
VALUES (gen_random_uuid(), 'uid456');
-- Expected: ERROR: null value in column "email" violates not-null constraint
```

---

## 📝 FILES CREATED

1. **scripts/lock_identity.sql** - SQL script for manual execution
2. **scripts/lock_identity.go** - Go automated script
3. **scripts/IDENTITY_LOCK_GUIDE.md** - This guide

---

## 🎯 GOAL

After successful execution:

> ✅ **Duplicate user secara DB = IMPOSSIBLE**

The database will enforce:
- ✅ Email MUST be unique
- ✅ Firebase UID MUST be unique
- ✅ Email CANNOT be null

This ensures data integrity at the database level, regardless of application logic.

---

## 🚨 ROLLBACK (IF NEEDED)

If you need to remove constraints later:

```sql
-- Remove email unique constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_unique;

-- Remove email not null constraint
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;

-- Remove firebase_uid unique index
DROP INDEX IF EXISTS users_firebase_uid_key;
```

---

## 📞 NEXT STEPS

After database is locked:
1. Update backend code to handle constraint violations gracefully
2. Add proper error messages for duplicate errors
3. Implement retry logic with proper error handling
4. Update authentication flow to be idempotent

See backend code audit for required code fixes.
