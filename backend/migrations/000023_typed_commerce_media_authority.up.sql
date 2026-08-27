CREATE TABLE IF NOT EXISTS fixed_price_sale_media (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    fixed_price_sale_id uuid NOT NULL REFERENCES fixed_price_sales(id) ON DELETE CASCADE,
    media_url text NOT NULL,
    media_type text NOT NULL CHECK (media_type IN ('image', 'video')),
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    UNIQUE (fixed_price_sale_id, position)
);

CREATE INDEX IF NOT EXISTS idx_fixed_price_sale_media_fixed_price_sale_id
    ON fixed_price_sale_media(fixed_price_sale_id, position);

CREATE TABLE IF NOT EXISTS auction_media (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    auction_id uuid NOT NULL REFERENCES auctions(id) ON DELETE CASCADE,
    media_url text NOT NULL,
    media_type text NOT NULL CHECK (media_type IN ('image', 'video')),
    position integer NOT NULL CHECK (position >= 0),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    UNIQUE (auction_id, position)
);

CREATE INDEX IF NOT EXISTS idx_auction_media_auction_id
    ON auction_media(auction_id, position);

WITH fixed_price_media_source AS (
    SELECT
        fps.id AS fixed_price_sale_id,
        url,
        ordinality - 1 AS position,
        fps.created_at,
        'image' AS media_type
    FROM fixed_price_sales fps
    JOIN products p ON p.id = fps.product_id
    CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(p.media_urls, '[]'::jsonb)) WITH ORDINALITY AS media(url, ordinality)
    WHERE btrim(url) <> ''
)
INSERT INTO fixed_price_sale_media (id, fixed_price_sale_id, media_url, media_type, position, created_at)
SELECT
    uuid_generate_v4(),
    fixed_price_sale_id,
    url,
    media_type,
    position,
    COALESCE(created_at, now())
FROM fixed_price_media_source
ON CONFLICT (fixed_price_sale_id, position) DO NOTHING;

WITH auction_media_source AS (
    SELECT
        a.id AS auction_id,
        url,
        ordinality - 1 AS position,
        a.created_at,
        'image' AS media_type
    FROM auctions a
    JOIN products p ON p.id = a.product_id
    CROSS JOIN LATERAL jsonb_array_elements_text(COALESCE(p.media_urls, '[]'::jsonb)) WITH ORDINALITY AS media(url, ordinality)
    WHERE btrim(url) <> ''
)
INSERT INTO auction_media (id, auction_id, media_url, media_type, position, created_at)
SELECT
    uuid_generate_v4(),
    auction_id,
    url,
    media_type,
    position,
    COALESCE(created_at, now())
FROM auction_media_source
ON CONFLICT (auction_id, position) DO NOTHING;
