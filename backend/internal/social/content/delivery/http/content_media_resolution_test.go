package http

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/labuda/backend/internal/social/content/entity"
)

func TestToContentResponse_ResolvesLegacyRawBucketMedia(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "us-east-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-uploads",
		},
		ReadTTL: time.Minute,
	})

	content := &entity.Content{
		ID:         uuid.New(),
		AuthorID:   uuid.New(),
		Status:     entity.StatusActive,
		Visibility: entity.VisibilityPublic,
		CreatedAt:  time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
	}
	media := []*entity.ContentMedia{{
		ID:        uuid.New(),
		MediaURL:  "https://labuda-uploads.s3.us-east-1.amazonaws.com/content/photo.jpg",
		MediaType: entity.MediaTypeImage,
		Position:  0,
		CreatedAt: time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC),
	}}

	resp := ToContentResponse(content, media)
	if len(resp.Media) != 1 {
		t.Fatalf("media length = %d; want 1", len(resp.Media))
	}
	if resp.Media[0].URL == media[0].MediaURL {
		t.Fatalf("media url = %q; want resolved read URL", resp.Media[0].URL)
	}
	if !strings.Contains(resp.Media[0].URL, "content/photo.jpg") {
		t.Fatalf("media url = %q; want object key path", resp.Media[0].URL)
	}
	if !strings.Contains(resp.Media[0].URL, "X-Amz-Signature=") {
		t.Fatalf("media url = %q; want presigned GET URL", resp.Media[0].URL)
	}
}
