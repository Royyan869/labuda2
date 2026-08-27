package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

func TestFeedItemToResponseCanonicalWithProjection_PrefersCanonicalProjection(t *testing.T) {
	item := &feedentity.FeedItem{
		ID:        uuid.New(),
		AuthorID:  uuid.New(),
		Status:    "active",
		Body:      "body",
		Caption:   strPtr("caption"),
		CreatedAt: time.Date(2026, time.August, 10, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.August, 10, 10, 0, 0, 0, time.UTC),
	}

	projection, err := contentApp.NewLiveContentResourceProjection(
		contententity.ContentResourceOccurrenceResourceTypeProfile,
		uuid.New(),
		contentApp.ProfileLivePayload{
			Username:  "alice",
			Lifecycle: "active",
		},
	)
	if err != nil {
		t.Fatalf("NewLiveContentResourceProjection: %v", err)
	}

	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, &projection)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}
	if _, ok := resp["resource_projection"]; !ok {
		t.Fatal("resource_projection missing from feed response")
	}
	if _, ok := resp["share_reference"]; ok {
		t.Fatal("share_reference must be omitted when canonical resource_projection is present")
	}
}

func strPtr(s string) *string { return &s }
