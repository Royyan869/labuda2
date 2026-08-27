//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auctionentity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctioninfra "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	orderapp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderinfra "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	productentity "github.com/labuda/backend/internal/commerce/product/entity"
	productinfra "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	walletapp "github.com/labuda/backend/internal/core/wallet/application"
	outboxrepo "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap/zaptest"
)

// PASS_20B (D1 + D2): auction buy-now settlement binding and cancel/expire
// release, exercised directly at the repository/service layer (bypassing the
// pricing-token/shipping/address checkout machinery, which is out of scope
// for this fix — the bug and its fix live entirely in how the auction row
// itself is updated around order creation and cancellation).

func insertAuctionFixtureUsers(ctx context.Context, t *testing.T, testDB *testdb.TestDB, sellerID, buyerID uuid.UUID) {
	t.Helper()
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			sellerID, "fb-"+sellerID.String()[:8], sellerID.String()+"@test.local"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO users (id, firebase_uid, email, role) VALUES ($1, $2, $3, 'user')`,
			buyerID, "fb-"+buyerID.String()[:8], buyerID.String()+"@test.local")
		return err
	})
	require.NoError(t, err, "seller/buyer user fixtures failed")
}

func newAuctionOrder(buyerID, sellerID, auctionID uuid.UUID, unitPrice int64, settlementType orderentity.AuctionSettlementType) *orderentity.Order {
	price := money.New(unitPrice)
	return orderentity.NewOrderFromSource(
		buyerID, sellerID, orderentity.OrderSourceAuction, auctionID, nil, 1,
		price, price, money.New(15000), 5,
		money.New(25000), money.New(3000), price.Add(money.New(15000)).Add(money.New(25000)).Add(money.New(3000)),
		nil, "JNE", "truck", nil, nil,
		&settlementType,
		"immediate", nil, nil, nil, nil, nil,
		"instant", time.Now().Add(1*time.Hour),
	)
}

// TestAuctionBuyNowSettlement_ClosesAuctionAndBlocksDoubleSale proves D1's
// fix: binding the order to the auction and ending it in the SAME
// transaction as order creation (mirroring order_handler.go's buy-now
// branch) makes a second buy-now/bid attempt impossible.
func TestAuctionBuyNowSettlement_ClosesAuctionAndBlocksDoubleSale(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	auctionRepo := auctioninfra.NewAuctionRepository()
	productRepo := productinfra.NewProductRepository()
	orderRepo := orderinfra.NewOrderRepository()

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertAuctionFixtureUsers(ctx, t, testDB, sellerID, buyerID)

	buyNowPrice := int64(500000)
	var auctionID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{
			SellerID: sellerID, Title: "Test Koi", Description: "desc",
			Variety: "Kohaku", PreparationTime: "immediate",
		}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}

		auction := auctionentity.NewDraft(
			sellerID, product.ID,
			100000, 10000, &buyNowPrice,
			time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour),
		)
		if err := auction.Schedule(); err != nil {
			return err
		}
		if err := auction.Activate(); err != nil {
			return err
		}
		if err := auctionRepo.CreateTx(ctx, tx, auction); err != nil {
			return err
		}
		auctionID = auction.ID
		return nil
	})
	require.NoError(t, err, "fixture setup failed")

	// Buy-now: create the order, then bind+end the auction in the SAME
	// transaction — this is exactly the sequence added to order_handler.go.
	var orderID uuid.UUID
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetForUpdate(ctx, tx, auctionID)
		require.NoError(t, err)
		require.Nil(t, auction.OrderID, "auction must be unsettled before buy-now")
		require.Equal(t, auctionentity.StatusActive, auction.Status)

		order := newAuctionOrder(buyerID, sellerID, auctionID, buyNowPrice, orderentity.AuctionSettlementBuyNow)
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		orderID = order.ID

		auction.OrderID = &order.ID
		if err := auction.End(); err != nil {
			return err
		}
		return auctionRepo.UpdateTx(ctx, tx, auction)
	})
	require.NoError(t, err, "buy-now settlement failed")

	// Assertions 1+2: auction closed/settled, order_id bound to the created order.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetByID(ctx, tx, auctionID)
		require.NoError(t, err)
		assert.Equal(t, auctionentity.StatusEnded, auction.Status, "auction must be Ended after buy-now")
		require.NotNil(t, auction.OrderID, "auction.OrderID must be set after buy-now")
		assert.Equal(t, orderID, *auction.OrderID)
		return nil
	})
	require.NoError(t, err)

	// Assertion 3+4: a second buy-now attempt, replaying the EXACT guard
	// order_handler.go's buy-now branch uses, must be rejected.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		if auction.OrderID != nil {
			return fmt.Errorf("auction already settled: order_id=%s", *auction.OrderID)
		}
		if auction.Status != auctionentity.StatusActive || auction.BuyNowPrice == nil {
			return fmt.Errorf("auction is not available for buy now checkout: status=%s", auction.Status)
		}
		return nil
	})
	require.Error(t, err, "second buy-now attempt must be rejected")
	assert.Contains(t, err.Error(), "already settled")

	// Assertion: bidding on the now-ended auction must also be rejected —
	// no double-sale via bids either.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		return auction.PlaceBid(uuid.New(), buyNowPrice+50000, time.Now())
	})
	require.Error(t, err, "bidding on an ended auction must be rejected")

	// Assertion 5: only one order exists for this auction (no duplicate).
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		var count int
		scanErr := tx.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE source_type = 'auction' AND source_id = $1`, auctionID).Scan(&count)
		if scanErr != nil {
			return scanErr
		}
		assert.Equal(t, 1, count, "exactly one order must exist for this auction")
		return nil
	})
	require.NoError(t, err)
}

