package shared

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pkg/mediaref"
	"github.com/labuda/backend/internal/pkg/publiccard"
)

// CANONICAL ENVELOPE RATCHET (scope #3).
//
// These tests lock the converged wire shape. Any resurrection of a former
// per-module divergence (payload can_interact, price scalar, image_url, flat
// seller, omitted tombstone resource_id, commerce_actions capability matrix,
// capabilities inside payloads) fails here first.

func liveFPSFixture(t *testing.T) ResourceProjection {
	t.Helper()
	caps := ProjectionViewerCapabilities{CanView: true, CanInteract: true}
	payload := ForSaleLivePayload{
		Title:              "Tomat Cherry",
		Media:              []mediaref.MediaRef{{URL: "https://cdn/x.jpg", Kind: nil, Width: nil, Height: nil}},
		Price:              LivePrice{Amount: 15000, Currency: "IDR"},
		Status:             "active",
		QuantityAvailable:  3,
		NegotiationEnabled: true,
		Seller:             publiccard.SellerCard{},
	}
	p, err := NewLiveResourceProjection(ProjectionResourceTypeForSale, uuid.New(), payload, caps)
	if err != nil {
		t.Fatalf("build live fps: %v", err)
	}
	return p
}

func marshalMap(t *testing.T, p ResourceProjection) map[string]any {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func requireKeys(t *testing.T, m map[string]any, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			t.Errorf("canonical envelope missing required key %q (got %v)", k, keySet(m))
		}
	}
}

// requireExactKeys asserts the exact wire key set (order-insensitive). This is
// the payload-contract lock the former per-module payload tests owned: a new
// or resurrected key fails here before it reaches a client.
func requireExactKeys(t *testing.T, m map[string]any, want ...string) {
	t.Helper()
	got := keySet(m)
	sort.Strings(got)
	sortedWant := append([]string(nil), want...)
	sort.Strings(sortedWant)
	if !reflect.DeepEqual(got, sortedWant) {
		t.Errorf("wire keys = %v, want %v", got, sortedWant)
	}
}

func forbidKeys(t *testing.T, m map[string]any, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := m[k]; ok {
			t.Errorf("canonical envelope carries forbidden key %q (resurrected divergence)", k)
		}
	}
}

