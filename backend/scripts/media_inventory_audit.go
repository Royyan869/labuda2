//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labuda/backend/internal/config"
)

type fieldSpec struct {
	Table        string
	Column       string
	Domain       string
	ValueFormat  string
	Migrated     bool
	UsesResolver bool
	Query        string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.Database.GetDSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}

	specs := []fieldSpec{
		{
			Table:        "seller_profiles",
			Column:       "store_image_url",
			Domain:       "seller profile",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: true,
			Query: `
				SELECT
					count(*) FILTER (WHERE store_image_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(store_image_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE store_image_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND store_image_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE store_image_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE store_image_url ~ '^https?://'
						AND store_image_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND store_image_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(store_image_url, '')) <> ''
						AND store_image_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM seller_profiles
			`,
		},
		{
			Table:        "user_profiles",
			Column:       "avatar_url",
			Domain:       "user profile",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: true,
			Query: `
				SELECT
					count(*) FILTER (WHERE avatar_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(avatar_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE avatar_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND avatar_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE avatar_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE avatar_url ~ '^https?://'
						AND avatar_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND avatar_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(avatar_url, '')) <> ''
						AND avatar_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM user_profiles
			`,
		},
		{
			Table:        "content_media",
			Column:       "media_url",
			Domain:       "content",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: true,
			Query: `
				SELECT
					count(*) FILTER (WHERE media_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(media_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(media_url, '')) <> ''
						AND media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM content_media
			`,
		},
		{
			Table:        "products",
			Column:       "media_urls",
			Domain:       "listing / auction product",
			ValueFormat:  "jsonb array elements",
			Migrated:     true,
			UsesResolver: true,
			Query: `
				WITH row_stats AS (
					SELECT
						count(*) FILTER (WHERE media_urls IS NULL) AS null_count,
						count(*) FILTER (
							WHERE media_urls IS NOT NULL
							AND jsonb_typeof(media_urls) = 'array'
							AND jsonb_array_length(media_urls) = 0
						) AS blank_count
					FROM products
				),
				elems AS (
					SELECT value AS media_url
					FROM products
					CROSS JOIN LATERAL jsonb_array_elements_text(media_urls) AS value
				)
				SELECT
					COALESCE(max(row_stats.null_count), 0),
					COALESCE(max(row_stats.blank_count), 0),
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM row_stats
				CROSS JOIN elems
			`,
		},
		{
			Table:        "order_shipping_proofs",
			Column:       "media_url",
			Domain:       "order shipping evidence",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				SELECT
					count(*) FILTER (WHERE media_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(media_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(media_url, '')) <> ''
						AND media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM order_shipping_proofs
			`,
		},
		{
			Table:        "refund_evidence",
			Column:       "media_url",
			Domain:       "refund evidence",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				SELECT
					count(*) FILTER (WHERE media_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(media_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(media_url, '')) <> ''
						AND media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM refund_evidence
			`,
		},
		{
			Table:        "refunds",
			Column:       "evidence_urls",
			Domain:       "refund evidence",
			ValueFormat:  "text[] array elements",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				WITH row_stats AS (
					SELECT
						count(*) FILTER (WHERE evidence_urls IS NULL) AS null_count,
						count(*) FILTER (
							WHERE evidence_urls IS NOT NULL
							AND cardinality(evidence_urls) = 0
						) AS blank_count
					FROM refunds
				),
				elems AS (
					SELECT value AS media_url
					FROM refunds
					CROSS JOIN LATERAL unnest(evidence_urls) AS value
				)
				SELECT
					COALESCE(max(row_stats.null_count), 0),
					COALESCE(max(row_stats.blank_count), 0),
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM row_stats
				CROSS JOIN elems
			`,
		},
		{
			Table:        "dispute_media",
			Column:       "media_url",
			Domain:       "dispute evidence",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				SELECT
					count(*) FILTER (WHERE media_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(media_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE media_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE media_url ~ '^https?://'
						AND media_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND media_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(media_url, '')) <> ''
						AND media_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM dispute_media
			`,
		},
		{
			Table:        "external_product_media",
			Column:       "url",
			Domain:       "external products",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				SELECT
					count(*) FILTER (WHERE url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE url ~ '^https?://'
						AND url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(url, '')) <> ''
						AND url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM external_product_media
			`,
		},
		{
			Table:        "external_product_media",
			Column:       "thumbnail_url",
			Domain:       "external products",
			ValueFormat:  "scalar text",
			Migrated:     true,
			UsesResolver: false,
			Query: `
				SELECT
					count(*) FILTER (WHERE thumbnail_url IS NULL) AS null_count,
					count(*) FILTER (WHERE btrim(COALESCE(thumbnail_url, '')) = '') AS blank_count,
					count(*) FILTER (
						WHERE thumbnail_url ~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND thumbnail_url !~ 'X-Amz-Signature='
					) AS raw_count,
					count(*) FILTER (WHERE thumbnail_url ~ 'X-Amz-Signature=') AS signed_count,
					count(*) FILTER (
						WHERE thumbnail_url ~ '^https?://'
						AND thumbnail_url !~ '^https?://(labuda-uploads\.s3\.[^/]+\.amazonaws\.com/|labuda-uploads\.s3\.amazonaws\.com/|s3\.[^/]+\.amazonaws\.com/labuda-uploads/|s3\.amazonaws\.com/labuda-uploads/)'
						AND thumbnail_url !~ 'X-Amz-Signature='
					) AS external_count,
					count(*) FILTER (
						WHERE btrim(COALESCE(thumbnail_url, '')) <> ''
						AND thumbnail_url !~ '^https?://'
					) AS key_count,
					count(*) AS total_count
				FROM external_product_media

	fmt.Println("MEDIA INVENTORY AUDIT")
	for _, spec := range specs {
		var nullCount, blankCount, rawCount, signedCount, externalCount, keyCount, totalCount int64
		if err := pool.QueryRow(ctx, spec.Query).Scan(&nullCount, &blankCount, &rawCount, &signedCount, &externalCount, &keyCount, &totalCount); err != nil {
			log.Fatalf("%s.%s: %v", spec.Table, spec.Column, err)
		}
		fmt.Printf(
			"%s.%s | domain=%s | format=%s | raw=%d | key=%d | signed=%d | external=%d | null=%d | blank=%d | total=%d | migrated=%t | resolver=%t\n",
			spec.Table, spec.Column, spec.Domain, spec.ValueFormat, rawCount, keyCount, signedCount, externalCount, nullCount, blankCount, totalCount, spec.Migrated, spec.UsesResolver,
		)
	}

	var applied int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE version = 19`).Scan(&applied); err == nil {
		fmt.Printf("schema_migrations.version=19 count=%d\n", applied)
	}

	var contentID, listingID, auctionID, sellerID string
	_ = strings.Join([]string{contentID, listingID, auctionID, sellerID}, "")
}
