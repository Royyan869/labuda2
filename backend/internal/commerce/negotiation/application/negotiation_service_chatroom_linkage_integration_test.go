//go:build integration

// Package application_test proves the PASS_7B fix end to end against a real
// database: NegotiationService.StartNegotiation now (1) rejects a chat room
// whose other participant is not the resolved seller, and (2) persists the
// session's chat_room_id at creation time, so GetNegotiation and
// CreateOrderFromChat's underlying repository lookups
// (GetLatestSessionByChatRoomID / GetAcceptedSessionByChatRoomIDForUpdate)
// can actually find the session — which they never could before this fix,
// since chat_room_id was always NULL (PASS_7A finding).
package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	forsaleEntity "github.com/labuda/backend/internal/commerce/forsale/entity"
	forsaleImpl "github.com/labuda/backend/internal/commerce/forsale/infrastructure/repository"
	forsaleRepo "github.com/labuda/backend/internal/commerce/forsale/repository"
	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	"github.com/labuda/backend/internal/config"
	chatEntity "github.com/labuda/backend/internal/interaction/chat/entity"
	chatImpl "github.com/labuda/backend/internal/interaction/chat/infrastructure/repository"
	outboxImpl "github.com/labuda/backend/internal/platform/outbox/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/money"
	"github.com/labuda/backend/pkg/testdb"
	"go.uber.org/zap"
)

// negotiationLinkageTestHarness bundles the real, DB-backed dependencies
// needed to exercise NegotiationService.StartNegotiation exactly as
// production wires it (minus the chat domain, which the service does not
// depend on — see StartNegotiationRequest doc comment).
type negotiationLinkageTestHarness struct {
	tdb         *testdb.TestDB
	realDB      *db.DB
	svc         *negotiationApp.NegotiationService
	forSaleRepo forsaleRepo.ForSaleRepository
}

func setupNegotiationLinkageHarness(t *testing.T) (*negotiationLinkageTestHarness, func()) {
	t.Helper()
	ctx := context.Background()

	tdb, cleanup := testdb.SetupDB(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load failed: %v", err)
	}
	realDB, err := db.New(ctx, db.Config{ConnString: cfg.Database.GetTestDSN()})
	if err != nil {
		t.Fatalf("db.New failed: %v", err)
	}

	forSaleRepo := forsaleImpl.NewForSaleRepository()
	outboxRepo := outboxImpl.NewOutboxRepository(realDB)
	svc := negotiationApp.NewNegotiationService(realDB, forSaleRepo, outboxRepo, nil, nil, zap.NewNop())

	return &negotiationLinkageTestHarness{
		tdb:         tdb,
		realDB:      realDB,
		svc:         svc,
		forSaleRepo: forSaleRepo,
	}, cleanup
}

func insertLinkageTestUser(t *testing.T, ctx context.Context, tdb *testdb.TestDB) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := tdb.Pool().Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	if err != nil {
		t.Fatalf("insertLinkageTestUser: %v", err)
	}
	return uid
}

func insertLinkageTestForSale(
	t *testing.T,
	ctx context.Context,
	tx db.Tx,
	repo forsaleRepo.ForSaleRepository,
	sellerID uuid.UUID,
) uuid.UUID {
	t.Helper()
	sale, err := forsaleEntity.NewForSale(
		sellerID,
		"Test Kohaku Koi",
		"Linkage fixture",
		[]byte(`["https://picsum.photos/seed/koi-linkage/800/600"]`),
		"Kohaku",
		intPtr(30),
		intPtr(12),
		strPtr("female"),
		nil,
		nil,
		[]string{"global"},
		forsaleEntity.ForSaleTypeFixedPrice,
		money.New(500000),
		1,
		true, // negotiationEnabled
		forsaleEntity.ForSaleVisibilityPublic,
		
		nil,
		forsaleEntity.PreparationTimeImmediate,
		nil,
	)
	if err != nil {
		t.Fatalf("NewForSale: %v", err)
	}
	if err := sale.Publish(); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := repo.Create(ctx, tx, sale); err != nil {
		t.Fatalf("forSaleRepo.Create: %v", err)
	}
	return sale.ID
}

func insertLinkageTestDirectRoom(t *testing.T, ctx context.Context, tx db.Tx, participantA, participantB uuid.UUID) uuid.UUID {
	t.Helper()
	repo := chatImpl.NewChatRepository()
	room := chatEntity.NewChatRoom(chatEntity.RoomTypeDirect, participantA, participantB)
	if err := repo.CreateRoom(ctx, tx, room); err != nil {
		t.Fatalf("chatRepo.CreateRoom: %v", err)
	}
	return room.ID
}

