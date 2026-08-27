package entity

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

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

func TestNegotiationSessionAccept(t *testing.T) {
	forSaleID := uuid.New()
	session := NewNegotiationSession(
		NegotiationResourceForSale,
		forSaleID,
		uuid.New(),
		uuid.New(),
	)

	// Successful accept
	err := session.Accept()
	if err != nil {
		t.Errorf("Accept() unexpected error: %v", err)
	}

	if session.Status != NegotiationStatusAccepted {
		t.Errorf("Accept() Status = %s, want %s", session.Status, NegotiationStatusAccepted)
	}

	// Cannot accept twice
	err = session.Accept()
	if err == nil {
		t.Error("Accept() should error when already accepted")
	}

	var transitionErr *InvalidTransitionError
	if err == nil || err.(*InvalidTransitionError) == nil {
		_, ok := err.(*InvalidTransitionError)
		if !ok {
			t.Errorf("Accept() should return InvalidTransitionError, got %T", err)
		}
	} else {
		transitionErr = err.(*InvalidTransitionError)
		if transitionErr.CurrentStatus != NegotiationStatusAccepted {
			t.Errorf("InvalidTransitionError.CurrentStatus = %s, want %s", transitionErr.CurrentStatus, NegotiationStatusAccepted)
		}
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
