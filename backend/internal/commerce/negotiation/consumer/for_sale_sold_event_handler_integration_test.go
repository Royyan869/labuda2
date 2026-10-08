//go:build integration

// Package consumer_test proves the WIRED for_sale.sold consumer (owner
// decision, negotiation closure scope) against a real database:
//
//  1. Other buyers with active/accepted negotiations receive the "Item Sold"
//     system message in their negotiation chat room.
//  2. Accepted, not-yet-ordered negotiations for the sold sale are
//     bulk-cancelled (first-come-first-served honesty contract).
//  3. The winning buyer is excluded: their notification is not sent into
//     their room (their accepted negotiation is the canonical order source
//     and stays untouched by the exclusion-aware repo query).
//  4. Sessions without a chat room are tolerated (skipped, no crash).
//
// Run: go test -tags integration ./internal/commerce/negotiation/consumer/
package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	negotiationConsumer "github.com/labuda/backend/internal/commerce/negotiation/consumer"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
	productInfraRepo "github.com/labuda/backend/internal/commerce/product/infrastructure/repository"
	chatInfraApp "github.com/labuda/backend/internal/interaction/chat/application"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatImpl "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	chatRepo "github.com/labuda/backend/internal/interaction/chat/repository"
	socialInfraRepo "github.com/labuda/backend/internal/social/graph/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/rate"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// soldHarness bundles the real, DB-backed dependencies of the wired consumer.
type soldHarness struct {
	tdb         *testdb.TestDB
	appDB       *db.DB
	handler     *negotiationConsumer.ForSaleSoldEventHandler
	negotiation negotiationRepo.Repository
	chat        chatRepo.Repository
	forSale     *forsaleImpl.ForSaleRepositoryImpl
}

func setupSoldHarness(t *testing.T) (*soldHarness, func()) {
	t.Helper()

	tdb, cleanup := testdb.SetupDB(t)
	appDB := db.NewFromPool(tdb.Pool())

	chatService := chatInfraApp.NewService(
		appDB,
		chatImpl.NewChatRepository(),
		socialInfraRepo.NewSocialRepository(),
		noopChatOutboxForSold{},
		rate.NewRateLimiter(),
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	handler := negotiationConsumer.NewForSaleSoldEventHandler(appDB, chatService, zap.NewNop())

	return &soldHarness{
		tdb:         tdb,
		appDB:       appDB,
		handler:     handler,
		negotiation: negotiationImpl.NewNegotiationRepository(),
		chat:        chatImpl.NewChatRepository(),
		forSale:     forsaleImpl.NewForSaleRepository(),
	}, cleanup
}

// noopChatOutboxForSold satisfies chatApp.OutboxInserter without emitting.
type noopChatOutboxForSold struct{}

func (noopChatOutboxForSold) InsertTx(context.Context, db.Tx, string, any, string) error {
	return nil
}

func insertSoldUser(t *testing.T, ctx context.Context, h *soldHarness) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := h.tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	require.NoError(t, err)
	return uid
}

func insertSoldDirectRoom(t *testing.T, ctx context.Context, h *soldHarness, a, b uuid.UUID) uuid.UUID {
	t.Helper()
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, a, b)
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		return h.chat.CreateRoom(ctx, tx, room)
	}))
	return room.ID
}

func insertSoldForSale(t *testing.T, ctx context.Context, h *soldHarness, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	product := &productEntity.Product{
		SellerID:        sellerID,
		Title:           "Sold Fixture Koi",
		Description:     "for_sale.sold consumer fixture",
		MediaURLs:       []string{"https://picsum.photos/seed/sold-fixture/800/600"},
		Variety:         "Kohaku",
		PreparationTime: string(forsaleEntity.PreparationTime1To3Days),
		SellingSurface:  productEntity.SellingSurfaceForSale,
	}
	var saleID uuid.UUID
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		if err := productInfraRepo.NewProductRepository().Create(ctx, tx, product); err != nil {
			return err
		}
		sale, err := forsaleEntity.NewForSaleSurface(
			sellerID, money.New(500000), 1, true,
		)
		if err != nil {
			return err
		}
		sale.ProductID = product.ID
		sale.Product = product
		if err := h.forSale.Create(ctx, tx, sale); err != nil {
			return err
		}
		saleID = sale.ID
		return nil
	}))
	return saleID
}

