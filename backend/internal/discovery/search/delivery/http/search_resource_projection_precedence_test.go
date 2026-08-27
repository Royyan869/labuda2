package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	contentApp "github.com/labuda/backend/internal/social/content/application"
	contententity "github.com/labuda/backend/internal/social/content/entity"
)

func TestContentPreviewsToResponseWithProjections_PrefersCanonicalProjection(t *testing.T) {
	preview := &entity.ContentPreview{
		ID:             uuid.New(),
		AuthorID:       uuid.New(),
		Caption:        "caption",
		MediaURLs:      []string{},
		CreatedAt:      time.Date(2026, time.August, 10, 10, 0, 0, 0, time.UTC),
		AuthorUsername: "alice",
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

	items := contentPreviewsToResponseWithProjections(
		[]*entity.ContentPreview{preview},
		nil,
		nil,
		map[uuid.UUID]*contentApp.ContentResourceProjection{preview.ID: &projection},
	)
	if len(items) != 1 {
		t.Fatalf("expected 1 item; got %d", len(items))
	}
	item := items[0]
	if _, ok := item["resource_projection"]; !ok {
		t.Fatal("resource_projection missing from search response")
	}
	for _, key := range []string{"share_reference", "for_sale", "auction", "profile"} {
		if _, ok := item[key]; ok {
			t.Fatalf("%q must be omitted when canonical resource_projection is present", key)
		}
	}
}
