package application

// Tests for PASS_18C auction create timing policy: immediate vs scheduled
// start, and the owner-approved 1-7 day duration bound enforced server-side
// (CreateDraft), not just at the HTTP/UI layer.

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	shippingEntity "github.com/labuda/backend/internal/commerce/shipping/entity"
	"github.com/labuda/backend/internal/identity/auth"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	outboxRepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeProductCreator satisfies ProductCreator without touching a real DB —
// CreateDraft only needs product.ID populated to proceed to auction creation.
type fakeProductCreator struct{}

func (fakeProductCreator) Create(_ context.Context, _ db.Tx, product *productEntity.Product) error {
	product.ID = uuid.New()
	return nil
}

// newAuctionServiceForCreateTiming builds a fully-wired AuctionService whose
// dependencies are either real (repos operating against fakeTx, which no-ops
// successfully) or minimal stubs, so CreateDraft can run end-to-end including
// its market-authority + shipping-coverage progression to scheduled/active
// (scheduleAuctionInternal) without a live Postgres.
func newAuctionServiceForCreateTiming() *AuctionService {
	optID := uuid.New()
	return &AuctionService{
		accountStatus: noopAccountStatusChecker{},
		auctionRepo:   &auctionRepo.AuctionRepository{},
		outboxRepo:    &outboxRepo.OutboxRepository{},
		configService: &platformconfigApp.ConfigService{},
		roleChecker:   noopRoleChecker{},
		ownership:     auth.NewOwnershipValidator(),
		productShippingRepo: &scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: optID}},
		},
		shippingCoverageRepo: &scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: true}},
			},
		},
		productRepo: fakeProductCreator{},
		log:         zap.NewNop(),
	}
}

func baseCreateDraftInput() CreateDraftInput {
	return CreateDraftInput{
		SellerID:     uuid.New(),
		Title:        "Test Auction",
		Description:  "PASS_18C timing test",
		StartPrice:   10000,
		BidIncrement: 1000,
	}
}

func TestCreateDraft_ImmediateStart_IsActiveWithServerTime(t *testing.T) {
	svc := newAuctionServiceForCreateTiming()
	input := baseCreateDraftInput()
	input.StartMode = entity.StartModeNow
	input.Duration = 24 * time.Hour

	before := time.Now()
	auction, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
	after := time.Now()
	require.NoError(t, err)
	require.NotNil(t, auction)

	assert.Equal(t, entity.StatusActive, auction.Status, "immediate start must go straight to active — no separate schedule call required")
	assert.False(t, auction.StartAt.Before(before), "start_at must be server time, not before the call")
	assert.False(t, auction.StartAt.After(after), "start_at must be server time, not after the call")
	assert.Equal(t, 24*time.Hour, auction.EndAt.Sub(auction.StartAt))
}

func TestCreateDraft_ScheduledStart_IsScheduledAtRequestedFutureTime(t *testing.T) {
	svc := newAuctionServiceForCreateTiming()
	future := time.Now().Add(48 * time.Hour)
	input := baseCreateDraftInput()
	input.StartMode = entity.StartModeScheduled
	input.ScheduledStartAt = &future
	input.Duration = 72 * time.Hour

	auction, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
	require.NoError(t, err)
	require.NotNil(t, auction)

	assert.Equal(t, entity.StatusScheduled, auction.Status, "scheduled start must reach scheduled — no separate seller action required after submit")
	assert.Equal(t, future, auction.StartAt)
	assert.Equal(t, future.Add(72*time.Hour), auction.EndAt)
}

func TestCreateDraft_ScheduledStart_RequiresFutureTime(t *testing.T) {
	svc := newAuctionServiceForCreateTiming()
	past := time.Now().Add(-time.Hour)
	input := baseCreateDraftInput()
	input.StartMode = entity.StartModeScheduled
	input.ScheduledStartAt = &past
	input.Duration = 24 * time.Hour

	_, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
	var futureErr *entity.ErrScheduledStartMustBeFuture
	assert.ErrorAs(t, err, &futureErr)
}

func TestCreateDraft_RejectsDurationBelowMinimum(t *testing.T) {
	svc := newAuctionServiceForCreateTiming()
	input := baseCreateDraftInput()
	input.StartMode = entity.StartModeNow
	input.Duration = entity.MinAuctionDuration - time.Hour

	_, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
	var durErr *entity.ErrAuctionDurationOutOfRange
	assert.ErrorAs(t, err, &durErr)
}

func TestCreateDraft_RejectsDurationAboveMaximum(t *testing.T) {
	svc := newAuctionServiceForCreateTiming()
	input := baseCreateDraftInput()
	input.StartMode = entity.StartModeNow
	input.Duration = entity.MaxAuctionDuration + time.Hour

	_, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
	var durErr *entity.ErrAuctionDurationOutOfRange
	assert.ErrorAs(t, err, &durErr)
}

func TestCreateDraft_AcceptsDurationAtBoundaries(t *testing.T) {
	for _, d := range []time.Duration{entity.MinAuctionDuration, entity.MaxAuctionDuration} {
		svc := newAuctionServiceForCreateTiming()
		input := baseCreateDraftInput()
		input.StartMode = entity.StartModeNow
		input.Duration = d

		auction, err := svc.CreateDraft(context.Background(), fakeTx{}, input)
		require.NoError(t, err)
		assert.Equal(t, d, auction.EndAt.Sub(auction.StartAt))
	}
}
