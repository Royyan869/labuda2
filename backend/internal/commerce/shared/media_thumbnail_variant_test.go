package shared

import (
	"strings"
	"testing"
	"time"

	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
	"github.com/hishumi/backend/internal/platform/mediaresolve"
	"github.com/hishumi/backend/internal/platform/s3presign"
)

func ptrStr(s string) *string { return &s }

func ptrInt(v int) *int { return &v }

func TestMediaWireItems_EmitsPersistedVideoMetadata(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.test",
		ReadTTL:    time.Minute,
	})

	media := []productEntity.ProductMedia{
		{
			URL:          "videos/clip.mp4",
			Blurhash:     ptrStr("LKO2?U%2Tw=w]~RBVZRi};RPxuwH"),
			ThumbnailURL: ptrStr("images/clip-thumb.jpg"),
			Width:        ptrInt(1920),
			Height:       ptrInt(1080),
			DurationMs:   ptrInt(12500),
		},
	}
	got := MediaWireItems(media, time.Now().UTC())
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	item := got[0]
	if item["type"] != "video" {
		t.Fatalf("type = %v, want video", item["type"])
	}
	dur, _ := item["duration"].(*int)
	if dur == nil || *dur != 12500 {
		t.Fatalf("duration = %#v, want 12500ms persisted", item["duration"])
	}
	w, _ := item["width"].(*int)
	if w == nil || *w != 1920 {
		t.Fatalf("width = %#v, want 1920 persisted", item["width"])
	}
	// Explicit thumbnail wins over the derived _poster.jpg fallback.
	if thumb, _ := item["thumbnail_url"].(string); !strings.HasSuffix(thumb, "images/clip-thumb.jpg") {
		t.Fatalf("thumbnail_url = %q, want explicit thumbnail", thumb)
	}
	if item["blurhash"] != "LKO2?U%2Tw=w]~RBVZRi};RPxuwH" {
		t.Fatalf("blurhash lost: %#v", item["blurhash"])
	}
}

func TestMediaWireItems_VideoWithoutMetadata_FallsBackToPoster(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.test",
		ReadTTL:    time.Minute,
	})

	got := MediaWireItems(
		[]productEntity.ProductMedia{{URL: "videos/bare.mp4"}},
		time.Now().UTC(),
	)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if thumb, _ := got[0]["thumbnail_url"].(string); !strings.HasSuffix(thumb, "videos/bare_poster.jpg") {
		t.Fatalf("thumbnail_url = %q, want derived poster fallback", thumb)
	}
	if d := got[0]["duration"]; d != nil {
		if dur, ok := d.(*int); !ok || dur != nil {
			t.Fatalf("duration = %#v, want null when unpersisted", d)
		}
	}
}

func TestVariantKey_MatchesLambdaRule(t *testing.T) {
	cases := []struct {
		name    string
		variant string
		in      string
		want    string
	}{
		{"medium storage key", "medium", "images/abc.jpg", "images/medium/abc.jpg"},
		{"medium nested key", "medium", "images/stores/uid.jpg", "images/stores/medium/uid.jpg"},
		{"thumbnail storage key", "thumbnail", "images/abc.jpg", "images/thumbnail/abc.jpg"},
		{"own bucket URL", "medium", "https://labuda-uploads.s3.us-east-1.amazonaws.com/images/abc.jpg", "images/medium/abc.jpg"},
		{"cdn URL", "medium", "https://cdn.example.test/images/abc.jpg", "images/medium/abc.jpg"},
		{"idempotent on variant", "medium", "images/medium/abc.jpg", "images/medium/abc.jpg"},
		{"idempotent on webp", "medium", "images/webp/abc.webp", "images/webp/abc.webp"},
		{"empty", "medium", "", ""},
		{"blank", "medium", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VariantKey(tc.in, tc.variant); got != tc.want {
				t.Fatalf("VariantKey(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFirstImageRef_SkipsVideo(t *testing.T) {
	if got := FirstImageRef([]string{
		"videos/clip.mp4",
		"images/b.jpg",
	}); got != "images/b.jpg" {
		t.Fatalf("got %q; want first image", got)
	}
	if got := FirstImageRef([]string{"videos/clip.mp4"}); got != "" {
		t.Fatalf("got %q; want empty for video-only", got)
	}
	if got := FirstImageRef(nil); got != "" {
		t.Fatalf("got %q; want empty for nil", got)
	}
}

func TestVideoPosterKey_MatchesRemuxRule(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"mp4 key", "videos/clip.mp4", "videos/clip_poster.jpg"},
		{"nested mp4", "videos/commerce/x.mp4", "videos/commerce/x_poster.jpg"},
		{"own bucket URL", "https://labuda-uploads.s3.us-east-1.amazonaws.com/videos/clip.mp4", "videos/clip_poster.jpg"},
		{"idempotent", "videos/clip_poster.jpg", "videos/clip_poster.jpg"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := VideoPosterKey(tc.in); got != tc.want {
				t.Fatalf("VideoPosterKey(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveReadableCardThumbnailURL_PrefersImageElsePoster(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.test",
		ReadTTL:    time.Minute,
	})

	if got := ResolveReadableCardThumbnailURL([]productEntity.ProductMedia{
		{URL: "videos/clip.mp4"},
		{URL: "images/b.jpg"},
	}); got != "https://cdn.example.test/images/medium/b.jpg" {
		t.Fatalf("got %q; want image variant", got)
	}
	if got := ResolveReadableCardThumbnailURL([]productEntity.ProductMedia{{URL: "videos/clip.mp4"}}); got != "https://cdn.example.test/videos/clip_poster.jpg" {
		t.Fatalf("got %q; want poster fallback", got)
	}
	if got := ResolveReadableCardThumbnailURL(nil); got != "" {
		t.Fatalf("got %q; want empty", got)
	}
	if got := ResolveReadableCardThumbnailURL([]productEntity.ProductMedia{{URL: "videos/c.mp4", Blurhash: ptrStr("L00000")}}); !strings.HasSuffix(got, "videos/c_poster.jpg") {
		t.Fatalf("got %q; want poster with hash-carrying video", got)
	}
}

func TestResolveReadableThumbnailURL_ServesListVariant(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.test",
		ReadTTL:    time.Minute,
	})

	got := ResolveReadableThumbnailURL("images/abc.jpg")
	want := "https://cdn.example.test/images/medium/abc.jpg"
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
	if strings.Contains(got, "X-Amz-Signature") {
		t.Fatalf("list variant URL must be stable CDN, got presigned %q", got)
	}
}
