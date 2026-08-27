package http

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	"github.com/labuda/backend/internal/governance/viewercontext"
	"github.com/labuda/backend/internal/pkg/publiccard"
	capabilityctx "github.com/labuda/backend/internal/platform/capability"
	capabilityentity "github.com/labuda/backend/internal/platform/capability/entity"
)

// D14 — bid wire-shape forensic tests.
//
// These tests pin the post-D14 public response contract: idempotency_key
// is gone, the flat bidder_username scalar is gone, and the bidder is a
// nested publiccard.UserCard carrying the coarsened lifecycle string.
//
// The pre-D14 leaks (idempotency_key + bidder_username) MUST NOT
// reappear; the tests below fail if either field surfaces in either the
// write-path (bidToResponse) or the read-path (bidToResponseWithBidderCard)
// JSON.

func newTestBid(t *testing.T) *entity.AuctionBid {
	t.Helper()
	return &entity.AuctionBid{
		ID:             uuid.New(),
		AuctionID:      uuid.New(),
		BidderID:       uuid.New(),
		Amount:         100_000,
		IdempotencyKey: "client-supplied-secret-token",
		CreatedAt:      time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC),
	}
}

func auctionSellerActor() *capabilityentity.Actor {
	status := string(capabilityentity.SellerStatusActive)
	return &capabilityentity.Actor{
		Role:          "user",
		AccountStatus: "active",
		EmailVerified: true,
		SellerStatus:  &status,
	}
}

// bidWireSnapshot encodes a response builder's output to JSON and
// decodes it back to a generic map — what mobile actually sees on the
// wire.
func bidWireSnapshot(t *testing.T, resp map[string]interface{}) map[string]interface{} {
	t.Helper()
	buf, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf, &out); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return out
}

func TestBidToResponse_RemovesIdempotencyKey_PlaceBidPath(t *testing.T) {
	bid := newTestBid(t)
	wire := bidWireSnapshot(t, bidToResponse(bid))

	if _, present := wire["idempotency_key"]; present {
		t.Fatalf("D14 regression: idempotency_key MUST NOT appear on the public bid wire; got %v", wire["idempotency_key"])
	}
}

func TestBidToResponse_RemovesFlatBidderUsername_PlaceBidPath(t *testing.T) {
	bid := newTestBid(t)
	wire := bidWireSnapshot(t, bidToResponse(bid))

	if _, present := wire["bidder_username"]; present {
		t.Fatalf("D14 regression: flat bidder_username scalar MUST NOT appear; got %v", wire["bidder_username"])
	}
}

func TestBidToResponse_EmitsAnonymousBidderCard_PlaceBidPath(t *testing.T) {
	bid := newTestBid(t)
	wire := bidWireSnapshot(t, bidToResponse(bid))

	bidder, ok := wire["bidder"].(map[string]interface{})
	if !ok {
		t.Fatalf("D14: write path MUST emit nested bidder card; got %T", wire["bidder"])
	}
	if bidder["id"] != bid.BidderID.String() {
		t.Fatalf("bidder.id mismatch: want %s, got %v", bid.BidderID, bidder["id"])
	}
	// Anonymous-safe username fallback (publiccard.AnonymousUsername).
	if got, _ := bidder["username"].(string); got == "" {
		t.Fatalf("bidder.username should fall back to anonymous-safe value, got empty string")
	}
	// Lifecycle is nil in the write-path fallback (caller hasn't
	// hydrated). The JSON form should be `null`, not "active".
	if lc, present := bidder["lifecycle"]; present && lc != nil {
		t.Fatalf("write-path bidder.lifecycle should be null, got %v", lc)
	}
}

func TestBidToResponseWithBidderCard_EmitsLifecycle(t *testing.T) {
	bid := newTestBid(t)
	avatar := "https://example.test/avatar.png"
	card := publiccard.NewWithLifecycle(bid.BidderID, "alice", &avatar, "active")
	wire := bidWireSnapshot(t, bidToResponseWithBidderCard(bid, card))

	bidder, ok := wire["bidder"].(map[string]interface{})
	if !ok {
		t.Fatalf("nested bidder card missing; got %T", wire["bidder"])
	}
	if got := bidder["username"]; got != "alice" {
		t.Fatalf("bidder.username = %v, want alice", got)
	}
	if got := bidder["avatar_url"]; got != avatar {
		t.Fatalf("bidder.avatar_url = %v, want %s", got, avatar)
	}
	if got := bidder["lifecycle"]; got != "active" {
		t.Fatalf("bidder.lifecycle = %v, want active", got)
	}
	if _, present := wire["idempotency_key"]; present {
		t.Fatalf("D14 regression: idempotency_key reappeared on read path")
	}
	if _, present := wire["bidder_username"]; present {
		t.Fatalf("D14 regression: flat bidder_username reappeared on read path")
	}
}