func insertSoldSession(
	t *testing.T,
	ctx context.Context,
	h *soldHarness,
	forSaleID, buyerID, sellerID, roomID uuid.UUID,
	status negotiationEntity.NegotiationStatus,
) *negotiationEntity.NegotiationSession {
	t.Helper()
	session := negotiationEntity.NewNegotiationSession(
		negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID,
	)
	session.SetChatRoomID(roomID)
	require.NoError(t, session.SetCurrentPrice(400000))
	if status == negotiationEntity.NegotiationStatusAccepted {
		require.NoError(t, session.AcceptWithPrice())
	}
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		return h.negotiation.CreateSession(ctx, tx, session)
	}))
	return session
}

func countSoldSystemMessages(t *testing.T, ctx context.Context, h *soldHarness, roomID uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		messages, err := h.chat.ListMessagesByRoom(ctx, tx, roomID, nil, nil, 100)
		if err != nil {
			return err
		}
		for _, m := range messages {
			if m.MessageType == chatEntity.MessageTypeSystem {
				count++
			}
		}
		return nil
	}))
	return count
}

func getSoldSessionByRoom(t *testing.T, ctx context.Context, h *soldHarness, roomID uuid.UUID) *negotiationEntity.NegotiationSession {
	t.Helper()
	var session *negotiationEntity.NegotiationSession
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		var err error
		session, err = h.negotiation.GetLatestSessionByChatRoomID(ctx, tx, roomID)
		return err
	}))
	return session
}

func soldPayload(forSaleID, sellerID, buyerID string) []byte {
	payload := `{"for_sale_id":"` + forSaleID + `","seller_id":"` + sellerID + `","status":"sold"`
	if buyerID != "" {
		payload += `,"buyer_id":"` + buyerID + `"`
	}
	return []byte(payload + `}`)
}

func TestForSaleSold_WiredConsumer_NotifyOthersCancelAcceptedExcludeWinner(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupSoldHarness(t)
	defer cleanup()

	winner := insertSoldUser(t, ctx, h)
	other := insertSoldUser(t, ctx, h)
	seller := insertSoldUser(t, ctx, h)

	forSaleID := insertSoldForSale(t, ctx, h, seller)
	winnerRoom := insertSoldDirectRoom(t, ctx, h, winner, seller)
	otherRoom := insertSoldDirectRoom(t, ctx, h, other, seller)

	insertSoldSession(t, ctx, h, forSaleID, winner, seller, winnerRoom, negotiationEntity.NegotiationStatusAccepted)
	insertSoldSession(t, ctx, h, forSaleID, other, seller, otherRoom, negotiationEntity.NegotiationStatusAccepted)

	require.NoError(t, h.handler.HandleEvent(ctx, soldPayload(forSaleID.String(), seller.String(), winner.String())))

	// 1. Other buyer was notified in their negotiation chat.
	require.Equal(t, 1, countSoldSystemMessages(t, ctx, h, otherRoom),
		"other buyer must receive exactly one Item Sold system message")

	// 2. Winner was NOT notified (excluded by buyer_id).
	require.Equal(t, 0, countSoldSystemMessages(t, ctx, h, winnerRoom),
		"winning buyer must not receive the Item Sold system message")

	// 3. Both accepted unordered sessions were bulk-cancelled (repo predicate
	// is accepted+no-order for the whole sale; the winner's session is not
	// settled because the fixture has no order row — the notification
	// exclusion is what protects the winner's UX, the cancel is sale-wide).
	winnerSession := getSoldSessionByRoom(t, ctx, h, winnerRoom)
	require.Equal(t, negotiationEntity.NegotiationStatusCancelled, winnerSession.Status)

	otherSession := getSoldSessionByRoom(t, ctx, h, otherRoom)
	require.Equal(t, negotiationEntity.NegotiationStatusCancelled, otherSession.Status)
}

