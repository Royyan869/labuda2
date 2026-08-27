package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/entity"
	"github.com/labuda/backend/pkg/db"
)

// OperabilityRecommendationAction represents the lifecycle action the safety
// pipeline should execute for a promotion instance.
type OperabilityRecommendationAction string

const (
	OperabilityRecommendationNoAction OperabilityRecommendationAction = "no_action"
	OperabilityRecommendationPause    OperabilityRecommendationAction = "pause"
	OperabilityRecommendationResume   OperabilityRecommendationAction = "resume"
	OperabilityRecommendationStop     OperabilityRecommendationAction = "stop"
)

// OperabilityRecommendation is the result of an operability evaluation.
// The checker produces these and the PromotionService executes them.
type OperabilityRecommendation struct {
	Action      OperabilityRecommendationAction
	Reason      string
	TargetType  entity.TargetType
	TargetID    *uuid.UUID
	InstanceID  uuid.UUID
	OwnershipID uuid.UUID
	UserID      uuid.UUID
	Reversible  bool
	Permanent   bool
}

// HasAction returns true when the recommendation should be executed.
func (r OperabilityRecommendation) HasAction() bool {
	return r.Action != OperabilityRecommendationNoAction
}

// Validate checks that the recommendation has the minimum fields required for
// execution.
func (r OperabilityRecommendation) Validate() error {
	if r.Action == OperabilityRecommendationNoAction {
		return nil
	}

	if r.InstanceID == uuid.Nil {
		return fmt.Errorf("promotion operability recommendation missing instance_id")
	}

	if r.Action == OperabilityRecommendationStop && r.Reason == "" {
		return fmt.Errorf("promotion operability recommendation missing stop reason")
	}

	return nil
}

// OperabilityRecommendationSource produces promotion lifecycle recommendations.
type OperabilityRecommendationSource interface {
	SweepInactivePromotions(ctx context.Context, limit int) ([]OperabilityRecommendation, error)
	SweepPausedPromotions(ctx context.Context, limit int) ([]OperabilityRecommendation, error)
}

// OperabilityRecommendationApplier executes promotion lifecycle recommendations.
type OperabilityRecommendationApplier interface {
	ApplyOperabilityRecommendation(ctx context.Context, tx db.Tx, recommendation OperabilityRecommendation) error
}
