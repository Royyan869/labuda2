//go:build integration

package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	negotiationApp "github.com/labuda/backend/internal/commerce/negotiation/application"
	negotiationEntity "github.com/labuda/backend/internal/commerce/negotiation/entity"
	negotiationImpl "github.com/labuda/backend/internal/commerce/negotiation/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// TestN8D_CounterRejectedWhenForSaleSold proves unavailable/sold → SendCounterOffer rejected.
func TestN8D_CounterRejectedWhenForSaleSold(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	var sessionID uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		return nil
	})
	sess, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err != nil {
		t.Fatalf("StartNegotiation failed: %v", err)
	}
	sessionID = sess.ID
	// Make for_sale sold via direct status update (canonical unavailable)
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	// Capture pre-state for atomicity proof
	repo := negotiationImpl.NewNegotiationRepository()
	var beforePrice int64
	var beforeHist int
	var beforeOutbox int
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		s, _ := repo.GetSession(ctx, tx, sessionID)
		if s.CurrentPrice != nil {
			beforePrice = *s.CurrentPrice
		}
		hist, _ := repo.GetPriceHistoryBySession(ctx, tx, sessionID)
		beforeHist = len(hist)
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.message_sent' AND aggregate_id=$1`, sessionID).Scan(&beforeOutbox)
		return nil
	})
	err = h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sessionID, SenderID: buyerID, Price: 350000})
	if err == nil {
		t.Fatal("expected SendCounterOffer rejected when for_sale sold, got nil")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
	// Atomicity: zero mutation
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		s, _ := repo.GetSession(ctx, tx, sessionID)
		if s.CurrentPrice == nil || *s.CurrentPrice != beforePrice {
			t.Fatalf("atomicity violation: current_price mutated after rejected counter: before=%d after=%v", beforePrice, s.CurrentPrice)
		}
		if s.Status != negotiationEntity.NegotiationStatusActive {
			t.Fatalf("atomicity violation: status mutated to %s", s.Status)
		}
		hist, _ := repo.GetPriceHistoryBySession(ctx, tx, sessionID)
		if len(hist) != beforeHist {
			t.Fatalf("atomicity violation: price history delta: before=%d after=%d", beforeHist, len(hist))
		}
		var afterOutbox int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.message_sent' AND aggregate_id=$1`, sessionID).Scan(&afterOutbox)
		if afterOutbox != beforeOutbox {
			t.Fatalf("atomicity violation: outbox delta before=%d after=%d", beforeOutbox, afterOutbox)
		}
		return nil
	})
}

// TestN8D_CounterRejectedWhenForSaleWithdrawn proves withdrawn → counter rejected.
func TestN8D_CounterRejectedWhenForSaleWithdrawn(t *testing.T) {
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
	sess, _ := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='withdrawn', withdrawn_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	err := h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sess.ID, SenderID: sellerID, Price: 380000})
	if err == nil {
		t.Fatal("expected counter rejected after withdrawn")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

// TestN8D_AcceptRejectedWhenForSaleSold proves sold → Accept rejected with atomicity.
func TestN8D_AcceptRejectedWhenForSaleSold(t *testing.T) {
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
	sess, _ := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	// Capture before
	repo := negotiationImpl.NewNegotiationRepository()
	var beforeHist int
	var beforeOutbox int
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		hist, _ := repo.GetPriceHistoryBySession(ctx, tx, sess.ID)
		beforeHist = len(hist)
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.accepted' AND aggregate_id=$1`, sess.ID).Scan(&beforeOutbox)
		return nil
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err == nil {
		t.Fatal("expected Accept rejected when for_sale sold")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		s, _ := repo.GetSession(ctx, tx, sess.ID)
		if s.Status != negotiationEntity.NegotiationStatusActive {
			t.Fatalf("atomicity violation: status mutated to %s after rejected accept", s.Status)
		}
		if s.AcceptedPrice != nil {
			t.Fatalf("atomicity violation: accepted_price set after rejected accept: %v", *s.AcceptedPrice)
		}
		hist, _ := repo.GetPriceHistoryBySession(ctx, tx, sess.ID)
		if len(hist) != beforeHist {
			t.Fatalf("atomicity violation: hist before=%d after=%d", beforeHist, len(hist))
		}
		var afterOutbox int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.accepted' AND aggregate_id=$1`, sess.ID).Scan(&afterOutbox)
		if afterOutbox != beforeOutbox {
			t.Fatalf("atomicity violation: outbox before=%d after=%d", beforeOutbox, afterOutbox)
		}
		return nil
	})
}

