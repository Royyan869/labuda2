package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	"github.com/labuda/backend/internal/identity/auth"
	"github.com/labuda/backend/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// auctionUpdateSpyRow simulates a row produced by joinedAuctionColumns
// (auction columns + canonical Product columns). Auction content
// (title, description, koi, preparation, media) is read from Product —
// never from the auctions table.
type auctionUpdateSpyRow struct {
	auction *entity.Auction
}

func (r auctionUpdateSpyRow) Scan(dest ...any) error {
	if len(dest) != 33 {
		return fmt.Errorf("expected 33 scan destinations, got %d", len(dest))
	}

	auction := r.auction
	var product productEntity.Product
	if auction.Product != nil {
		product = *auction.Product
	}

	*dest[0].(*uuid.UUID) = auction.ID
	*dest[1].(*uuid.UUID) = auction.SellerID
	*dest[2].(*uuid.UUID) = auction.ProductID
	*dest[3].(**uuid.UUID) = auction.OrderID
	*dest[4].(**time.Time) = auction.SettlementDeadline
	*dest[5].(*int64) = auction.StartPrice
	*dest[6].(*int64) = auction.BidIncrement
	*dest[7].(**int64) = auction.BuyNowPrice
	*dest[8].(*time.Time) = auction.StartAt
	*dest[9].(*time.Time) = auction.EndAt
	*dest[10].(**int64) = auction.CurrentBid
	*dest[11].(**uuid.UUID) = auction.CurrentWinnerID
	*dest[12].(*string) = string(auction.Status)
	*dest[13].(*time.Time) = auction.CreatedAt
	*dest[14].(*time.Time) = auction.UpdatedAt
	*dest[15].(*int64) = int64(auction.AntiSnipeExtensionTotal / time.Second)
	*dest[16].(*uuid.UUID) = product.ID
	*dest[17].(*uuid.UUID) = product.SellerID
	*dest[18].(*string) = product.Title
	*dest[19].(*string) = product.Description
	mediaJSON := json.RawMessage("[]")
	if len(product.MediaURLs) > 0 {
		mediaJSON, _ = json.Marshal(product.MediaURLs)
	}
	*dest[20].(*json.RawMessage) = mediaJSON
	*dest[21].(*string) = product.Variety
	*dest[22].(**int) = product.SizeCm
	*dest[23].(**int) = product.AgeMonths
	*dest[24].(**string) = product.Gender
	*dest[25].(**string) = product.Breeder
	*dest[26].(**string) = product.Bloodline
	certs := product.Certificates
	if certs == nil {
		certs = []string{}
	}
	*dest[27].(*[]string) = certs
	*dest[28].(**uuid.UUID) = product.FarmAddressID
	*dest[29].(*string) = product.PreparationTime
	*dest[30].(**string) = product.PreparationNote
	*dest[31].(*time.Time) = product.CreatedAt
	*dest[32].(*time.Time) = product.UpdatedAt
	return nil
}

var _ pgx.Row = auctionUpdateSpyRow{}

type auctionUpdateSpyTx struct {
	row      pgx.Row
	execSQL  []string
	execArgs [][]any
}

func (t *auctionUpdateSpyTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return t.row
}

func (t *auctionUpdateSpyTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execSQL = append(t.execSQL, sql)
	t.execArgs = append(t.execArgs, args)
	return pgconn.CommandTag{}, nil
}

