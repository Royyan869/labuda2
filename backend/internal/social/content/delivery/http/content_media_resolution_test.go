package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
	"github.com/labuda/backend/internal/social/content/entity"
)

const testCDNBase = "https://d358tu61i1wrtt.cloudfront.net"

func configureContentMediaResolver(t *testing.T, cdnBase string) {
	t.Helper()
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

func contentWithMedia(t *testing.T, mediaURLs ...string) (*entity.Content, []*entity.ContentMedia) {
	t.Helper()
	content := &entity.Content{
		ID:         uuid.New(),
		AuthorID:   uuid.New(),
		Status:     entity.StatusActive,
		Visibility: entity.VisibilityPublic,
		CreatedAt:  time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
	}
	media := make([]*entity.ContentMedia, 0, len(mediaURLs))
	for i, url := range mediaURLs {
		media = append(media, &entity.ContentMedia{
			ID:        uuid.New(),
			ContentID: content.ID,
			MediaURL:  url,
			MediaType: entity.MediaTypeImage,
			Position:  i,
			CreatedAt: time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
		})
	}
	return content, media
}

// content_media.media_url may hold a storage reference or a URL. The mobile
// content card renders media[].url directly, so this surface must project the
// persisted reference through the canonical mediaresolve authority.
func TestToContentResponse_ResolvesStorageReferenceToReadURL(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	content, media := contentWithMedia(t, "images/1749600000000_author.jpg")
	resp := ToContentResponse(content, media)

	if len(resp.Media) != 1 {
		t.Fatalf("media length = %d; want 1", len(resp.Media))
	}
	want := testCDNBase + "/images/1749600000000_author.jpg"
	if resp.Media[0].URL != want {
		t.Fatalf("media[0].url = %q; want resolved read URL %q", resp.Media[0].URL, want)
	}
}

func TestToContentResponse_ResolvesRawBucketURLToReadURL(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	content, media := contentWithMedia(t,
		"https://labuda-media.s3.ap-southeast-1.amazonaws.com/images/photo.jpg",
	)
	resp := ToContentResponse(content, media)

	want := testCDNBase + "/images/photo.jpg"
	if resp.Media[0].URL != want {
		t.Fatalf("media[0].url = %q; want resolved read URL %q", resp.Media[0].URL, want)
	}
}

func TestToContentResponse_PreservesExternalAbsoluteURL(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	external := "https://cdn.example.com/content/image-a.jpg"
	content, media := contentWithMedia(t, external)
	resp := ToContentResponse(content, media)

	if resp.Media[0].URL != external {
		t.Fatalf("media[0].url = %q; want external URL passed through %q", resp.Media[0].URL, external)
	}
}

// Fail-open contract: resolution failure must never erase a persisted
// reference. The raw reference is emitted unchanged.
func TestToContentResponse_FailOpenKeepsPersistedReference(t *testing.T) {
	// Unconfigured resolver: no CDN base, invalid presign config.
	mediaresolve.SetDefaultConfig(mediaresolve.Config{})

	raw := "https://labuda-media.s3.ap-southeast-1.amazonaws.com/images/photo.jpg"
	content, media := contentWithMedia(t, raw)
	resp := ToContentResponse(content, media)

	if resp.Media[0].URL != raw {
		t.Fatalf("media[0].url = %q; want fail-open raw reference %q", resp.Media[0].URL, raw)
	}
}

func TestToContentResponse_EmptyMediaURLStaysEmpty(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	content, media := contentWithMedia(t, "")
	resp := ToContentResponse(content, media)

	if resp.Media[0].URL != "" {
		t.Fatalf("media[0].url = %q; want empty", resp.Media[0].URL)
	}
}

// POSITIVE + NEGATIVE PROOF — the canonical card media refs and the flat
// `media[].url` array are ONE authority: both must carry the resolved read
// URL and can never disagree. The negative arm pins that the raw persisted
// `content_media.media_url` storage reference never reaches the card, which
// was the competing resolution authority this convergence removed.
func TestToContentResponseWithAuthorAndProjection_CardMediaMatchesResolvedFlatMedia(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	content, media := contentWithMedia(t, "images/a.jpg", "videos/b.mp4")
	media[1].MediaType = entity.MediaTypeVideo
	author := publiccard.NewWithLifecycle(content.AuthorID, "author", nil, "active")

	resp := ToContentResponseWithAuthorAndProjection(content, media, &author, nil)

	if resp.Card == nil {
		t.Fatal("card is nil; want canonical ContentCard")
	}
	want := []string{testCDNBase + "/images/a.jpg", testCDNBase + "/videos/b.mp4"}
	if len(resp.Card.Media) != len(resp.Media) {
		t.Fatalf("card media length = %d; want %d (must mirror flat media)", len(resp.Card.Media), len(resp.Media))
	}
	for i := range resp.Card.Media {
		if resp.Card.Media[i].URL != want[i] {
			t.Fatalf("card media[%d].url = %q; want resolved read URL %q", i, resp.Card.Media[i].URL, want[i])
		}
		if resp.Card.Media[i].URL != resp.Media[i].URL {
			t.Fatalf("card media[%d].url = %q disagrees with flat media[%d].url = %q", i, resp.Card.Media[i].URL, i, resp.Media[i].URL)
		}
	}

	// NEGATIVE: the raw persisted reference must never be emitted on the card.
	for i, ref := range resp.Card.Media {
		if ref.URL == "images/a.jpg" || ref.URL == "videos/b.mp4" {
			t.Fatalf("card media[%d] leaked raw persisted reference %q", i, ref.URL)
		}
	}
}

// The response must preserve the canonical media identity (type + position)
// alongside the resolved URL — resolution must not reshape the contract.
func TestToContentResponse_ResolutionPreservesTypeAndPosition(t *testing.T) {
	configureContentMediaResolver(t, testCDNBase)

	content, media := contentWithMedia(t, "images/a.jpg", "videos/b.mp4")
	media[1].MediaType = entity.MediaTypeVideo

	resp := ToContentResponse(content, media)

	if resp.Media[0].Type != "image" || resp.Media[1].Type != "video" {
		t.Fatalf("types = %q,%q; want image,video", resp.Media[0].Type, resp.Media[1].Type)
	}
	if resp.Media[0].Position != 0 || resp.Media[1].Position != 1 {
		t.Fatalf("positions = %d,%d; want 0,1", resp.Media[0].Position, resp.Media[1].Position)
	}
	if resp.Media[1].URL != testCDNBase+"/videos/b.mp4" {
		t.Fatalf("media[1].url = %q; want resolved video read URL", resp.Media[1].URL)
	}
}
