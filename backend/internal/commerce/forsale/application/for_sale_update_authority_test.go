package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeForSaleRepository is a minimal stub for Update authority tests.
// It returns a configurable "current" ForSale for GetByID and tracks Update calls.
type fakeForSaleRepository struct {
	current *entity.ForSale
	updateCalled bool
}

func (r *fakeForSaleRepository) Create(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ForSale, error) {
	return r.current, nil
}
func (r *fakeForSaleRepository) GetByProductID(_ context.Context, _ db.Tx, _ uuid.UUID) (*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetForUpdate(_ context.Context, _ db.Tx, id uuid.UUID) (*entity.ForSale, error) {
	return r.current, nil
}
func (r *fakeForSaleRepository) Update(_ context.Context, _ db.Tx, _ *entity.ForSale) error {
	r.updateCalled = true
	return nil
}
func (r *fakeForSaleRepository) UpdateStock(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) UpdateStatus(_ context.Context, _ db.Tx, _ *entity.ForSale) error { return nil }
func (r *fakeForSaleRepository) GetBySellerID(_ context.Context, _ db.Tx, _ uuid.UUID, _ bool) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetBySellerIDPaginated(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ int, _ bool) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetPublicBySellerID(_ context.Context, _ db.Tx, _ uuid.UUID, _, _ int) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) GetPublic(_ context.Context, _ db.Tx, _, _ int) ([]*entity.ForSale, error) {
	return nil, nil
}
func (r *fakeForSaleRepository) Search(_ context.Context, _ db.Tx, _ forsaleRepo.SearchFilters) ([]*entity.ForSale, *time.Time, error) {
	return nil, nil, nil
}

func TestUpdate_RejectsInvalidOrdinaryTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    entity.ForSaleStatus
		to      entity.ForSaleStatus
	}{
		{"active to draft", entity.ForSaleStatusActive, entity.ForSaleStatusDraft},
		{"sold to withdrawn", entity.ForSaleStatusSold, entity.ForSaleStatusWithdrawn},
		{"withdrawn to sold", entity.ForSaleStatusWithdrawn, entity.ForSaleStatusSold},
		{"draft to sold", entity.ForSaleStatusDraft, entity.ForSaleStatusSold},
		{"sold to withdrawn", entity.ForSaleStatusSold, entity.ForSaleStatusWithdrawn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forSaleID := uuid.New()
			repo := &fakeForSaleRepository{
				current: &entity.ForSale{
					ID:     forSaleID,
					Status: tt.from,
				},
			}
			svc := &ForSaleService{repo: repo}

			err := svc.Update(context.Background(), nil, &entity.ForSale{
				ID:     forSaleID,
				Status: tt.to,
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid status transition")
			assert.False(t, repo.updateCalled, "Update should not be called on invalid transition")
		})
	}
}

func TestUpdate_RejectsGovernedTransition(t *testing.T) {
	tests := []struct {
		name string
		from entity.ForSaleStatus
		to   entity.ForSaleStatus
	}{
		{"sold to active via Update", entity.ForSaleStatusSold, entity.ForSaleStatusActive},
		{"withdrawn to active via Update", entity.ForSaleStatusWithdrawn, entity.ForSaleStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forSaleID := uuid.New()
			repo := &fakeForSaleRepository{
				current: &entity.ForSale{
					ID:     forSaleID,
					Status: tt.from,
				},
			}
			svc := &ForSaleService{repo: repo}

			err := svc.Update(context.Background(), nil, &entity.ForSale{
				ID:     forSaleID,
				Status: tt.to,
			})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not permitted through Update")
			assert.False(t, repo.updateCalled, "Update should not be called on governed transition")
		})
	}
}

func TestUpdate_AllowsValidOrdinaryTransition(t *testing.T) {
	tests := []struct {
		name string
		from entity.ForSaleStatus
		to   entity.ForSaleStatus
	}{
		{"draft to active", entity.ForSaleStatusDraft, entity.ForSaleStatusActive},
		{"active to withdrawn", entity.ForSaleStatusActive, entity.ForSaleStatusWithdrawn},
		{"draft to withdrawn", entity.ForSaleStatusDraft, entity.ForSaleStatusWithdrawn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forSaleID := uuid.New()
			repo := &fakeForSaleRepository{
				current: &entity.ForSale{
					ID:     forSaleID,
					Status: tt.from,
				},
			}
			svc := &ForSaleService{repo: repo}

			err := svc.Update(context.Background(), nil, &entity.ForSale{
				ID:     forSaleID,
				Status: tt.to,
			})

			require.NoError(t, err)
			assert.True(t, repo.updateCalled, "Update should be called for valid transition")
		})
	}
}

func TestUpdate_AllowsStatusUnchanged(t *testing.T) {
	forSaleID := uuid.New()
	repo := &fakeForSaleRepository{
		current: &entity.ForSale{
			ID:     forSaleID,
			Status: entity.ForSaleStatusActive,
		},
	}
	svc := &ForSaleService{repo: repo}

	err := svc.Update(context.Background(), nil, &entity.ForSale{
		ID:     forSaleID,
		Status: entity.ForSaleStatusActive, // same status
	})

	require.NoError(t, err)
	assert.True(t, repo.updateCalled)
}