// TestN8D_StartRejectedWhenForSaleSold proves sold → Start rejected.
func TestN8D_StartRejectedWhenForSaleSold(t *testing.T) {
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
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected Start rejected when for_sale sold")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	var notFound *negotiationApp.ErrResourceNotFound
	if !errors.As(err, &notNegotiable) && !errors.As(err, &notFound) {
		t.Fatalf("expected ErrResourceNotNegotiable or ErrResourceNotFound, got %T: %v", err, err)
	}
	// Zero side effects
	repo := negotiationImpl.NewNegotiationRepository()
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		existing, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
		if existing != nil {
			t.Fatalf("expected no session created, found %v", existing.ID)
		}
		return nil
	})
}

// TestN8D_StartConcurrency_SoldVsCreate proves race: sold concurrently with Start does not leave invalid actionable negotiation.
// Uses real DB transactions with barrier to force overlap.
func TestN8D_StartConcurrency_SoldVsCreate(t *testing.T) {
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
	// Two concurrent actors: one tries StartNegotiation, one flips for_sale to sold via raw FOR UPDATE.
	// Barrier to maximize overlap.
	barrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	var startErr error
	var soldErr error
	go func() {
		defer wg.Done()
		<-barrier
		_, startErr = h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
			ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
		})
	}()
	go func() {
		defer wg.Done()
		<-barrier
		// Sold transaction: lock for_sale FOR UPDATE then mutate
		soldErr = h.tdb.WithTx(ctx, func(tx db.Tx) error {
			// Lock via GetForUpdate to mirror canonical sold path (OrderCreationService)
			forSale, err := h.forSaleRepo.GetForUpdate(ctx, tx, forSaleID)
			if err != nil {
				return err
			}
			// Small delay to increase overlap window
			// We are already holding lock; the other TX is waiting for same lock.
			if forSale.Status == "active" {
				_, err = tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
				return err
			}
			return nil
		})
	}()
	close(barrier)
	wg.Wait()
	if soldErr != nil {
		t.Fatalf("sold transaction failed: %v", soldErr)
	}
	// After both, exactly one outcome: either Start succeeded before sold (then for_sale now sold but session historically active is allowed)
	// or Start was rejected because it saw sold. In both cases, there must be NO invalid actionable negotiation.
	// Invalid = a committed session with status active/accepted AND order_id NULL when for_sale is sold AND session was created AFTER sold commit.
	// Our guard guarantees: if sold committed before Start's for_sale FOR UPDATE, Start sees sold and returns ErrResourceNotNegotiable, no session.
	// If Start committed before sold, session exists but for_sale was active at its lock time — historical, but future actions must be blocked (proven by other tests).
	// So we assert: if startErr == nil, session must exist and for_sale is now sold; if startErr != nil, it must be ErrResourceNotNegotiable.
	if startErr != nil {
		var notNegotiable *negotiationApp.ErrResourceNotNegotiable
		var notFound *negotiationApp.ErrResourceNotFound
		if !errors.As(startErr, &notNegotiable) && !errors.As(startErr, &notFound) {
			t.Fatalf("expected Start to be rejected with ErrResourceNotNegotiable when raced with sold, got %T: %v", startErr, startErr)
		}
		// Must have zero sessions
		repo := negotiationImpl.NewNegotiationRepository()
		_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
			existing, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
			if existing != nil {
				t.Fatalf("race left invalid actionable negotiation: session %s status %s when for_sale sold and Start should have been rejected", existing.ID, existing.Status)
			}
			return nil
		})
	} else {
		// Start succeeded — verify it was created before sold (historical allowed) and future counter is now blocked
		repo := negotiationImpl.NewNegotiationRepository()
		var sessID uuid.UUID
		_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
			s, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
			if s == nil {
				t.Fatalf("expected session to exist when Start succeeded")
			}
			sessID = s.ID
			return nil
		})
		// Future mutation must be blocked now that for_sale is sold
		err := h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sessID, SenderID: buyerID, Price: 390000})
		var notNegotiable2 *negotiationApp.ErrResourceNotNegotiable
		if !errors.As(err, &notNegotiable2) {
			t.Fatalf("expected future counter to be blocked after concurrent sold, got %T: %v", err, err)
		}
	}
}

