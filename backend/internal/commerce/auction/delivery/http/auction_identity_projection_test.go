package http

import (
	"testing"

	"github.com/google/uuid"
)

func TestProjectAuctionUserCard_ActiveCanonical(t *testing.T) {
	id := uuid.New()
	card := projectAuctionUserCard(
		id,
		true,
		"alice",
		"https://example.test/avatar.png",
		"active",
		false,
	)

	if card.ID != id {
		t.Fatalf("card.ID = %v, want %v", card.ID, id)
	}
	if card.Username != "alice" {
		t.Fatalf("card.Username = %q, want alice", card.Username)
	}
	if card.AvatarURL == nil || *card.AvatarURL != "https://example.test/avatar.png" {
		t.Fatalf("card.AvatarURL = %#v, want avatar", card.AvatarURL)
	}
	if card.Lifecycle == nil || *card.Lifecycle != "active" {
		t.Fatalf("card.Lifecycle = %#v, want active", card.Lifecycle)
	}
}

func TestProjectAuctionUserCard_ActiveStoredUserPrefixPreserved(t *testing.T) {
	id := uuid.New()
	card := projectAuctionUserCard(
		id,
		true,
		"user_deadbeef",
		"",
		"active",
		false,
	)

	if card.ID != id {
		t.Fatalf("card.ID = %v, want %v", card.ID, id)
	}
	if card.Username != "user_deadbeef" {
		t.Fatalf("card.Username = %q, want user_deadbeef", card.Username)
	}
	if card.AvatarURL != nil {
		t.Fatalf("card.AvatarURL = %#v, want nil", card.AvatarURL)
	}
	if card.Lifecycle == nil || *card.Lifecycle != "active" {
		t.Fatalf("card.Lifecycle = %#v, want active", card.Lifecycle)
	}
}

func TestProjectAuctionUserCard_ActiveBlankUsernameRedacts(t *testing.T) {
	id := uuid.New()
	card := projectAuctionUserCard(id, true, "", "https://example.test/avatar.png", "active", false)

	if card.Username != "" {
		t.Fatalf("card.Username = %q, want empty", card.Username)
	}
	if card.AvatarURL != nil {
		t.Fatalf("card.AvatarURL = %#v, want nil", card.AvatarURL)
	}
	if card.Lifecycle == nil || *card.Lifecycle != "unavailable" {
		t.Fatalf("card.Lifecycle = %#v, want unavailable", card.Lifecycle)
	}
}

func TestProjectAuctionUserCard_NilIDFailsClosed(t *testing.T) {
	card := projectAuctionUserCard(uuid.Nil, true, "user_deadbeef", "https://example.test/avatar.png", "active", false)

	if card.ID != uuid.Nil {
		t.Fatalf("card.ID = %v, want nil", card.ID)
	}
	if card.Username != "" {
		t.Fatalf("card.Username = %q, want empty", card.Username)
	}
	if card.AvatarURL != nil {
		t.Fatalf("card.AvatarURL = %#v, want nil", card.AvatarURL)
	}
	if card.Lifecycle != nil {
		t.Fatalf("card.Lifecycle = %#v, want nil", card.Lifecycle)
	}
}

func TestProjectAuctionUserCard_MissingRowRemoved(t *testing.T) {
	id := uuid.New()
	card := projectAuctionUserCard(id, false, "", "", "", false)

	if card.Username != "" {
		t.Fatalf("card.Username = %q, want empty", card.Username)
	}
	if card.AvatarURL != nil {
		t.Fatalf("card.AvatarURL = %#v, want nil", card.AvatarURL)
	}
	if card.Lifecycle == nil || *card.Lifecycle != "removed" {
		t.Fatalf("card.Lifecycle = %#v, want removed", card.Lifecycle)
	}
}

func TestProjectAuctionSellerCard_UsesNestedAuthority(t *testing.T) {
	id := uuid.New()
	card := projectAuctionSellerCard(auctionIdentityProjectionRow{
		ID:                 id,
		UserFound:          true,
		Username:           "alice",
		AvatarURL:          "https://example.test/avatar.png",
		FarmName:           "Farm One",
		AccountStatus:      "active",
		IsDeleted:          false,
		SubscriptionStatus: "active",
		Tier:               "pro",
	})

	if card.User.ID != id {
		t.Fatalf("seller.user.id = %v, want %v", card.User.ID, id)
	}
	if card.User.Username != "alice" {
		t.Fatalf("seller.user.username = %q, want alice", card.User.Username)
	}
	if card.FarmName == nil || *card.FarmName != "Farm One" {
		t.Fatalf("seller.farm_name = %#v, want Farm One", card.FarmName)
	}
	if card.Lifecycle == nil || *card.Lifecycle != "active" {
		t.Fatalf("seller.lifecycle = %#v, want active", card.Lifecycle)
	}
}
