package http

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
	"github.com/labuda/backend/internal/platform/mediaresolve"
	"github.com/labuda/backend/internal/platform/s3presign"
)

const testSearchCDNBase = "https://d358tu61i1wrtt.cloudfront.net"

// configureSearchMediaResolver installs the canonical mediaresolve config for
// /search/content read-resolution proofs. Every other test in this package
// exercises absolute URLs, whose resolution is a pass-through under any config.
func configureSearchMediaResolver(cdnBase string) {
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

func searchContentPreviewWithMedia(mediaURLs ...string) *entity.ContentPreview {
	return &entity.ContentPreview{
		ID:             uuid.New(),
		AuthorID:       uuid.New(),
		Type:           "post",
		Caption:        "searchable content",
		MediaURLs:      mediaURLs,
		CreatedAt:      time.Date(2026, time.September, 16, 10, 0, 0, 0, time.UTC),
		AuthorUsername: "alice",
	}
}

func searchContentRow(t *testing.T, preview *entity.ContentPreview) map[string]interface{} {
	t.Helper()
	items := contentPreviewsToResponseWithProjections([]*entity.ContentPreview{preview}, nil, nil, nil)
	if len(items) != 1 {
		t.Fatalf("expected 1 search content row; got %d", len(items))
	}
	return items[0]
}

func searchRowMediaRefs(t *testing.T, item map[string]interface{}) []mediaref.MediaRef {
	t.Helper()
	refs, ok := item["media"].([]mediaref.MediaRef)
	if !ok {
		t.Fatalf("media = %#v; want []mediaref.MediaRef", item["media"])
	}
	return refs
}

func searchRowFlatMediaURLs(t *testing.T, item map[string]interface{}) []string {
	t.Helper()
	urls, ok := item["media_urls"].([]string)
	if !ok {
		t.Fatalf("media_urls = %#v; want []string", item["media_urls"])
	}
	return urls
}

func searchRowCard(t *testing.T, item map[string]interface{}) publiccard.ContentCard {
	t.Helper()
	card, ok := item["card"].(publiccard.ContentCard)
	if !ok {
		t.Fatalf("card = %#v; want publiccard.ContentCard", item["card"])
	}
	return card
}

// SearchContent loads persisted `content_media.media_url` references. The
// /search/content read surface must project them through the canonical
// mediaresolve authority on EVERY media surface of the row — and the three
// surfaces must agree, since they are one resolved list.
func TestContentPreviewsToResponse_ResolvesContentMediaStorageReference(t *testing.T) {
	configureSearchMediaResolver(testSearchCDNBase)

	preview := searchContentPreviewWithMedia("images/1749600000000_search.jpg")
	item := searchContentRow(t, preview)

	want := testSearchCDNBase + "/images/1749600000000_search.jpg"

	flat := searchRowFlatMediaURLs(t, item)
	if len(flat) != 1 || flat[0] != want {
		t.Fatalf("media_urls = %#v; want [%q]", flat, want)
	}

	refs := searchRowMediaRefs(t, item)
	if len(refs) != 1 || refs[0].URL != want {
		t.Fatalf("media[0].url = %#v; want %q", refs, want)
	}

	card := searchRowCard(t, item)
	if len(card.Media) != 1 || card.Media[0].URL != want {
		t.Fatalf("card.media[0].url = %#v; want %q", card.Media, want)
	}

	// NEGATIVE: the raw persisted reference must never be emitted anywhere on
	// the row.
	for _, surface := range []string{flat[0], refs[0].URL, card.Media[0].URL} {
		if surface == "images/1749600000000_search.jpg" {
			t.Fatalf("row leaked raw persisted reference %q", surface)
		}
	}
}

func TestContentPreviewsToResponse_ResolvesRawBucketURL(t *testing.T) {
	configureSearchMediaResolver(testSearchCDNBase)

	preview := searchContentPreviewWithMedia(
		"https://labuda-media.s3.ap-southeast-1.amazonaws.com/images/search-photo.jpg",
	)
	item := searchContentRow(t, preview)

	want := testSearchCDNBase + "/images/search-photo.jpg"
	if got := searchRowFlatMediaURLs(t, item)[0]; got != want {
		t.Fatalf("media_urls[0] = %q; want resolved read URL %q", got, want)
	}
	if got := searchRowMediaRefs(t, item)[0].URL; got != want {
		t.Fatalf("media[0].url = %q; want resolved read URL %q", got, want)
	}
}

func TestContentPreviewsToResponse_PreservesExternalAbsoluteURL(t *testing.T) {
	configureSearchMediaResolver(testSearchCDNBase)

	external := "https://cdn.example.com/content/search-image.jpg"
	item := searchContentRow(t, searchContentPreviewWithMedia(external))

	if got := searchRowFlatMediaURLs(t, item)[0]; got != external {
		t.Fatalf("media_urls[0] = %q; want external URL passed through %q", got, external)
	}
	if got := searchRowCard(t, item).Media[0].URL; got != external {
		t.Fatalf("card.media[0].url = %q; want external URL passed through %q", got, external)
	}
}

// Fail-open contract: resolution failure must never erase a persisted
// reference. With an unconfigured resolver the raw reference is emitted
// trimmed and unchanged.
func TestContentPreviewsToResponse_FailOpenKeepsPersistedReference(t *testing.T) {
	mediaresolve.SetDefaultConfig(mediaresolve.Config{})

	raw := "https://labuda-media.s3.ap-southeast-1.amazonaws.com/images/photo.jpg"
	item := searchContentRow(t, searchContentPreviewWithMedia(raw))

	if got := searchRowFlatMediaURLs(t, item)[0]; got != raw {
		t.Fatalf("media_urls[0] = %q; want fail-open raw reference %q", got, raw)
	}
	if got := searchRowCard(t, item).Media[0].URL; got != raw {
		t.Fatalf("card.media[0].url = %q; want fail-open raw reference %q", got, raw)
	}
}

// Search-specific business semantics: the DISTINCT + ORDER BY media_url array
// produced by SearchContent is preserved element-for-element (length, order,
// and empty entries) — resolution must not reshape, dedupe, or reorder it.
func TestContentPreviewsToResponse_PreservesSearchMediaOrderAndShape(t *testing.T) {
	configureSearchMediaResolver(testSearchCDNBase)

	item := searchContentRow(t, searchContentPreviewWithMedia("", "images/b.jpg", "images/a.jpg"))

	flat := searchRowFlatMediaURLs(t, item)
	if len(flat) != 3 {
		t.Fatalf("media_urls length = %d; want 3 (shape preserved)", len(flat))
	}
	if flat[0] != "" {
		t.Fatalf("media_urls[0] = %q; want empty preserved", flat[0])
	}
	if flat[1] != testSearchCDNBase+"/images/b.jpg" || flat[2] != testSearchCDNBase+"/images/a.jpg" {
		t.Fatalf("media_urls = %#v; want [\"\" resolved(b) resolved(a)] in input order", flat)
	}
}

// No content media at all: the row must keep the shape SearchContent produced
// (absent media_urls stays absent-shaped, media refs stay an empty non-nil
// slice) and must not invent a URL.
func TestContentPreviewsToResponse_NoMediaEmitsEmptyShape(t *testing.T) {
	configureSearchMediaResolver(testSearchCDNBase)

	item := searchContentRow(t, searchContentPreviewWithMedia())

	flat := searchRowFlatMediaURLs(t, item)
	if len(flat) != 0 {
		t.Fatalf("media_urls = %#v; want empty", flat)
	}
	if refs := searchRowMediaRefs(t, item); len(refs) != 0 {
		t.Fatalf("media = %#v; want empty refs", refs)
	}
	if card := searchRowCard(t, item); len(card.Media) != 0 {
		t.Fatalf("card.media = %#v; want empty", card.Media)
	}
}
