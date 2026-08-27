package entity

// FIX-2 — Content detail repost governance regression lock.
//
// The content detail handler (GET /contents/:id) now returns 404 when the
// requested content is a repost whose original is hidden, deleted, or
// otherwise non-visible. These tests pin the entity predicates that the
// handler's governance check depends on.
//
// The canonical repost marker is OriginalAuthorID.

import (
	"testing"

	"github.com/google/uuid"
)

// TestContentRepostGovernance_EntityPredicates validates the entity-level
// predicates that FIX-2 relies on. Each sub-test pins a specific field or
// constant combination used in the handler guard.
func TestContentRepostGovernance_EntityPredicates(t *testing.T) {
	t.Run("StatusDeleted constant is non-empty", func(t *testing.T) {
		if StatusDeleted == "" {
			t.Error("StatusDeleted must be a non-empty string")
		}
	})

	t.Run("Content with StatusDeleted is not IsVisible", func(t *testing.T) {
		c := &Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   StatusDeleted,
			IsHidden: false,
		}
		if c.IsVisible() {
			t.Error("deleted content must not be IsVisible()")
		}
	})

	t.Run("Content with IsHidden=true is not IsVisible", func(t *testing.T) {
		c := &Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   StatusActive,
			IsHidden: true,
		}
		if c.IsVisible() {
			t.Error("hidden content must not be IsVisible()")
		}
	})

	t.Run("Active visible content IsVisible returns true", func(t *testing.T) {
		c := &Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   StatusActive,
			IsHidden: false,
		}
		if !c.IsVisible() {
			t.Error("active visible content must be IsVisible()")
		}
	})

	t.Run("ShareTargetTypeContent constant is 'content'", func(t *testing.T) {
		// The handler compares TargetType to this constant; if it drifts the
		// repost governance check would silently never fire.
		if ShareTargetTypeContent != "content" {
			t.Errorf("ShareTargetTypeContent = %q; want \"content\"", ShareTargetTypeContent)
		}
	})

	t.Run("repost content has non-nil OriginalAuthorID", func(t *testing.T) {
		origID := uuid.New()
		c := &Content{
			ID:               uuid.New(),
			AuthorID:         uuid.New(),
			Status:           StatusActive,
			OriginalAuthorID: &origID,
		}
		if c.OriginalAuthorID == nil {
			t.Error("repost content must have non-nil OriginalAuthorID")
		}
	})

	t.Run("plain content has nil OriginalAuthorID", func(t *testing.T) {
		plain := &Content{
			ID:       uuid.New(),
			AuthorID: uuid.New(),
			Status:   StatusActive,
		}
		if plain.OriginalAuthorID != nil {
			t.Error("plain content must have nil OriginalAuthorID")
		}
	})
}


