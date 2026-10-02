package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

// OWNER TRUTH: a deal (accept) is valid for 24h FROM THE DEAL — AcceptWithPrice
// must refresh ExpiresAt instead of inheriting the creation-time expiry.
func TestAcceptWithPriceRefreshesDealExpiry(t *testing.T) {
	session := NewNegotiationSession(
		NegotiationResourceForSale,
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)

	// Simulate a counter cycle that consumed (and overshot) the original window.
	past := time.Now().Add(-time.Hour)
	session.ExpiresAt = &past
	if err := session.SetCurrentPrice(25000); err != nil {
		t.Fatalf("SetCurrentPrice() unexpected error: %v", err)
	}

	if err := session.AcceptWithPrice(); err != nil {
		t.Fatalf("AcceptWithPrice() unexpected error: %v", err)
	}

	if session.IsExpired() {
		t.Error("accepted deal must not inherit the pre-deal expiry")
	}
	if session.ExpiresAt == nil {
		t.Fatal("accepted deal must carry a concrete ExpiresAt")
	}
	remaining := time.Until(*session.ExpiresAt)
	if remaining < 23*time.Hour || remaining > 24*time.Hour+time.Minute {
		t.Errorf("deal expiry = %v after accept, want ~24h from the deal", remaining)
	}
}

func TestNewNegotiationSession(t *testing.T) {
	resourceType := NegotiationResourceForSale
	forSaleID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()

	session := NewNegotiationSession(resourceType, forSaleID, buyerID, sellerID)

	if session.ID == uuid.Nil {
		t.Error("NewNegotiationSession() should generate non-nil ID")
	}

	if session.ResourceType != resourceType {
		t.Errorf("NewNegotiationSession() ResourceType = %s, want %s", session.ResourceType, resourceType)
	}

	if session.ForSaleID != forSaleID {
		t.Errorf("NewNegotiationSession() ForSaleID = %s, want %s", session.ForSaleID, forSaleID)
	}

	if session.BuyerID != buyerID {
		t.Errorf("NewNegotiationSession() BuyerID = %s, want %s", session.BuyerID, buyerID)
	}

	if session.SellerID != sellerID {
		t.Errorf("NewNegotiationSession() SellerID = %s, want %s", session.SellerID, sellerID)
	}

	if session.Status != NegotiationStatusActive {
		t.Errorf("NewNegotiationSession() Status = %s, want %s", session.Status, NegotiationStatusActive)
	}

	if time.Now().Sub(session.CreatedAt) > time.Second {
		t.Error("NewNegotiationSession() CreatedAt should be very recent")
	}

	if time.Now().Sub(session.UpdatedAt) > time.Second {
		t.Error("NewNegotiationSession() UpdatedAt should be very recent")
	}
}

func TestNegotiationSessionCancel(t *testing.T) {
	forSaleID := uuid.New()
	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		uuid.New(),
		uuid.New(),
	)

	// Successful cancel
	err := session.Cancel()
	if err != nil {
		t.Errorf("Cancel() unexpected error: %v", err)
	}

	if session.Status != NegotiationStatusCancelled {
		t.Errorf("Cancel() Status = %s, want %s", session.Status, NegotiationStatusCancelled)
	}

	// Cannot cancel twice
	err = session.Cancel()
	if err == nil {
		t.Error("Cancel() should error when already cancelled")
	}
}

func TestNegotiationSessionExpire(t *testing.T) {
	forSaleID := uuid.New()
	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		uuid.New(),
		uuid.New(),
	)

	// Successful expire
	err := session.Expire()
	if err != nil {
		t.Errorf("Expire() unexpected error: %v", err)
	}

	if session.Status != NegotiationStatusExpired {
		t.Errorf("Expire() Status = %s, want %s", session.Status, NegotiationStatusExpired)
	}

	// Cannot expire twice
	err = session.Expire()
	if err == nil {
		t.Error("Expire() should error when already expired")
	}
}

func TestNegotiationSessionEnsureActive(t *testing.T) {
	t.Run("active session", func(t *testing.T) {
		forSaleID := uuid.New()
		session := NewNegotiationSession(
			NegotiationResourceForSale,
			forSaleID,
			uuid.New(),
			uuid.New(),
		)

		err := session.EnsureSessionActive()
		if err != nil {
			t.Errorf("EnsureSessionActive() unexpected error: %v", err)
		}
	})

	t.Run("accepted session", func(t *testing.T) {
		forSaleID := uuid.New()
		session := NewNegotiationSession(
			NegotiationResourceForSale,
			forSaleID,
			uuid.New(),
			uuid.New(),
		)
		session.Status = NegotiationStatusAccepted

		err := session.EnsureSessionActive()
		if err == nil {
			t.Error("EnsureSessionActive() should error when not active")
		}

		var notActiveErr *SessionNotActiveError
		if err == nil || err.(*SessionNotActiveError) == nil {
			_, ok := err.(*SessionNotActiveError)
			if !ok {
				t.Errorf("EnsureSessionActive() should return SessionNotActiveError, got %T", err)
			}
		} else {
			notActiveErr = err.(*SessionNotActiveError)
			if notActiveErr.CurrentStatus != NegotiationStatusAccepted {
				t.Errorf("SessionNotActiveError.CurrentStatus = %s, want %s", notActiveErr.CurrentStatus, NegotiationStatusAccepted)
			}
		}
	})
}

func TestNegotiationSessionIsParticipant(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	otherID := uuid.New()
	forSaleID := uuid.New()

	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		buyerID,
		sellerID,
	)

	if !session.IsParticipant(buyerID) {
		t.Error("IsParticipant() should return true for buyer")
	}

	if !session.IsParticipant(sellerID) {
		t.Error("IsParticipant() should return true for seller")
	}

	if session.IsParticipant(otherID) {
		t.Error("IsParticipant() should return false for non-participant")
	}
}

func TestNegotiationSessionIsBuyer(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()

	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		buyerID,
		sellerID,
	)

	if !session.IsBuyer(buyerID) {
		t.Error("IsBuyer() should return true for buyer")
	}

	if session.IsBuyer(sellerID) {
		t.Error("IsBuyer() should return false for seller")
	}
}

func TestNegotiationSessionIsSeller(t *testing.T) {
	buyerID := uuid.New()
	sellerID := uuid.New()
	forSaleID := uuid.New()

	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		buyerID,
		sellerID,
	)

	if !session.IsSeller(sellerID) {
		t.Error("IsSeller() should return true for seller")
	}

	if session.IsSeller(buyerID) {
		t.Error("IsSeller() should return false for buyer")
	}
}

func TestInvalidTransitionError(t *testing.T) {
	err := &InvalidTransitionError{
		SessionID:     uuid.New(),
		CurrentStatus: NegotiationStatusActive,
		TargetStatus:  NegotiationStatusAccepted,
	}

	expected := "invalid negotiation status transition: session_id=" + err.SessionID.String() + ", active -> accepted"
	if err.Error() != expected {
		t.Errorf("InvalidTransitionError.Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSessionNotActiveError(t *testing.T) {
	err := &SessionNotActiveError{
		SessionID:     uuid.New(),
		CurrentStatus: NegotiationStatusCancelled,
	}

	expected := "negotiation session not active: session_id=" + err.SessionID.String() + ", current_status=cancelled"
	if err.Error() != expected {
		t.Errorf("SessionNotActiveError.Error() = %s, want %s", err.Error(), expected)
	}
}
