-- 000018 down: Remove store_image_url column.
ALTER TABLE seller_profiles
    DROP COLUMN IF EXISTS store_image_url;
