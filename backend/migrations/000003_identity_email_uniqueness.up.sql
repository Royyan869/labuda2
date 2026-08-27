-- PASS 3A identity hardening:
-- Enforce a single canonical normalized email per active identity row.

DO $$
DECLARE
	duplicate_count integer;
BEGIN
	SELECT COUNT(*) INTO duplicate_count
	FROM (
		SELECT LOWER(BTRIM(email)) AS normalized_email
		FROM users
		WHERE email IS NOT NULL AND BTRIM(email) <> ''
		GROUP BY LOWER(BTRIM(email))
		HAVING COUNT(*) > 1
	) dup;

	IF duplicate_count > 0 THEN
		RAISE EXCEPTION
			'Cannot apply 000003_identity_email_uniqueness: duplicate normalized email rows already exist';
	END IF;
END
$$;

CREATE UNIQUE INDEX users_email_normalized_key
	ON public.users USING btree (LOWER(BTRIM(email)))
	WHERE (email IS NOT NULL AND BTRIM(email) <> ''::text);
