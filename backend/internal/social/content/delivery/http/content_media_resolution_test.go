package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/social/content/entity"
)

func TestToContentResponse_PassesThroughMediaURL(t *testing.T) {
	// MEDIA RESOLUTION V2: ToContentResponse passes media URLs through
	// without resolution. Legacy S3 bucket URL resolution now happens in
	// the projection layer (content_resource_projection_resolver.go via
	// resolveMediaRefs). This test verifies the current pass-through
	// contract.

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

	// ToContentResponse passes the URL through without resolution.
	// Resolution is handled by the projection layer.
	if resp.Media[0].URL != media[0].MediaURL {
		t.Fatalf("media url = %q; want pass-through URL %q", resp.Media[0].URL, media[0].MediaURL)
	}
}
