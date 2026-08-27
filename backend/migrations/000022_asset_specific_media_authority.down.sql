ALTER TABLE seller_profiles
    DROP COLUMN IF EXISTS store_image_updated_at;

ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS cover_photo_updated_at;
