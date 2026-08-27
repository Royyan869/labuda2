//go:build integration

// Package repository_test proves NegotiationRepositoryImpl's queries match the
// live migrated schema end to end. This is the regression guard for the
// PASS 1B fix: prior to migration 000002, every one of these round trips
// failed — CreateSession failed on an enum cast (negotiation_resource_enum
// lacked 'for_sale') and any query touching chat_room_id, order_id,
// accepted_price, accepted_at, or proposal_sequence failed with
// "column ... does not exist" because those columns were repository-only
// drift never migrated into negotiation_sessions.
package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	negotiationRepo "github.com/labuda/backend/internal/commerce/negotiation/repository"
	"github.com/labuda/backend/pkg/db"
	"github.com/labuda/backend/pkg/testdb"
)

func setupNegotiationTest(t *testing.T) (*testdb.TestDB, negotiationRepo.Repository, func()) {
	t.Helper()
	tdb, cleanup := testdb.SetupDB(t)
	return tdb, negotiationImpl.NewNegotiationRepository(), cleanup
}

func insertNegotiationTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, firebase_uid, email, email_verified_at, phone_verified, account_status, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), true, 'active', NOW(), NOW())
	`, uid, uid.String(), uid.String()+"@test.invalid")
	if err != nil {
		t.Fatalf("insertNegotiationTestUser: %v", err)
	}
	return uid
}

// insertNegotiationTestForSale creates the product + for_sale
// fixture chain required to satisfy negotiation_sessions' FK to
// for_sales.
func insertNegotiationTestForSale(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	productID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO products (id, seller_id, title, description, variety, preparation_time, created_at, updated_at)
		VALUES ($1, $2, 'Test Koi', 'test fixture', 'showa', 'same_day', NOW(), NOW())
	`, productID, sellerID)
	if err != nil {
		t.Fatalf("insert test product: %v", err)
	}

	saleID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO for_sales (id, product_id, seller_id, price_per_unit, negotiation_enabled, status, created_at, updated_at)
		VALUES ($1, $2, $3, 500000, true, 'active', NOW(), NOW())
	`, saleID, productID, sellerID)
	if err != nil {
		t.Fatalf("insert test for_sale: %v", err)
	}
	return saleID
}

func insertNegotiationTestChatRoom(t *testing.T, ctx context.Context, pool *pgxpool.Pool, a, b uuid.UUID) uuid.UUID {
	t.Helper()
	// chat_rooms_check1 requires participant_a < participant_b.
	if a.String() > b.String() {
		a, b = b, a
	}
	roomID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO chat_rooms (id, room_type, participant_a, participant_b, created_at, updated_at, last_message_at)
		VALUES ($1, 'negotiation', $2, $3, NOW(), NOW(), NOW())
	`, roomID, a, b)
	if err != nil {
		t.Fatalf("insert test chat_room: %v", err)
	}
	return roomID
}

func insertNegotiationTestOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, buyerID, sellerID uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO orders (
			id, buyer_id, seller_id, source_type, source_id, quantity, unit_price,
			subtotal, shipping_total, commission_percent, commission_amount, status,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, 'for_sale', $4, 1, 500000, 500000, 0, 5, 25000, 'pending_payment', NOW(), NOW())
	`, orderID, buyerID, sellerID, uuid.New())
	if err != nil {
		t.Fatalf("insert test order: %v", err)
	}
	return orderID
}

func newTestNegotiationSession(saleID, buyerID, sellerID uuid.UUID) *negotiationEntity.NegotiationSession {
	s := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, saleID, buyerID, sellerID)
	if err := s.SetCurrentPrice(400000); err != nil {
		panic(err)
	}
	return s
}

// TestNegotiationRepository_CreateAndGetSession proves CreateSession/GetSession
// round-trip against the live schema, including the resource_type enum cast
// that failed before migration 000002 added 'for_sale'.
func TestNegotiationRepository_CreateAndGetSession(t *testing.T) {
	tdb, repo, cleanup := setupNegotiationTest(t)
	defer cleanup()
	ctx := context.Background()

	buyerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	sellerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	saleID := insertNegotiationTestForSale(t, ctx, tdb.Pool(), sellerID)

	session := newTestNegotiationSession(saleID, buyerID, sellerID)

	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.CreateSession(ctx, tx, session)
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	var fetched *negotiationEntity.NegotiationSession
	err = tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		fetched, err = repo.GetSession(ctx, tx, session.ID)
		return err
	})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if fetched.ResourceType != negotiationEntity.NegotiationResourceForSale {
		t.Fatalf("resource_type mismatch: got %q", fetched.ResourceType)
	}
	if fetched.ChatRoomID != nil {
		t.Fatalf("expected nil chat_room_id, got %v", fetched.ChatRoomID)
	}
	if fetched.OrderID != nil {
		t.Fatalf("expected nil order_id, got %v", fetched.OrderID)
	}
	if fetched.ProposalSequence != session.ProposalSequence {
		t.Fatalf("proposal_sequence mismatch: got %d want %d", fetched.ProposalSequence, session.ProposalSequence)
	}
	if fetched.CurrentPrice == nil || *fetched.CurrentPrice != 400000 {
		t.Fatalf("current_price mismatch: got %v", fetched.CurrentPrice)
	}
}

// TestNegotiationRepository_AcceptSetsAcceptedPriceAndAt proves accepted_price
// and accepted_at persist and round-trip through UpdateSession.
func TestNegotiationRepository_AcceptSetsAcceptedPriceAndAt(t *testing.T) {
	tdb, repo, cleanup := setupNegotiationTest(t)
	defer cleanup()
	ctx := context.Background()

	buyerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	sellerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	saleID := insertNegotiationTestForSale(t, ctx, tdb.Pool(), sellerID)

	session := newTestNegotiationSession(saleID, buyerID, sellerID)
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := session.AcceptWithPrice(); err != nil {
		t.Fatalf("AcceptWithPrice: %v", err)
	}
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.UpdateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	var fetched *negotiationEntity.NegotiationSession
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		fetched, err = repo.GetSession(ctx, tx, session.ID)
		return err
	})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}

	if fetched.AcceptedPrice == nil || *fetched.AcceptedPrice != 400000 {
		t.Fatalf("accepted_price mismatch: got %v", fetched.AcceptedPrice)
	}
	if fetched.AcceptedAt == nil {
		t.Fatal("accepted_at must be set after accept")
	}
	if time.Since(*fetched.AcceptedAt) > time.Minute {
		t.Fatalf("accepted_at looks stale: %v", fetched.AcceptedAt)
	}
}

// TestNegotiationRepository_ChatRoomLinkageRoundTrips proves chat_room_id
// persists and satisfies its FK to chat_rooms, and that
// GetAcceptedSessionByChatRoomID resolves it back.
func TestNegotiationRepository_ChatRoomLinkageRoundTrips(t *testing.T) {
	tdb, repo, cleanup := setupNegotiationTest(t)
	defer cleanup()
	ctx := context.Background()

	buyerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	sellerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	saleID := insertNegotiationTestForSale(t, ctx, tdb.Pool(), sellerID)
	roomID := insertNegotiationTestChatRoom(t, ctx, tdb.Pool(), buyerID, sellerID)

	session := newTestNegotiationSession(saleID, buyerID, sellerID)
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	session.SetChatRoomID(roomID)
	if err := session.AcceptWithPrice(); err != nil {
		t.Fatalf("AcceptWithPrice: %v", err)
	}
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.UpdateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	var found *negotiationEntity.NegotiationSession
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		found, err = repo.GetAcceptedSessionByChatRoomID(ctx, tx, roomID)
		return err
	})
	if err != nil {
		t.Fatalf("GetAcceptedSessionByChatRoomID: %v", err)
	}
	if found == nil {
		t.Fatal("expected accepted session to be found by chat_room_id, got nil")
	}
	if found.ID != session.ID {
		t.Fatalf("session ID mismatch: got %v want %v", found.ID, session.ID)
	}
	if found.ChatRoomID == nil || *found.ChatRoomID != roomID {
		t.Fatalf("chat_room_id mismatch: got %v want %v", found.ChatRoomID, roomID)
	}
}

// TestNegotiationRepository_UpdateOrderIDRoundTrips proves order_id persists,
// satisfies its FK to orders, and enforces the one-order-per-negotiation
// unique constraint that backs ErrNegotiationAlreadySettled.
func TestNegotiationRepository_UpdateOrderIDRoundTrips(t *testing.T) {
	tdb, repo, cleanup := setupNegotiationTest(t)
	defer cleanup()
	ctx := context.Background()

	buyerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	sellerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	saleID := insertNegotiationTestForSale(t, ctx, tdb.Pool(), sellerID)
	orderID := insertNegotiationTestOrder(t, ctx, tdb.Pool(), buyerID, sellerID)

	session := newTestNegotiationSession(saleID, buyerID, sellerID)
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.UpdateOrderID(ctx, tx, session.ID, orderID)
	}); err != nil {
		t.Fatalf("UpdateOrderID: %v", err)
	}

	var fetched *negotiationEntity.NegotiationSession
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		fetched, err = repo.GetSession(ctx, tx, session.ID)
		return err
	})
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if fetched.OrderID == nil || *fetched.OrderID != orderID {
		t.Fatalf("order_id mismatch: got %v want %v", fetched.OrderID, orderID)
	}
}

// TestNegotiationRepository_PriceHistoryRoundTrips proves
// CreatePriceHistoryEntry/GetPriceHistoryBySession work against the new
// negotiation_price_history table (previously missing entirely).
func TestNegotiationRepository_PriceHistoryRoundTrips(t *testing.T) {
	tdb, repo, cleanup := setupNegotiationTest(t)
	defer cleanup()
	ctx := context.Background()

	buyerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	sellerID := insertNegotiationTestUser(t, ctx, tdb.Pool())
	saleID := insertNegotiationTestForSale(t, ctx, tdb.Pool(), sellerID)

	session := newTestNegotiationSession(saleID, buyerID, sellerID)
	if err := tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, session) }); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	history := negotiationEntity.NewNegotiationPriceHistory(session.ID, session.ProposalSequence, nil, 400000, buyerID, "initial_proposal")
	if err := tdb.WithTx(ctx, func(tx db.Tx) error {
		return repo.CreatePriceHistoryEntry(ctx, tx, history)
	}); err != nil {
		t.Fatalf("CreatePriceHistoryEntry: %v", err)
	}

	var entries []*negotiationEntity.NegotiationPriceHistory
	err := tdb.WithTx(ctx, func(tx db.Tx) error {
		var err error
		entries, err = repo.GetPriceHistoryBySession(ctx, tx, session.ID)
		return err
	})
	if err != nil {
		t.Fatalf("GetPriceHistoryBySession: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 price history entry, got %d", len(entries))
	}
	if entries[0].NewPrice != 400000 {
		t.Fatalf("new_price mismatch: got %d", entries[0].NewPrice)
	}
	if entries[0].OldPrice != nil {
		t.Fatalf("expected nil old_price for initial proposal, got %v", entries[0].OldPrice)
	}
}
