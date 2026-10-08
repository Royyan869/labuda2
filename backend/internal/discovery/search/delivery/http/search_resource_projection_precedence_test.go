package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	commerceshared "github.com/labuda/backend/internal/commerce/shared"
	"github.com/labuda/backend/internal/discovery/search/entity"
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

	projection, err := commerceshared.NewLiveResourceProjection(
		commerceshared.ProjectionResourceTypeProfile,
		uuid.New(),
		commerceshared.ProfileLivePayload{
			Username:  "alice",
			Lifecycle: "active",
		},
		commerceshared.ProjectionViewerCapabilities{CanView: true},
	)
	if err != nil {
		t.Fatalf("NewLiveResourceProjection: %v", err)
	}

	items := contentPreviewsToResponseWithProjections(
		[]*entity.ContentPreview{preview},
		nil,
		nil,
		map[uuid.UUID]*commerceshared.ResourceProjection{preview.ID: &projection},
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
