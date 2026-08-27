package http

// FIX-3 — Feed original-author lifecycle regression lock.
//
// Tests for hydrateOriginalAuthorLifecycles, the batch hydration helper
// that emits `original_author_lifecycle` on repost feed items so the
// mobile client can degrade attribution display when the original author
// is unavailable or removed.
//
// Mirrors the nil-tx / empty-input / no-repost patterns used in
// feed_viewercontext_w3a_test.go for the other hydration helpers.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	feedentity "github.com/labuda/backend/internal/social/feed/entity"
)

// TestHydrateOriginalAuthorLifecycles_NilTxReturnsEmpty verifies the
// fail-open contract: a nil transaction must return an empty map (not panic).
func TestHydrateOriginalAuthorLifecycles_NilTxReturnsEmpty(t *testing.T) {
	items := []*feedentity.FeedItem{
		{
			ID:               uuid.New(),
			AuthorID:         uuid.New(),
			Status:           "active",
			OriginalAuthorID: ptrUUID(uuid.New()),
		},
	}
	got := hydrateOriginalAuthorLifecycles(context.Background(), nil, items)
	if len(got) != 0 {
		t.Errorf("nil-tx must return empty map; got %d entries", len(got))
	}
}

// TestHydrateOriginalAuthorLifecycles_EmptyItemsReturnsEmpty verifies that
// an empty or nil items slice returns an empty map without panicking.
func TestHydrateOriginalAuthorLifecycles_EmptyItemsReturnsEmpty(t *testing.T) {
	got := hydrateOriginalAuthorLifecycles(context.Background(), nil, nil)
	if len(got) != 0 {
		t.Errorf("nil items must return empty map; got %d entries", len(got))
	}

	got2 := hydrateOriginalAuthorLifecycles(context.Background(), nil, []*feedentity.FeedItem{})
	if len(got2) != 0 {
		t.Errorf("empty items must return empty map; got %d entries", len(got2))
	}
}

// TestHydrateOriginalAuthorLifecycles_NonRepostItemsReturnsEmpty verifies
// that items with nil OriginalAuthorID (plain posts, not reposts) produce no
// output — the function must short-circuit the ID collection loop.
func TestHydrateOriginalAuthorLifecycles_NonRepostItemsReturnsEmpty(t *testing.T) {
	items := []*feedentity.FeedItem{
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"}, // no OriginalAuthorID
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"}, // no OriginalAuthorID
	}
	// Nil tx is safe here because the ID set will be empty; the function
	// returns before ever touching the tx.
	got := hydrateOriginalAuthorLifecycles(context.Background(), nil, items)
	if len(got) != 0 {
		t.Errorf("non-repost items must produce empty map; got %d entries", len(got))
	}
}

// TestHydrateOriginalAuthorLifecycles_NilItemSkipped verifies that nil
// entries in the items slice are safely skipped without panicking.
func TestHydrateOriginalAuthorLifecycles_NilItemSkipped(t *testing.T) {
	items := []*feedentity.FeedItem{
		nil,
		{ID: uuid.New(), AuthorID: uuid.New(), Status: "active"},
	}
	// Should not panic even with a nil element.
	got := hydrateOriginalAuthorLifecycles(context.Background(), nil, items)
	if len(got) != 0 {
		t.Errorf("nil-item slice must produce empty map; got %d entries", len(got))
	}
}

// ptrUUID returns a pointer to the given UUID value.
func ptrUUID(id uuid.UUID) *uuid.UUID {
	return &id
}


