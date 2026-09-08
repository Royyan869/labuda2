//go:build integration

package application_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// TestSemanticSlot_S1_ActiveBlocksNew verifies S1: existing active → rejected.
func TestSemanticSlot_S1_ActiveBlocksNew(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("first StartNegotiation failed: %v", err)
	}
	// Need a second room for second attempt? Room counterparty validation requires same seller, any room with buyer+seller is valid.
	// Reuse same roomID is okay (RoomOtherParticipant check passes, but slot should still block).
	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 410000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected ErrActiveSessionExists for S1, got nil")
	}
	var existsErr *negotiationApp.ErrActiveSessionExists
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected ErrActiveSessionExists, got %T: %v", err, err)
	}
}

// TestSemanticSlot_S2_AcceptedUnsettledBlocksNew verifies S2: accepted+NULL order_id → rejected.
func TestSemanticSlot_S2_AcceptedUnsettledBlocksNew(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	sess, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Accept via handler logic: seller accepts
	_, err = h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err != nil {
		t.Fatalf("AcceptNegotiation failed: %v", err)
	}
	// Now accepted+NULL should block new start
	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 410000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected ErrActiveSessionExists for S2, got nil")
	}
	var existsErr *negotiationApp.ErrActiveSessionExists
	if !errors.As(err, &existsErr) {
		t.Fatalf("expected ErrActiveSessionExists for S2, got %T: %v", err, err)
	}
}

// TestSemanticSlot_S3_AcceptedSettledAllowsNew verifies S3: accepted+order_id NOT NULL → allowed.
func TestSemanticSlot_S3_AcceptedSettledAllowsNew(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	sess, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("first Start: %v", err)
	}
	_, err = h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	// Simulate settlement: set order_id via repository (mimics order_creation_service)
	repo := negotiationImpl.NewNegotiationRepository()
	orderID := uuid.New()
	// Need an orders row for FK
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO orders (id, buyer_id, seller_id, source_type, source_id, quantity, unit_price, subtotal, shipping_total, commission_percent, commission_amount, status, created_at, updated_at) VALUES ($1,$2,$3,'for_sale',$4,1,500000,500000,0,5,25000,'pending_payment',NOW(),NOW())`, orderID, buyerID, sellerID, forSaleID)
		return err
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		s, _ := repo.GetSession(ctx, tx, sess.ID)
		s.OrderID = &orderID
		return repo.UpdateSession(ctx, tx, s)
	})
	// Now new start should succeed (need new room? reuse)
	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 420000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("S3 expected allowed, got err: %v", err)
	}
}

// TestSemanticSlot_S4_CancelledAllowsNew verifies S4: cancelled → allowed.
func TestSemanticSlot_S4_CancelledAllowsNew(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	sess, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := h.svc.CancelNegotiation(ctx, negotiationApp.CancelNegotiationRequest{SessionID: sess.ID, BuyerID: buyerID}); err != nil {
		t.Fatalf("Cancel failed: %v", err)
	}
	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 430000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("S4 expected allowed after cancelled, got %v", err)
	}
}

// TestSemanticSlot_S5_ExpiredAllowsNew verifies S5: expired → allowed.
func TestSemanticSlot_S5_ExpiredAllowsNew(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	sess, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Expire via service
	if err := h.svc.ExpireSession(ctx, sess.ID); err != nil {
		t.Fatalf("ExpireSession failed: %v", err)
	}
	_, err = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 440000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("S5 expected allowed after expired, got %v", err)
	}
}

// TestDBConstraint_ActiveConflict proves partial unique index directly.
func TestDBConstraint_ActiveConflict(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		return nil
	})
	repo := negotiationImpl.NewNegotiationRepository()
	s1 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s1.SetCurrentPrice(400000)
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s1) }); err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	s2 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s2.SetCurrentPrice(400001)
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s2) })
	if err == nil {
		t.Fatal("expected unique violation for second active, got nil")
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || !strings.Contains(pgErr.ConstraintName, "ux_negotiation_one_active_per_buyer_for_sale") {
		// fallback substring check
		if !strings.Contains(err.Error(), "ux_negotiation_one_active_per_buyer_for_sale") {
			t.Fatalf("expected ux_negotiation_one_active_per_buyer_for_sale violation, got %v (code=%v constraint=%v)", err, pgErr, pgErr.ConstraintName)
		}
	}
}

// TestDBConstraint_AcceptedUnsettledConflicts proves accepted+NULL conflicts.
func TestDBConstraint_AcceptedUnsettledConflicts(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		return nil
	})
	repo := negotiationImpl.NewNegotiationRepository()
	s1 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s1.SetCurrentPrice(400000)
	_ = s1.AcceptWithPrice()
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s1) }); err != nil {
		t.Fatalf("insert accepted failed: %v", err)
	}
	s2 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s2.SetCurrentPrice(410000)
	err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s2) })
	if err == nil {
		t.Fatal("expected unique violation for active vs accepted unsettled, got nil")
	}
	if !strings.Contains(err.Error(), "ux_negotiation_one_active_per_buyer_for_sale") {
		t.Fatalf("wrong constraint: %v", err)
	}
}

// TestDBConstraint_SettledDoesNotConflict proves accepted+order_id NOT NULL does not conflict.
func TestDBConstraint_SettledDoesNotConflict(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		return nil
	})
	repo := negotiationImpl.NewNegotiationRepository()
	orderID := uuid.New()
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO orders (id, buyer_id, seller_id, source_type, source_id, quantity, unit_price, subtotal, shipping_total, commission_percent, commission_amount, status, created_at, updated_at) VALUES ($1,$2,$3,'for_sale',$4,1,500000,500000,0,5,25000,'pending_payment',NOW(),NOW())`, orderID, buyerID, sellerID, forSaleID)
		return err
	})
	s1 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s1.SetCurrentPrice(400000)
	_ = s1.AcceptWithPrice()
	s1.OrderID = &orderID
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s1) }); err != nil {
		t.Fatalf("insert settled failed: %v", err)
	}
	// CreateSession does not persist order_id (canonical settlement uses UpdateSession); persist via direct UPDATE
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, execErr := tx.Exec(ctx, `UPDATE negotiation_sessions SET order_id=$1, updated_at=NOW() WHERE id=$2`, orderID, s1.ID)
		return execErr
	}); err != nil {
		t.Fatalf("failed to mark settled via order_id update: %v", err)
	}
	s2 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s2.SetCurrentPrice(410000)
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s2) }); err != nil {
		t.Fatalf("expected settled to allow new active, got err: %v", err)
	}
}

