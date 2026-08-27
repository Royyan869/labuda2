package http

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/sellerdisplay"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
)

func TestForSaleToResponseWithSellerProjection_ResolvesLegacyRawBucketMedia(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		ReadTTL: time.Minute,
	})

	for_sale := testForSale(uuid.New())
	for_sale.Product.MediaURLs = []string{
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/thumb.jpg",
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AvatarURL:          "https://labuda-uploads.s3.us-east-1.amazonaws.com/avatars/seller.jpg",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToResponseWithSellerProjection(for_sale, sellerInfo)

	media, ok := resp["media"].([]map[string]interface{})
	if !ok {
		t.Fatalf("media = %#v; want []map[string]interface{}", resp["media"])
	}
	if len(media) != 1 {
		t.Fatalf("media length = %d; want 1", len(media))
	}
	if media[0]["thumbnail_url"] != "" {
		t.Fatalf("media[0].thumbnail_url = %v; want empty", media[0]["thumbnail_url"])
	}

	mediaURLs, ok := resp["media_urls"].([]string)
	if !ok {
		t.Fatalf("media_urls = %#v; want []string", resp["media_urls"])
	}
	if len(mediaURLs) != 1 {
		t.Fatalf("media_urls length = %d; want 1", len(mediaURLs))
	}
	if !strings.Contains(mediaURLs[0], "for_sales/thumb.jpg") {
		t.Fatalf("media_urls[0] = %q; want object key path", mediaURLs[0])
	}
	if !strings.Contains(mediaURLs[0], "X-Amz-Signature=") {
		t.Fatalf("media_urls[0] = %q; want presigned GET URL", mediaURLs[0])
	}

	if got := resp["seller_avatar_url"]; got == "https://labuda-uploads.s3.us-east-1.amazonaws.com/avatars/seller.jpg" {
		t.Fatalf("seller_avatar_url = %v; want resolved read URL", got)
	}
}

func TestForSaleToResponseWithSellerProjection_PrefersProductMediaOverLegacyJSON(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		ReadTTL: time.Minute,
	})

	for_sale := testForSale(uuid.New())
	for_sale.Product.MediaURLs = []string{
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/typed-1.jpg",
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/typed-2.jpg",
	}
	for_sale.MediaURLs = []byte(`["https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/legacy.jpg"]`)
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AvatarURL:          "https://labuda-uploads.s3.us-east-1.amazonaws.com/avatars/seller.jpg",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToResponseWithSellerProjection(for_sale, sellerInfo)
	media, ok := resp["media"].([]map[string]interface{})
	if !ok {
		t.Fatalf("media = %#v; want []map[string]interface{}", resp["media"])
	}
	if len(media) != 2 {
		t.Fatalf("media length = %d; want 2", len(media))
	}
	mediaURLs, ok := resp["media_urls"].([]string)
	if !ok {
		t.Fatalf("media_urls = %#v; want []string", resp["media_urls"])
	}
	if len(mediaURLs) != 2 {
		t.Fatalf("media_urls length = %d; want 2", len(mediaURLs))
	}
	if !strings.Contains(mediaURLs[0], "typed-1.jpg") {
		t.Fatalf("media_urls[0] = %q; want product media first", mediaURLs[0])
	}
	if strings.Contains(mediaURLs[0], "legacy.jpg") {
		t.Fatalf("media_urls[0] = %q; want legacy JSON ignored when product media exists", mediaURLs[0])
	}
	if media[0]["url"] == media[1]["url"] {
		t.Fatalf("media array did not preserve distinct media entries")
	}
}
