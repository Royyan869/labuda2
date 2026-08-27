package worker

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/pricing/promotion/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type recordingRecommendationSource struct {
	activeCalls int
	pausedCalls int

	activeRecommendations []application.OperabilityRecommendation
	pausedRecommendations []application.OperabilityRecommendation
}

func (r *recordingRecommendationSource) SweepInactivePromotions(ctx context.Context, limit int) ([]application.OperabilityRecommendation, error) {
	r.activeCalls++
	return r.activeRecommendations, nil
}

func (r *recordingRecommendationSource) SweepPausedPromotions(ctx context.Context, limit int) ([]application.OperabilityRecommendation, error) {
	r.pausedCalls++
	return r.pausedRecommendations, nil
}

type recordingRecommendationApplier struct {
	calls []application.OperabilityRecommendation
}

func (r *recordingRecommendationApplier) ApplyOperabilityRecommendation(
	ctx context.Context,
	tx db.Tx,
	recommendation application.OperabilityRecommendation,
) error {
	r.calls = append(r.calls, recommendation)
	return nil
}

type recordingTransactor struct {
	calls int
}

func (r *recordingTransactor) WithTx(ctx context.Context, fn func(tx db.Tx) error) error {
	r.calls++
	return fn(&mockTx{})
}

func TestPromotionSafetyWorker_DispatchesRecommendationsThroughService(t *testing.T) {
	source := &recordingRecommendationSource{
		activeRecommendations: []application.OperabilityRecommendation{
			{
				Action:      application.OperabilityRecommendationPause,
				Reason:      "for_sale_hidden",
				InstanceID:  uuid.New(),
				OwnershipID: uuid.New(),
				UserID:      uuid.New(),
			},
		},
		pausedRecommendations: []application.OperabilityRecommendation{
			{
				Action:      application.OperabilityRecommendationResume,
				InstanceID:  uuid.New(),
				OwnershipID: uuid.New(),
				UserID:      uuid.New(),
			},
		},
	}
	applier := &recordingRecommendationApplier{}
	txManager := &recordingTransactor{}

	worker := NewPromotionSafetyWorker(
		source,
		applier,
		txManager,
		zap.NewNop(),
		DefaultPromotionSafetyWorkerConfig(),
	)
	worker.shutdownCtx = context.Background()

	worker.processSweep()

	require.Equal(t, 1, source.activeCalls)
	require.Equal(t, 1, source.pausedCalls)
	require.Len(t, applier.calls, 2)
	assert.Equal(t, 2, txManager.calls)
	assert.Equal(t, application.OperabilityRecommendationPause, applier.calls[0].Action)
	assert.Equal(t, application.OperabilityRecommendationResume, applier.calls[1].Action)
}
