-- Add nullable typed-media metadata for commerce listings and auctions.
-- duration is stored in milliseconds.

ALTER TABLE fixed_price_sale_media
	ADD COLUMN thumbnail_url TEXT,
	ADD COLUMN width INTEGER,
	ADD COLUMN height INTEGER,
	ADD COLUMN duration INTEGER;

ALTER TABLE fixed_price_sale_media
	ADD CONSTRAINT fixed_price_sale_media_thumbnail_url_not_blank_chk
	CHECK (thumbnail_url IS NULL OR btrim(thumbnail_url) <> ''),
	ADD CONSTRAINT fixed_price_sale_media_dimensions_positive_chk
	CHECK ((width IS NULL OR width > 0) AND (height IS NULL OR height > 0)),
	ADD CONSTRAINT fixed_price_sale_media_duration_nonnegative_chk
	CHECK (duration IS NULL OR duration >= 0);

ALTER TABLE auction_media
	ADD COLUMN thumbnail_url TEXT,
	ADD COLUMN width INTEGER,
	ADD COLUMN height INTEGER,
	ADD COLUMN duration INTEGER;

ALTER TABLE auction_media
	ADD CONSTRAINT auction_media_thumbnail_url_not_blank_chk
	CHECK (thumbnail_url IS NULL OR btrim(thumbnail_url) <> ''),
	ADD CONSTRAINT auction_media_dimensions_positive_chk
	CHECK ((width IS NULL OR width > 0) AND (height IS NULL OR height > 0)),
	ADD CONSTRAINT auction_media_duration_nonnegative_chk
	CHECK (duration IS NULL OR duration >= 0);
