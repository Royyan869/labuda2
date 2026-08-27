package mediaresolve

import (
	"strings"
	"testing"
	"time"

	"github.com/labuda/backend/internal/platform/s3presign"
)

func TestResolveMediaReadURLWithConfig_PresignedGetFromStorageKey(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		ReadTTL: time.Minute,
	}

	got, err := ResolveMediaReadURLWithConfig("images/sample.jpg", cfg)
	if err != nil {
		t.Fatalf("ResolveMediaReadURLWithConfig returned error: %v", err)
	}
	if !strings.Contains(got, "images/sample.jpg") {
		t.Fatalf("resolved URL = %q; want storage key path", got)
	}
	if !strings.Contains(got, "X-Amz-Signature=") {
		t.Fatalf("resolved URL = %q; want presigned GET signature", got)
	}
}

func TestNormalizeStorageReferenceWithConfig_LegacyBucketURLToKey(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region: "us-east-1",
			Bucket: "labuda-uploads",
		},
	}

	got, err := NormalizeStorageReferenceWithConfig(
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/images/sample.jpg",
		cfg,
	)
	if err != nil {
		t.Fatalf("NormalizeStorageReferenceWithConfig returned error: %v", err)
	}
	want := "images/sample.jpg"
	if got != want {
		t.Fatalf("normalized reference = %q; want %q", got, want)
	}
}

func TestNormalizeStorageReferenceWithConfig_WrongRegionBucketURLToKey(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region: "us-east-1",
			Bucket: "labuda-uploads",
		},
	}

	cases := []struct {
		name string
		ref  string
	}{
		{
			name: "virtual hosted style",
			ref:  "https://labuda-uploads.s3.eu-west-1.amazonaws.com/images/sample.jpg",
		},
		{
			name: "path style",
			ref:  "https://s3.eu-west-1.amazonaws.com/labuda-uploads/images/sample.jpg",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeStorageReferenceWithConfig(tc.ref, cfg)
			if err != nil {
				t.Fatalf("NormalizeStorageReferenceWithConfig returned error: %v", err)
			}
			want := "images/sample.jpg"
			if got != want {
				t.Fatalf("normalized reference = %q; want %q", got, want)
			}
		})
	}
}

func TestNormalizeStorageReferenceWithConfig_PresignedBucketURLToKey(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region: "us-east-1",
			Bucket: "labuda-uploads",
		},
	}

	got, err := NormalizeStorageReferenceWithConfig(
		"https://labuda-uploads.s3.us-east-1.amazonaws.com/images/stores/62d7e998-f5d8-4486-be84-63d81f9c0e6f.jpg?X-Amz-Signature=redacted&X-Amz-Credential=redacted",
		cfg,
	)
	if err != nil {
		t.Fatalf("NormalizeStorageReferenceWithConfig returned error: %v", err)
	}
	want := "images/stores/62d7e998-f5d8-4486-be84-63d81f9c0e6f.jpg"
	if got != want {
		t.Fatalf("normalized reference = %q; want %q", got, want)
	}
}

func TestResolveMediaReadURLWithConfig_WrongRegionBucketURLBecomesReadURL(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		ReadTTL: time.Minute,
	}

	got, err := ResolveMediaReadURLWithConfig(
		"https://labuda-uploads.s3.eu-west-1.amazonaws.com/images/sample.jpg",
		cfg,
	)
	if err != nil {
		t.Fatalf("ResolveMediaReadURLWithConfig returned error: %v", err)
	}
	if !strings.Contains(got, "images/sample.jpg") {
		t.Fatalf("resolved URL = %q; want storage key path", got)
	}
	if !strings.Contains(got, "X-Amz-Signature=") {
		t.Fatalf("resolved URL = %q; want presigned GET signature", got)
	}
}

func TestResolveMediaReadURLWithConfig_CDNBaseWins(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		CDNBaseURL: "https://cdn.example.com/media",
		ReadTTL:    time.Minute,
	}

	got, err := ResolveMediaReadURLWithConfig("images/sample.jpg", cfg)
	if err != nil {
		t.Fatalf("ResolveMediaReadURLWithConfig returned error: %v", err)
	}
	want := "https://cdn.example.com/media/images/sample.jpg"
	if got != want {
		t.Fatalf("resolved CDN URL = %q; want %q", got, want)
	}
}

func TestResolveMediaReadURLWithConfig_InvalidReferenceRejected(t *testing.T) {
	cfg := Config{
		PresignCfg: s3presign.Config{
			Region: "us-east-1",
			Bucket: "labuda-uploads",
		},
	}

	if _, err := ResolveMediaReadURLWithConfig("not a reference", cfg); err == nil {
		t.Fatal("expected error for malformed reference, got nil")
	}
}
