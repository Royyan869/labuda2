ALTER TABLE auction_media
	DROP CONSTRAINT IF EXISTS auction_media_duration_nonnegative_chk,
	DROP CONSTRAINT IF EXISTS auction_media_dimensions_positive_chk,
	DROP CONSTRAINT IF EXISTS auction_media_thumbnail_url_not_blank_chk,
	DROP COLUMN IF EXISTS duration,
	DROP COLUMN IF EXISTS height,
	DROP COLUMN IF EXISTS width,
	DROP COLUMN IF EXISTS thumbnail_url;

ALTER TABLE fixed_price_sale_media
	DROP CONSTRAINT IF EXISTS fixed_price_sale_media_duration_nonnegative_chk,
	DROP CONSTRAINT IF EXISTS fixed_price_sale_media_dimensions_positive_chk,
	DROP CONSTRAINT IF EXISTS fixed_price_sale_media_thumbnail_url_not_blank_chk,
	DROP COLUMN IF EXISTS duration,
	DROP COLUMN IF EXISTS height,
	DROP COLUMN IF EXISTS width,
	DROP COLUMN IF EXISTS thumbnail_url;