func TestBidToResponseWithBidderCard_DegradedLifecycle(t *testing.T) {
	cases := []struct {
		name      string
		accountSt string
		deleted   bool
		want      string
	}{
		{"suspended", "suspended", false, "unavailable"},
		{"banned", "banned", false, "unavailable"},
		{"deleted_enum", "deleted", false, "removed"},
		{"deleted_at_present", "active", true, "removed"},
		{"active", "active", false, "active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bid := newTestBid(t)
			lc := string(viewercontext.CoarsenLifecycle(tc.accountSt, tc.deleted))
			if lc != tc.want {
				t.Fatalf("CoarsenLifecycle(%q, %v) = %q, want %q", tc.accountSt, tc.deleted, lc, tc.want)
			}
			card := publiccard.NewWithLifecycle(bid.BidderID, "alice", nil, lc)
			wire := bidWireSnapshot(t, bidToResponseWithBidderCard(bid, card))
			bidder := wire["bidder"].(map[string]interface{})
			if got := bidder["lifecycle"]; got != tc.want {
				t.Fatalf("bidder.lifecycle = %v, want %s", got, tc.want)
			}
		})
	}
}

func TestConstructAuctionBidsViewerContext_AnonymousOnMissingUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions/xxx/bids", nil)

	vc := constructAuctionBidsViewerContext(c, nil)
	if vc == nil {
		t.Fatalf("expected non-nil ViewerContext")
	}
	if !vc.IsAnonymous() {
		t.Fatalf("expected AnonymousViewer when no userID is in gin context")
	}
}

func TestConstructAuctionBidsViewerContext_AnonymousOnNilUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions/xxx/bids", nil)
	c.Set("user_id", uuid.Nil)

	vc := constructAuctionBidsViewerContext(c, nil)
	if !vc.IsAnonymous() {
		t.Fatalf("nil UUID userID must yield AnonymousViewer per F6")
	}
}

func TestConstructAuctionBidsViewerContext_AuthenticatedOnValidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/auctions/xxx/bids", nil)
	uid := uuid.New()
	c.Set("user_id", uid)
	c.Set("firebase_uid", "fb-test-uid")
	c.Set("is_admin", false)
	c.Set("is_moderator", false)
	c.Request = c.Request.WithContext(capabilityctx.WithActor(c.Request.Context(), auctionSellerActor()))

	vc := constructAuctionBidsViewerContext(c, nil)
	if vc.IsAnonymous() {
		t.Fatalf("expected AuthenticatedViewer when valid UUID is in gin context")
	}
	if vc.Identity().CanonicalUserID != uid {
		t.Fatalf("CanonicalUserID = %v, want %v", vc.Identity().CanonicalUserID, uid)
	}
	if vc.Identity().FirebaseUID != "fb-test-uid" {
		t.Fatalf("FirebaseUID propagation broke")
	}
}

func TestHydrateBidsBlockedSet_AnonymousReturnsEmpty(t *testing.T) {
	bids := []*entity.AuctionBid{
		{BidderID: uuid.New()},
		{BidderID: uuid.New()},
	}
	anon := viewercontext.NewAnonymous(
		viewercontext.SurfacePublicDiscovery,
		viewercontext.RequestOriginREST,
	)
	got := hydrateBidsBlockedSet(context.Background(), nil, anon, bids)
	if len(got) != 0 {
		t.Fatalf("anonymous viewer must yield empty blocked set; got %v", got)
	}
}

func TestHydrateBidsBlockedSet_NilViewerReturnsEmpty(t *testing.T) {
	bids := []*entity.AuctionBid{{BidderID: uuid.New()}}
	got := hydrateBidsBlockedSet(context.Background(), nil, nil, bids)
	if len(got) != 0 {
		t.Fatalf("nil ViewerContext must yield empty blocked set; got %v", got)
	}
}

func TestHydrateBidderLifecycleCards_EmptyInput(t *testing.T) {
	got := hydrateBidderLifecycleCards(context.Background(), nil, nil)
	if len(got) != 0 {
		t.Fatalf("empty bids must yield empty map; got %v", got)
	}
	got = hydrateBidderLifecycleCards(context.Background(), nil, []*entity.AuctionBid{})
	if len(got) != 0 {
		t.Fatalf("zero-len bids must yield empty map; got %v", got)
	}
}

// TestPublicCardBoundary_NoRawAccountStatusOnWire is a static-wire-
// boundary regression. UserCard exposes only the public-safe shape
// {id, username, display_name, avatar_url, lifecycle}; the test fails
// if a future change adds a raw account_status / email / deleted_at
// JSON tag.
func TestPublicCardBoundary_NoRawAccountStatusOnWire(t *testing.T) {
	card := publiccard.NewWithLifecycle(uuid.New(), "alice", nil, "unavailable")
	buf, err := json.Marshal(card)
	if err != nil {
		t.Fatal(err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(buf, &generic); err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"account_status", "email", "deleted_at", "firebase_uid", "is_admin", "is_id_verified"}
	for _, f := range forbidden {
		if _, leaked := generic[f]; leaked {
			t.Fatalf("PublicCard boundary breach: %q present on UserCard wire", f)
		}
	}
}
