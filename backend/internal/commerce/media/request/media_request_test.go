package request

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	mediaentity "github.com/labuda/backend/internal/commerce/media/entity"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
)

func TestNormalizeSelection_ImageOnly_Success(t *testing.T) {
	items, err := NormalizeSelection([]MediaRequest{
		{
			Type: "image",
			URL:  "https://cdn.example.com/image-a.jpg",
		},
	}, nil)
	if err != nil {
		t.Fatalf("NormalizeSelection() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Type != mediaentity.MediaTypeImage {
		t.Fatalf("items[0].Type = %s, want image", items[0].Type)
	}
}

func TestNormalizeSelection_VideoOnly_Success(t *testing.T) {
	items, err := NormalizeSelection([]MediaRequest{
		{
			Type:     "video",
			URL:      "https://cdn.example.com/video-a.mp4",
			Duration: intPtr(12),
			Width:    intPtr(1920),
			Height:   intPtr(1080),
			ThumbnailURL: strPtr("https://cdn.example.com/video-a-thumb.jpg"),
		},
	}, nil)
	if err != nil {
		t.Fatalf("NormalizeSelection() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Type != mediaentity.MediaTypeVideo {
		t.Fatalf("items[0].Type = %s, want video", items[0].Type)
	}
	if items[0].Duration == nil || *items[0].Duration != 12 {
		t.Fatalf("items[0].Duration = %#v, want 12ms", items[0].Duration)
	}
	if items[0].Width == nil || *items[0].Width != 1920 {
		t.Fatalf("items[0].Width = %#v, want 1920", items[0].Width)
	}
	if items[0].Height == nil || *items[0].Height != 1080 {
		t.Fatalf("items[0].Height = %#v, want 1080", items[0].Height)
	}
	if items[0].ThumbnailURL == nil || *items[0].ThumbnailURL != "https://cdn.example.com/video-a-thumb.jpg" {
		t.Fatalf("items[0].ThumbnailURL = %#v, want normalized thumbnail", items[0].ThumbnailURL)
	}
}

func TestNormalizeSelection_MixedInterleaved_PreservesOwnerOrdering(t *testing.T) {
	items, err := NormalizeSelection([]MediaRequest{
		{Type: "video", URL: "https://cdn.example.com/video-b.mp4", Duration: intPtr(11)},
		{Type: "image", URL: "https://cdn.example.com/image-a.jpg"},
		{Type: "video", URL: "https://cdn.example.com/video-a.mp4", Duration: intPtr(10)},
		{Type: "image", URL: "https://cdn.example.com/image-b.jpg"},
	}, nil)
	if err != nil {
		t.Fatalf("NormalizeSelection() error = %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("len(items) = %d, want 4", len(items))
	}
	want := []struct {
		url     string
		kind    mediaentity.MediaType
		duration *int
	}{
		{"https://cdn.example.com/video-b.mp4", mediaentity.MediaTypeVideo, intPtr(11)},
		{"https://cdn.example.com/image-a.jpg", mediaentity.MediaTypeImage, nil},
		{"https://cdn.example.com/video-a.mp4", mediaentity.MediaTypeVideo, intPtr(10)},
		{"https://cdn.example.com/image-b.jpg", mediaentity.MediaTypeImage, nil},
	}
	for i, w := range want {
		if items[i].URL != w.url {
			t.Fatalf("items[%d].URL = %q, want %q", i, items[i].URL, w.url)
		}
		if items[i].Type != w.kind {
			t.Fatalf("items[%d].Type = %s, want %s", i, items[i].Type, w.kind)
		}
		if items[i].Position != i {
			t.Fatalf("items[%d].Position = %d, want %d", i, items[i].Position, i)
		}
		if w.duration == nil {
			if items[i].Duration != nil {
				t.Fatalf("items[%d].Duration = %#v, want nil", i, items[i].Duration)
			}
		} else if items[i].Duration == nil || *items[i].Duration != *w.duration {
			t.Fatalf("items[%d].Duration = %#v, want %d", i, items[i].Duration, *w.duration)
		}
	}
}

func TestNormalizeSelection_RejectsAmbiguousPayload(t *testing.T) {
	_, err := NormalizeSelection(
		[]MediaRequest{{Type: "image", URL: "https://cdn.example.com/image-a.jpg"}},
		[]string{"https://cdn.example.com/legacy-a.jpg"},
	)
	if err == nil {
		t.Fatal("NormalizeSelection() error = nil, want conflict")
	}
	typed, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if typed.Code != ErrCodeAmbiguousPayload {
		t.Fatalf("error code = %s, want %s", typed.Code, ErrCodeAmbiguousPayload)
	}
}

func TestNormalizeSelection_RejectsInvalidType(t *testing.T) {
	_, err := NormalizeSelection([]MediaRequest{{Type: "gif", URL: "https://cdn.example.com/image-a.gif"}}, nil)
	assertTypedError(t, err, ErrCodeInvalidType)
}

func TestNormalizeSelection_RejectsBlankURL(t *testing.T) {
	_, err := NormalizeSelection([]MediaRequest{{Type: "image", URL: "   "}}, nil)
	assertTypedError(t, err, ErrCodeInvalidURL)
}

func TestNormalizeSelection_RejectsBlankLegacyURL(t *testing.T) {
	_, err := NormalizeSelection(nil, []string{"   "})
	assertTypedError(t, err, ErrCodeInvalidURL)
}

func TestNormalizeSelection_RejectsInvalidDimensions(t *testing.T) {
	_, err := NormalizeSelection([]MediaRequest{{
		Type:  "image",
		URL:   "https://cdn.example.com/image-a.jpg",
		Width: intPtr(0),
	}}, nil)
	assertTypedError(t, err, ErrCodeInvalidDimensions)
}

func TestNormalizeSelection_RejectsInvalidDuration(t *testing.T) {
	_, err := NormalizeSelection([]MediaRequest{{
		Type:     "video",
		URL:      "https://cdn.example.com/video-a.mp4",
		Duration: intPtr(0),
	}}, nil)
	assertTypedError(t, err, ErrCodeInvalidDuration)
}

func TestNormalizeSelection_RejectsInvalidThumbnailURL(t *testing.T) {
	blank := " "
	_, err := NormalizeSelection([]MediaRequest{{
		Type:         "image",
		URL:          "https://cdn.example.com/image-a.jpg",
		ThumbnailURL: &blank,
	}}, nil)
	assertTypedError(t, err, ErrCodeInvalidThumbnailURL)
}

func TestNormalizeSelection_LegacyMediaURLsBecomeTypedImages(t *testing.T) {
	items, err := NormalizeSelection(nil, []string{
		"https://cdn.example.com/legacy-a.jpg",
		"https://cdn.example.com/legacy-b.jpg",
	})
	if err != nil {
		t.Fatalf("NormalizeSelection() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	for i, item := range items {
		if item.Type != mediaentity.MediaTypeImage {
			t.Fatalf("items[%d].Type = %s, want image", i, item.Type)
		}
	}
}

func TestNormalizeSelection_ExplicitEmptyArrays_ClearMedia(t *testing.T) {
	items, err := NormalizeSelection([]MediaRequest{}, []string{})
	if err != nil {
		t.Fatalf("NormalizeSelection() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func assertTypedError(t *testing.T, err error, wantCode string) {
	t.Helper()
	if err == nil {
		t.Fatal("NormalizeSelection() error = nil, want validation error")
	}
	var typed *ValidationError
	if !errors.As(err, &typed) {
		t.Fatalf("error type = %T, want wrapped *ValidationError", err)
	}
	if typed.Code != wantCode {
		t.Fatalf("error code = %s, want %s", typed.Code, wantCode)
	}
}

func intPtr(v int) *int { return &v }

func strPtr(v string) *string { return &v }

func TestToProductMedia_PreservesVideoMetadata(t *testing.T) {
	now := time.Now().UTC()
	video, err := mediaentity.NewMedia("videos/clip.mp4", mediaentity.MediaTypeVideo, 0, now)
	if err != nil {
		t.Fatalf("NewMedia() error = %v", err)
	}
	video.ThumbnailURL = strPtr("images/clip-thumb.jpg")
	video.Width = intPtr(1920)
	video.Height = intPtr(1080)
	video.Duration = intPtr(12500)
	video.Blurhash = strPtr("LKO2?U%2Tw=w]~RBVZRi};RPxuwH")

	got := ToProductMedia([]mediaentity.Media{*video})
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	slot := got[0]
	if slot.URL != "videos/clip.mp4" {
		t.Fatalf("slot.URL = %q, want videos/clip.mp4", slot.URL)
	}
	if slot.ThumbnailURL == nil || *slot.ThumbnailURL != "images/clip-thumb.jpg" {
		t.Fatalf("slot.ThumbnailURL = %#v, want persisted thumbnail", slot.ThumbnailURL)
	}
	if slot.Width == nil || *slot.Width != 1920 || slot.Height == nil || *slot.Height != 1080 {
		t.Fatalf("slot dims = %#v/%#v, want 1920/1080", slot.Width, slot.Height)
	}
	if slot.DurationMs == nil || *slot.DurationMs != 12500 {
		t.Fatalf("slot.DurationMs = %#v, want 12500ms", slot.DurationMs)
	}
	if slot.Blurhash == nil || *slot.Blurhash != "LKO2?U%2Tw=w]~RBVZRi};RPxuwH" {
		t.Fatalf("slot.Blurhash lost: %#v", slot.Blurhash)
	}
}

func TestProductMedia_JSONRoundTrip_ToleratesLegacyShapes(t *testing.T) {
	full := productentity.ProductMedia{
		URL:          "videos/clip.mp4",
		Blurhash:     strPtr("LKO2?U%2Tw=w]~RBVZRi};RPxuwH"),
		ThumbnailURL: strPtr("images/clip-thumb.jpg"),
		Width:        intPtr(1920),
		Height:       intPtr(1080),
		DurationMs:   intPtr(12500),
	}
	raw, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var back productentity.ProductMedia
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal(canonical) error = %v", err)
	}
	if back.URL != full.URL || back.DurationMs == nil || *back.DurationMs != 12500 {
		t.Fatalf("round trip lost data: %#v", back)
	}

	// Legacy bare string.
	var bare productentity.ProductMedia
	if err := json.Unmarshal([]byte(`"videos/old.mp4"`), &bare); err != nil {
		t.Fatalf("Unmarshal(bare string) error = %v", err)
	}
	if bare.URL != "videos/old.mp4" {
		t.Fatalf("bare.URL = %q, want videos/old.mp4", bare.URL)
	}

	// Legacy capital-key object (pre-tag marshal shape).
	var caps productentity.ProductMedia
	if err := json.Unmarshal([]byte(`{"URL":"videos/caps.mp4","Blurhash":"L00000"}`), &caps); err != nil {
		t.Fatalf("Unmarshal(capital keys) error = %v", err)
	}
	if caps.URL != "videos/caps.mp4" || caps.Blurhash == nil {
		t.Fatalf("capital-key shape lost data: %#v", caps)
	}
}