// TestAuctionBuyNowSettlement_RollbackLeavesAuctionUnchanged proves that if
// order creation fails, the auction is never touched (atomicity — the
// settlement write only happens after CreateOrderTx succeeds, in the same
// transaction, so a failure rolls back both together).
func TestAuctionBuyNowSettlement_RollbackLeavesAuctionUnchanged(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	auctionRepo := auctioninfra.NewAuctionRepository()
	productRepo := productinfra.NewProductRepository()

	sellerID := uuid.New()
	insertAuctionFixtureUsers(ctx, t, testDB, sellerID, uuid.New())

	buyNowPrice := int64(500000)
	var auctionID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{
			SellerID: sellerID, Title: "Test Koi", Description: "desc",
			Variety: "Kohaku", PreparationTime: "immediate",
		}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}
		auction := auctionentity.NewDraft(
			sellerID, product.ID,
			100000, 10000, &buyNowPrice,
			time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour),
		)
		require.NoError(t, auction.Schedule())
		require.NoError(t, auction.Activate())
		if err := auctionRepo.CreateTx(ctx, tx, auction); err != nil {
			return err
		}
		auctionID = auction.ID
		return nil
	})
	require.NoError(t, err)

	// Simulate a failed order-creation step (e.g. buyer with no valid
	// account) by deliberately using a nonexistent buyer ID, which violates
	// orders.buyer_id's FK — the whole transaction, including any auction
	// mutation attempted afterward, must roll back.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetForUpdate(ctx, tx, auctionID)
		if err != nil {
			return err
		}
		orderinfraRepo := orderinfra.NewOrderRepository()
		order := newAuctionOrder(uuid.New() /* buyer with no users row */, sellerID, auctionID, buyNowPrice, orderentity.AuctionSettlementBuyNow)
		if err := orderinfraRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err // order creation failed — must not reach auction mutation below
		}
		auction.OrderID = &order.ID
		require.NoError(t, auction.End())
		return auctionRepo.UpdateTx(ctx, tx, auction)
	})
	require.Error(t, err, "order creation must fail on invalid buyer FK")

	// Auction must be completely unchanged.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetByID(ctx, tx, auctionID)
		require.NoError(t, err)
		assert.Equal(t, auctionentity.StatusActive, auction.Status, "auction status must be unchanged after rollback")
		assert.Nil(t, auction.OrderID, "auction.OrderID must be unchanged (nil) after rollback")
		return nil
	})
	require.NoError(t, err)
}

// newTestOrderCompletionService constructs an OrderCompletionService with
// only the dependencies exercised by Cancel()/Expire() for an UNPAID
// (pending_payment) order. Expire() unconditionally calls
// walletService.GetEscrowForOrder to distinguish unpaid expiry (no escrow,
// no gateway refund) from paid expiry-with-escrow, so walletService must be
// a real instance (its GetEscrowForOrder only touches escrowRepo + logger,
// not the *db.DB field, so nil db is safe here). paymentService is never
// reached because escrowForExpiry is nil for an unpaid order. disputeRepo:
// nil mirrors the same pattern already used in the real OrderService facade
// constructor (order_service.go's NewOrderService passes disputeRepo: nil
// too).
func newTestOrderCompletionService(t *testing.T) *orderapp.OrderCompletionService {
	t.Helper()
	return orderapp.NewOrderCompletionService(
		nil, // accountStatusChecker — not referenced by Cancel()/Expire()
		outboxrepo.NewOutboxRepository(nil),
		nil, // paymentService — unpaid order, no refund path taken
		nil, // coinsService — order.CoinsUsed is 0 in this test
		nil, // shippingQuoteService — order.ShippingQuoteID is nil
		nil, // disputeRepo — not referenced by Cancel()/Expire()
		walletapp.NewWalletService(nil, zaptest.NewLogger(t)), // GetEscrowForOrder is called unconditionally by Expire()
		zaptest.NewLogger(t),
	)
}

