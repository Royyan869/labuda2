package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/governance/commercegov"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auctionCommerceRestrictionRepo is a minimal commercegov.Repository stub
// for auction service tests.
type auctionCommerceRestrictionRepo struct {
	restricted bool
}

func (f *auctionCommerceRestrictionRepo) InsertViolation(_ context.Context, _ db.Tx, _ *commercegov.Violation) error {
	return nil
}

func (f *auctionCommerceRestrictionRepo) GetRestrictionForUpdate(_ context.Context, _ db.Tx, userID uuid.UUID) (*commercegov.Restriction, error) {
	if f.restricted {
		return &commercegov.Restriction{
			ID:              uuid.New(),
			UserID:          userID,
			ViolationCount:  1,
			RestrictedUntil: time.Now().Add(7 * 24 * time.Hour),
			LastViolationID: uuid.New(),
		}, nil
	}
	return nil, nil
}

func (f *auctionCommerceRestrictionRepo) UpsertRestriction(_ context.Context, _ db.Tx, _ *commercegov.Restriction) error {
	return nil
}

// TestAuctionService_RequireSellerNotRestricted_Restricted_Blocks proves that
// a restricted seller is rejected at auction mutation boundaries.
func TestAuctionService_RequireSellerNotRestricted_Restricted_Blocks(t *testing.T) {
	svc := &AuctionService{
		commerceGovRepo: &auctionCommerceRestrictionRepo{restricted: true},
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
		"restricted seller should be blocked with ErrCommerceRestricted")
}

// TestAuctionService_RequireSellerNotRestricted_Unrestricted_Allows proves
// that an unrestricted seller passes the restriction check.
func TestAuctionService_RequireSellerNotRestricted_Unrestricted_Allows(t *testing.T) {
	svc := &AuctionService{
		commerceGovRepo: &auctionCommerceRestrictionRepo{restricted: false},
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "unrestricted seller should be allowed")
}

// TestAuctionService_RequireUserNotRestricted_Restricted_Blocks proves that
// a restricted bidder is rejected at the bid/claim boundary.
func TestAuctionService_RequireUserNotRestricted_Restricted_Blocks(t *testing.T) {
	svc := &AuctionService{
		commerceGovRepo: &auctionCommerceRestrictionRepo{restricted: true},
	}

	err := svc.requireUserNotRestricted(context.Background(), nil, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrCommerceRestricted,
	"restricted bidder should be blocked with ErrCommerceRestricted")
}

// TestAuctionService_RequireUserNotRestricted_Unrestricted_Allows proves
// that an unrestricted bidder passes the restriction check.
func TestAuctionService_RequireUserNotRestricted_Unrestricted_Allows(t *testing.T) {
	svc := &AuctionService{
		commerceGovRepo: &auctionCommerceRestrictionRepo{restricted: false},
	}

	err := svc.requireUserNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "unrestricted bidder should be allowed")
}

// TestAuctionService_RequireSellerNotRestricted_NilRepo_FailOpen proves
// backward-compatible fail-open when the repository is not wired.
func TestAuctionService_RequireSellerNotRestricted_NilRepo_FailOpen(t *testing.T) {
	svc := &AuctionService{
		commerceGovRepo: nil,
	}

	err := svc.requireSellerNotRestricted(context.Background(), nil, uuid.New())
	assert.NoError(t, err, "nil repo should fail-open")
}
