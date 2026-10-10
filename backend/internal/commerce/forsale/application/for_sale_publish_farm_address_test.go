package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	addressEntity "github.com/hishumi/backend/internal/identity/address/entity"
	addressRepoTypes "github.com/hishumi/backend/internal/identity/address/repository"
	"github.com/hishumi/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Mock address repository for testing
// ============================================================================

type mockAddressRepo struct {
	primary *addressEntity.Address
	err     error
}

func (m *mockAddressRepo) GetPrimaryByUserID(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressEntity.Address, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.primary, nil
}

// Unused interface methods — required by AddressRepository interface
func (m *mockAddressRepo) Create(_ context.Context, _ db.Tx, _ *addressEntity.Address) error { return nil }
func (m *mockAddressRepo) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*addressEntity.Address, error) {
	return nil, nil
}
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
func (m *mockAddressRepo) GetByUserIDForDisplay(_ context.Context, _ db.Tx, _ uuid.UUID) ([]*addressEntity.Address, error) {
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

func TestEnsureSellerOriginValid_NoPrimaryAddress(t *testing.T) {
	svc := &ForSaleService{addressRepo: &mockAddressRepo{}}

	err := svc.EnsureSellerOriginValid(context.Background(), nil, uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSellerOriginNotConfigured)
}

func TestEnsureSellerOriginValid_RepositoryError(t *testing.T) {
	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{err: fmt.Errorf("db down")},
	}

	err := svc.EnsureSellerOriginValid(context.Background(), nil, uuid.New())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSellerOriginNotConfigured)
}

func TestEnsureSellerOriginValid_HasPrimaryAddress(t *testing.T) {
	sellerID := uuid.New()
	svc := &ForSaleService{
		addressRepo: &mockAddressRepo{
			primary: &addressEntity.Address{ID: uuid.New(), UserID: sellerID},
		},
	}

	err := svc.EnsureSellerOriginValid(context.Background(), nil, sellerID)

	assert.NoError(t, err)
}
