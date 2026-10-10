//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	orderApp "github.com/hishumi/backend/internal/commerce/order/application"
	orderentity "github.com/hishumi/backend/internal/commerce/order/entity"
	shippingquoteEntity "github.com/hishumi/backend/internal/commerce/shipping/quote/entity"
	platformconfigApp "github.com/hishumi/backend/internal/platform/config/application"
	platformconfigRepo "github.com/hishumi/backend/internal/platform/config/infrastructure/repository"
	pricingtokenApp "github.com/hishumi/backend/internal/pricing/token/application"
	pricingtokenRepo "github.com/hishumi/backend/internal/pricing/token/infrastructure/repository"
	"github.com/hishumi/backend/pkg/db"
	"github.com/hishumi/backend/pkg/money"
	"github.com/hishumi/backend/pkg/testdb"
)

// ============================================================================
// SHIPPING QUOTE — CONVERSATION SCOPE ON THE PRODUCTION CHECKOUT PATH
//
// OWNER INVARIANT: a Shipping Quote only applies to the opponent of the Chat
// that created it. A quote created in Chat A must NOT be usable from Chat B,
// even when Chat B has the same seller and buyer.
//
// This drives the REAL application path end to end:
//   PricingTokenService.GenerateForForSale (the preview authority)
//     -> pricing_tokens persistence (shipping_quote_id + chat_id)
//     -> reload through the real repository
//     -> OrderCreationService.CreateFromSaleSurface (the order authority)
//     -> ShippingQuoteService.ConsumeQuoteForCheckout (the ONE quote authority)
//
// and proves: correct conversation succeeds + quote becomes USED; a different
// conversation for the SAME buyer/seller is rejected and does not consume it.
// ============================================================================

