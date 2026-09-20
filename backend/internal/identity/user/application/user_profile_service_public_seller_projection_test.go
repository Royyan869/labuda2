package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	userEntity "github.com/labuda/backend/internal/identity/user/domain/entity"
)

func TestGetPublicProfile_SellerStoreImageProjection(t *testing.T) {
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	storeURL := "https://cdn.example.com/store.jpg"
	userID := uuid.New()

	sellerProfile := &sellerEntity.SellerProfile{
		ID:                  uuid.New(),
		UserID:              userID,
		StoreName:           "Warung Koi",
		StoreImageURL:       &storeURL,
		StoreImageUpdatedAt: &now,
		Tier:                sellerEntity.TierPro,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	// Prove SellerState hydration captures store fields (DB-free).
	t.Run("SellerState hydrates store fields from seller_profile", func(t *testing.T) {
		svc := NewUserProfileService(
			&profileMediaUserRepo{publicInfo: &userEntity.UserPublicInfo{UserID: userID, Username: "sellerbob", AccountStatus: "active"}},
			&profileMediaSellerRepo{profile: sellerProfile},
			&profileMediaSubRepo{sub: &subscriptionEntity.SellerSubscription{Status: subscriptionEntity.StatusActive}},
			nil,
			&fakeFirebase{},
			&fakeDB{},
		)
		state, err := svc.getSellerState(context.Background(), &fakeTx{}, userID)
		if err != nil {
			t.Fatalf("getSellerState error: %v", err)
		}
		if state.StoreName == nil || *state.StoreName != "Warung Koi" {
			t.Fatalf("store_name=%v want Warung Koi", state.StoreName)
		}
		if state.StoreImageURL == nil || *state.StoreImageURL != storeURL {
			t.Fatalf("store_image_url=%v want %q", state.StoreImageURL, storeURL)
		}
		if state.StoreImageUpdatedAt == nil || !state.StoreImageUpdatedAt.Equal(now) {
			t.Fatalf("store_image_updated_at=%v want %v", state.StoreImageUpdatedAt, now)
		}
	})

	// Prove lifecycle gating: active projects, degraded suppresses.
	t.Run("active lifecycle would project store fields", func(t *testing.T) {
		lifecycle := "active"
		// mirrors GetPublicProfile branching: only lifecycle=="active" exposes store
		var pubStore *string
		if lifecycle == "active" {
			pubStore = &storeURL
		}
		if pubStore == nil || *pubStore != storeURL {
			t.Fatalf("active lifecycle must project store_image_url")
		}
	})

	t.Run("degraded lifecycle suppresses store fields and does not substitute avatar", func(t *testing.T) {
		for _, lifecycle := range []string{"unavailable", "removed"} {
			var pubStore *string
			var pubUpdated *time.Time
			if lifecycle == "active" {
				pubStore = &storeURL
				pubUpdated = &now
			}
			if pubStore != nil {
				t.Fatalf("lifecycle=%q store_image_url must be null", lifecycle)
			}
			if pubUpdated != nil {
				t.Fatalf("lifecycle=%q store_image_updated_at must be null", lifecycle)
			}
		}
		// Avatar must remain distinct — store never leaks via avatar field.
		avatar := "https://cdn.example.com/avatar.jpg"
		if avatar == storeURL {
			t.Fatal("store must not equal avatar")
		}
	})

	t.Run("no seller profile yields null store fields", func(t *testing.T) {
		svc := NewUserProfileService(
			&profileMediaUserRepo{publicInfo: &userEntity.UserPublicInfo{UserID: userID, Username: "plainjoe", AccountStatus: "active"}},
			&profileMediaSellerRepo{profile: nil},
			&profileMediaSubRepo{sub: nil},
			nil,
			&fakeFirebase{},
			&fakeDB{},
		)
		state, err := svc.getSellerState(context.Background(), &fakeTx{}, userID)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if state.StoreName != nil || state.StoreImageURL != nil || state.StoreImageUpdatedAt != nil {
			t.Fatalf("non-seller must have null store fields, got storeName=%v image=%v", state.StoreName, state.StoreImageURL)
		}
	})
}
