package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

const testFeedCDNBase = "https://d358tu61i1wrtt.cloudfront.net"

func configureFeedMediaResolver(cdnBase string) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{
		PresignCfg: s3presign.Config{
			Region:    "ap-southeast-1",
			AccessKey: "test-access-key",
			SecretKey: "test-secret-key",
			Bucket:    "labuda-media",
		},
		CDNBaseURL: cdnBase,
		ReadTTL:    time.Minute,
	})
}

func feedItemWithMedia(media ...feedentity.FeedMedia) *feedentity.FeedItem {
	return &feedentity.FeedItem{
		ID:        uuid.New(),
		AuthorID:  uuid.New(),
		Type:      "post",
		Status:    "active",
		Body:      "caption",
		CreatedAt: time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
		Media:     media,
	}
}

func feedMediaOf(url, mediaType string, position int) feedentity.FeedMedia {
	kind := mediaType
	return feedentity.FeedMedia{
		URL:      url,
		Type:     mediaType,
		Position: position,
		Kind:     &kind,
	}
}

// The mobile feed card renders feed `media[].url` directly, so the feed
// surface must project persisted references through the canonical
// mediaresolve authority.
func TestFeedItemToResponse_ResolvesMediaStorageReference(t *testing.T) {
	configureFeedMediaResolver(testFeedCDNBase)

	item := feedItemWithMedia(feedMediaOf("images/1749600000000_author.jpg", "image", 0))
	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, nil)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}

	media, ok := resp["media"].([]feedentity.FeedMedia)
	if !ok {
		t.Fatalf("media = %#v; want []feedentity.FeedMedia", resp["media"])
	}
	if len(media) != 1 {
		t.Fatalf("media length = %d; want 1", len(media))
	}
	want := testFeedCDNBase + "/images/1749600000000_author.jpg"
	if media[0].URL != want {
		t.Fatalf("media[0].url = %q; want resolved read URL %q", media[0].URL, want)
	}
}

func TestFeedItemToResponse_ResolvesRawBucketURL(t *testing.T) {
	configureFeedMediaResolver(testFeedCDNBase)

	item := feedItemWithMedia(feedMediaOf(
		"https://labuda-media.s3.ap-southeast-1.amazonaws.com/videos/clip.mp4",
		"video",
		0,
	))
	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, nil)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}

	media := resp["media"].([]feedentity.FeedMedia)
	want := testFeedCDNBase + "/videos/clip.mp4"
	if media[0].URL != want {
		t.Fatalf("media[0].url = %q; want resolved read URL %q", media[0].URL, want)
	}
	if media[0].Type != "video" {
		t.Fatalf("media[0].type = %q; want video", media[0].Type)
	}
	if media[0].Kind == nil || *media[0].Kind != "video" {
		t.Fatalf("media[0].kind = %v; want video (additive field preserved)", media[0].Kind)
	}
}

func TestFeedItemToResponse_PreservesExternalAbsoluteURL(t *testing.T) {
	configureFeedMediaResolver(testFeedCDNBase)

	external := "https://cdn.example.com/content/image-a.jpg"
	item := feedItemWithMedia(feedMediaOf(external, "image", 0))
	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, nil)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}

	media := resp["media"].([]feedentity.FeedMedia)
	if media[0].URL != external {
		t.Fatalf("media[0].url = %q; want external URL passed through %q", media[0].URL, external)
	}
}

// Fail-open contract: resolution failure must never erase a persisted
// reference in the feed either.
func TestFeedItemToResponse_FailOpenKeepsPersistedReference(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{})

	raw := "https://labuda-media.s3.ap-southeast-1.amazonaws.com/images/photo.jpg"
	item := feedItemWithMedia(feedMediaOf(raw, "image", 0))
	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, nil)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}

	media := resp["media"].([]feedentity.FeedMedia)
	if media[0].URL != raw {
		t.Fatalf("media[0].url = %q; want fail-open raw reference %q", media[0].URL, raw)
	}
}

// The canonical card media output must agree with the flat media array.
func TestFeedItemToResponse_CardMediaMatchesResolvedFlatMedia(t *testing.T) {
	configureFeedMediaResolver(testFeedCDNBase)

	item := feedItemWithMedia(
		feedMediaOf("images/a.jpg", "image", 0),
		feedMediaOf("videos/b.mp4", "video", 1),
	)
	resp, err := feedItemToResponseCanonicalWithProjection(item, nil, nil, nil)
	if err != nil {
		t.Fatalf("feedItemToResponseCanonicalWithProjection: %v", err)
	}

	card, ok := resp["card"].(publiccard.ContentCard)
	if !ok {
		t.Fatalf("card = %#v; want publiccard.ContentCard", resp["card"])
	}
	if len(card.Media) != 2 {
		t.Fatalf("card media length = %d; want 2", len(card.Media))
	}
	for _, ref := range card.Media {
		if ref.URL != testFeedCDNBase+"/images/a.jpg" && ref.URL != testFeedCDNBase+"/videos/b.mp4" {
			t.Fatalf("card media url = %q; want resolved read URL", ref.URL)
		}
	}
}
