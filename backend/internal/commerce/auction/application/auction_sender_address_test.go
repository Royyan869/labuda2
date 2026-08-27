package application

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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type captureAuctionProductCreator struct {
	product *productEntity.Product
}

func (r *captureAuctionProductCreator) Create(_ context.Context, _ db.Tx, product *productEntity.Product) error {
	r.product = product
	product.ID = uuid.New()
	return nil
}

// newAuctionServiceForFarmAddressTests builds a fully-wired AuctionService
// whose Product creation is captured, so tests can verify that the canonical
// Product minted by CreateDraft carries the FarmAddressID passed through the
// CreateDraftInput (Product owns farm/address information; Auction never
// resolves it itself).
func newAuctionServiceForFarmAddressTests(productRepo *captureAuctionProductCreator) *AuctionService {
	optID := uuid.New()
	return &AuctionService{
		accountStatus: noopAccountStatusChecker{},
		auctionRepo:   &auctionRepo.AuctionRepository{},
		outboxRepo:    &outboxRepo.OutboxRepository{},
		configService: &platformconfigApp.ConfigService{},
		roleChecker:   noopRoleChecker{},
		ownership:     auth.NewOwnershipValidator(),
		productRepo:   productRepo,
		productShippingRepo: &scheduleStubProductShippingRepo{
			options: []*shippingEntity.ShippingOption{{ID: optID}},
		},
		shippingCoverageRepo: &scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {{ID: uuid.New(), ShippingOptionID: optID, IsAvailable: true}},
			},
		},
		log: zap.NewNop(),
	}
}

// TestCreateDraft_PassesFarmAddressIDToCanonicalProduct verifies that the
// FarmAddressID supplied on CreateDraftInput lands on the minted Product —
// Product is the single authority for farm/address information.
func TestCreateDraft_PassesFarmAddressIDToCanonicalProduct(t *testing.T) {
	sellerID := uuid.New()
	farmAddressID := uuid.New()
	productRepo := &captureAuctionProductCreator{}
	svc := newAuctionServiceForFarmAddressTests(productRepo)

	auction, err := svc.CreateDraft(context.Background(), fakeTx{}, CreateDraftInput{
		SellerID:          sellerID,
		Title:             "Test Auction",
		Description:       "Canonical farm address",
		StartPrice:        10000,
		BidIncrement:      1000,
		BuyNowPrice:       ptrInt64(12000),
		StartMode:         entity.StartModeNow,
		Duration:          24 * time.Hour,
		MediaURLs:         []string{"https://example.com/1.jpg"},
		Variety:           "Kohaku",
		SizeCM:            intPtr(50),
		FarmAddressID:     &farmAddressID,
		ShippingOptionIDs: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, auction)
	require.NotNil(t, productRepo.product)
	require.NotNil(t, productRepo.product.FarmAddressID)
	require.Equal(t, farmAddressID, *productRepo.product.FarmAddressID)
}

func intPtr(v int) *int { return &v }