func intPtr(i int) *int       { return &i }
func strPtr(s string) *string { return &s }

// TestStartNegotiation_PersistsChatRoomIDAndFindableByGetNegotiation is the
// linkage happy-path test (task item 1): a legitimate buyer, in the room
// they actually share with the seller, starts a negotiation. The session
// must be created with chat_room_id set to that room, and
// GetLatestSessionByChatRoomID (what GetNegotiation calls) must find it.
func TestStartNegotiation_PersistsChatRoomIDAndFindableByGetNegotiation(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()

	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)

	var forSaleID, roomID uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	if err != nil {
		t.Fatalf("fixture setup failed: %v", err)
	}

	session, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                buyerID,
		InitialPrice:           400000,
		RoomID:                 roomID,
		RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("StartNegotiation failed: %v", err)
	}
	if session.ChatRoomID == nil || *session.ChatRoomID != roomID {
		t.Fatalf("session.ChatRoomID = %v, want %s", session.ChatRoomID, roomID)
	}

	repo := negotiationImpl.NewNegotiationRepository()
	var found *negotiationEntity.NegotiationSession
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		found, lookupErr = repo.GetLatestSessionByChatRoomID(ctx, tx, roomID)
		return lookupErr
	})
	if err != nil {
		t.Fatalf("GetLatestSessionByChatRoomID failed: %v", err)
	}
	if found == nil {
		t.Fatal("GetLatestSessionByChatRoomID returned nil — GetNegotiation would 404 for the legitimate buyer/seller (the exact PASS_7A defect)")
	}
	if found.ID != session.ID || found.BuyerID != buyerID || found.SellerID != sellerID {
		t.Fatalf("found session mismatch: got %+v", found)
	}
}

// TestStartNegotiation_RejectsUnrelatedRoomCounterparty is the counterparty
// negative test (task item 3): buyer tries to start a negotiation using a
// room shared with an unrelated third party (not the seller). Must be
// rejected before any session is created.
func TestStartNegotiation_RejectsUnrelatedRoomCounterparty(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()

	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	unrelatedUserID := insertLinkageTestUser(t, ctx, h.tdb)

	var forSaleID, unrelatedRoomID uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		unrelatedRoomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, unrelatedUserID)
		return nil
	})
	if err != nil {
		t.Fatalf("fixture setup failed: %v", err)
	}

	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                buyerID,
		InitialPrice:           400000,
		RoomID:                 unrelatedRoomID,
		RoomOtherParticipantID: unrelatedUserID, // NOT the seller
	})
	if err == nil {
		t.Fatal("expected ErrNegotiationRoomMismatch, got nil error (session was created for an unrelated room!)")
	}
	var mismatchErr *negotiationApp.ErrNegotiationRoomMismatch
	if !errors.As(err, &mismatchErr) {
		t.Fatalf("err = %v, want *ErrNegotiationRoomMismatch", err)
	}

	// Confirm no session was created for this buyer+listing at all.
	repo := negotiationImpl.NewNegotiationRepository()
	var existing *negotiationEntity.NegotiationSession
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		existing, lookupErr = repo.GetActiveSessionByResourceAndBuyer(
			ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
		return lookupErr
	})
	if err != nil {
		t.Fatalf("GetActiveSessionByResourceAndBuyer failed: %v", err)
	}
	if existing != nil {
		t.Fatalf("expected no session created, found one: %+v", existing)
	}
}

