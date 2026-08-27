package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// DiscoveryService handles promotion discovery with read-time operability filtering.
// This ensures that even if stale state exists, dead targets never appear as promoted.
type DiscoveryService struct {
	db                 *db.DB
	operabilityChecker OperabilityChecker
}

// NewDiscoveryService creates a new DiscoveryService.
func NewDiscoveryService(
	dbConn *db.DB,
	operabilityChecker OperabilityChecker,
) *DiscoveryService {
	return &DiscoveryService{
		db:                 dbConn,
		operabilityChecker: operabilityChecker,
	}
}

// GetPromotedItems returns active promotion instances with read-time operability filtering.
//
// This is the PRIMARY method for discovery surfaces (feed, search, etc.) to fetch promotions.
// It ensures that:
// - Only active instances are returned
// - Target domains are still operable (for_sale active/visible, auction promotable)
// - External products pass full operability: approved + active media + seller 4-gate
//
// Returns only instances whose targets are still operable.
func (s *DiscoveryService) GetPromotedItems(
	ctx context.Context,
	limit int,
) ([]*entity.PromotionInstance, error) {
	if s == nil || s.db == nil {
		return []*entity.PromotionInstance{}, nil
	}
	// Get all active instances directly from database
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason, created_at, updated_at
		FROM promotion_instances
		WHERE status = 'active'
		ORDER BY RANDOM()
		LIMIT $1
	`

	rows, err := s.db.Pool().Query(ctx, query, limit*2) // Fetch more for filtering
	if err != nil {
		return nil, fmt.Errorf("failed to get active instances: %w", err)
	}
	defer rows.Close()

	var allInstances []*entity.PromotionInstance
	for rows.Next() {
		var inst entity.PromotionInstance
		var stoppedAt, stopReason *interface{} // Nullable fields

		err := rows.Scan(
			&inst.ID, &inst.OwnershipID, &inst.UserID, &inst.TargetType, &inst.TargetID,
			&inst.Status, &inst.ActivatedAt, &stoppedAt, &stopReason,
			&inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			continue
		}

		if stoppedAt != nil {
			// This shouldn't happen for active status but handle gracefully
			continue
		}

		allInstances = append(allInstances, &inst)
	}

	// Filter instances by operability at read-time
	// This is the READ-TIME HONESTY layer — promotion discovery must never
	// expose stronger visibility than normal search/discovery.
	var operableInstances []*entity.PromotionInstance
	for _, instance := range filterPublicPromotionInstances(allInstances) {
		// For for_sale/auction: CheckOperability already covers target status
		// AND seller governance (account + subscription).
		isOperable, _, err := s.operabilityChecker.CheckOperability(ctx, instance.TargetType, instance.TargetID)
		if err != nil {
			// Log error but don't fail the entire query
			// The safety worker will catch these instances later
			continue
		}

		if isOperable {
			operableInstances = append(operableInstances, instance)
		}
	}

	// Apply limit
	if len(operableInstances) > limit {
		operableInstances = operableInstances[:limit]
	}

	return operableInstances, nil
}

// GetPromotedItemsByTargetType returns promoted items filtered by target type.
// This is useful when discovery surfaces need specific target types only.
func (s *DiscoveryService) GetPromotedItemsByTargetType(
	ctx context.Context,
	targetType entity.TargetType,
	limit int,
) ([]*entity.PromotionInstance, error) {
	if s == nil || s.db == nil {
		return []*entity.PromotionInstance{}, nil
	}
	if !targetType.IsPublicPromotable() {
		return []*entity.PromotionInstance{}, nil
	}

	// Get all active instances for the target type
	query := `
		SELECT id, ownership_id, user_id, target_type, target_id,
		       status, activated_at, stopped_at, stop_reason, created_at, updated_at
		FROM promotion_instances
		WHERE status = 'active'
		AND target_type = $1
		ORDER BY RANDOM()
		LIMIT $2
	`

	rows, err := s.db.Pool().Query(ctx, query, string(targetType), limit*2)
	if err != nil {
		return nil, fmt.Errorf("failed to get active instances: %w", err)
	}
	defer rows.Close()

	var instances []*entity.PromotionInstance
	for rows.Next() {
		var inst entity.PromotionInstance
		var stoppedAt, stopReason *interface{}

		err := rows.Scan(
			&inst.ID, &inst.OwnershipID, &inst.UserID, &inst.TargetType, &inst.TargetID,
			&inst.Status, &inst.ActivatedAt, &stoppedAt, &stopReason,
			&inst.CreatedAt, &inst.UpdatedAt,
		)
		if err != nil {
			continue
		}

		instances = append(instances, &inst)
	}

	// Filter by operability/eligibility — same governance as GetPromotedItems.
	var operableInstances []*entity.PromotionInstance
	for _, instance := range instances {
		isOperable, _, err := s.operabilityChecker.CheckOperability(ctx, instance.TargetType, instance.TargetID)
		if err != nil || !isOperable {
			continue
		}
		operableInstances = append(operableInstances, instance)
	}

	// Apply limit
	if len(operableInstances) > limit {
		operableInstances = operableInstances[:limit]
	}

	return operableInstances, nil
}

// IsTargetPromoted checks if a specific target has an active promotion.
// This includes operability checking.
func (s *DiscoveryService) IsTargetPromoted(
	ctx context.Context,
	targetType entity.TargetType,
	targetID uuid.UUID,
) (bool, error) {
	if s == nil || s.db == nil {
		return false, nil
	}
	if !targetType.IsPublicPromotable() {
		return false, nil
	}

	// Get active instance for the target (include user_id for eligibility check)
	query := `
		SELECT id, user_id FROM promotion_instances
		WHERE status = 'active'
		AND target_type = $1
		AND target_id = $2
		LIMIT 1
	`

	var instanceID uuid.UUID
	var userID uuid.UUID
	err := s.db.Pool().QueryRow(ctx, query, string(targetType), targetID).Scan(&instanceID, &userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if target is promoted: %w", err)
	}

	// For for_sale/auction, verify operability (includes full seller governance)
	isOperable, _, err := s.operabilityChecker.CheckOperability(ctx, targetType, &targetID)
	if err != nil {
		return false, fmt.Errorf("failed to check operability: %w", err)
	}

	return isOperable, nil
}

func filterPublicPromotionInstances(instances []*entity.PromotionInstance) []*entity.PromotionInstance {
	result := make([]*entity.PromotionInstance, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		if !instance.TargetType.IsPublicPromotable() {
			continue
		}
		result = append(result, instance)
	}
	return result
}
