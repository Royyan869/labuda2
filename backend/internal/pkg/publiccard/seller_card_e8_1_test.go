package publiccard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/governance/viewercontext"
)

// E8.1 — Unit tests for NewSellerCardWithUserLifecycle.
//
// Scope: the helper that activates the USER-IDENTITY axis (axis 1) on
// SellerCard.User.Lifecycle while preserving the axis boundary by leaving
// the top-level SellerCard.Lifecycle nil. The coarsening pipeline is
// exercised end-to-end (raw account_status + deleted_at -> CoarsenLifecycle
// -> NewWithLifecycle -> nested user card -> SellerCard wire shape).

func buildSellerCardForLifecycle(t *testing.T, username string, avatarURL *string, rawStatus string, isDeleted bool) SellerCard {
	t.Helper()
	lifecycle := string(viewercontext.CoarsenLifecycle(rawStatus, isDeleted))
	return NewSellerCardWithUserLifecycle(
		uuid.New(), username, avatarURL,
		"Acme Farm",
		lifecycle,
	)
}

func TestNewSellerCardWithUserLifecycle_ActiveSeller(t *testing.T) {
	avatar := "https://cdn.example.com/seller.jpg"
	card := buildSellerCardForLifecycle(t, "alice_seller", &avatar, "active", false)

	if card.User.Lifecycle == nil {
		t.Fatal("active seller must coarsen to non-nil User.Lifecycle")
	}
	if got := *card.User.Lifecycle; got != "active" {
		t.Fatalf("active seller user lifecycle = %q; want \"active\"", got)
	}
	// AXIS BOUNDARY: top-level SellerCard.Lifecycle must stay nil.
	if card.Lifecycle != nil {
		t.Fatalf("top-level SellerCard.Lifecycle must remain nil; got %v", *card.Lifecycle)
	}
}

func TestNewSellerCardWithUserLifecycle_SuspendedSeller(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"suspended", "unavailable"},
		{"banned", "unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			card := buildSellerCardForLifecycle(t, "bob_seller", nil, tc.raw, false)
			if card.User.Lifecycle == nil {
				t.Fatalf("%s seller must coarsen to non-nil User.Lifecycle", tc.raw)
			}
			if got := *card.User.Lifecycle; got != tc.want {
				t.Fatalf("%s user lifecycle = %q; want %q", tc.raw, got, tc.want)
			}
			if card.Lifecycle != nil {
				t.Fatalf("top-level SellerCard.Lifecycle must remain nil; got %v", *card.Lifecycle)
			}
		})
	}
}

func TestNewSellerCardWithUserLifecycle_RemovedSeller(t *testing.T) {
	// Slot-persistence: sellerdisplay.FetchMany deliberately does NOT filter
	// `WHERE u.deleted_at IS NULL` so a tombstoned seller surfaces as
	// User.Lifecycle="removed". This pins that contract for the three raw
	// inputs that coarsen to "removed".
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
			card := buildSellerCardForLifecycle(t, "ghost_seller", nil, tc.raw, tc.isDeleted)
			if card.User.Lifecycle == nil {
				t.Fatalf("%s must coarsen to non-nil User.Lifecycle", tc.name)
			}
			if got := *card.User.Lifecycle; got != "removed" {
				t.Fatalf("%s user lifecycle = %q; want \"removed\"", tc.name, got)
			}
			if card.Lifecycle != nil {
				t.Fatalf("top-level SellerCard.Lifecycle must remain nil; got %v", *card.Lifecycle)
			}
		})
	}
}

func TestNewSellerCardWithUserLifecycle_EmptyLifecycleRollback(t *testing.T) {
	// Empty lifecycle string (e.g. caller pre-E8.1 or hydration skipped)
	// must leave the User.Lifecycle slot nil — rollback-safe; matches
	// publiccard.NewSellerCard wire shape exactly.
	card := NewSellerCardWithUserLifecycle(
		uuid.New(), "alice", nil,
		"Acme Farm",
		"",
	)
	if card.User.Lifecycle != nil {
		t.Fatalf("empty lifecycle must leave User.Lifecycle nil; got %v", *card.User.Lifecycle)
	}
	if card.Lifecycle != nil {
		t.Fatalf("top-level SellerCard.Lifecycle must remain nil; got %v", *card.Lifecycle)
	}
}

func TestNewSellerCardWithUserLifecycle_UsernameDoctrine(t *testing.T) {
	card := buildSellerCardForLifecycle(t, "alice", nil, "active", false)
	if card.User.Username != "alice" {
		t.Fatalf("UserCard.Username must mirror username truth; got %q", card.User.Username)
	}
}

func TestNewSellerCardWithUserLifecycle_LegacyFieldsPreserved(t *testing.T) {
	// SellerCard fields (FarmName, AvatarURL) must continue to round-trip
	// on the wire alongside the new User.Lifecycle. This is the rollback
	// contract for current mobile consumers that do not yet parse
	// seller.user.lifecycle.
	avatar := "https://cdn.example.com/seller.jpg"
	card := NewSellerCardWithUserLifecycle(
		uuid.New(), "alice", &avatar,
		"Acme Farm",
		"active",
	)

	b, err := json.Marshal(card)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)

	for _, want := range []string{
		`"user":{`,
		`"username":"alice"`,
		`"avatar_url":"https://cdn.example.com/seller.jpg"`,
		`"farm_name":"Acme Farm"`,
		`"lifecycle":"active"`, // nested user lifecycle
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing field on wire: %q\nfull=%s", want, s)
		}
	}
}

func TestNewSellerCardWithUserLifecycle_TopLevelLifecycleAlwaysNil(t *testing.T) {
	// AXIS BOUNDARY pin: every coarsened user state must leave
	// SellerCard.Lifecycle (top-level) absent on the wire. This is the
	// hard guarantee that E8.1 does NOT activate the seller-trust /
	// capability axis — only the user-identity axis.
	cases := []struct {
		name      string
		raw       string
		isDeleted bool
	}{
		{"active", "active", false},
		{"suspended", "suspended", false},
		{"banned", "banned", false},
		{"deleted_at", "active", true},
		{"account_deleted", "deleted", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := buildSellerCardForLifecycle(t, "alice", nil, tc.raw, tc.isDeleted)

			_, err := json.Marshal(card)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			// Top-level "lifecycle":null must appear (json tag is
			// `json:"lifecycle"` with pointer; nil → null).
			if card.Lifecycle != nil {
				t.Fatalf("[%s] top-level SellerCard.Lifecycle must remain nil; got %v", tc.name, *card.Lifecycle)
			}
		})
	}
}


