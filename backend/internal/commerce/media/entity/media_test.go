package entity

import (
	"testing"
	"time"
)

func TestNewLegacyImageListFromReferences_UsesImageForAllLegacyMedia(t *testing.T) {
	items, err := NewLegacyImageListFromReferences([]string{
		"https://cdn.example.com/legacy/image-a.jpg",
		"https://cdn.example.com/legacy/video-b.mp4",
	}, time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewLegacyImageListFromReferences() error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Type != MediaTypeImage {
		t.Fatalf("items[0].Type = %s, want image", items[0].Type)
	}
	if items[1].Type != MediaTypeImage {
		t.Fatalf("items[1].Type = %s, want image", items[1].Type)
	}
	if items[0].Position != 0 || items[1].Position != 1 {
		t.Fatalf("positions = %d,%d, want 0,1", items[0].Position, items[1].Position)
	}
}

func TestNewListFromReferences_InfersTypedMediaForExplicitTypedPath(t *testing.T) {
	items, err := NewListFromReferences([]string{
		"https://cdn.example.com/typed/image-a.jpg",
		"https://cdn.example.com/typed/video-b.mp4",
	}, time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewListFromReferences() error = %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Type != MediaTypeImage {
		t.Fatalf("items[0].Type = %s, want image", items[0].Type)
	}
	if items[1].Type != MediaTypeVideo {
		t.Fatalf("items[1].Type = %s, want video", items[1].Type)
	}
}
