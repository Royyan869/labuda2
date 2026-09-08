package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/internal/pricing/promotion/repository"
	"github.com/labuda/backend/pkg/db"
	"go.uber.org/zap"
)

// PromotionService handles external product operations only.
// Legacy package/ownership/instance lifecycle has been purged per hard
// convergence — canonical promotion authority is promotion_contracts +
// promotion_contract_targets.
type PromotionService struct {
	repo repository.ExternalProductRepository
	db   *db.DB
	log  *zap.Logger
}

// NewPromotionService creates the external product promotion service. The
// operability checker parameter is retained for wiring compatibility; target
// operability lives in the shared OperabilityChecker used by the contract
// domain.
func NewPromotionService(checker interface{}) *PromotionService {
	return &PromotionService{log: zap.NewNop()}
}

// NewPromotionServiceWithRepo creates with injected repo.
func NewPromotionServiceWithRepo(repo repository.ExternalProductRepository, db *db.DB) *PromotionService {
	return &PromotionService{repo: repo, db: db, log: zap.NewNop()}
}

// SetRepo wires the external product repository.
func (s *PromotionService) SetRepo(repo repository.ExternalProductRepository) {
	s.repo = repo
}

// IsTargetPromotedInTx reports whether the target is currently queued in a
// non-finalized promotion contract (promotion_contract_targets joined to
// promotion_contracts) — the canonical queue authority. It is used by the
// external product surface to decide public visibility of an approved
// external product.
func (s *PromotionService) IsTargetPromotedInTx(ctx context.Context, tx db.Tx, targetType entity.TargetType, targetID uuid.UUID) (bool, error) {
	var promoted bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM promotion_contract_targets t
			JOIN promotion_contracts c ON c.id = t.contract_id
			WHERE t.target_type = $1
			  AND t.target_id = $2
			  AND c.status IN ('prepared', 'active', 'paused')
		)
	`, string(targetType), targetID).Scan(&promoted)
	if err != nil {
		return false, fmt.Errorf("check target promoted state: %w", err)
	}
	return promoted, nil
}