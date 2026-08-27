package application

import (
	"context"

	"github.com/google/uuid"
	sellerEntity "github.com/labuda/backend/internal/commerce/seller/entity"
	sellerRepo "github.com/labuda/backend/internal/commerce/seller/repository"
	subscriptionEntity "github.com/labuda/backend/internal/commerce/subscription/entity"
	subscriptionRepo "github.com/labuda/backend/internal/commerce/subscription/repository"
	"github.com/labuda/backend/pkg/db"
)

// SubscriptionService handles seller subscription operations.
//
// This service integrates with Seller Domain to manage seller eligibility:
// - On subscription activation: ensures seller_profile exists
//
// Eligibility is derived from subscription status, not from seller_profile status field.
type SubscriptionService struct {
	db         Transactor
	subRepo    subscriptionRepo.SellerSubscriptionRepository
	sellerRepo sellerRepo.SellerRepository
}

// Transactor represents the ability to execute functions within transactions.
type Transactor interface {
	WithTx(ctx context.Context, fn func(tx db.Tx) error) error
}

// NewSubscriptionService creates a new SubscriptionService.
func NewSubscriptionService(
	db Transactor,
	subRepo subscriptionRepo.SellerSubscriptionRepository,
	sellerRepo sellerRepo.SellerRepository,
) *SubscriptionService {
	return &SubscriptionService{
		db:         db,
		subRepo:    subRepo,
		sellerRepo: sellerRepo,
	}
}

// DeactivateSellerProfile is a no-op retained for backward compatibility.
//
// Seller profile deactivation is no longer needed because seller capability
// is now derived from seller_subscriptions table only. The seller_profiles
// table no longer has a status field.
func (s *SubscriptionService) DeactivateSellerProfile(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) error {
	// No-op: seller capability is derived from seller_subscriptions table only
	return nil
}

// GetActiveSubscription retrieves the active subscription for a user.
func (s *SubscriptionService) GetActiveSubscription(
	ctx context.Context,
	userID uuid.UUID,
) (*subscriptionEntity.SellerSubscription, error) {
	var subscription *subscriptionEntity.SellerSubscription
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		subscription, err = s.subRepo.GetActiveByUserID(ctx, tx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return subscription, nil
}

// GetSellerProfile retrieves the seller profile for a user.
func (s *SubscriptionService) GetSellerProfile(
	ctx context.Context,
	userID uuid.UUID,
) (*sellerEntity.SellerProfile, error) {
	var profile *sellerEntity.SellerProfile
	err := s.db.WithTx(ctx, func(tx db.Tx) error {
		var err error
		profile, err = s.sellerRepo.GetByUserID(ctx, tx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	return profile, nil
}

// HasActiveSellerProfile checks if a seller has an active subscription.
// Returns true if user has an active subscription.
func (s *SubscriptionService) HasActiveSellerProfile(
	ctx context.Context,
	userID uuid.UUID,
) (bool, error) {
	subscription, err := s.GetActiveSubscription(ctx, userID)
	if err != nil {
		return false, err
	}

	return subscription != nil, nil
}
