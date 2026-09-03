// Tests for PASS_5B admin emergency auction cancel/override.
//
// applyAdminCancel is the pure, in-memory decision-and-mutation core of
// AdminCancel — no repository or transaction is needed to test the
// safe/conflict state contract exhaustively. The repo/transaction glue
// (GetForUpdate / UpdateTx / outbox insert) is identical in shape to the
// already-proven Cancel and CancelForModeration methods and is exercised
// end-to-end at the HTTP handler layer via a fake service.
package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
)

// TestAdminCancel_EmptyReason_RejectsBeforeTouchingRepo proves the
// reason-required guard fires before any repository call — a zero-value
// AuctionService (nil auctionRepo) is safe to use here because the method
// must never reach s.auctionRepo.GetForUpdate for an empty/whitespace
// reason.
func TestAdminCancel_EmptyReason_RejectsBeforeTouchingRepo(t *testing.T) {
	svc := &AuctionService{}

	cases := []string{"", "   ", "\t\n"}
	for _, reason := range cases {
		_, _, err := svc.AdminCancel(context.Background(), nil, AdminCancelInput{
			AuctionID: uuid.New(),
			Reason:    reason,
		})
		if err != ErrAuctionCancelReasonRequired {
			t.Fatalf("reason=%q: expected ErrAuctionCancelReasonRequired, got %v", reason, err)
		}
	}
}

func newTestAuctionForCancel(status entity.Status) *entity.Auction {
	return &entity.Auction{
		ID:       uuid.New(),
		SellerID: uuid.New(),
		Status:   status,
	}
}

func TestApplyAdminCancel_SafeStates_Succeed(t *testing.T) {
	cases := []struct {
		name   string
		status entity.Status
	}{
		{"draft", entity.StatusDraft},
		{"scheduled", entity.StatusScheduled},
		{"active_no_bids", entity.StatusActive},
		{"waiting_settlement", entity.StatusWaitingSettlement},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auction := newTestAuctionForCancel(tc.status)
			if err := applyAdminCancel(auction); err != nil {
				t.Fatalf("expected admin cancel to succeed from %s, got error: %v", tc.status, err)
			}
			if auction.Status != entity.StatusCancelled {
				t.Fatalf("expected status=cancelled, got %s", auction.Status)
			}
		})
	}
}

// TestApplyAdminCancel_ActiveWithBids_Succeeds proves the admin path
// bypasses the bid-count restriction in entity.Auction.CanCancel() (which
// the seller-facing Cancel service method respects) — this is the emergency-
// intervention capability an admin needs that a seller does not have.
// Bid rows themselves are never touched by this function; only the
// auction's own Status field changes.
func TestApplyAdminCancel_ActiveWithBids_Succeeds(t *testing.T) {
	auction := newTestAuctionForCancel(entity.StatusActive)
	bid := int64(50_000)
	winner := uuid.New()
	auction.CurrentBid = &bid
	auction.CurrentWinnerID = &winner

	if err := applyAdminCancel(auction); err != nil {
		t.Fatalf("expected admin cancel to succeed on active auction with bids, got error: %v", err)
	}
	if auction.Status != entity.StatusCancelled {
		t.Fatalf("expected status=cancelled, got %s", auction.Status)
	}
	// Bid fields must survive untouched — history/traceability preserved.
	if auction.CurrentBid == nil || *auction.CurrentBid != bid {
		t.Fatal("expected CurrentBid to remain unchanged after admin cancel")
	}
	if auction.CurrentWinnerID == nil || *auction.CurrentWinnerID != winner {
		t.Fatal("expected CurrentWinnerID to remain unchanged after admin cancel")
	}
}

func TestApplyAdminCancel_TerminalStates_ReturnConflict(t *testing.T) {
	cases := []struct {
		name   string
		status entity.Status
	}{
		{"ended", entity.StatusEnded},
		{"already_cancelled", entity.StatusCancelled},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auction := newTestAuctionForCancel(tc.status)
			err := applyAdminCancel(auction)
			if err == nil {
				t.Fatalf("expected conflict error from terminal state %s, got nil", tc.status)
			}
			var conflictErr *ErrAuctionCancelConflict
			if !isAuctionCancelConflict(err, &conflictErr) {
				t.Fatalf("expected *ErrAuctionCancelConflict, got %T: %v", err, err)
			}
			// Status must remain unchanged on conflict — no partial mutation.
			if auction.Status != tc.status {
				t.Fatalf("expected status to remain %s on conflict, got %s", tc.status, auction.Status)
			}
		})
	}
}

// TestApplyAdminCancel_HasOrder_ReturnsConflict proves the defense-in-depth
// OrderID guard fires even in a state the transition matrix would otherwise
// allow (constructed deliberately to prove the guard, not because this
// combination is reachable in production — see applyAdminCancel's comment).
func TestApplyAdminCancel_HasOrder_ReturnsConflict(t *testing.T) {
	auction := newTestAuctionForCancel(entity.StatusActive)
	orderID := uuid.New()
	auction.OrderID = &orderID

	err := applyAdminCancel(auction)
	if err == nil {
		t.Fatal("expected conflict error when auction already has an order")
	}
	var conflictErr *ErrAuctionCancelConflict
	if !isAuctionCancelConflict(err, &conflictErr) {
		t.Fatalf("expected *ErrAuctionCancelConflict, got %T: %v", err, err)
	}
	if auction.Status != entity.StatusActive {
		t.Fatal("expected status to remain unchanged when blocked by OrderID guard")
	}
}

// isAuctionCancelConflict is a tiny errors.As wrapper kept local to this
// test file to avoid importing the errors package solely for one helper.
func isAuctionCancelConflict(err error, target **ErrAuctionCancelConflict) bool {
	ce, ok := err.(*ErrAuctionCancelConflict)
	if !ok {
		return false
	}
	*target = ce
	return true
}