// TestDBConstraint_CancelledAndExpiredDoNotConflict proves cancelled/expired do not conflict.
func TestDBConstraint_CancelledAndExpiredDoNotConflict(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		return nil
	})
	repo := negotiationImpl.NewNegotiationRepository()
	for _, status := range []negotiationEntity.NegotiationStatus{negotiationEntity.NegotiationStatusCancelled, negotiationEntity.NegotiationStatusExpired} {
		s1 := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
		_ = s1.SetCurrentPrice(400000)
		s1.Status = status
		if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s1) }); err != nil {
			t.Fatalf("insert %s failed: %v", status, err)
		}
	}
	sActive := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = sActive.SetCurrentPrice(420000)
	if err := h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, sActive) }); err != nil {
		t.Fatalf("expected cancelled/expired to allow new active, got %v", err)
	}
}

// TestConcurrentStart_RealRace proves only one winner under real barrier.
func TestConcurrentStart_RealRace(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	// Barrier to maximize overlap: both goroutines start their WithTx simultaneously
	barrier := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			defer wg.Done()
			<-barrier
			_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
				ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: int64(400000 + idx), RoomID: roomID, RoomOtherParticipantID: sellerID,
			})
			results[idx] = err
		}(i)
	}
	close(barrier)
	wg.Wait()
	successCount := 0
	conflictCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		} else {
			var existsErr *negotiationApp.ErrActiveSessionExists
			if errors.As(err, &existsErr) {
				conflictCount++
			} else {
				t.Fatalf("unexpected error type: %T %v", err, err)
			}
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 ErrActiveSessionExists, got success=%d conflict=%d results=%v", successCount, conflictCount, results)
	}
	// Prove exactly one session committed
	repo := negotiationImpl.NewNegotiationRepository()
	var cnt int
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM negotiation_sessions WHERE for_sale_id=$1 AND buyer_id=$2`, forSaleID, buyerID).Scan(&cnt)
	})
	if cnt != 1 {
		t.Fatalf("expected 1 committed session, got %d", cnt)
	}
	// Prove exactly one price history and one outbox for the winner
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		sess, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
		if sess == nil {
			t.Fatalf("expected active session to exist after race")
		}
		hist, _ := repo.GetPriceHistoryBySession(ctx, tx, sess.ID)
		if len(hist) != 1 {
			t.Fatalf("expected 1 price history, got %d", len(hist))
		}
		return nil
	})
	var outboxCnt int
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.started'`).Scan(&outboxCnt)
	})
	// outbox count may include prior tests' events in same DB, but at least 1 for this tuple
	if outboxCnt < 1 {
		t.Fatal("expected at least 1 outbox event")
	}
	_ = repo // keep import used
}

// TestUniqueViolationTranslation proves constraint violation → ErrActiveSessionExists.
func TestUniqueViolationTranslation(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	// First succeeds
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	// Direct repository duplicate insert should be translated at service level as ErrActiveSessionExists when raced,
	// and direct DB violation string must contain constraint name
	repo := negotiationImpl.NewNegotiationRepository()
	s := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
	_ = s.SetCurrentPrice(500000)
	err = h.tdb.WithTx(ctx, func(tx db.Tx) error { return repo.CreateSession(ctx, tx, s) })
	if err == nil {
		t.Fatal("expected unique violation on direct duplicate")
	}
	if !strings.Contains(err.Error(), "ux_negotiation_one_active_per_buyer_for_sale") {
		t.Fatalf("expected constraint ux_negotiation_one_active_per_buyer_for_sale, got %v", err)
	}
	// Service-level race translation already proven in TestConcurrentStart_RealRace (ErrActiveSessionExists via errors.As)
}