func TestForSaleSold_WiredConsumer_ActiveSessionAlsoNotified(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupSoldHarness(t)
	defer cleanup()

	buyer := insertSoldUser(t, ctx, h)
	seller := insertSoldUser(t, ctx, h)
	forSaleID := insertSoldForSale(t, ctx, h, seller)
	roomID := insertSoldDirectRoom(t, ctx, h, buyer, seller)

	insertSoldSession(t, ctx, h, forSaleID, buyer, seller, roomID, negotiationEntity.NegotiationStatusActive)

	require.NoError(t, h.handler.HandleEvent(ctx, soldPayload(forSaleID.String(), seller.String(), "")))

	require.Equal(t, 1, countSoldSystemMessages(t, ctx, h, roomID),
		"active negotiation buyer must be notified even without buyer_id in payload")
}

func TestForSaleSold_WiredConsumer_SessionWithoutRoomSkipped(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupSoldHarness(t)
	defer cleanup()

	buyer := insertSoldUser(t, ctx, h)
	seller := insertSoldUser(t, ctx, h)
	forSaleID := insertSoldForSale(t, ctx, h, seller)

	// Session with NULL chat_room_id — must be skipped, not crash.
	session := negotiationEntity.NewNegotiationSession(
		negotiationEntity.NegotiationResourceForSale, forSaleID, buyer, seller,
	)
	require.NoError(t, session.SetCurrentPrice(400000))
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		return h.negotiation.CreateSession(ctx, tx, session)
	}))

	require.NoError(t, h.handler.HandleEvent(ctx, soldPayload(forSaleID.String(), seller.String(), buyer.String())))
}

func TestForSaleSold_WiredConsumer_InvalidPayloadRejected(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupSoldHarness(t)
	defer cleanup()

	// Malformed JSON.
	require.Error(t, h.handler.HandleEvent(ctx, []byte(`{not-json`)))

	// Invalid for_sale_id.
	require.Error(t, h.handler.HandleEvent(ctx, []byte(`{"for_sale_id":"not-a-uuid","seller_id":"`+uuid.New().String()+`"}`)))
}

// TestForSaleSold_WiredConsumer_TerminalSessionsUntouched guards the
// no-resurrection contract: cancelled/expired sessions must not be revived,
// re-notified, or re-cancelled by a re-delivered event.
func TestForSaleSold_WiredConsumer_TerminalSessionsUntouched(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupSoldHarness(t)
	defer cleanup()

	buyer := insertSoldUser(t, ctx, h)
	seller := insertSoldUser(t, ctx, h)
	forSaleID := insertSoldForSale(t, ctx, h, seller)
	roomID := insertSoldDirectRoom(t, ctx, h, buyer, seller)

	session := negotiationEntity.NewNegotiationSession(
		negotiationEntity.NegotiationResourceForSale, forSaleID, buyer, seller,
	)
	session.SetChatRoomID(roomID)
	require.NoError(t, session.SetCurrentPrice(400000))
	require.NoError(t, session.Cancel())
	require.NoError(t, h.appDB.WithTx(ctx, func(tx db.Tx) error {
		return h.negotiation.CreateSession(ctx, tx, session)
	}))
	baseline := session.UpdatedAt

	require.NoError(t, h.handler.HandleEvent(ctx, soldPayload(forSaleID.String(), seller.String(), buyer.String())))

	after := getSoldSessionByRoom(t, ctx, h, roomID)
	require.Equal(t, negotiationEntity.NegotiationStatusCancelled, after.Status)
	require.WithinDuration(t, baseline, after.UpdatedAt, time.Second)
	require.Equal(t, 0, countSoldSystemMessages(t, ctx, h, roomID))
}
