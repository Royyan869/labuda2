package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hishumi/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/hishumi/backend/internal/commerce/auction/infrastructure/repository"
	productEntity "github.com/hishumi/backend/internal/commerce/product/entity"
	shippingEntity "github.com/hishumi/backend/internal/commerce/shipping/entity"
	"github.com/hishumi/backend/internal/identity/auth"
	platformconfigApp "github.com/hishumi/backend/internal/platform/config/application"
	outboxRepo "github.com/hishumi/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/hishumi/backend/pkg/db"
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

func (r *captureAuctionProductCreator) ClaimSellingSurface(_ context.Context, _ db.Tx, _ uuid.UUID, _ productEntity.SellingSurface) error {
	return nil
}

func (r *captureAuctionProductCreator) GetByID(_ context.Context, _ db.Tx, _ uuid.UUID) (*productEntity.Product, error) {
	return &productEntity.Product{ID: uuid.New(), SellerID: uuid.New(), Title: "dummy", Description: "dummy"}, nil
}

func (r *captureAuctionProductCreator) Update(_ context.Context, _ db.Tx, _ *productEntity.Product) error {
	return nil
}

// newAuctionServiceForFarmAddressTests builds a fully-wired AuctionService
// whose Product creation is captured, so tests can verify that the canonical
// Product minted by Create carries the FarmAddressID passed through the
// CreateAuctionInput (Product owns farm/address information; Auction never
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
			options: []*shippingEntity.ShippingSetup{{ID: optID}},
		},
		shippingCoverageRepo: &scheduleStubCoverageRepo{
			coveragesByOption: map[uuid.UUID][]*shippingEntity.ShippingCoverage{
				optID: {{ID: uuid.New(), ShippingSetupID: optID, IsAvailable: true}},
			},
		},
		log: zap.NewNop(),
	}
}

// TestCreate_MintsCanonicalProduct verifies the minted Product carries the
// auction content. There is no product-level origin address: every product's
// origin is the seller account's primary address, resolved at read time.
func TestCreate_MintsCanonicalProduct(t *testing.T) {
	sellerID := uuid.New()
	productRepo := &captureAuctionProductCreator{}
	svc := newAuctionServiceForFarmAddressTests(productRepo)

	auction, err := svc.Create(context.Background(), fakeTx{}, CreateAuctionInput{
		SellerID:         sellerID,
		Title:            "Test Auction",
		Description:      "Canonical product",
		StartPrice:       10000,
		BidIncrement:     1000,
		BuyNowPrice:      ptrInt64(12000),
		StartMode:        entity.StartModeNow,
		Duration:         24 * time.Hour,
		Media:            []productEntity.ProductMedia{{URL: "https://example.com/1.jpg"}},
		Variety:          "Kohaku",
		SizeCM:           intPtr(50),
		ShippingSetupIDs: nil,
	})

	require.NoError(t, err)
	require.NotNil(t, auction)
	require.NotNil(t, productRepo.product)
	require.Equal(t, "Test Auction", productRepo.product.Title)
	require.Equal(t, productEntity.SellingSurfaceAuction, productRepo.product.SellingSurface)
}

func intPtr(v int) *int { return &v }
