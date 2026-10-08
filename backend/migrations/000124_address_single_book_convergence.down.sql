-- ============================================================
-- 000124_address_single_book_convergence.down.sql
--
-- Rollback restores the pre-convergence shape (single-purpose column).
-- The tag set model is NOT restored: it is a rejected design. This
-- rollback is a rollback of the convergence, not a preservation of it.
-- ============================================================

ALTER TABLE addresses ADD COLUMN purpose text;
UPDATE addresses SET purpose = 'shipping' WHERE purpose IS NULL;
ALTER TABLE addresses ALTER COLUMN purpose SET NOT NULL;
ALTER TABLE addresses ADD CONSTRAINT addresses_purpose_check
    CHECK (purpose = ANY (ARRAY['shipping'::text, 'sender'::text]));
CREATE INDEX idx_addresses_purpose ON public.addresses USING btree (purpose);

ALTER TABLE products ADD COLUMN farm_address_id uuid;
ALTER TABLE products ADD CONSTRAINT products_farm_address_id_fkey
    FOREIGN KEY (farm_address_id) REFERENCES addresses(id) ON DELETE SET NULL;
