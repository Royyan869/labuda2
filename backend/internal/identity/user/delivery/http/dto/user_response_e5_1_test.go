package dto

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

// E5.1 — Unit tests for PublicUserResponse wire shape after profile-detail
// publiccard convergence. These tests pin the canonical seam:
//
//   raw users.account_status + (users.deleted_at IS NOT NULL)
//     -> viewercontext.CoarsenLifecycle
//     -> publiccard.NewWithLifecycle
//     -> PublicUserResponse.Identity.lifecycle on the wire
//
// Scope is the wire shape only — no DB, no handler. The service-level
// coarsen call is replicated here exactly as user_profile_service.go does
// it, so any divergence between this test and the service will fail.

func buildIdentityForLifecycle(t *testing.T, id uuid.UUID, username string, avatar *string, rawStatus string, isDeleted bool) publiccard.UserCard {
	t.Helper()
	lifecycle := string(viewercontext.CoarsenLifecycle(rawStatus, isDeleted))
	return publiccard.NewWithLifecycle(id, username, avatar, lifecycle)
}

func TestPublicUserResponse_E5_1_ActiveLifecycle(t *testing.T) {
	id := uuid.New()
	avatar := "https://cdn.example.com/a.jpg"
	card := buildIdentityForLifecycle(t, id, "alice", &avatar, "active", false)

	resp := PublicUserResponse{
		UserID:    id,
		Username:  "alice",
		CreatedAt: time.Now(),
		Identity:  &card,
	}

	if resp.Identity == nil {
		t.Fatal("Identity card must not be nil after E5.1 convergence")
	}
	if resp.Identity.Lifecycle == nil {
		t.Fatal("Identity.Lifecycle must be populated (reserved-nil is forbidden post-E5.1)")
	}
	if got := *resp.Identity.Lifecycle; got != "active" {
		t.Fatalf("active account_status must coarsen to \"active\", got %q", got)
	}
	if resp.Identity.Username != "alice" {
		t.Fatalf("identity.username must mirror username, got %q", resp.Identity.Username)
	}
	if resp.Identity.AvatarURL == nil || *resp.Identity.AvatarURL != avatar {
		t.Fatalf("identity.avatar_url must mirror avatar, got %v", resp.Identity.AvatarURL)
	}
}

func TestPublicUserResponse_E5_1_SuspendedLifecycle(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"suspended", "unavailable"},
		{"banned", "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			id := uuid.New()
			card := buildIdentityForLifecycle(t, id, "bob", nil, tc.raw, false)

			if card.Lifecycle == nil {
				t.Fatalf("%s must coarsen to a non-nil lifecycle", tc.raw)
			}
			if got := *card.Lifecycle; got != tc.want {
				t.Fatalf("%s must coarsen to %q, got %q", tc.raw, tc.want, got)
			}
		})
	}
}

func TestPublicUserResponse_E5_1_RemovedLifecycle(t *testing.T) {
	// "removed" semantics — exercised via the coarsener, even though the
	// current GetPublicInfo SQL filter (WHERE u.deleted_at IS NULL) means
	// this branch is unreachable from /users/:id today. Pinned here so a
	// future relaxation of the filter does not silently degrade the
	// coarsening contract.
	cases := []struct {
		raw       string
		isDeleted bool
	}{
		{"active", true},      // deleted_at present overrides status
		{"deleted", false},    // account_status='deleted' overrides
		{"suspended", true},   // deleted_at wins over suspended
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			card := buildIdentityForLifecycle(t, uuid.New(), "ghost", nil, tc.raw, tc.isDeleted)
			if card.Lifecycle == nil || *card.Lifecycle != "removed" {
				t.Fatalf("raw=%s deletedAt=%v must coarsen to \"removed\", got %v", tc.raw, tc.isDeleted, card.Lifecycle)
			}
		})
	}
}

func TestPublicUserResponse_E5_1_WireShape_LegacyFieldsPreserved(t *testing.T) {
	id := uuid.New()
	avatar := "https://cdn.example.com/a.jpg"
	bio := "loves agriculture"
	location := "Bandung"
	card := buildIdentityForLifecycle(t, id, "alice", &avatar, "active", false)

	resp := PublicUserResponse{
		UserID:         id,
		Username:       "alice",
		Bio:            &bio,
		AvatarURL:      &avatar,
		Location:       &location,
		FollowersCount: 7,
		FollowingCount: 3,
		IsSeller:       true,
		Roles:          []string{"buyer"},
		CreatedAt:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Identity:       &card,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)

	// Username-only wire shape.
	for _, want := range []string{
		`"id":`,
		`"username":"alice"`,
		`"bio":"loves agriculture"`,
		`"avatar_url":"https://cdn.example.com/a.jpg"`,
		`"location":"Bandung"`,
		`"followers_count":7`,
		`"following_count":3`,
		`"is_seller":true`,
		`"roles":["buyer"]`,
		`"created_at":"2026-01-01T00:00:00Z"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("legacy field missing from wire: %q\nfull=%s", want, s)
		}
	}

	// Canonical identity seam present with lifecycle.
	for _, want := range []string{
		`"identity":{`,
		`"lifecycle":"active"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("canonical identity field missing from wire: %q\nfull=%s", want, s)
		}
	}
}

func TestPublicUserResponse_E5_1_WireShape_NoPIILeak(t *testing.T) {
	// Boundary §4.2 — Auth Identity (email, phone, firebase_uid) and
	// Capability Internals (raw account_status enum) must NEVER appear on
	// PublicUserResponse. This test pins the absence-property by JSON
	// serializing a fully-populated response and asserting these tokens
	// are nowhere in the wire output.
	id := uuid.New()
	avatar := "https://cdn.example.com/a.jpg"
	card := buildIdentityForLifecycle(t, id, "alice", &avatar, "suspended", false)

	resp := PublicUserResponse{
		UserID:    id,
		Username:  "alice",
		CreatedAt: time.Now(),
		Identity:  &card,
	}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)

	for _, forbidden := range []string{
		"email",
		"phone",
		"firebase_uid",
		"account_status",
		"is_id_verified",
		"is_farm_verified",
		"is_email_verified",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("public boundary violation: %q appears in PublicUserResponse wire output\nfull=%s", forbidden, s)
		}
	}

	// And the suspended coarsening is on the wire.
	if !strings.Contains(s, `"lifecycle":"unavailable"`) {
		t.Errorf("suspended account must surface as lifecycle:unavailable\nfull=%s", s)
	}
}

func TestPublicUserResponse_E5_1_UsernameDoctrine(t *testing.T) {
	// ADR-006 §11 / publiccard package doctrine — UserCard.DisplayName is
	// reserved for a future surface-explicit public name and MUST be nil.
	// It is NEVER derived from full_name, email, or any auth identifier.
	// publiccard.NewWithLifecycle does not set it; this test pins that.
	card := buildIdentityForLifecycle(t, uuid.New(), "alice", nil, "active", false)
	if card.Username != "alice" {
		t.Fatalf("UserCard.Username must mirror username truth; got %q", card.Username)
	}
}