func (t *auctionUpdateSpyTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (t *auctionUpdateSpyTx) Commit(_ context.Context) error   { return nil }
func (t *auctionUpdateSpyTx) Rollback(_ context.Context) error { return nil }

var _ db.Tx = (*auctionUpdateSpyTx)(nil)

type auctionRoleCheckerStub struct {
	hasCapability bool
	err           error
}

func (r auctionRoleCheckerStub) IsAdmin(context.Context, uuid.UUID) (bool, error)  { return false, nil }
func (r auctionRoleCheckerStub) IsSeller(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (r auctionRoleCheckerStub) HasActiveSellerCapability(context.Context, uuid.UUID) (bool, error) {
	return r.hasCapability, r.err
}
func (r auctionRoleCheckerStub) HasSellerProfile(context.Context, uuid.UUID) (bool, error) {
	return true, nil
}

var _ auth.RoleChecker = auctionRoleCheckerStub{}

func newAuctionForUpdateAuthority(status entity.Status, sellerID uuid.UUID) *entity.Auction {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	buyNow := int64(1_500_000)
	currentBid := int64(1_200_000)
	productID := uuid.New()
	return &entity.Auction{
		ID:                      uuid.New(),
		SellerID:                sellerID,
		ProductID:               productID,
		StartPrice:              1_000_000,
		BidIncrement:            100_000,
		BuyNowPrice:             &buyNow,
		StartAt:                 now.Add(2 * time.Hour),
		EndAt:                   now.Add(26 * time.Hour),
		CurrentBid:              &currentBid,
		Status:                  status,
		CreatedAt:               now,
		UpdatedAt:               now,
		AntiSnipeExtensionTotal: 0,
		Product: &productEntity.Product{
			ID:              productID,
			SellerID:        sellerID,
			Title:           "Kohaku 50cm",
			Description:     "Draft awal",
			PreparationTime: string(forsaleEntity.PreparationTimeImmediate),
		},
	}
}

func TestUpdateDraft_OwnerCanUpdateDraft_PersistsUpdatedRow(t *testing.T) {
	sellerID := uuid.New()
	auction := newAuctionForUpdateAuthority(entity.StatusDraft, sellerID)
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     sellerID,
		StartPrice:   1_100_000,
		BidIncrement: 150_000,
		BuyNowPrice:  nil,
		StartAt:      auction.StartAt.Add(30 * time.Minute),
		EndAt:        auction.EndAt.Add(30 * time.Minute),
	})

	require.NoError(t, err)
	require.Len(t, tx.execSQL, 1)
	assert.Contains(t, tx.execSQL[0], "UPDATE auctions")
	// Auction content (title, description, preparation) must NEVER be written
	// through the auctions table — Product is the sole canonical authority.
	assert.NotContains(t, tx.execSQL[0], "title")
	assert.NotContains(t, tx.execSQL[0], "description")
	assert.NotContains(t, tx.execSQL[0], "preparation")
	require.Len(t, tx.execArgs, 1)
	assert.Equal(t, int64(1_100_000), tx.execArgs[0][3])
	assert.Equal(t, int64(150_000), tx.execArgs[0][4])
	assert.Nil(t, tx.execArgs[0][5])
}

func TestUpdateDraft_NonOwnerRejected_DoesNotPersist(t *testing.T) {
	sellerID := uuid.New()
	auction := newAuctionForUpdateAuthority(entity.StatusDraft, sellerID)
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     uuid.New(),
		StartPrice:   1_100_000,
		BidIncrement: 150_000,
		StartAt:      auction.StartAt,
		EndAt:        auction.EndAt,
	})

	require.ErrorIs(t, err, auth.ErrSellerRequired)
	assert.Empty(t, tx.execSQL)
}

func TestUpdateDraft_FailedStatusDoesNotPersistRow(t *testing.T) {
	sellerID := uuid.New()
	auction := newAuctionForUpdateAuthority(entity.StatusScheduled, sellerID)
	tx := &auctionUpdateSpyTx{row: auctionUpdateSpyRow{auction: auction}}
	svc := &AuctionService{
		auctionRepo: &auctionRepo.AuctionRepository{},
		ownership:   auth.NewOwnershipValidator(),
		log:         zap.NewNop(),
	}

	err := svc.UpdateDraft(context.Background(), tx, UpdateDraftInput{
		AuctionID:    auction.ID,
		CallerID:     sellerID,
		StartPrice:   1_100_000,
		BidIncrement: 150_000,
		StartAt:      auction.StartAt,
		EndAt:        auction.EndAt,
	})

	require.Error(t, err)
	var invalidOp *entity.InvalidOperationError
	require.ErrorAs(t, err, &invalidOp)
	assert.Empty(t, tx.execSQL)
}

func TestScheduleAuctionInternal_ExpiredSellerIsDenied(t *testing.T) {
	sellerID := uuid.New()
	auction := newAuctionForUpdateAuthority(entity.StatusDraft, sellerID)
	svc := newAuctionServiceForCreateTiming()
	svc.roleChecker = auctionRoleCheckerStub{hasCapability: false}

	err := svc.scheduleAuctionInternal(context.Background(), nil, auction, sellerID)

	require.ErrorIs(t, err, auth.ErrMarketAuthorityRequired)
	assert.Equal(t, entity.StatusDraft, auction.Status)
}
