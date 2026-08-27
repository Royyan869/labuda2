ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_user_role_valid;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT chk_user_role_valid CHECK ((role = ANY (ARRAY['user'::text, 'admin'::text])));
