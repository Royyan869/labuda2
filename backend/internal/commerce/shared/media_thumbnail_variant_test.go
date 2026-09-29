package shared

import (
	"strings"
	"testing"
	"time"

	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
)

func TestThumbnailVariantKey_MatchesLambdaRule(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"storage key", "images/abc.jpg", "images/thumbnail/abc.jpg"},
		{"nested key", "images/stores/uid.jpg", "images/stores/thumbnail/uid.jpg"},
		{"own bucket URL", "https://labuda-uploads.s3.us-east-1.amazonaws.com/images/abc.jpg", "images/thumbnail/abc.jpg"},
		{"cdn URL", "https://cdn.example.test/images/abc.jpg", "images/thumbnail/abc.jpg"},
		{"idempotent on variant", "images/thumbnail/abc.jpg", "images/thumbnail/abc.jpg"},
		{"idempotent on webp", "images/webp/abc.webp", "images/webp/abc.webp"},
		{"empty", "", ""},
		{"blank", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ThumbnailVariantKey(tc.in); got != tc.want {
				t.Fatalf("ThumbnailVariantKey(%q) = %q; want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveReadableThumbnailURL_ResolvesToCDN(t *testing.T) {
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
	want := "https://cdn.example.test/images/thumbnail/abc.jpg"
	if got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
	if strings.Contains(got, "X-Amz-Signature") {
		t.Fatalf("thumbnail URL must be stable CDN, got presigned %q", got)
	}
}
