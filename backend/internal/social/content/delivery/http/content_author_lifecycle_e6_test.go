package http

import (
	"testing"

	"github.com/google/uuid"
)

// E6 — Unit tests for the pure, DB-free contentAuthorCardFromRow builder.
//
// Scope: the coarsening rule + the wire shape of the UserCard emitted by the
// content_handler author hydrator (used by CreateContent, UpdateContent,
// GetContent, RepostContent). The SQL projection / fallback paths are
// exercised against a live database in scenario_logs/e6_artifacts; this
// suite keeps the lifecycle threading deterministic without a DB.

func TestContentAuthorCardFromRow_ActiveLifecycle(t *testing.T) {
	id := uuid.New()
	avatar := "https://cdn.example.com/alice.jpg"
	card := contentAuthorCardFromRow(id, "alice", &avatar, "active", false)

	if card.Lifecycle == nil {
		t.Fatal("active author must coarsen to non-nil Lifecycle")
	}
	if got := *card.Lifecycle; got != "active" {
		t.Fatalf("active author lifecycle = %q; want \"active\"", got)
	}
	if card.Username != "alice" {
		t.Fatalf("username = %q; want \"alice\"", card.Username)
	}
	if card.AvatarURL == nil || *card.AvatarURL != avatar {
		t.Fatalf("avatar mismatch; got %v want %s", card.AvatarURL, avatar)
	}
}

func TestContentAuthorCardFromRow_SuspendedLifecycle(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"suspended", "unavailable"},
		{"banned", "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			card := contentAuthorCardFromRow(uuid.New(), "bob", nil, tc.raw, false)
			if card.Lifecycle == nil {
				t.Fatalf("%s must coarsen to non-nil Lifecycle", tc.raw)
			}
			if got := *card.Lifecycle; got != tc.want {
				t.Fatalf("%s author lifecycle = %q; want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestContentAuthorCardFromRow_RemovedLifecycle(t *testing.T) {
	// Slot-persistence: content_handler hydrator deliberately does NOT
	// filter `WHERE u.deleted_at IS NULL` so a tombstoned author surfaces
	// as Lifecycle="removed" instead of falling through to anonymous-safe
	// nil. This pins that contract.
	cases := []struct {
		name      string
		raw       string
		isDeleted bool
	}{
		{"active_with_deleted_at", "active", true},
		{"account_status_deleted", "deleted", false},
		{"suspended_with_deleted_at", "suspended", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := contentAuthorCardFromRow(uuid.New(), "ghost", nil, tc.raw, tc.isDeleted)
			if card.Lifecycle == nil {
				t.Fatalf("%s must coarsen to non-nil Lifecycle", tc.name)
			}
			if got := *card.Lifecycle; got != "removed" {
				t.Fatalf("%s author lifecycle = %q; want \"removed\"", tc.name, got)
			}
		})
	}
}

func TestContentAuthorCardFromRow_EmptyAvatarNormalised(t *testing.T) {
	// Empty-string avatar pointer must normalise to nil on the wire so the
	// card never emits an empty `avatar_url`. Matches publiccard.New
	// semantics inherited via NewWithLifecycle.
	empty := ""
	card := contentAuthorCardFromRow(uuid.New(), "alice", &empty, "active", false)
	if card.AvatarURL != nil {
		t.Fatalf("empty-string avatar must normalise to nil; got %v", card.AvatarURL)
	}
}

func TestContentAuthorCardFromRow_AnonymousUsernameFallback(t *testing.T) {
	// Empty username (no user_profiles row) must fall through to the
	// deterministic anonymous-safe form via publiccard.New. Lifecycle
	// emission is independent of the username fallback path.
	id := uuid.New()
	card := contentAuthorCardFromRow(id, "", nil, "active", false)
	if card.Username == "" {
		t.Fatal("empty username must fall through to anonymous-safe form, got empty")
	}
	if card.Lifecycle == nil || *card.Lifecycle != "active" {
		t.Fatalf("lifecycle must remain canonical even with anonymous username; got %v", card.Lifecycle)
	}
}


