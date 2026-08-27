package entity

import (
	"testing"
	"time"
)

// ============================================================================
// MODERATION STATE MACHINE TESTS
// ============================================================================
//
// These tests verify the Cancel() transition matrix for governance enforcement.
// FIX-1A: waiting_settlement → cancelled must be allowed (owner decision 2026-05-28).

// TestCancel_FromWaitingSettlement_Succeeds proves the new state machine
// transition is wired: moderation can cancel a waiting_settlement auction.
func TestCancel_FromWaitingSettlement_Succeeds(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusWaitingSettlement
	deadline := time.Now().Add(24 * time.Hour)
	auction.SettlementDeadline = &deadline

	err := auction.Cancel()
	if err != nil {
		t.Errorf("Cancel() from waiting_settlement failed: %v — want nil", err)
	}
	if auction.Status != StatusCancelled {
		t.Errorf("Status = %s, want %s", auction.Status, StatusCancelled)
	}
}

// TestCancel_FromEnded_Fails proves ended is a terminal state (unchanged).
func TestCancel_FromEnded_Fails(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusEnded

	err := auction.Cancel()
	if err == nil {
		t.Error("Cancel() from ended returned nil, want InvalidTransitionError")
	}
	var ite *InvalidTransitionError
	if ok := isInvalidTransition(err, &ite); !ok {
		t.Errorf("Cancel() from ended: error type = %T, want *InvalidTransitionError", err)
	}
}

// TestCancel_FromExpiredBNR_Fails proves expired_bnr is a terminal state (unchanged).
func TestCancel_FromExpiredBNR_Fails(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusExpiredBNR

	err := auction.Cancel()
	if err == nil {
		t.Error("Cancel() from expired_bnr returned nil, want InvalidTransitionError")
	}
	var ite *InvalidTransitionError
	if ok := isInvalidTransition(err, &ite); !ok {
		t.Errorf("Cancel() from expired_bnr: error type = %T, want *InvalidTransitionError", err)
	}
}

// TestCancel_FromCancelled_Fails proves cancelled is a terminal state (idempotency guard).
func TestCancel_FromCancelled_Fails(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusCancelled

	err := auction.Cancel()
	if err == nil {
		t.Error("Cancel() from cancelled returned nil, want InvalidTransitionError")
	}
	var ite *InvalidTransitionError
	if ok := isInvalidTransition(err, &ite); !ok {
		t.Errorf("Cancel() from cancelled: error type = %T, want *InvalidTransitionError", err)
	}
}

// TestCancel_FromActive_WithBids_Succeeds proves moderation enforcement applies
// regardless of existing bids (governance bypass — unlike seller Cancel which
// requires CanCancel() = no bids).
func TestCancel_FromActive_WithBids_Succeeds(t *testing.T) {
	auction := createTestDraftAuction()
	auction.Status = StatusActive
	// Simulate bids
	bid := int64(25000)
	auction.CurrentBid = &bid

	// entity.Cancel() does NOT check bids — CanCancel() is only called by
	// AuctionService.Cancel (seller path). CancelForModeration calls Cancel() directly.
	err := auction.Cancel()
	if err != nil {
		t.Errorf("Cancel() from active-with-bids failed: %v — want nil", err)
	}
	if auction.Status != StatusCancelled {
		t.Errorf("Status = %s, want %s", auction.Status, StatusCancelled)
	}
}

// isInvalidTransition is a test helper that checks if an error is *InvalidTransitionError.
func isInvalidTransition(err error, out **InvalidTransitionError) bool {
	if err == nil {
		return false
	}
	ite, ok := err.(*InvalidTransitionError)
	if ok && out != nil {
		*out = ite
	}
	return ok
}