// sqSeedNegotiationRoom creates a SECOND chat for the same buyer/seller pair
// using a different room_type (the unique pair index is per room_type). This is
// a genuinely different conversation, not the same canonical direct room.
func sqSeedNegotiationRoom(t *testing.T, ctx context.Context, tdb *testdb.TestDB, userA, userB uuid.UUID) uuid.UUID {
	t.Helper()
	if userA.String() > userB.String() {
		userA, userB = userB, userA
	}
	roomID := uuid.New()
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
			VALUES ($1, 'negotiation', $2, $3, NOW(), NOW(), NOW())
		`, roomID, userA, userB)
		return err
	}))
	return roomID
}

type pricingTokenRow struct {
	ShippingQuoteID *uuid.UUID
	ChatID          *uuid.UUID
}

func sqLoadPricingToken(t *testing.T, ctx context.Context, tdb *testdb.TestDB, token uuid.UUID) pricingTokenRow {
	t.Helper()
	repo := pricingtokenRepo.NewPricingTokenRepository()
	var loaded pricingTokenRow
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		pt, err := repo.GetByTokenForUpdate(ctx, tx, token)
		if err != nil {
			return err
		}
		loaded = pricingTokenRow{ShippingQuoteID: pt.ShippingQuoteID, ChatID: pt.ChatID}
		return nil
	}))
	return loaded
}

// sqGenerateQuoteToken runs the REAL pricing preview authority for a manual
// shipping quote and returns the persisted token id.
func sqGenerateQuoteToken(
	t *testing.T,
	ctx context.Context,
	tdb *testdb.TestDB,
	svc *pricingtokenApp.PricingTokenService,
	buyerID, productID, saleID, quoteID, chatID, addressID uuid.UUID,
) uuid.UUID {
	t.Helper()
	var token uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		res, err := svc.GenerateForForSale(ctx, tx, &pricingtokenApp.GenerateForForSaleRequest{
			UserID:          buyerID,
			ProductID:       productID,
			SourceType:      "for_sale",
			SourceID:        saleID,
			Quantity:        1,
			ShippingQuoteID: &quoteID,
			ChatID:          &chatID,
			AddressID:       addressID,
		})
		if err != nil {
			return err
		}
		token = res.Token
		return nil
	}))
	return token
}

func sqConversationSnapshot(token, quoteID, chatID uuid.UUID) *orderApp.PricingSnapshot {
	shippingSource := "shipping_quote"
	return &orderApp.PricingSnapshot{
		UnitPrice:             money.New(100_000),
		Subtotal:              money.New(100_000),
		ShippingTotal:         money.New(25_000),
		CommissionPercent:     4,
		CommissionAmount:      money.New(4_000),
		EscrowAmount:          money.New(125_000),
		ServiceFeeAmount:      money.New(3_000),
		TotalPayableAmount:    money.New(128_000),
		DiscountAmount:        money.New(0),
		OrderValueForCoins:    125_000,
		ShippingSetupName:     "Ongkir Manual",
		ShippingTransportType: "manual",
		ShippingSource:        &shippingSource,
		ShippingQuoteID:       &quoteID,
		ChatID:                &chatID,
		TokenID:               token,
		PaymentMethod:         orderApp.PaymentMethodInstant,
	}
}

func TestShippingQuote_ConversationScope_ProductionCheckoutPath(t *testing.T) {
	ctx := context.Background()
	tdb, cleanup := testdb.SetupDB(t)
	defer cleanup()

	sellerID := uuid.New()
	buyerID := uuid.New()
	stage5User(t, ctx, tdb, sellerID)
	stage5User(t, ctx, tdb, buyerID)
	stage5Address(t, ctx, tdb, uuid.New(), sellerID, "31")
	buyerAddressID := uuid.New()
	sqAddress(t, ctx, tdb, buyerAddressID, buyerID, "31", "3171")

	productID, saleID := sqActiveForSale(t, ctx, tdb, sellerID, "SQ Conversation Koi", 2)

	// Chat A is the direct conversation the quote is created in.
	roomA := sqSeedRoom(t, ctx, tdb, sellerID, buyerID)
	// Chat B is a DIFFERENT conversation for the SAME buyer/seller pair.
	roomB := sqSeedNegotiationRoom(t, ctx, tdb, sellerID, buyerID)
	require.NotEqual(t, roomA, roomB)

	city := "3171"
	province := "31"

	configSvc := platformconfigApp.NewConfigService(platformconfigRepo.NewPlatformConfigRepository())
	pricingSvc := pricingtokenApp.NewPricingTokenService(configSvc)
	orderSvc := newStage5OrderService()

	// ---- POSITIVE: quote created in Chat A, checked out from Chat A ----
	quote := shippingquoteEntity.NewShippingQuote(
		roomA, productID, "for_sale", saleID, sellerID, buyerID,
		money.New(25_000), nil, &city, &province, time.Now().Add(24*time.Hour),
	)
	sqCreateQuote(t, ctx, tdb, quote)

	tokenA := sqGenerateQuoteToken(t, ctx, tdb, pricingSvc, buyerID, productID, saleID, quote.ID, roomA, buyerAddressID)

	loaded := sqLoadPricingToken(t, ctx, tdb, tokenA)
	require.NotNil(t, loaded.ShippingQuoteID, "pricing token must persist shipping_quote_id")
	require.Equal(t, quote.ID, *loaded.ShippingQuoteID)
	require.NotNil(t, loaded.ChatID, "pricing token must persist the conversation id")
	require.Equal(t, roomA, *loaded.ChatID)

	var orderID uuid.UUID
	require.NoError(t, tdb.WithTx(ctx, func(tx db.Tx) error {
		order, err := orderSvc.CreateFromSaleSurface(ctx, tx, orderApp.CreateFromSaleSurfaceInput{
			ProductID:       productID,
			SourceType:      orderentity.OrderSourceForSale,
			SourceID:        saleID,
			BuyerID:         buyerID,
			Quantity:        1,
			AddressID:       buyerAddressID,
			PricingSnapshot: sqConversationSnapshot(tokenA, quote.ID, roomA),
		})
		if err != nil {
			return err
		}
		orderID = order.ID
		return nil
	}))
	require.NotEqual(t, uuid.Nil, orderID)
	var status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM shipping_quotes WHERE id = $1`, quote.ID).Scan(&status))
	require.Equal(t, string(shippingquoteEntity.QuoteStatusUsed), status)

	// ---- NEGATIVE: SAME BUYER, DIFFERENT CHAT is rejected ----
	quote2 := shippingquoteEntity.NewShippingQuote(
		roomA, productID, "for_sale", saleID, sellerID, buyerID,
		money.New(25_000), nil, &city, &province, time.Now().Add(24*time.Hour),
	)
	// quote (roomA context) is now USED, so this ACTIVE quote occupies the
	// one-current-active-quote index for the roomA context without colliding.
	sqCreateQuote(t, ctx, tdb, quote2)

	// The preview mints a token for the SAME quote but claiming Chat B.
	tokenB := sqGenerateQuoteToken(t, ctx, tdb, pricingSvc, buyerID, productID, saleID, quote2.ID, roomB, buyerAddressID)
	loadedB := sqLoadPricingToken(t, ctx, tdb, tokenB)
	require.NotNil(t, loadedB.ChatID)
	require.Equal(t, roomB, *loadedB.ChatID)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := orderSvc.CreateFromSaleSurface(ctx, tx, orderApp.CreateFromSaleSurfaceInput{
			ProductID:       productID,
			SourceType:      orderentity.OrderSourceForSale,
			SourceID:        saleID,
			BuyerID:         buyerID,
			Quantity:        1,
			AddressID:       buyerAddressID,
			PricingSnapshot: sqConversationSnapshot(tokenB, quote2.ID, roomB),
		})
		return err
	})
	var rejection *shippingquoteEntity.CheckoutRejectionError
	require.ErrorAs(t, err, &rejection)
	require.Equal(t, "chat_mismatch", rejection.Field,
		"a quote created in Chat A must be rejected when checked out from Chat B")

	var quote2Status string
	require.NoError(t, tdb.Pool().QueryRow(ctx,
		`SELECT status FROM shipping_quotes WHERE id = $1`, quote2.ID).Scan(&quote2Status))
	require.Equal(t, string(shippingquoteEntity.QuoteStatusActive), quote2Status,
		"rejected cross-conversation checkout must not consume the quote")
}
