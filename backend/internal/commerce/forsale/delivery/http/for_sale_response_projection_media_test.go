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

func TestForSaleToResponseWithSeller_TypedMedia_InferredFromURL(t *testing.T) {
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

	resp := for_saleToResponseWithSeller(for_sale, sellerInfo)

	media, ok := resp["media"].([]map[string]interface{})
	if !ok {
		t.Fatalf("media = %#v; want []map[string]interface{}", resp["media"])
	}
	if len(media) != 1 {
		t.Fatalf("media length = %d; want 1", len(media))
	}

	// Verify type is inferred as image from URL extension
	if media[0]["type"] != "image" {
		t.Fatalf("media[0].type = %v; want image", media[0]["type"])
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
}

func TestForSaleToResponseWithSeller_MixedMedia_ImageFirstVideoSecond(t *testing.T) {
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
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/photo1.jpg",
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/for_sales/video1.mp4",
	}
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
	}

	resp := for_saleToResponseWithSeller(for_sale, sellerInfo)

	media, ok := resp["media"].([]map[string]interface{})
	if !ok {
		t.Fatalf("media = %#v; want []map[string]interface{}", resp["media"])
	}
	if len(media) != 2 {
		t.Fatalf("media length = %d; want 2", len(media))
	}

	// Verify first item is image
	if media[0]["type"] != "image" {
		t.Fatalf("media[0].type = %v; want image", media[0]["type"])
	}
	// Verify second item is video
	if media[1]["type"] != "video" {
		t.Fatalf("media[1].type = %v; want video", media[1]["type"])
	}

	// Verify ordering preserved
	mediaURLs, ok := resp["media_urls"].([]string)
	if !ok {
		t.Fatalf("media_urls = %#v; want []string", resp["media_urls"])
	}
	if len(mediaURLs) != 2 {
		t.Fatalf("media_urls length = %d; want 2", len(mediaURLs))
	}
	if !strings.Contains(mediaURLs[0], "photo1.jpg") {
		t.Fatalf("media_urls[0] = %q; want photo URL", mediaURLs[0])
	}
	if !strings.Contains(mediaURLs[1], "video1.mp4") {
		t.Fatalf("media_urls[1] = %q; want video URL", mediaURLs[1])
	}
}

func TestForSaleToResponseWithSeller_PrefersProductMediaOverLegacyJSON(t *testing.T) {
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
	// Legacy MediaURLs removed — Product is authority
	sellerInfo := sellerdisplay.Info{
		Username:           "seller_user",
		FarmName:           "Acme Farm",
		AvatarURL:          "https://labuda-uploads.s3.us-east-1.amazonaws.com/avatars/seller.jpg",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	}

	resp := for_saleToResponseWithSeller(for_sale, sellerInfo)
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
