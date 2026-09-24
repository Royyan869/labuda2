-- 000110 down: restore the pre-hardening shape (nullable, no default).
-- Data written as '' stays '' — we do not resurrect NULLs.

ALTER TABLE shipping_options ALTER COLUMN internal_purpose DROP DEFAULT;
ALTER TABLE shipping_options ALTER COLUMN internal_purpose DROP NOT NULL;
