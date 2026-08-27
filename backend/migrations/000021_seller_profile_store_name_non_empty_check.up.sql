-- 000021: Enforce trimmed non-empty seller store names.
-- seller_profiles.store_name is the canonical seller business identity and
-- must never be empty or whitespace-only.
ALTER TABLE seller_profiles
    ADD CONSTRAINT seller_profiles_store_name_non_empty_chk
    CHECK (btrim(store_name) <> '');
