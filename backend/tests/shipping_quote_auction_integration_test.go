//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	auctionEntity "github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepoImpl "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	orderApp "github.com/labuda/backend/internal/commerce/order/application"
	orderentity "github.com/labuda/backend/internal/commerce/order/entity"
	orderRepo "github.com/labuda/backend/internal/commerce/order/infrastructure/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productInfraRepo "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	shippingQuoteApp "github.com/labuda/backend/internal/commerce/shipping/quote/application"
	shippingQuoteEntity "github.com/labuda/backend/internal/commerce/shipping/quote/entity"
	shippingQuoteRepo "github.com/labuda/backend/internal/commerce/shipping/quote/infrastructure/repository"
	"github.com/labuda/backend/internal/identity/auth"
	chatApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatInfraRepo "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	platformconfigApp "github.com/labuda/backend/internal/platform/config/application"
	platformconfigRepo "github.com/labuda/backend/internal/platform/config/infrastructure/repository"
	pricingtokenApp "github.com/labuda/backend/internal/pricing/token/application"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
)

// ============================================================================
// AUCTION SHIPPING QUOTE — REAL END-TO-END FLOW
//
// OWNER DECISION: auction shipping quotes ARE supported from Chat, following
// the For Sale flow where possible while honoring auction-specific conditions
// (auction must be in waiting_settlement, seller is a participant, and the
// buyer is the auction WINNER — all enforced by the canonical Commerce
// authority, not re-implemented here).
//
// Proves, with the real application services against real Postgres:
//   seller (Chat) -> CreateShippingQuote (auction context) -> atomic quote +
//   conversation message -> winner previews (GenerateForAuction) -> winner
//   checks out (CreateFromAuction) -> ConsumeQuoteForCheckout -> order + USED.
//   plus: wrong buyer (auction winner gate), wrong conversation, expired, reuse.
// ============================================================================

type sqRoomGetterAdapter struct{ repo chatRepo.Repository }

func (a sqRoomGetterAdapter) GetRoomByID(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByID(ctx, tx, roomID)
}
func (a sqRoomGetterAdapter) GetRoomByIDForUpdate(ctx context.Context, tx db.Tx, roomID uuid.UUID) (*chatEntity.ChatRoom, error) {
	return a.repo.GetRoomByIDForUpdate(ctx, tx, roomID)
}

type sqNoopOutbox struct{}

func (sqNoopOutbox) InsertTx(context.Context, db.Tx, string, any, string) error { return nil }

func sqSeedAuctionProduct(t *testing.T, ctx context.Context, tdb *testdb.TestDB, sellerID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	product := &productEntity.Product{
		SellerID:        sellerID,
		Title:           title,
		Description:     "desc",
		Variety:         "Kohaku",
		PreparationTime: "1_3_days",
	}
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return productInfraRepo.NewProductRepository().Create(ctx, tx, product)
	}))
	return product.ID
}

