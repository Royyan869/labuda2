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
type BiddingItem struct {
	AuctionID    uuid.UUID
	Title        string
	YourLastBid  int64
	CurrentBid   int64
	Status       string // leading | outbid | won | lost | waiting_claim
	EndAt        time.Time
	UpdatedAt    time.Time
}

// BiddingResult holds the result of GetUserBidding with aggregated counts.
type BiddingResult struct {
	Items       []BiddingItem
	ActiveCount int
	WonCount    int
	LostCount   int
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

// GetUserBidding retrieves all auctions where the user has placed bids,
// aggregated with user's bid information and derived status.
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
			WonCount:    0,
			LostCount:   0,
		}, nil
	}

	// Load all auctions
	items := make([]BiddingItem, 0, len(auctionIDs))
	var activeCount, wonCount, lostCount int

	for _, auctionID := range auctionIDs {
		// Get auction
		auction, err := s.auctionRepo.GetByID(ctx, tx, auctionID)
		if err != nil {
			// Skip auctions that can't be loaded (may have been deleted)
			continue
		}

		// Get user's last bid for this auction
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

		// Update counters
		switch status {
		case "leading", "waiting_claim":
			activeCount++
		case "won":
			wonCount++
		case "outbid", "lost":
			lostCount++
		}
	}

	// Sort items
	s.sortBiddingItems(items)

	return &BiddingResult{
		Items:       items,
		ActiveCount: activeCount,
		WonCount:    wonCount,
		LostCount:   lostCount,
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
//	IF auction.status == "expired_bnr":
//	  IF user_id == auction.current_winner_id:
//	    status = "lost"
//	  ELSE:
//	    status = "lost"
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

	case entity.StatusExpiredBNR:
		// Everyone loses when settlement deadline expires
		return "lost"

	default:
		// For draft, scheduled, cancelled - treat as lost
		return "lost"
	}
}

// sortBiddingItems sorts bidding items by status priority:
// 1. ACTIVE (active + waiting_settlement) -> sort by EndAt ASC
// 2. ENDED (ended + expired_bnr) -> sort by EndAt DESC
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


