package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/labuda/backend/internal/platform/config/entity"
	"github.com/labuda/backend/pkg/db"
)

// stubConfigRepo is a minimal in-memory repository.Repository for getter tests.
type stubConfigRepo struct {
	repo map[string]*entity.Config
}

func (s *stubConfigRepo) Get(_ context.Context, _ db.Tx, key string) (*entity.Config, error) {
	if c, ok := s.repo[key]; ok {
		return c, nil
	}
	return nil, &entity.ConfigNotFoundError{Key: key}
}

func (s *stubConfigRepo) GetAll(context.Context, db.Tx) ([]*entity.Config, error) { return nil, nil }

func (s *stubConfigRepo) SetNumeric(context.Context, db.Tx, string, decimal.Decimal, uuid.UUID) error {
	return nil
}

func (s *stubConfigRepo) SetText(context.Context, db.Tx, string, string, uuid.UUID) error {
	return nil
}

func numericConfig(key string, v int64) *entity.Config {
	d := decimal.NewFromInt(v)
	return &entity.Config{Key: key, ValueNum: &d}
}

// TestGetSellerWithdrawalFee_ReadsConfiguredValue proves the configured
// baseline (5000) is returned as-is.
func TestGetSellerWithdrawalFee_ReadsConfiguredValue(t *testing.T) {
	svc := NewConfigService(&stubConfigRepo{repo: map[string]*entity.Config{
		KeySellerWithdrawalFeeRupiah: numericConfig(KeySellerWithdrawalFeeRupiah, 5_000),
	}})
	require.Equal(t, int64(5_000), svc.GetSellerWithdrawalFee(context.Background(), nil))
}

// TestGetSellerWithdrawalFee_ZeroIsValid proves Rp0 (free withdrawal) is
// accepted by the runtime config authority.
func TestGetSellerWithdrawalFee_ZeroIsValid(t *testing.T) {
	svc := NewConfigService(&stubConfigRepo{repo: map[string]*entity.Config{
		KeySellerWithdrawalFeeRupiah: numericConfig(KeySellerWithdrawalFeeRupiah, 0),
	}})
	require.Equal(t, int64(0), svc.GetSellerWithdrawalFee(context.Background(), nil))
}

// TestGetSellerWithdrawalFee_NegativePanics proves negative values fail fast
// (no silent fallback to a hardcoded fee).
func TestGetSellerWithdrawalFee_NegativePanics(t *testing.T) {
	svc := NewConfigService(&stubConfigRepo{repo: map[string]*entity.Config{
		KeySellerWithdrawalFeeRupiah: numericConfig(KeySellerWithdrawalFeeRupiah, -1),
	}})
	require.Panics(t, func() { svc.GetSellerWithdrawalFee(context.Background(), nil) })
}

// TestGetSellerWithdrawalFee_MissingPanics proves a missing key fails fast
// (canonical fail-fast behavior; migration guarantees the key exists).
func TestGetSellerWithdrawalFee_MissingPanics(t *testing.T) {
	svc := NewConfigService(&stubConfigRepo{repo: map[string]*entity.Config{}})
	require.Panics(t, func() { svc.GetSellerWithdrawalFee(context.Background(), nil) })
}