func sqSeedWaitingSettlementAuction(
	t *testing.T, ctx context.Context, tdb *testdb.TestDB,
	sellerID, productID, winnerID uuid.UUID, winningBid int64,
) uuid.UUID {
	t.Helper()
	auction := auctionEntity.NewScheduled(
		sellerID, productID, 50_000, 5_000, nil,
		time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour),
	)
	require.NoError(t, auction.Activate())
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		return auctionRepoImpl.NewAuctionRepository().CreateTx(ctx, tx, auction)
	}))
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE auctions
			SET status = 'waiting_settlement',
			    current_winner_id = $2,
			    current_bid = $3,
			    end_at = NOW() - INTERVAL '1 hour',
			    updated_at = NOW()
			WHERE id = $1
		`, auction.ID, winnerID, winningBid)
		return err
	}))
	return auction.ID
}

func sqNewShippingQuoteService(t *testing.T, ctx context.Context, tdb *testdb.TestDB) (*shippingQuoteApp.Service, *chatApp.Service) {
	t.Helper()
	appDB := db.NewFromPool(tdb.Pool())
	chatSvc := chatApp.NewServiceWithDefaults(
		appDB,
		sqNoopOutbox{},
		rate.NewRateLimiter(),
		nil,
		auth.NewAccountStatusCheckerDB(appDB),
		nil,
		zap.NewNop(),
	)
	quoteSvc := shippingQuoteApp.NewService(
		appDB,
		shippingQuoteRepo.NewShippingQuoteRepository(),
		sqRoomGetterAdapter{repo: chatInfraRepo.NewChatRepository()},
		forsaleRepo.NewForSaleRepository(),
		auctionRepoImpl.NewAuctionRepository(),
		chatSvc,
		orderRepo.NewOrderRepository(),
		zap.NewNop(),
	)
	return quoteSvc, chatSvc
}

func sqAuctionSnapshot(token, quoteID, chatID uuid.UUID, unitPrice, shippingTotal int64) *orderApp.PricingSnapshot {
	shippingSource := "shipping_quote"
	return &orderApp.PricingSnapshot{
		UnitPrice:             money.New(unitPrice),
		Subtotal:              money.New(unitPrice),
		ShippingTotal:         money.New(shippingTotal),
		CommissionPercent:     4,
		CommissionAmount:      money.New(unitPrice * 4 / 100),
		EscrowAmount:          money.New(unitPrice + shippingTotal),
		ServiceFeeAmount:      money.New(0),
		TotalPayableAmount:    money.New(unitPrice + shippingTotal),
		DiscountAmount:        money.New(0),
		OrderValueForCoins:    unitPrice + shippingTotal,
		ShippingSetupName:     "Ongkir Manual",
		ShippingTransportType: "manual",
		ShippingSource:        &shippingSource,
		ShippingQuoteID:       &quoteID,
		ChatID:                &chatID,
		TokenID:               token,
		PaymentMethod:         orderApp.PaymentMethodDefault,
	}
}

func TestShippingQuote_Auction_ProductionFlow(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	winnerID := uuid.New()
	otherID := uuid.New()
	stage5User(t, ctx, tdb, sellerID)
	stage5User(t, ctx, tdb, winnerID)
	stage5User(t, ctx, tdb, otherID)
	stage5Address(t, ctx, tdb, uuid.New(), sellerID, "31")
	winnerAddressID := uuid.New()
	sqAddress(t, ctx, tdb, winnerAddressID, winnerID, "31", "3171")
	otherAddressID := uuid.New()
	sqAddress(t, ctx, tdb, otherAddressID, otherID, "31", "3171")

	// Auction product + auction in waiting_settlement with a winner.
	product := sqSeedAuctionProduct(t, ctx, tdb, sellerID, "SQ Auction Koi")
	const winningBid int64 = 100_000
	auctionID := sqSeedWaitingSettlementAuction(t, ctx, tdb, sellerID, product, winnerID, winningBid)

	// Chat A: seller <-> winner (direct). Chat B: a DIFFERENT conversation for
	// the same pair (different room_type), used for wrong-conversation/expired.
	roomA := sqSeedRoom(t, ctx, tdb, sellerID, winnerID)
	roomB := sqSeedNegotiationRoom(t, ctx, tdb, sellerID, winnerID)
	require.NotEqual(t, roomA, roomB)

	city := "3171"
	province := "31"

	quoteSvc, _ := sqNewShippingQuoteService(t, ctx, tdb)

	// ---- 1-3. Seller creates the auction quote through Chat ----
	quote, err := quoteSvc.CreateShippingQuote(ctx, shippingQuoteApp.CreateShippingQuoteInput{
		ChatID:                roomA,
		ProductID:             product,
		SourceType:            "auction",
		SourceID:              auctionID,
		SellerID:              sellerID,
		Cost:                  money.New(25_000),
		DestinationCityID:     &city,
		DestinationProvinceID: &province,
	})
	require.NoError(t, err)
	require.NotNil(t, quote)
	require.Equal(t, "auction", *quote.SourceType)
	require.Equal(t, auctionID, *quote.SourceID)
	require.Equal(t, winnerID, quote.BuyerID)

	// Quote + conversation message are persisted atomically.
	var msgCount int
	require.NoError(t, tdb.Pool().QueryRow(ctx, `
		SELECT count(*) FROM chat_messages
		WHERE room_id = $1 AND sender_id = $2 AND message_type = 'shipping_quote'
		  AND attachment_json -> 'data' ->> 'offer_id' = $3
	`, roomA, sellerID, quote.ID.String()).Scan(&msgCount))
	require.Equal(t, 1, msgCount, "auction quote must persist its conversation message")

	// ---- Real pricing preview + order services ----
	configSvc := platformconfigApp.NewConfigService(platformconfigRepo.NewPlatformConfigRepository())
	pricingSvc := pricingtokenApp.NewPricingTokenService(configSvc)
	orderSvc := newStage5OrderService()

	// ---- 4-5. Wrong buyer cannot preview (auction winner gate) ----
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := pricingSvc.GenerateForAuction(ctx, tx, &pricingtokenApp.GenerateForAuctionRequest{
			UserID:          otherID,
			AuctionID:       auctionID,
			AddressID:       otherAddressID,
			ShippingQuoteID: &quote.ID,
			ChatID:          &roomA,
		})
		return e
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not the auction winner")

	// ---- 4/7/8/9/10. Winner checks out with the quote -> order + USED ----
	var tokenA uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		res, e := pricingSvc.GenerateForAuction(ctx, tx, &pricingtokenApp.GenerateForAuctionRequest{
			UserID:          winnerID,
			AuctionID:       auctionID,
			AddressID:       winnerAddressID,
			ShippingQuoteID: &quote.ID,
			ChatID:          &roomA,
		})
		if e != nil {
			return e
		}
		tokenA = res.Token
		require.Equal(t, "quote", res.PricingSnapshot.ShippingMode)
		return nil
	}))
	require.NotEqual(t, uuid.Nil, tokenA)

	loadedA := sqLoadPricingToken(t, ctx, tdb, tokenA)
	require.NotNil(t, loadedA.ShippingQuoteID)
	require.Equal(t, quote.ID, *loadedA.ShippingQuoteID)
	require.NotNil(t, loadedA.ChatID)
	require.Equal(t, roomA, *loadedA.ChatID)

	// Mirror the POST /orders bid-win path: validate + lock the pricing token before order
	// creation (quote mode => shipping_option_id is uuid.Nil).
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := pricingSvc.ValidateForOrderLocked(ctx, tx, tokenA, winnerID, product, "auction", auctionID, 0, winnerAddressID, uuid.Nil)
		return e
	}))

	var orderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, e := orderSvc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             auctionID,
			AuctionSellerID:       sellerID,
			ProductID:             product,
			BuyerID:               winnerID,
			WinningBid:            winningBid,
			AddressID:             winnerAddressID,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       sqAuctionSnapshot(tokenA, quote.ID, roomA, winningBid, 25_000),
		})
		if e != nil {
			return e
		}
		orderID = order.ID
		return nil
	}))
	require.NotEqual(t, uuid.Nil, orderID)

	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM shipping_quotes WHERE id = $1`, quote.ID).Scan(&status))
	require.Equal(t, string(shippingQuoteEntity.QuoteStatusUsed), status)

	var sourceType, shipSource string
	var orderQuoteID uuid.UUID
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT source_type, shipping_source, shipping_quote_id FROM orders WHERE id = $1`, orderID).
		Scan(&sourceType, &shipSource, &orderQuoteID))
	require.Equal(t, "auction", sourceType)
	require.Equal(t, "shipping_quote", shipSource)
	require.Equal(t, quote.ID, orderQuoteID)

	// ---- 12. USED quote cannot produce a second order ----
	var reusedOrderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, e := orderSvc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             auctionID,
			AuctionSellerID:       sellerID,
			ProductID:             product,
			BuyerID:               winnerID,
			WinningBid:            winningBid,
			AddressID:             winnerAddressID,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       sqAuctionSnapshot(uuid.New(), quote.ID, roomA, winningBid, 25_000),
		})
		if e != nil {
			return e
		}
		reusedOrderID = order.ID
		return nil
	}))
	require.Equal(t, orderID, reusedOrderID, "reusing a USED auction quote must converge on the existing order")

	// ---- 6. Same buyer + WRONG conversation is rejected ----
	quote2, err := quoteSvc.CreateShippingQuote(ctx, shippingQuoteApp.CreateShippingQuoteInput{
		ChatID:                roomA,
		ProductID:             product,
		SourceType:            "auction",
		SourceID:              auctionID,
		SellerID:              sellerID,
		Cost:                  money.New(25_000),
		DestinationCityID:     &city,
		DestinationProvinceID: &province,
	})
	require.NoError(t, err)

	var tokenB uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		res, e := pricingSvc.GenerateForAuction(ctx, tx, &pricingtokenApp.GenerateForAuctionRequest{
			UserID:          winnerID,
			AuctionID:       auctionID,
			AddressID:       winnerAddressID,
			ShippingQuoteID: &quote2.ID,
			ChatID:          &roomB, // wrong conversation
		})
		if e != nil {
			return e
		}
		tokenB = res.Token
		return nil
	}))

	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := orderSvc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             auctionID,
			AuctionSellerID:       sellerID,
			ProductID:             product,
			BuyerID:               winnerID,
			WinningBid:            winningBid,
			AddressID:             winnerAddressID,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       sqAuctionSnapshot(tokenB, quote2.ID, roomB, winningBid, 25_000),
		})
		return e
	})
	var rejection *shippingQuoteEntity.CheckoutRejectionError
	require.ErrorAs(t, err, &rejection)
	require.Equal(t, "chat_mismatch", rejection.Field)

	// ---- 11. EXPIRED auction quote cannot be used ----
	quote3, err := quoteSvc.CreateShippingQuote(ctx, shippingQuoteApp.CreateShippingQuoteInput{
		ChatID:                roomB,
		ProductID:             product,
		SourceType:            "auction",
		SourceID:              auctionID,
		SellerID:              sellerID,
		Cost:                  money.New(25_000),
		DestinationCityID:     &city,
		DestinationProvinceID: &province,
	})
	require.NoError(t, err)
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := tx.Exec(ctx, `UPDATE shipping_quotes SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, quote3.ID)
		return e
	}))

	var tokenC uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		res, e := pricingSvc.GenerateForAuction(ctx, tx, &pricingtokenApp.GenerateForAuctionRequest{
			UserID:          winnerID,
			AuctionID:       auctionID,
			AddressID:       winnerAddressID,
			ShippingQuoteID: &quote3.ID,
			ChatID:          &roomB,
		})
		if e != nil {
			return e
		}
		tokenC = res.Token
		return nil
	}))
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		_, e := orderSvc.CreateFromAuction(ctx, tx, orderApp.CreateFromAuctionInput{
			AuctionID:             auctionID,
			AuctionSellerID:       sellerID,
			ProductID:             product,
			BuyerID:               winnerID,
			WinningBid:            winningBid,
			AddressID:             winnerAddressID,
			AuctionSettlementType: orderentity.AuctionSettlementBidWin,
			PricingSnapshot:       sqAuctionSnapshot(tokenC, quote3.ID, roomB, winningBid, 25_000),
		})
		return e
	})
	var expiredRejection *shippingQuoteEntity.CheckoutRejectionError
	require.ErrorAs(t, err, &expiredRejection)
	require.Equal(t, "expired", expiredRejection.Field)
}
