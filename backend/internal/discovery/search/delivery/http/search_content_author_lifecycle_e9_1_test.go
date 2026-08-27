package http

// E9.1 — /search/content author user-identity lifecycle convergence tests.
//
// Scope is pinned to two seams:
//  1. hydrateContentAuthorLifecycle — batch SQL → coarsened lifecycle map.
//     Validated via contentAuthorCardFromRow (the shared pure builder that
//     both content_handler and search_handler now delegate to) so we avoid
//     a live DB dependency while exercising the coarsening contract.
//  2. contentPreviewsToResponse — author card carries lifecycle after E9.1.
//     Validated by inspecting the emitted item map for the author card shape.
//
// Wire preservation: legacy flat fields (author_id, author_username,
// author_avatar_url, media_urls, created_at, author, card) must remain
// byte-for-byte identical to their pre-E9.1 values — only
// author.lifecycle becomes non-nil when the hydrator supplies a lifecycle.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/discovery/search/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

// ---------------------------------------------------------------------------
// 1. Lifecycle coarsening contract (pure, no DB)
// ---------------------------------------------------------------------------

// contentAuthorLifecycleFromRaw is the pure per-row coarsening path exercised
// by hydrateContentAuthorLifecycle after scanning each DB row. We pin it
// directly via viewercontext.CoarsenLifecycle so the test does not need a
// live transaction.
func contentAuthorLifecycleFromRaw(accountStatus string, isDeleted bool) string {
	return string(viewercontext.CoarsenLifecycle(accountStatus, isDeleted))
}