func keySet(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestCanonicalEnvelope_LiveFPSWireShape(t *testing.T) {
	m := marshalMap(t, liveFPSFixture(t))

	requireKeys(t, m, "state", "resource_type", "resource_id", "canonical_url",
		"viewer_capabilities", "for_sale")

	// Divergences that must never come back:
	forbidKeys(t, m, "commerce_actions", "can_interact", "image_url", "price_scalar_marker")

	// price must be the money object {amount, currency}, never a scalar.
	price, ok := m["for_sale"].(map[string]any)
	if !ok {
		t.Fatalf("for_sale payload missing: %v", keySet(m))
	}
	priceVal, ok := price["price"].(map[string]any)
	if !ok {
		t.Fatalf("for_sale.price must be {amount,currency} object, got %T", price["price"])
	}
	if _, ok := priceVal["amount"]; !ok {
		t.Errorf("for_sale.price missing amount")
	}
	if _, ok := priceVal["currency"]; !ok {
		t.Errorf("for_sale.price missing currency")
	}

	// The canonical PRODUCT-LEVEL negotiation attribute is present on the payload.
	if _, ok := price["negotiation_enabled"]; !ok {
		t.Errorf("for_sale.negotiation_enabled missing (product negotiation attribute)")
	}

	// payload-level can_interact is dead — viewer truth lives on the envelope.
	if _, ok := price["can_interact"]; ok {
		t.Errorf("payload-level can_interact resurrected (must live in viewer_capabilities)")
	}
	// flat seller (id/store_name/username top-level) is dead — SellerCard shape.
	seller, ok := price["seller"].(map[string]any)
	if !ok {
		t.Fatalf("for_sale.seller missing: %v", keySet(price))
	}
	if _, ok := seller["id"]; ok {
		t.Errorf("flat ForSaleLiveSeller resurrected (seller must be publiccard.SellerCard)")
	}
	if _, ok := seller["user"]; !ok {
		t.Errorf("for_sale.seller must be SellerCard with user field")
	}
	// media[] replaces singular image_url.
	if _, ok := price["media"]; !ok {
		t.Errorf("for_sale.media missing (media[] replaced image_url)")
	}
	if _, ok := price["image_url"]; ok {
		t.Errorf("image_url resurrected (media[] is canonical)")
	}
	// canonical_url present and correct.
	if got, _ := CanonicalResourceURL(ProjectionResourceTypeForSale, mustUUID(t, m["resource_id"])); got != m["canonical_url"] {
		t.Errorf("canonical_url = %v, want %v", m["canonical_url"], got)
	}
}

func TestCanonicalEnvelope_LiveProfileShape(t *testing.T) {
	p, err := NewLiveResourceProjection(
		ProjectionResourceTypeProfile,
		uuid.New(),
		ProfileLivePayload{Username: "ani", Lifecycle: "active"},
		ProjectionViewerCapabilities{CanView: true},
	)
	if err != nil {
		t.Fatalf("build live profile: %v", err)
	}
	m := marshalMap(t, p)
	// The commerce_actions capability matrix is purged from the contract.
	forbidKeys(t, m, "commerce_actions", "for_sale", "auction", "content")
	requireKeys(t, m, "profile", "viewer_capabilities", "canonical_url")
}

func TestCanonicalEnvelope_TombstoneWireShape(t *testing.T) {
	id := uuid.New()
	p, err := NewTombstoneResourceProjection(ProjectionResourceTypeForSale, id)
	if err != nil {
		t.Fatalf("build tombstone: %v", err)
	}
	m := marshalMap(t, p)

	// resource_id survives death (canonical contract; chat's omission died).
	requireKeys(t, m, "state", "resource_type", "resource_id", "viewer_capabilities")
	forbidKeys(t, m, "canonical_url", "commerce_actions", "for_sale", "auction",
		"profile", "content")

	if m["resource_id"] != id.String() {
		t.Errorf("tombstone resource_id = %v, want %v", m["resource_id"], id)
	}
	vc, ok := m["viewer_capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("tombstone viewer_capabilities missing")
	}
	if vc["blocked_by_tombstone"] != true || vc["can_view"] != false || vc["can_interact"] != false {
		t.Errorf("tombstone capabilities wrong: %v", vc)
	}
}

func TestCanonicalEnvelope_ValidationRejectsResurrectedShapes(t *testing.T) {
	id := uuid.New()

	cases := []struct {
		name    string
		build   func() (ResourceProjection, error)
		wantErr string
	}{
		{
			name: "LIVE profile with can_interact",
			build: func() (ResourceProjection, error) {
				return NewLiveResourceProjection(ProjectionResourceTypeProfile, id,
					ProfileLivePayload{Username: "u", Lifecycle: "active"},
					ProjectionViewerCapabilities{CanView: true, CanInteract: true})
			},
			wantErr: "can_interact=false",
		},
		{
			name: "LIVE content with can_interact",
			build: func() (ResourceProjection, error) {
				return NewLiveResourceProjection(ProjectionResourceTypeContent, id,
					ContentLivePayload{},
					ProjectionViewerCapabilities{CanView: true, CanInteract: true})
			},
			wantErr: "can_interact=false",
		},
		{
			name: "TOMBSTONE with payload",
			build: func() (ResourceProjection, error) {
				p, err := NewTombstoneResourceProjection(ProjectionResourceTypeContent, id)
				if err != nil {
					return p, err
				}
				p.Content = &ContentLivePayload{}
				return p, p.Validate()
			},
			wantErr: "forbids payloads",
		},
		{
			name: "TOMBSTONE with unblocked capabilities",
			build: func() (ResourceProjection, error) {
				p, err := NewTombstoneResourceProjection(ProjectionResourceTypeContent, id)
				if err != nil {
					return p, err
				}
				p.ViewerCapabilities = ProjectionViewerCapabilities{}
				return p, p.Validate()
			},
			wantErr: "blocked_by_tombstone=true",
		},
		{
			name: "LIVE without resource_id",
			build: func() (ResourceProjection, error) {
				return NewLiveResourceProjection(ProjectionResourceTypeContent, uuid.Nil,
					ContentLivePayload{}, ProjectionViewerCapabilities{CanView: true})
			},
			wantErr: "resource id",
		},
		{
			name: "invalid resource type",
			build: func() (ResourceProjection, error) {
				return NewLiveResourceProjection(ProjectionResourceType("forSale"), id,
					ContentLivePayload{}, ProjectionViewerCapabilities{CanView: true})
			},
			wantErr: "invalid resource type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.build()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestCanonicalEnvelope_MarshalValidatesBeforeEmitting(t *testing.T) {
	// A struct mutated after construction must not escape unvalidated.
	p, err := NewTombstoneResourceProjection(ProjectionResourceTypeAuction, uuid.New())
	if err != nil {
		t.Fatalf("build tombstone: %v", err)
	}
	p.ViewerCapabilities.CanView = true // corrupt
	if _, err := json.Marshal(p); err == nil {
		t.Errorf("marshal of corrupted projection must fail")
	}
}

// TestCanonicalEnvelope_PayloadJSONContracts locks the exact payload wire key
// sets for all four resource types (the contract formerly split across the chat
// module's private payload tests + the content module's payload tests).
func TestCanonicalEnvelope_PayloadJSONContracts(t *testing.T) {
	avatar := "https://cdn/a.png"
	farm := "Tomat Farm"

	t.Run("profile payload keys", func(t *testing.T) {
		p, err := NewLiveResourceProjection(ProjectionResourceTypeProfile, uuid.New(),
			ProfileLivePayload{Username: "ani", AvatarURL: &avatar, StoreName: &farm,
				IsSeller: true, Lifecycle: "active"},
			ProjectionViewerCapabilities{CanView: true})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		m := marshalMap(t, p)
		prof, _ := m["profile"].(map[string]any)
		requireExactKeys(t, prof, "username", "avatar_url", "store_name", "is_seller", "lifecycle")
		if _, ok := prof["can_interact"]; ok {
			t.Errorf("payload-level can_interact resurrected in profile payload")
		}
	})

	t.Run("content payload keys", func(t *testing.T) {
		caption := "halo"
		p, err := NewLiveResourceProjection(ProjectionResourceTypeContent, uuid.New(),
			ContentLivePayload{Caption: &caption,
				Media:     []mediaref.MediaRef{{URL: "https://cdn/c.jpg"}},
				Lifecycle: "active", CreatedAt: "2026-09-01T00:00:00Z",
				Author: publiccard.NewWithLifecycle(uuid.New(), "penulis", &avatar, "active")},
			ProjectionViewerCapabilities{CanView: true})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		m := marshalMap(t, p)
		content, _ := m["content"].(map[string]any)
		requireExactKeys(t, content, "caption", "media", "lifecycle", "created_at", "author")
		author, _ := content["author"].(map[string]any)
		requireExactKeys(t, author, "id", "username", "avatar_url", "lifecycle")
	})

	t.Run("for_sale payload keys", func(t *testing.T) {
		p := liveFPSFixture(t)
		// liveFPSFixture builds a minimal payload; enrich to full wire shape.
		m := marshalMap(t, p)
		fps, _ := m["for_sale"].(map[string]any)
		requireExactKeys(t, fps, "title", "media", "price", "status",
			"quantity_available", "negotiation_enabled", "seller")
	})

	t.Run("auction payload keys", func(t *testing.T) {
		bid := int64(9000)
		buyNow := int64(15000)
		p, err := NewLiveResourceProjection(ProjectionResourceTypeAuction, uuid.New(),
			AuctionLivePayload{Title: "Lelang Koi",
				Media:        []mediaref.MediaRef{{URL: "https://cdn/k.jpg"}},
				ThumbnailURL: &avatar, CurrentBid: &bid, BuyNowPrice: &buyNow,
				EndAt: "2026-09-10T00:00:00Z", Lifecycle: "active",
				Seller: publiccard.NewSellerCardWithUserLifecycle(uuid.New(), "petani", &avatar, farm, "active")},
			ProjectionViewerCapabilities{CanView: true, CanInteract: true})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		m := marshalMap(t, p)
		auction, _ := m["auction"].(map[string]any)
		requireExactKeys(t, auction, "title", "media", "thumbnail_url", "current_bid",
			"buy_now_price", "end_at", "lifecycle", "seller")
		seller, _ := auction["seller"].(map[string]any)
		requireExactKeys(t, seller, "user", "farm_name", "avatar_url", "lifecycle")
		if _, ok := seller["id"]; ok {
			t.Errorf("flat seller id resurrected (seller must be nested SellerCard)")
		}
	})
}

func mustUUID(t *testing.T, v any) uuid.UUID {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("resource_id is not a string: %T", v)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse resource_id: %v", err)
	}
	return id
}