// TestN8D_ValidPathStillSucceeds proves available for_sale → all three succeed.
func TestN8D_ValidPathStillSucceeds(t *testing.T) {
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
		t.Fatalf("valid Start failed: %v", err)
	}
	if err := h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sess.ID, SenderID: sellerID, Price: 410000}); err != nil {
		t.Fatalf("valid Counter failed: %v", err)
	}
	if _, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID}); err != nil {
		t.Fatalf("valid Accept failed: %v", err)
	}
}

// TestN8D_CanSettleWallClockIndependent proves CanSettle remains wall-clock authoritative.
func TestN8D_CanSettleWallClockIndependent(t *testing.T) {
	// Unit-level but exercised: accepted + expired wall-clock must not be settleable even though status accepted
	s := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, uuid.New(), uuid.New(), uuid.New())
	_ = s.SetCurrentPrice(500000)
	_ = s.AcceptWithPrice()
	// Force expires_at in past
	past := s.ExpiresAt
	_ = past
	// Set expired time
	expired := s.ExpiresAt
	_ = expired
	expiredAt := s.CreatedAt.Add(-24 * 1000000000 * 3600) // past
	s.ExpiresAt = &expiredAt
	if s.CanSettle() {
		t.Fatal("expected CanSettle false when wall-clock expired, worker not needed")
	}
	if s.Status.IsTerminal() {
		t.Fatal("wall-clock expiry must not make status terminal")
	}
}

// ---------------------------------------------------------------------------
// N8-D MATRIX COMPLETION — every unavailable predicate × every mutation path
// Predicates discovered from production code (for_sale.go:269-271):
//   IsAvailable() == false  when status != active OR quantity==0
//   NegotiationEnabled == false
// Concrete states: sold, withdrawn, draft, quantityZero, negotiationDisabled
// ---------------------------------------------------------------------------

func TestN8D_StartRejectedWhenForSaleWithdrawn(t *testing.T) {
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
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='withdrawn', withdrawn_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected Start rejected when withdrawn")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
	repo := negotiationImpl.NewNegotiationRepository()
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		existing, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
		if existing != nil {
			t.Fatalf("session created despite withdrawn for_sale")
		}
		var cnt int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.started' AND aggregate_id IN (SELECT id FROM negotiation_sessions WHERE for_sale_id=$1)`, forSaleID).Scan(&cnt)
		if cnt != 0 {
			t.Fatalf("outbox not zero after rejected start withdrawn: %d", cnt)
		}
		return nil
	})
}

func TestN8D_StartRejectedWhenNegotiationDisabled(t *testing.T) {
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
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET negotiation_enabled=false, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected Start rejected when negotiation disabled")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable for disabled, got %T: %v", err, err)
	}
}

func TestN8D_StartRejectedWhenQuantityZero(t *testing.T) {
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
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET quantity_available=0, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected Start rejected when quantity zero")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable for qty0, got %T: %v", err, err)
	}
}

func TestN8D_StartRejectedWhenDraft(t *testing.T) {
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
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='draft', published_at=NULL, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	if err == nil {
		t.Fatal("expected Start rejected when draft")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable for draft, got %T: %v", err, err)
	}
}