func TestContentAuthorLifecycleCoarsening(t *testing.T) {
	tests := []struct {
		name          string
		accountStatus string
		isDeleted     bool
		want          string
	}{
		{
			name:          "active user → active",
			accountStatus: "active",
			isDeleted:     false,
			want:          "active",
		},
		{
			name:          "suspended user → unavailable",
			accountStatus: "suspended",
			isDeleted:     false,
			want:          "unavailable",
		},
		{
			name:          "banned user → unavailable",
			accountStatus: "banned",
			isDeleted:     false,
			want:          "unavailable",
		},
		{
			name:          "deleted_at IS NOT NULL → removed regardless of status",
			accountStatus: "active",
			isDeleted:     true,
			want:          "removed",
		},
		{
			name:          "account_status=deleted → removed",
			accountStatus: "deleted",
			isDeleted:     false,
			want:          "removed",
		},
		{
			name:          "empty account_status + not deleted → active (unknown maps to active)",
			accountStatus: "",
			isDeleted:     false,
			want:          "active",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contentAuthorLifecycleFromRaw(tt.accountStatus, tt.isDeleted)
			if got != tt.want {
				t.Errorf("coarsenLifecycle(%q, %v) = %q; want %q",
					tt.accountStatus, tt.isDeleted, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. contentPreviewsToResponse — author card lifecycle field
// ---------------------------------------------------------------------------

func makeTestContentPreview(authorID uuid.UUID) *entity.ContentPreview {
	return &entity.ContentPreview{
		ID:              uuid.New(),
		AuthorID:        authorID,
		Type:            "post",
		Caption:         "test caption",
		MediaURLs:       []string{},
		CreatedAt:       time.Now(),
		AuthorUsername:  "alice",
		AuthorAvatarURL: nil,
	}
}

func TestContentPreviewsToResponse_AuthorLifecycle(t *testing.T) {
	authorID := uuid.New()
	preview := makeTestContentPreview(authorID)

	tests := []struct {
		name                string
		authorLifecycleByID map[uuid.UUID]string
		wantLifecycle       *string
	}{
		{
			name:                "nil map (pre-E9.1 / rollback) → author.lifecycle nil",
			authorLifecycleByID: nil,
			wantLifecycle:       nil,
		},
		{
			name:                "empty map → author.lifecycle nil",
			authorLifecycleByID: map[uuid.UUID]string{},
			wantLifecycle:       nil,
		},
		{
			name:                "active lifecycle → author.lifecycle = 'active'",
			authorLifecycleByID: map[uuid.UUID]string{authorID: "active"},
			wantLifecycle:       strPtr("active"),
		},
		{
			name:                "unavailable lifecycle → author.lifecycle = 'unavailable'",
			authorLifecycleByID: map[uuid.UUID]string{authorID: "unavailable"},
			wantLifecycle:       strPtr("unavailable"),
		},
		{
			name:                "removed lifecycle → author.lifecycle = 'removed'",
			authorLifecycleByID: map[uuid.UUID]string{authorID: "removed"},
			wantLifecycle:       strPtr("removed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items := contentPreviewsToResponse([]*entity.ContentPreview{preview}, nil, tt.authorLifecycleByID)
			if len(items) != 1 {
				t.Fatalf("expected 1 item; got %d", len(items))
			}
			item := items[0]

			// author field must be a UserCard
			rawAuthor, ok := item["author"]
			if !ok {
				t.Fatal("item missing 'author' key")
			}
			card, ok := rawAuthor.(publiccard.UserCard)
			if !ok {
				t.Fatalf("author is %T; want publiccard.UserCard", rawAuthor)
			}

			if tt.wantLifecycle == nil {
				if card.Lifecycle != nil {
					t.Errorf("author.Lifecycle = %q; want nil", *card.Lifecycle)
				}
			} else {
				if card.Lifecycle == nil {
					t.Errorf("author.Lifecycle = nil; want %q", *tt.wantLifecycle)
				} else if *card.Lifecycle != *tt.wantLifecycle {
					t.Errorf("author.Lifecycle = %q; want %q", *card.Lifecycle, *tt.wantLifecycle)
				}
			}
		})
	}
}

func TestContentPreviewsToResponse_LegacyFieldsPreserved(t *testing.T) {
	authorID := uuid.New()
	preview := makeTestContentPreview(authorID)
	lifecycleMap := map[uuid.UUID]string{authorID: "active"}

	items := contentPreviewsToResponse([]*entity.ContentPreview{preview}, nil, lifecycleMap)
	if len(items) != 1 {
		t.Fatalf("expected 1 item; got %d", len(items))
	}
	item := items[0]

	requiredKeys := []string{"id", "author_id", "type", "caption", "media_urls", "created_at", "author", "media", "card"}
	for _, key := range requiredKeys {
		if _, ok := item[key]; !ok {
			t.Errorf("legacy field %q missing from response item", key)
		}
	}

	if got := item["author_id"]; got != preview.AuthorID.String() {
		t.Errorf("author_id = %v; want %v", got, preview.AuthorID.String())
	}
}

func TestContentPreviewsToResponse_ContentLifecycleOverrideUnchanged(t *testing.T) {
	authorID := uuid.New()
	contentID := uuid.New()
	preview := &entity.ContentPreview{
		ID:             contentID,
		AuthorID:       authorID,
		Type:           "post",
		Caption:        "tombstoned content",
		MediaURLs:      []string{},
		CreatedAt:      time.Now(),
		AuthorUsername: "alice",
	}

	lifecycleOverrides := map[uuid.UUID]string{contentID: "removed"}
	authorLifecycleMap := map[uuid.UUID]string{authorID: "active"}

	items := contentPreviewsToResponse([]*entity.ContentPreview{preview}, lifecycleOverrides, authorLifecycleMap)
	if len(items) != 1 {
		t.Fatalf("expected 1 item; got %d", len(items))
	}
	item := items[0]

	rawCard, ok := item["card"]
	if !ok {
		t.Fatal("item missing 'card' key")
	}
	contentCard, ok := rawCard.(publiccard.ContentCard)
	if !ok {
		t.Fatalf("card is %T; want publiccard.ContentCard", rawCard)
	}
	if contentCard.Lifecycle == nil || *contentCard.Lifecycle != "removed" {
		lc := "<nil>"
		if contentCard.Lifecycle != nil {
			lc = *contentCard.Lifecycle
		}
		t.Errorf("card.Lifecycle = %q; want 'removed' (content lifecycle override must be preserved)", lc)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }
