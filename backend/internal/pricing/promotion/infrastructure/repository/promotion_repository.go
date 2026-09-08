package repository

import (
	"context"
	"time"

	"github.com/labuda/backend/pkg/db"
)

// PromotionRepositoryImpl is the canonical external product repository
// implementation (receiver type shared with external_product_repository_impl.go).
// Legacy package/ownership/instance lifecycle methods have been purged per
// hard convergence — the canonical promotion lifecycle is promotion_contracts.
type PromotionRepositoryImpl struct{}

func NewPromotionRepository() *PromotionRepositoryImpl { return &PromotionRepositoryImpl{} }

func (r *PromotionRepositoryImpl) GetDBTime(ctx context.Context, tx db.Tx) (time.Time, error) {
	var t time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&t); err != nil {
		return time.Time{}, err
	}
	return t, nil
}