func TestN8D_CounterRejectedWhenNegotiationDisabled(t *testing.T) {
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
		t.Fatalf("StartNegotiation failed: %v", err)
	}
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET negotiation_enabled=false, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	err = h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sess.ID, SenderID: buyerID, Price: 350000})
	if err == nil {
		t.Fatal("expected counter rejected when negotiation disabled")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

func TestN8D_CounterRejectedWhenQuantityZero(t *testing.T) {
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
		t.Fatalf("StartNegotiation failed: %v", err)
	}
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET quantity_available=0, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	err = h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sess.ID, SenderID: sellerID, Price: 380000})
	if err == nil {
		t.Fatal("expected counter rejected when quantity zero")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

func TestN8D_AcceptRejectedWhenWithdrawn(t *testing.T) {
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
	sess, _ := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET status='withdrawn', withdrawn_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err == nil {
		t.Fatal("expected Accept rejected when withdrawn")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

func TestN8D_AcceptRejectedWhenNegotiationDisabled(t *testing.T) {
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
	sess, _ := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET negotiation_enabled=false, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err == nil {
		t.Fatal("expected Accept rejected when negotiation disabled")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

func TestN8D_AcceptRejectedWhenQuantityZero(t *testing.T) {
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
	sess, _ := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
		ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
	})
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE for_sales SET quantity_available=0, updated_at=NOW() WHERE id=$1`, forSaleID)
		return err
	})
	_, err := h.svc.AcceptNegotiation(ctx, negotiationApp.AcceptNegotiationRequest{SessionID: sess.ID, SellerID: sellerID})
	if err == nil {
		t.Fatal("expected Accept rejected when qty zero")
	}
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err, &notNegotiable) {
		t.Fatalf("expected ErrResourceNotNegotiable, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// DETERMINISTIC CONCURRENCY PROOFS — lock serialization via real PG FOR UPDATE
// ---------------------------------------------------------------------------

// TestN8D_Deterministic_Start_SoldWins proves Case A: sold holder acquires
// for_sales FOR UPDATE, StartNegotiation blocks, sold commits, Start sees
// unavailable and returns ErrResourceNotNegotiable with zero side effects.
// Synchronization: holder signals locked, main launches Start which must block,
// verified via timeout check, then holder commits.
func TestN8D_Deterministic_Start_SoldWins(t *testing.T) {
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

	locked := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)

	go func() {
		conn, err := h.tdb.Pool().Acquire(ctx)
		if err != nil {
			holderDone <- err
			return
		}
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		if err != nil {
			holderDone <- err
			return
		}
		_, err = tx.Exec(ctx, `SELECT id FROM for_sales WHERE id=$1 FOR UPDATE`, forSaleID)
		if err != nil {
			_ = tx.Rollback(ctx)
			holderDone <- err
			return
		}
		close(locked)
		<-release
		_, err = tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		if err != nil {
			_ = tx.Rollback(ctx)
			holderDone <- err
			return
		}
		holderDone <- tx.Commit(ctx)
	}()

	<-locked

	startCh := make(chan error, 1)
	go func() {
		_, err := h.svc.StartNegotiation(ctx, negotiationApp.StartNegotiationRequest{
			ResourceType: negotiationEntity.NegotiationResourceForSale, ForSaleID: forSaleID, BuyerID: buyerID, InitialPrice: 400000, RoomID: roomID, RoomOtherParticipantID: sellerID,
		})
		startCh <- err
	}()

	select {
	case err := <-startCh:
		t.Fatalf("StartNegotiation returned early while holder still holds FOR UPDATE — no blocking: err=%v", err)
	case <-time.After(300 * time.Millisecond):
		// expected blocked
	}

	close(release)

	select {
	case err := <-holderDone:
		if err != nil {
			t.Fatalf("holder commit failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("holder commit timeout")
	}

	select {
	case err := <-startCh:
		var notNegotiable *negotiationApp.ErrResourceNotNegotiable
		var notFound *negotiationApp.ErrResourceNotFound
		if !errors.As(err, &notNegotiable) && !errors.As(err, &notFound) {
			t.Fatalf("expected ErrResourceNotNegotiable after sold wins, got %T: %v", err, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartNegotiation did not return after holder release")
	}

	repo := negotiationImpl.NewNegotiationRepository()
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		existing, _ := repo.GetActiveSessionByResourceAndBuyer(ctx, tx, negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID)
		if existing != nil {
			t.Fatalf("atomicity: session was created despite sold lock winning")
		}
		var cnt int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM outbox WHERE event_type='negotiation.started' AND aggregate_id IN (SELECT id FROM negotiation_sessions WHERE for_sale_id=$1)`, forSaleID).Scan(&cnt)
		if cnt != 0 {
			t.Fatalf("outbox leaked after blocked Start: %d", cnt)
		}
		var histCnt int
		_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM negotiation_price_history ph JOIN negotiation_sessions ns ON ns.id=ph.session_id WHERE ns.for_sale_id=$1`, forSaleID).Scan(&histCnt)
		if histCnt != 0 {
			t.Fatalf("price_history leaked after blocked Start: %d", histCnt)
		}
		return nil
	})
}

// TestN8D_Deterministic_Counter_SoldWins proves counter path blocks on same row.
func TestN8D_Deterministic_Counter_SoldWins(t *testing.T) {
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
		t.Fatalf("Start failed: %v", err)
	}

	locked := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		conn, err := h.tdb.Pool().Acquire(ctx)
		if err != nil {
			holderDone <- err
			return
		}
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		if err != nil {
			holderDone <- err
			return
		}
		_, err = tx.Exec(ctx, `SELECT id FROM for_sales WHERE id=$1 FOR UPDATE`, forSaleID)
		if err != nil {
			_ = tx.Rollback(ctx)
			holderDone <- err
			return
		}
		close(locked)
		<-release
		_, err = tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
		if err != nil {
			_ = tx.Rollback(ctx)
			holderDone <- err
			return
		}
		holderDone <- tx.Commit(ctx)
	}()

	<-locked
	counterCh := make(chan error, 1)
	go func() {
		counterCh <- h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sess.ID, SenderID: sellerID, Price: 380000})
	}()

	select {
	case err := <-counterCh:
		t.Fatalf("Counter returned early while holder holds lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-holderDone:
		if err != nil {
			t.Fatalf("holder commit failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("holder timeout")
	}
	select {
	case err := <-counterCh:
		var notNegotiable *negotiationApp.ErrResourceNotNegotiable
		if !errors.As(err, &notNegotiable) {
			t.Fatalf("expected ErrResourceNotNegotiable after sold wins (counter), got %T: %v", err, err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("counter did not return after release")
	}

	repo := negotiationImpl.NewNegotiationRepository()
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		s, _ := repo.GetSession(ctx, tx, sess.ID)
		if s.CurrentPrice == nil || *s.CurrentPrice != 400000 {
			t.Fatalf("counter mutated price despite block: %v", s.CurrentPrice)
		}
		return nil
	})
}

// TestN8D_Deterministic_NegotiationWins proves Case B: negotiation holder
// acquires for_sales FOR UPDATE while negotiable, sold waiter must wait,
// negotiation commits, sold commits, subsequent mutation rejected.
func TestN8D_Deterministic_NegotiationWins(t *testing.T) {
	ctx := context.Background()
	h, cleanup := setupNegotiationLinkageHarness(t)
	defer cleanup()
	buyerID := insertLinkageTestUser(t, ctx, h.tdb)
	sellerID := insertLinkageTestUser(t, ctx, h.tdb)
	var forSaleID, roomID uuid.UUID
	var sellerOtherRoom uuid.UUID
	_ = h.tdb.WithTx(ctx, func(tx db.Tx) error {
		forSaleID = insertLinkageTestForSale(t, ctx, tx, h.forSaleRepo, sellerID)
		roomID = insertLinkageTestDirectRoom(t, ctx, tx, buyerID, sellerID)
		sellerOtherRoom = roomID
		return nil
	})

	negLocked := make(chan struct{})
	negRelease := make(chan struct{})
	negDone := make(chan uuid.UUID, 1)
	negErrCh := make(chan error, 1)

	go func() {
		conn, err := h.tdb.Pool().Acquire(ctx)
		if err != nil {
			negErrCh <- err
			return
		}
		defer conn.Release()
		tx, err := conn.Begin(ctx)
		if err != nil {
			negErrCh <- err
			return
		}
		_, err = tx.Exec(ctx, `SELECT id FROM for_sales WHERE id=$1 FOR UPDATE`, forSaleID)
		if err != nil {
			_ = tx.Rollback(ctx)
			negErrCh <- err
			return
		}
		// verify still active/available before holding
		var status string
		var qty int
		var negEnabled bool
		_ = tx.QueryRow(ctx, `SELECT status, quantity_available, negotiation_enabled FROM for_sales WHERE id=$1`, forSaleID).Scan(&status, &qty, &negEnabled)
		if status != "active" || qty == 0 || !negEnabled {
			_ = tx.Rollback(ctx)
			negErrCh <- errors.New("for_sale not negotiable in holder")
			return
		}
		close(negLocked)
		<-negRelease

		sess := negotiationEntity.NewNegotiationSession(negotiationEntity.NegotiationResourceForSale, forSaleID, buyerID, sellerID)
		sess.SetChatRoomID(sellerOtherRoom)
		_ = sess.SetCurrentPrice(400000)
		repo := negotiationImpl.NewNegotiationRepository()
		if err := repo.CreateSession(ctx, tx, sess); err != nil {
			_ = tx.Rollback(ctx)
			negErrCh <- err
			return
		}
		hist := negotiationEntity.NewNegotiationPriceHistory(sess.ID, sess.ProposalSequence, nil, 400000, buyerID, "initial_proposal")
		if err := repo.CreatePriceHistoryEntry(ctx, tx, hist); err != nil {
			_ = tx.Rollback(ctx)
			negErrCh <- err
			return
		}
		{
			payload := []byte(`{}`)
			id := sess.ID
			// mirror OutboxRepository.InsertEvent columns
			_, err2 := tx.Exec(ctx, `INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, status, retry_count, next_attempt_at, created_at, updated_at, idempotency_key) VALUES (gen_random_uuid(),'negotiation',$1,'negotiation.started',$2,'pending',0,NOW(),NOW(),NOW(),$3)`, sess.ID, payload, "negotiation.started."+id.String())
			if err2 != nil {
				_ = tx.Rollback(ctx)
				negErrCh <- err2
				return
			}
		}
		if err := tx.Commit(ctx); err != nil {
			negErrCh <- err
			return
		}
		negDone <- sess.ID
		negErrCh <- nil
	}()

	<-negLocked

	soldStarted := make(chan struct{})
	soldDone := make(chan error, 1)
	go func() {
		close(soldStarted)
		// This will block on same for_sales row until neg holder commits
		err := h.tdb.WithTx(ctx, func(tx db.Tx) error {
			_, err := tx.Exec(ctx, `SELECT id FROM for_sales WHERE id=$1 FOR UPDATE`, forSaleID)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE for_sales SET status='sold', quantity_available=0, sold_at=NOW(), updated_at=NOW() WHERE id=$1`, forSaleID)
			return err
		})
		soldDone <- err
	}()

	<-soldStarted
	// sold must be blocked — verify not done quickly
	select {
	case err := <-soldDone:
		t.Fatalf("sold transaction returned early while negotiation holder still holds lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	close(negRelease)

	var sessID uuid.UUID
	select {
	case id := <-negDone:
		sessID = id
	case <-time.After(5 * time.Second):
		t.Fatal("negotiation holder did not commit")
	}
	select {
	case err := <-negErrCh:
		if err != nil {
			t.Fatalf("negotiation holder error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("negErr timeout")
	}
	select {
	case err := <-soldDone:
		if err != nil {
			t.Fatalf("sold commit failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sold did not unblock after negotiation commit")
	}

	// for_sale now sold, but session was created while active — historical allowed
	// subsequent counter must be blocked
	err2 := h.svc.SendCounterOffer(ctx, negotiationApp.SendCounterOfferRequest{SessionID: sessID, SenderID: buyerID, Price: 390000})
	var notNegotiable *negotiationApp.ErrResourceNotNegotiable
	if !errors.As(err2, &notNegotiable) {
		t.Fatalf("expected subsequent counter blocked after negotiation-win then sold, got %T: %v", err2, err2)
	}
}
