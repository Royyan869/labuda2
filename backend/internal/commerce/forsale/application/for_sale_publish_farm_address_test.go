package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	addressEntity "github.com/labuda/backend/internal/identity/address/entity"
	addressRepoTypes "github.com/labuda/backend/internal/identity/address/repository"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock address repository for testing
// ============================================================================

type mockAddressRepo struct {
	address *addressEntity.Address
	err     error
}

func (m *mockAddressRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressEntity.Address, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.address, nil
}

// Unused interface methods — required by AddressRepository interface
func (m *mockAddressRepo) Create(_ context.Context, _ db.Tx, _ *addressEntity.Address) error { return nil }
func (m *mockAddressRepo) GetForUpdate(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressEntity.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) Update(_ context.Context, _ db.Tx, _ *addressEntity.Address) error {
	return nil
}
func (m *mockAddressRepo) Delete(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (m *mockAddressRepo) GetByUserID(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*addressEntity.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) GetByUserIDFiltered(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) ([]*addressEntity.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) GetPrimaryByUserID(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressEntity.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) GetPrimaryByUserIDFiltered(_ context.Context, _ db.Tx, _ uuid.UUID, _ string) (*addressEntity.Address, error) {
	return nil, nil
}
func (m *mockAddressRepo) SetPrimary(_ context.Context, _ db.Tx, _ uuid.UUID) error { return nil }
func (m *mockAddressRepo) UnsetAllPrimary(_ context.Context, _ db.Tx, _ uuid.UUID) error {
	return nil
}
func (m *mockAddressRepo) CountByUserID(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressRepoTypes.AddressCount, error) {
	return nil, nil
}

// ============================================================================
// Tests
// ============================================================================

func TestEnsureFarmAddressValid_NilFarmAddressID(t *testing.T) {
	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{},
	}

	for_sale := &entity.ForSale{
		ID:            uuid.New(),
		SellerID:      uuid.New(),
		Product: &productEntity.Product{FarmAddressID: nil}, // NOT SET
	}

	err := svc.EnsureFarmAddressValid(context.Background(), nil, for_sale)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFarmAddressNotConfigured))
	assert.Contains(t, err.Error(), "farm_address_id is required")
}

func TestEnsureFarmAddressValid_AddressNotFound(t *testing.T) {
	addressID := uuid.New()
	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{
			err: fmt.Errorf("address not found: %s", addressID),
		},
	}

	for_sale := &entity.ForSale{
		ID:            uuid.New(),
		SellerID:      uuid.New(),
		Product: &productEntity.Product{FarmAddressID: &addressID},
	}

	err := svc.EnsureFarmAddressValid(context.Background(), nil, for_sale)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFarmAddressNotConfigured))
	assert.Contains(t, err.Error(), "farm address not found")
}

func TestEnsureFarmAddressValid_WrongOwner(t *testing.T) {
	sellerID := uuid.New()
	otherUserID := uuid.New()
	addressID := uuid.New()

	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{
			address: &addressEntity.Address{
				ID:      addressID,
				UserID:  otherUserID, // Different from seller
				Purpose: addressEntity.AddressPurposeSender,
			},
		},
	}

	for_sale := &entity.ForSale{
		ID:            uuid.New(),
		SellerID:      sellerID,
		Product: &productEntity.Product{FarmAddressID: &addressID},
	}

	err := svc.EnsureFarmAddressValid(context.Background(), nil, for_sale)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFarmAddressNotConfigured))
	assert.Contains(t, err.Error(), "does not belong to seller")
}

func TestEnsureFarmAddressValid_WrongPurpose(t *testing.T) {
	sellerID := uuid.New()
	addressID := uuid.New()

	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{
			address: &addressEntity.Address{
				ID:      addressID,
				UserID:  sellerID,
				Purpose: addressEntity.AddressPurposeShipping, // Wrong: should be "sender"
			},
		},
	}

	for_sale := &entity.ForSale{
		ID:            uuid.New(),
		SellerID:      sellerID,
		Product: &productEntity.Product{FarmAddressID: &addressID},
	}

	err := svc.EnsureFarmAddressValid(context.Background(), nil, for_sale)

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrFarmAddressNotConfigured))
	assert.Contains(t, err.Error(), "must have purpose 'sender'")
}

func TestEnsureFarmAddressValid_ValidSenderAddress(t *testing.T) {
	sellerID := uuid.New()
	addressID := uuid.New()

	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{
			address: &addressEntity.Address{
				ID:      addressID,
				UserID:  sellerID,
				Purpose: addressEntity.AddressPurposeSender,
			},
		},
	}

	for_sale := &entity.ForSale{
		ID:            uuid.New(),
		SellerID:      sellerID,
		Product: &productEntity.Product{FarmAddressID: &addressID},
	}

	err := svc.EnsureFarmAddressValid(context.Background(), nil, for_sale)

	assert.NoError(t, err)
}




