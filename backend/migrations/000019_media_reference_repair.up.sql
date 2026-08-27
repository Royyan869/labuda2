BEGIN;

-- Normalize legacy bucket URLs from any S3 region into storage keys.
-- The canonical authority is the storage key, not the raw bucket URL.
-- This preserves CDN and external URLs while repairing private S3 refs.

UPDATE seller_profiles
SET store_image_url = regexp_replace(
	store_image_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	'',
	''
)
WHERE store_image_url IS NOT NULL
  AND store_image_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE user_profiles
SET avatar_url = regexp_replace(
	avatar_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE avatar_url IS NOT NULL
  AND avatar_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE content_media
SET media_url = regexp_replace(
	media_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE media_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE order_shipping_proofs
SET media_url = regexp_replace(
	media_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE media_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE refund_evidence
SET media_url = regexp_replace(
	media_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE media_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE dispute_media
SET media_url = regexp_replace(
	media_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE media_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE external_product_media
SET url = regexp_replace(
	url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE external_product_media
SET thumbnail_url = regexp_replace(
	thumbnail_url,
	'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
	''
)
WHERE thumbnail_url IS NOT NULL
  AND thumbnail_url ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)';

UPDATE products p
SET media_urls = COALESCE((
	SELECT jsonb_agg(
		CASE
			WHEN elem ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)' THEN
				regexp_replace(
					elem,
					'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
					''
				)
			ELSE elem
		END
	)
	FROM jsonb_array_elements_text(p.media_urls) AS elem
), '[]'::jsonb)
WHERE EXISTS (
	SELECT 1
	FROM jsonb_array_elements_text(p.media_urls) AS elem
	WHERE elem ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)'
);

UPDATE refunds r
SET evidence_urls = COALESCE(ARRAY(
	SELECT CASE
		WHEN elem ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)' THEN
			regexp_replace(
				elem,
				'^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)',
				''
			)
		ELSE elem
	END
	FROM unnest(r.evidence_urls) AS elem
), ARRAY[]::text[])
WHERE EXISTS (
	SELECT 1
	FROM unnest(r.evidence_urls) AS elem
	WHERE elem ~ '^https?://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/|s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/)'
);

-- comments.share_reference removed — Universal Content canonical Comment Commerce uses fixed_price_sale_id FK

COMMIT;
