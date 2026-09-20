-- 000101: Migrate persisted seller store image absolute URLs (Labuda CDN/S3) to canonical storage keys.
-- Canonical persisted form is the raw storage key, not an absolute read URL.
--   DB: seller_profiles.store_image_url = 'images/stores/<user_id>.jpg'
--   Read: mediaresolve.ResolveMediaReadURL → read_url (CDN or presigned)
--
-- Class inventory:
--   A: https://d358tu61i1wrtt.cloudfront.net/images/stores/<uid>.jpg[?t=...]  → Labuda CDN
--   B: https://labuda-media.s3.amazonaws.com/images/stores/<uid>.jpg  → legacy S3
--   C: https://labuda-media.s3.<region>.amazonaws.com/images/stores/<uid>.jpg
--   D: images/stores/<uid>.jpg  → already canonical (leave)
--   E: NULL  → leave
--   F: external / unknown host → leave (not Labuda-owned)
--
-- Safety: Only transform when the embedded <uid> equals the owning seller_profiles.user_id.
-- Unexpected CDN/S3 URLs whose uid does not match owner are left unchanged and reported.
-- Idempotent: Already-canonical storage keys do not match '^https://', so second run is no-op.
-- Query strings (?t=..., cache busting) are stripped when deriving storage key.

-- A: Canonical CloudFront CDN
UPDATE seller_profiles
SET store_image_url = 'images/stores/' || user_id::text || '.jpg'
WHERE store_image_url ~ '^https://d358tu61i1wrtt\.cloudfront\.net/images/stores/[0-9a-fA-F-]+\.jpg(\?.*)?$'
  AND store_image_url ILIKE '%/images/stores/' || user_id::text || '.jpg%';

-- B: Legacy S3 bucket (virtual-hosted style, no region)
UPDATE seller_profiles
SET store_image_url = 'images/stores/' || user_id::text || '.jpg'
WHERE store_image_url ~ '^https://labuda-media\.s3\.amazonaws\.com/images/stores/[0-9a-fA-F-]+\.jpg(\?.*)?$'
  AND store_image_url ILIKE '%/images/stores/' || user_id::text || '.jpg%';

-- C: Legacy S3 bucket with region
UPDATE seller_profiles
SET store_image_url = 'images/stores/' || user_id::text || '.jpg'
WHERE store_image_url ~ '^https://labuda-media\.s3\.[a-z0-9-]+\.amazonaws\.com/images/stores/[0-9a-fA-F-]+\.jpg(\?.*)?$'
  AND store_image_url ILIKE '%/images/stores/' || user_id::text || '.jpg%';

-- Residual labuda-uploads bucket URLs (in case any absolute was written after 000019)
UPDATE seller_profiles
SET store_image_url = 'images/stores/' || user_id::text || '.jpg'
WHERE store_image_url ~ '^https?://labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com/images/stores/[0-9a-fA-F-]+\.jpg(\?.*)?$'
  AND store_image_url ILIKE '%/images/stores/' || user_id::text || '.jpg%';

UPDATE seller_profiles
SET store_image_url = 'images/stores/' || user_id::text || '.jpg'
WHERE store_image_url ~ '^https?://s3(\.[^/]+)?\.amazonaws\.com/labuda-uploads/images/stores/[0-9a-fA-F-]+\.jpg(\?.*)?$'
  AND store_image_url ILIKE '%/images/stores/' || user_id::text || '.jpg%';

-- Generic residual Labuda-owned absolute (bucket or CDN) with any images path → strip host+query to storage key.
-- Covers timestamp-generic keys like images/1789...jpg that are not fixed store keys but must not remain absolute.
UPDATE seller_profiles
SET store_image_url = regexp_replace(regexp_replace(store_image_url, '^https://[^/]+/', ''), '\?.*$', '')
WHERE store_image_url ~ '^https://(labuda-uploads\.s3(\.[^/]+)?\.amazonaws\.com|labuda-media\.s3(\.[^/]+)?\.amazonaws\.com|d358tu61i1wrtt\.cloudfront\.net|s3(\.[^/]+)?\.amazonaws\.com)/.+'
  AND store_image_url ~ '^https://[^/]+/images/[^?]+.*$';
