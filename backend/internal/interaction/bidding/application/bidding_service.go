package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/labuda/backend/internal/commerce/auction/entity"
	auctionRepo "github.com/labuda/backend/internal/commerce/auction/infrastructure/repository"
	"github.com/labuda/backend/pkg/db"
)

// BiddingItem represents a user's bidding view for a single auction.
// Wire contract is snake_case (canonical My Bids API).
type BiddingItem struct {
	AuctionID   uuid.UUID `json:"auction_id"`
	Title       string    `json:"title"`
	YourLastBid int64     `json:"your_last_bid"`
	CurrentBid  int64     `json:"current_bid"`
	Status      string    `json:"status"` // leading | outbid | waiting_claim
	EndAt       time.Time `json:"end_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BiddingResult holds the result of GetUserBidding with aggregated counts.
// My Bids has no history tab: only open (active + waiting_settlement)
// auctions are returned, so the only count is the active one.
type BiddingResult struct {
	Items       []BiddingItem `json:"items"`
	ActiveCount int           `json:"active_count"`
}

// BiddingService provides user bidding view aggregation.
// This is a read-only service that aggregates data from auction and auction_bid repositories.
type BiddingService struct {
	auctionRepo *auctionRepo.AuctionRepository
	bidRepo     *auctionRepo.AuctionBidRepository
}

// NewBiddingService creates a new BiddingService.
func NewBiddingService() *BiddingService {
	return &BiddingService{
		auctionRepo: auctionRepo.NewAuctionRepository(),
		bidRepo:     auctionRepo.NewAuctionBidRepository(),
	}
}

// GetUserBidding retrieves the user's My Bids view: auctions with an open
// (not yet final) bidding process for this user, i.e. canonical auction
// states active and waiting_settlement only. Ended, cancelled and lapsed
// auctions are hidden — My Bids is not a bid-history archive (raw history
// stays canonical in auction_bids). This service is the single visibility
// authority; consumers are pure projection/presentation.
func (s *BiddingService) GetUserBidding(
	ctx context.Context,
	tx db.Tx,
	userID uuid.UUID,
) (*BiddingResult, error) {
	// Fetch all auction IDs where user has placed bids
	auctionIDs, err := s.bidRepo.ListAuctionIDsByBidder(ctx, tx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list auction IDs by bidder: %w", err)
	}

	// Early return if no auctions
	if len(auctionIDs) == 0 {
		return &BiddingResult{
			Items:       []BiddingItem{},
			ActiveCount: 0,
		}, nil
	}

	// Load all auctions
	items := make([]BiddingItem, 0, len(auctionIDs))

	for _, auctionID := range auctionIDs {
		// Get auction
		auction, err := s.auctionRepo.GetByID(ctx, tx, auctionID)
		if err != nil {
			// Skip auctions that can't be loaded (may have been deleted)
			continue
		}

		// Canonical My Bids visibility: only open processes.
		// Ended/cancelled/lapsed (and scheduled) are hidden, never
		// re-labeled as lost for My Bids presentation.
		if auction.Status != entity.StatusActive &&
			auction.Status != entity.StatusWaitingSettlement {
			continue
		}

		// Get user's latest bid for this auction
		userBid, err := s.bidRepo.GetUserLastBidForAuction(ctx, tx, userID, auctionID)
		if err != nil {
			// Skip if we can't get user bid
			continue
		}
		if userBid == nil {
			// User should have a bid if auction ID came from ListAuctionIDsByBidder
			// Skip this entry if something is inconsistent
			continue
		}

		// Derive status based on auction status and current winner
		status := s.deriveStatus(userID, auction)

		// Map data to BiddingItem
		currentBid := int64(0)
		if auction.CurrentBid != nil {
			currentBid = *auction.CurrentBid
		}

		item := BiddingItem{
			AuctionID:   auction.ID,
			Title:       auction.Product.Title,
			YourLastBid: userBid.Amount,
			CurrentBid:  currentBid,
			Status:      status,
			EndAt:       auction.EndAt,
			UpdatedAt:   auction.UpdatedAt,
		}

		items = append(items, item)
	}

	// Sort items
	s.sortBiddingItems(items)

	return &BiddingResult{
		Items:       items,
		ActiveCount: len(items),
	}, nil
}

// deriveStatus determines the user's bidding status for an auction.
// Status mapping:
//
//	IF auction.status == "active":
//	  IF user_id == auction.current_winner_id:
//	    status = "leading"
//	  ELSE:
//	    status = "outbid"
//
//	IF auction.status == "waiting_settlement":
//	  IF user_id == auction.current_winner_id:
//	    status = "waiting_claim"
//	  ELSE:
//	    status = "lost"
//
//	IF auction.status == "ended":
//	  IF user_id == auction.current_winner_id:
//	    status = "won"
//	  ELSE:
//	    status = "lost"
//
//	Settlement failure AUTO-RESCHEDULES the auction (scheduled, start=now);
//	the previous winner is no longer the current winner, so every participant
//	derives "lost" from the default branch.
func (s *BiddingService) deriveStatus(userID uuid.UUID, auction *entity.Auction) string {
	isWinner := auction.CurrentWinnerID != nil && *auction.CurrentWinnerID == userID

	switch auction.Status {
	case entity.StatusActive:
		if isWinner {
			return "leading"
		}
		return "outbid"

	case entity.StatusWaitingSettlement:
		if isWinner {
			return "waiting_claim"
		}
		return "lost"

	case entity.StatusEnded:
		if isWinner {
			return "won"
		}
		return "lost"

	default:
		// For scheduled, cancelled, lapsed (and unknown) - treat as lost
		return "lost"
	}
}

// sortBiddingItems sorts bidding items by status priority:
//  1. ACTIVE (leading + outbid) + WAITING_SETTLEMENT (waiting_claim)
//     -> sort by EndAt ASC (soonest ending first)
//
// The non-open tail branch is retained for determinism; My Bids visibility
// only surfaces open auctions so it is normally empty.
func (s *BiddingService) sortBiddingItems(items []BiddingItem) {
	sort.SliceStable(items, func(i, j int) bool {
		iActive := isActiveStatus(items[i].Status)
		jActive := isActiveStatus(items[j].Status)

		// Active items come first
		if iActive && !jActive {
			return true
		}
		if !iActive && jActive {
			return false
		}

		// Within active: sort by EndAt ASC (soonest ending first)
		if iActive {
			return items[i].EndAt.Before(items[j].EndAt)
		}

		// Within ended: sort by EndAt DESC (most recently ended first)
		return items[i].EndAt.After(items[j].EndAt)
	})
}

// isActiveStatus checks if a status represents an "active" bidding state.
func isActiveStatus(status string) bool {
	return status == "leading" || status == "outbid" || status == "waiting_claim"
}