// TestGetLatestSessionByChatRoomID_UnrelatedRoomCannotDiscloseSession is the
// disclosure negative test (task item 4): buyer has a real negotiation with
// sellerA (linked to roomA). The SAME buyer also has an unrelated room with
// userB. userB must not be able to discover the buyer/sellerA negotiation
// by querying their own shared room.
func TestGetLatestSessionByChatRoomID_UnrelatedRoomCannotDiscloseSession(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()

	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerAID := insertLinkageTestUser(t, ctx, h.tdb)
	userBID := insertLinkageTestUser(t, ctx, h.tdb)

	var forSaleID, roomAID, roomBID uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerAID)
		roomAID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerAID)
		roomBID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, userBID)
		return nil
	})
	if err != nil {
		t.Fatalf("fixture setup failed: %v", err)
	}

	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                buyerID,
		InitialPrice:           400000,
		RoomID:                 roomAID,
		RoomOtherParticipantID: sellerAID,
	})
	if err != nil {
		t.Fatalf("StartNegotiation (legitimate) failed: %v", err)
	}

	// userB queries GetNegotiation-equivalent lookup on THEIR room (roomB).
	repo := negotiationImpl.NewNegotiationRepository()
	var foundForBystander *negotiationEntity.NegotiationSession
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		foundForBystander, lookupErr = repo.GetLatestSessionByChatRoomID(ctx, tx, roomBID)
		return lookupErr
	})
	if err != nil {
		t.Fatalf("GetLatestSessionByChatRoomID(roomB) failed: %v", err)
	}
	if foundForBystander != nil {
		t.Fatalf("DISCLOSURE: userB's room lookup found buyer/sellerA's negotiation: %+v", foundForBystander)
	}
}

// TestAcceptedNegotiation_FindableByGetAcceptedSessionByChatRoomIDForUpdate
// is the checkout happy-path test (task item 2), at the exact repository
// lookup CreateOrderFromChat depends on: after StartNegotiation + Accept,
// the accepted session must be findable by the same room CreateOrderFromChat
// will be called against.
func TestAcceptedNegotiation_FindableByGetAcceptedSessionByChatRoomIDForUpdate(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()

	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)

	var forSaleID, roomID uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	if err != nil {
		t.Fatalf("fixture setup failed: %v", err)
	}

	session, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                buyerID,
		InitialPrice:           400000,
		RoomID:                 roomID,
		RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("StartNegotiation failed: %v", err)
	}

	accepted, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{
		SessionID: session.ID,
		SellerID:  sellerID,
	})
	if err != nil {
		t.Fatalf("AcceptNegotiation failed: %v", err)
	}
	if accepted.AcceptedPrice == nil || *accepted.AcceptedPrice != 400000 {
		t.Fatalf("accepted price = %v, want 400000", accepted.AcceptedPrice)
	}

	repo := negotiationImpl.NewNegotiationRepository()
	var forCheckout *negotiationEntity.NegotiationSession
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		var lookupErr error
		forCheckout, lookupErr = repo.GetAcceptedSessionByChatRoomIDForUpdate(ctx, tx, roomID)
		return lookupErr
	})
	if err != nil {
		t.Fatalf("GetAcceptedSessionByChatRoomIDForUpdate failed: %v", err)
	}
	if forCheckout == nil {
		t.Fatal("GetAcceptedSessionByChatRoomIDForUpdate returned nil — CreateOrderFromChat would fail with ErrNoAcceptedNegotiation (the exact PASS_7A defect)")
	}
	if forCheckout.ID != session.ID || forCheckout.AcceptedPrice == nil || *forCheckout.AcceptedPrice != 400000 {
		t.Fatalf("checkout lookup mismatch: got %+v", forCheckout)
	}
}

// TestNegotiationMutation_RejectsUnrelatedBystanderEvenKnowingSessionID is
// the mutation-safety regression test (task item 5): a bystander who is
// neither buyer nor seller cannot counter, accept, or cancel — even if they
// somehow learned the real session_id.
func TestNegotiationMutation_RejectsUnrelatedBystanderEvenKnowingSessionID(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()

	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	bystanderID := insertLinkageTestUser(t, ctx, h.tdb)

	var forSaleID, roomID uuid.UUID
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	if err != nil {
		t.Fatalf("fixture setup failed: %v", err)
	}

	session, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType:           negotiationEntity.NegotiationResourceForSale,
		ForSaleID:              forSaleID,
		BuyerID:                buyerID,
		InitialPrice:           400000,
		RoomID:                 roomID,
		RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("StartNegotiation failed: %v", err)
	}

	if err := h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{
		SessionID: session.ID,
		SenderID:  bystanderID,
		Price:     350000,
	}); err == nil {
		t.Fatal("bystander SendCounterOffer succeeded — must be rejected")
	}

	if _, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{
		SessionID: session.ID,
		SellerID:  bystanderID,
	}); err == nil {
		t.Fatal("bystander AcceptNegotiation succeeded — must be rejected")
	}

	if err := h.svc.CancelNegotiation(ctx, negotiationApp.CancelNegotiationRequest{
		SessionID: session.ID,
		BuyerID:   bystanderID,
	}); err == nil {
		t.Fatal("bystander CancelNegotiation succeeded — must be rejected")
	}
}
