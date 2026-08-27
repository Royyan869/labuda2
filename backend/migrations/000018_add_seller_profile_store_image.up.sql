-- 000018: Add store_image_url to seller_profiles for canonical seller identity.
-- store_image_url is the public store/brand image distinct from personal avatar.
-- Null means no image uploaded; consumers must render a placeholder, not fall back
-- to avatar_url or farm_photo_url from a competing model.
ALTER TABLE seller_profiles
    ADD COLUMN store_image_url text;
