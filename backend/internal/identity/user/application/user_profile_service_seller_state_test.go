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

// newSellerStateFixtureProfileService builds a user-profile service for a user
// that HAS a seller profile (onboarding completed) whose subscription read
// returns `sub`. `sub == nil` means the user has no seller_subscriptions row at
// all — the state of a freshly registered seller whose payment has not settled.
func newSellerStateFixtureProfileService(
	sub *subscriptionEntity.SellerSubscription,
) (*UserProfileService, uuid.UUID) {
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	userID := uuid.New()

	sellerProfile := &sellerEntity.SellerProfile{
		ID:        uuid.New(),
		UserID:    userID,
		StoreName: "Fresh Store",
		Tier:      sellerEntity.TierBasic,
		CreatedAt: now,
		UpdatedAt: now,
	}

	svc := NewUserProfileService(
		&profileMediaUserRepo{
			myProfile: &userEntity.MyProfileResponse{
				User: &userEntity.User{
					ID:            userID,
					Email:         strPtr("fresheller@example.com"),
					AccountStatus: "active",
					CreatedAt:     now,
					UpdatedAt:     now,
				},
				Profile: &userEntity.UserProfile{
					ID:       uuid.New(),
					UserID:   userID,
					Username: strPtr("fresheller"),
				},
				Roles: []string{"user"},
			},
		},
		&profileMediaSellerRepo{profile: sellerProfile},
		&profileMediaSubRepo{sub: sub},
		nil,
		&fakeFirebase{},
		&fakeDB{},
	)

	return svc, userID
}

// TestGetSellerState_FreshSellerReportsNoneNotExpired is the RF-02 contract at
// the authority level: a seller who finished onboarding but has no subscription
// row yet must be reported with subscription status "none" — never "expired" —
// and without market authority. Collapsing "no row" into an absent value is what
// made freshly registered sellers get renewal prompts.
func TestGetSellerState_FreshSellerReportsNoneNotExpired(t *testing.T) {
	svc, userID := newSellerStateFixtureProfileService(nil)

	state, err := svc.getSellerState(context.Background(), &fakeTx{}, userID)
	if err != nil {
		t.Fatalf("getSellerState returned error: %v", err)
	}

	if !state.HasProfile {
		t.Fatal("fresh seller must report HasProfile = true")
	}

	if state.SubscriptionStatus == nil {
		t.Fatal("fresh seller must report a subscription status (got nil/omitted)")
	}
	if got := *state.SubscriptionStatus; got != "none" {
		t.Fatalf("fresh seller subscription status = %q, want %q", got, "none")
	}

	// Capability stays false: no active interval means no market authority.
	if state.HasActiveSubscription {
		t.Fatal("fresh seller without a subscription row must not report an active subscription")
	}
	if state.HasMarketAuthority {
		t.Fatal("fresh seller without a subscription row must not have market authority")
	}

	// Negative proof: the reported state must never be the ENDED state.
	if *state.SubscriptionStatus == "expired" {
		t.Fatal("fresh seller must never be reported as expired")
	}
}

// TestGetSellerState_SubscriptionAxesStaySeparate pins the two axes apart:
// market authority comes from an ACTIVE interval, while the reported
// subscription status is the latest row's status regardless of window.
func TestGetSellerState_SubscriptionAxesStaySeparate(t *testing.T) {
	tests := []struct {
		name          string
		sub           *subscriptionEntity.SellerSubscription
		wantStatus    string
		wantAuthority bool
	}{
		{
			name:          "active interval grants authority",
			sub:           &subscriptionEntity.SellerSubscription{Status: subscriptionEntity.StatusActive},
			wantStatus:    "active",
			wantAuthority: true,
		},
		{
			name:          "ended subscription reports expired without authority",
			sub:           &subscriptionEntity.SellerSubscription{Status: subscriptionEntity.StatusExpired},
			wantStatus:    "expired",
			wantAuthority: false,
		},
		{
			name:          "never subscribed reports none without authority",
			sub:           nil,
			wantStatus:    "none",
			wantAuthority: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, userID := newSellerStateFixtureProfileService(tc.sub)

			state, err := svc.getSellerState(context.Background(), &fakeTx{}, userID)
			if err != nil {
				t.Fatalf("getSellerState returned error: %v", err)
			}

			if state.SubscriptionStatus == nil {
				t.Fatal("subscription status must be reported, got nil/omitted")
			}
			if got := *state.SubscriptionStatus; got != tc.wantStatus {
				t.Fatalf("subscription status = %q, want %q", got, tc.wantStatus)
			}
			if state.HasMarketAuthority != tc.wantAuthority {
				t.Fatalf("HasMarketAuthority = %v, want %v", state.HasMarketAuthority, tc.wantAuthority)
			}
		})
	}
}
