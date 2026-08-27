ALTER TABLE user_profiles
    ADD COLUMN IF NOT EXISTS cover_photo_updated_at timestamp with time zone;

ALTER TABLE seller_profiles
    ADD COLUMN IF NOT EXISTS store_image_updated_at timestamp with time zone;

UPDATE user_profiles
SET cover_photo_updated_at = COALESCE(cover_photo_updated_at, updated_at)
WHERE cover_photo_url IS NOT NULL AND cover_photo_updated_at IS NULL;

UPDATE seller_profiles
SET store_image_updated_at = COALESCE(store_image_updated_at, updated_at)
WHERE store_image_url IS NOT NULL AND store_image_updated_at IS NULL;