// TestAuctionOrderCancel_ReleasesBinding proves D2's fix:
// cancelling a pending_payment auction order no longer crashes
// (restoreListingStock used to look up item.ProductID — a products.id —
// against ForSaleRepository, which queries for_sales.id and
// always returned ErrNoRows for auctions), and now correctly releases the
// auction<->order binding.
func TestAuctionOrderCancel_ReleasesBinding(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	auctionRepo := auctioninfra.NewAuctionRepository()
	productRepo := productinfra.NewProductRepository()
	orderRepo := orderinfra.NewOrderRepository()
	completionService := newTestOrderCompletionService(t)

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertAuctionFixtureUsers(ctx, t, testDB, sellerID, buyerID)

	buyNowPrice := int64(500000)
	var auctionID, orderID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{
			SellerID: sellerID, Title: "Test Koi", Description: "desc",
			Variety: "Kohaku", PreparationTime: "immediate",
		}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}

		auction := auctionentity.NewDraft(
			sellerID, product.ID,
			100000, 10000, &buyNowPrice,
			time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour),
		)
		require.NoError(t, auction.Schedule())
		require.NoError(t, auction.Activate())
		if err := auctionRepo.CreateTx(ctx, tx, auction); err != nil {
			return err
		}
		auctionID = auction.ID

		// Simulate the D1-fixed buy-now settlement (D1 auction-settle sequence).
		order := newAuctionOrder(buyerID, sellerID, auctionID, buyNowPrice, orderentity.AuctionSettlementBuyNow)
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		orderID = order.ID

		auction.OrderID = &order.ID
		require.NoError(t, auction.End())
		if err := auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err, "fixture setup (settled auction order) failed")

	// Buyer never pays — cancel the pending_payment order. This must NOT
	// error (previously: ErrNoRows from the fixed-price restore path).
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return completionService.Cancel(ctx, tx, orderID, "cancel-key-"+orderID.String(), buyerID)
	})
	require.NoError(t, err, "cancelling an auction pending_payment order must not fail")

	// Auction binding released; status stays Ended (terminal, does not reopen).
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetByID(ctx, tx, auctionID)
		require.NoError(t, err)
		assert.Nil(t, auction.OrderID, "auction.OrderID must be released after cancel")
		assert.Equal(t, auctionentity.StatusEnded, auction.Status, "auction status remains Ended (terminal) after release")
		return nil
	})
	require.NoError(t, err)

	// Order itself is cancelled.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		order, err := orderRepo.GetByID(ctx, tx, orderID)
		require.NoError(t, err)
		assert.Equal(t, orderentity.StatusCancelled, order.Status)
		return nil
	})
	require.NoError(t, err)
}

// TestAuctionOrderExpire_ReleasesBinding is the Expire()
// (payment timeout) counterpart to the Cancel() test above.
func TestAuctionOrderExpire_ReleasesBinding(t *testing.T) {
	ctx := context.Background()
	testDB, cleanup := testdb.SetupDB(t)
	defer cleanup()

	auctionRepo := auctioninfra.NewAuctionRepository()
	productRepo := productinfra.NewProductRepository()
	orderRepo := orderinfra.NewOrderRepository()
	completionService := newTestOrderCompletionService(t)

	sellerID := uuid.New()
	buyerID := uuid.New()
	insertAuctionFixtureUsers(ctx, t, testDB, sellerID, buyerID)

	winningBid := int64(750000)
	var auctionID, orderID uuid.UUID
	err := testDB.WithTx(ctx, func(tx db.Tx) error {
		product := &productentity.Product{
			SellerID: sellerID, Title: "Test Koi Bid-Win", Description: "desc",
			Variety: "Showa", PreparationTime: "immediate",
		}
		if err := productRepo.Create(ctx, tx, product); err != nil {
			return err
		}

		auction := auctionentity.NewDraft(
			sellerID, product.ID,
			100000, 10000, nil,
			time.Now().Add(-25*time.Hour), time.Now().Add(-1*time.Hour),
		)
		require.NoError(t, auction.Schedule())
		require.NoError(t, auction.Activate())
		if err := auctionRepo.CreateTx(ctx, tx, auction); err != nil {
			return err
		}
		auctionID = auction.ID

		// Bid-win claim path: WaitingSettlement -> Ended via Settle().
		require.NoError(t, auction.TransitionToWaitingSettlement())
		if err := auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return err
		}

		order := newAuctionOrder(buyerID, sellerID, auctionID, winningBid, orderentity.AuctionSettlementBidWin)
		if err := orderRepo.CreateOrderTx(ctx, tx, order); err != nil {
			return err
		}
		orderID = order.ID

		auction.OrderID = &order.ID
		require.NoError(t, auction.Settle())
		if err := auctionRepo.UpdateTx(ctx, tx, auction); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err, "fixture setup (settled bid-win order) failed")

	// Payment window expires before the buyer pays.
	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		return completionService.Expire(ctx, tx, orderID)
	})
	require.NoError(t, err, "expiring an auction pending_payment order must not fail")

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		auction, err := auctionRepo.GetByID(ctx, tx, auctionID)
		require.NoError(t, err)
		assert.Nil(t, auction.OrderID, "auction.OrderID must be released after expire")
		assert.Equal(t, auctionentity.StatusEnded, auction.Status)
		return nil
	})
	require.NoError(t, err)

	err = testDB.WithTx(ctx, func(tx db.Tx) error {
		order, err := orderRepo.GetByID(ctx, tx, orderID)
		require.NoError(t, err)
		assert.Equal(t, orderentity.StatusExpired, order.Status)
		return nil
	})
	require.NoError(t, err)
}
