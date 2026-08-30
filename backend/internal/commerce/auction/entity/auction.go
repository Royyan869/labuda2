// DOMAIN: COMMERCE
// NOTE: Commerce auction system for dynamic pricing

package entity

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	productEntity "github.com/labuda/backend/internal/commerce/product/entity"
)

// Status represents the auction status as a strict state machine.
//
// BOUNDARY NORMALIZATION (PHASE 1D):
// Auction lifecycle follows explicit states with clear boundaries:
//
// WORKSPACE (draft):
//   - Fully editable by seller
//   - No market authority required
//   - Can transition to: scheduled, cancelled
//
// PRE-MARKET COMMITMENT (scheduled):
//   - Seller has committed auction for future market run
//   - Limited editing allowed (title, description, timing)
//   - Market authority checked at schedule time
//   - Can transition to: active, cancelled, draft
//   - Market authority RE-CHECKED at activation time
//
// LIVE MARKET (active):
//   - Auction is currently accepting bids
//   - Immutable except for bid updates
//   - Can be cancelled only if no bids
//   - Can transition to: ended, cancelled
//
// HISTORICAL/TERMINAL (ended, cancelled):
//   - ended: Normal completion (time expired or buy now)
//   - cancelled: Seller cancelled or subscription expired before activation
//   - Terminal states with no further transitions
//
// IMPORTANT: Status is the AUTHORITY for all business decisions.
// Time boundaries (start_at, end_at) are TRIGGERS for state transitions,
// not direct decision factors for bid operability.
//
// TIME vs LIFECYCLE:
// - start_at triggers scheduled -> active transition
// - end_at triggers active -> ended transition
// - BUT: actual status determines what actions are allowed
//
// BID OPERABILITY:
// - Bids ONLY accepted when status == StatusActive
// - PlaceBid() enforces: status check AND time check
// - Time check is belt-and-suspenders; status is primary
type Status string

const (
	// StatusDraft is the initial state when auction is created.
	// Fully editable.
	StatusDraft Status = "draft"

	// StatusScheduled is when auction is scheduled but not yet started.
	// Editable with restricted fields.
	StatusScheduled Status = "scheduled"

	// StatusActive is when auction is running.
	// Immutable except for bids and cancellation (if no bids).
	StatusActive Status = "active"

	// StatusWaitingSettlement is when auction has ended but order not yet created.
	// Winner can claim auction to create order.
	StatusWaitingSettlement Status = "waiting_settlement"

	// StatusExpiredBNR is when auction winner didn't claim within settlement deadline.
	// Terminal state - no order can be created for this auction.
	StatusExpiredBNR Status = "expired_bnr"

	// StatusEnded is when auction completes normally (time expires or buy now).
	// Terminal state - auction has been settled (order created).
	StatusEnded Status = "ended"

	// StatusCancelled is when auction is cancelled.
	// Terminal state.
	StatusCancelled Status = "cancelled"
)

// transitionAllowed defines valid state transitions.
// The state machine enforces business rules at the entity level.
var transitionAllowed = map[Status][]Status{
	StatusDraft:             {StatusScheduled, StatusCancelled},
	StatusScheduled:         {StatusActive, StatusCancelled, StatusDraft}, // Can revert to draft
	StatusActive:            {StatusWaitingSettlement, StatusEnded, StatusCancelled},
	StatusWaitingSettlement: {StatusEnded, StatusExpiredBNR, StatusCancelled}, // After claim/order created OR settlement deadline expired OR moderation enforcement
	StatusExpiredBNR:        {},                                               // Terminal state - winner didn't claim in time
	StatusEnded:             {},                                               // Terminal state
	StatusCancelled:         {},                                               // Terminal state
}

// canTransition checks if a state transition is allowed.
func canTransition(from, to Status) bool {
	allowed, exists := transitionAllowed[from]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// IsRepostable returns true if the auction is in a state where social reposts
// are permitted.
//
// REPOST POLICY: Only scheduled and active auctions can be reposted.
// Terminal states (ended, cancelled, expired_bnr, waiting_settlement) and
// draft/unknown statuses are not repostable.
//
// waiting_settlement is excluded because the auction's outcome is decided
// (winner exists) but settlement hasn't completed — reposting is semantically
// misleading. Use scheduled/active to confirm the auction is still open.
//
// This is the single source of truth for the repost creation gate
// (commerceResponse.Validator via content_service.validateCommerceReference) and the read-side governance filter
// (feed/search SQL NOT EXISTS checks for targetType='auction').
func (s Status) IsRepostable() bool {
	return s == StatusScheduled || s == StatusActive
}

// PublicLifecycle returns the coarsened public lifecycle string for this
// auction status. The public vocabulary is intentionally narrow:
//
//	active       — buyable / bid-able now (active or awaiting winner settlement)
//	unavailable  — not currently buyable (draft / scheduled / terminal states)
//	removed      — reserved for moderation/hard-delete; Status does not model
//	               these today so this method never returns "removed".
//
// Internal enum values (waiting_settlement, expired_bnr, scheduled, …) MUST NOT
// cross the public boundary. Public surfaces should call this method and emit
// the result instead of the raw enum text.
func (s Status) PublicLifecycle() string {
	switch s {
	case StatusActive, StatusWaitingSettlement:
		return "active"
	case StatusDraft, StatusScheduled, StatusExpiredBNR, StatusEnded, StatusCancelled:
		return "unavailable"
	default:
		return "unavailable"
	}
}

// String returns the string representation of the auction status.
func (s Status) String() string {
	return string(s)
}

// IsPublicDiscoverable returns true when this auction status is eligible to
// appear in anonymous public discovery (browse/search). Only pre-sale and
// live-sale surfaces qualify: draft (workspace), cancelled, waiting_settlement,
// ended (settled/no-winner) and expired_bnr are non-public/historical states.
func (s Status) IsPublicDiscoverable() bool {
	switch s {
	case StatusScheduled, StatusActive:
		return true
	default:
		return false
	}
}

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid auction status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// InvalidOperationError is returned when an operation is not allowed in current state.
type InvalidOperationError struct {
	Status Status
	Reason string
}

func (e *InvalidOperationError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid operation in status %s: %s", e.Status, e.Reason)
	}
	return fmt.Sprintf("invalid operation in status %s", e.Status)
}

// BidTooLowError is returned when bid amount is below minimum.
type BidTooLowError struct {
	MinimumBid int64
	OfferedBid int64
}

func (e *BidTooLowError) Error() string {
	return fmt.Sprintf("bid too low: minimum %d, offered %d", e.MinimumBid, e.OfferedBid)
}

// SelfBidError is returned when bidder tries to bid on own auction.
type SelfBidError struct {
	BidderID uuid.UUID
	SellerID uuid.UUID
}

func (e *SelfBidError) Error() string {
	return fmt.Sprintf("cannot bid on own auction: bidder=%s, seller=%s", e.BidderID, e.SellerID)
}

// AuctionEndedError is returned when trying to bid on ended auction.
type AuctionEndedError struct {
	AuctionID uuid.UUID
	EndAt     time.Time
}

func (e *AuctionEndedError) Error() string {
	return fmt.Sprintf("auction has ended: id=%s, ended_at=%s", e.AuctionID, e.EndAt.Format(time.RFC3339))
}

// AuctionNotActiveError is returned when operation requires active status.
type AuctionNotActiveError struct {
	AuctionID uuid.UUID
	Status    Status
}

func (e *AuctionNotActiveError) Error() string {
	return fmt.Sprintf("auction not active: id=%s, status=%s", e.AuctionID, e.Status)
}

// ErrAlreadySettled is returned when attempting to create an order for an auction
// that already has an order_id set (prevents double settlement).
var ErrAlreadySettled = fmt.Errorf("auction already settled")

// ErrNotClaimable is returned when the auction status is not waiting_settlement.
var ErrNotClaimable = fmt.Errorf("auction not claimable")

// ErrSettlementDeadlinePassed is returned when the settlement deadline has expired.
var ErrSettlementDeadlinePassed = fmt.Errorf("auction settlement deadline has passed")

// ErrNoWinner is returned when the auction has no winner set.
var ErrNoWinner = fmt.Errorf("auction has no winner")

// ErrNotWinner is returned when the caller is not the auction winner.
var ErrNotWinner = fmt.Errorf("caller is not the auction winner")

// BNRAuctionRestrictedError is returned when a buyer is restricted from
// bidding due to BNR (Bid No Response) strikes.
type BNRAuctionRestrictedError struct {
	ActiveStrikes    int
	PermanentBan     bool
	RestrictionUntil *time.Time // nil for permanent bans
}

func (e *BNRAuctionRestrictedError) Error() string {
	if e.PermanentBan {
		return "buyer permanently banned from auctions due to repeated BNR violations"
	}
	if e.RestrictionUntil != nil {
		return fmt.Sprintf("buyer restricted from auctions until %s (%d BNR strikes)",
			e.RestrictionUntil.Format(time.RFC3339), e.ActiveStrikes)
	}
	return fmt.Sprintf("buyer restricted from auctions (%d BNR strikes)", e.ActiveStrikes)
}

// IsBNRAuctionRestricted returns true if err is a *BNRAuctionRestrictedError.
func IsBNRAuctionRestricted(err error) bool {
	var target *BNRAuctionRestrictedError
	return errors.As(err, &target)
}

// Auction represents an auction for a single product.
// This is a Commerce Entry Layer - it creates orders but doesn't touch the ledger.
//
// STATE MACHINE:
// - Draft: Fully editable, can schedule or cancel
// - Scheduled: Limited editing, can activate, revert to draft, or cancel
// - Active: Immutable except bid updates, can end or cancel (if no bids)
// - Ended: Terminal, order created if winner exists
// - Cancelled: Terminal, no order created
//
// SETTLEMENT SAFETY:
// - OrderID is set atomically when order is created
// - Once OrderID is set, no further order creation is possible
// - This prevents double settlement (multiple orders for same auction)
type Auction struct {
	// Identity
	ID uuid.UUID

	// Relations
	SellerID  uuid.UUID
	ProductID uuid.UUID

	// Settlement
	OrderID            *uuid.UUID // Set atomically when order is created, prevents double settlement
	SettlementDeadline *time.Time // Set when entering WAITING_SETTLEMENT; deadline for winner to claim (24h)

	// Pricing (in minor unit, e.g., cents for IDR)
	StartPrice   int64
	BidIncrement int64
	BuyNowPrice  *int64 // NULL means no buy now option

	// Timing
	StartAt time.Time
	EndAt   time.Time

	// AntiSnipeExtensionTotal is the cumulative soft-close extension already
	// applied to EndAt (PASS_18C). Capped at MaxAntiSnipingTotalExtension.
	AntiSnipeExtensionTotal time.Duration

	// Current State
	CurrentBid      *int64 // NULL if no bids yet
	CurrentWinnerID *uuid.UUID

	// Status
	Status Status

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time

	// Product is the canonical joined Product entity. Product is the sole
	// authority for title, description, media, koi attributes, preparation
	// and farm address. This is a read-through reference — never written
	// through the Auction surface.
	Product *productEntity.Product
}

// NewDraft creates a new draft auction.
// Product identity (title, description, media, koi attributes, preparation)
// is owned by the Product entity — Auction only carries surface-specific
// configuration (pricing, timing, bid state).
func NewDraft(
	sellerID, productID uuid.UUID,
	startPrice, bidIncrement int64,
	buyNowPrice *int64,
	startAt, endAt time.Time,
) *Auction {
	now := time.Now()

	return &Auction{
		ID:              uuid.New(),
		SellerID:        sellerID,
		ProductID:       productID,
		OrderID:         nil, // Initially not settled
		StartPrice:      startPrice,
		BidIncrement:    bidIncrement,
		BuyNowPrice:     buyNowPrice,
		StartAt:         startAt,
		EndAt:           endAt,
		CurrentBid:      nil,
		CurrentWinnerID: nil,
		Status:          StatusDraft,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Schedule transitions the auction from draft to scheduled.
func (a *Auction) Schedule() error {
	if !canTransition(a.Status, StatusScheduled) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusScheduled}
	}
	a.Status = StatusScheduled
	a.UpdatedAt = time.Now()
	return nil
}

// Activate transitions the auction from scheduled to active.
func (a *Auction) Activate() error {
	if !canTransition(a.Status, StatusActive) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusActive}
	}
	a.Status = StatusActive
	a.UpdatedAt = time.Now()
	return nil
}

// End transitions the auction from active to ended.
// Used for buy-now flow or auctions without winners.
func (a *Auction) End() error {
	if !canTransition(a.Status, StatusEnded) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusEnded}
	}
	a.Status = StatusEnded
	a.UpdatedAt = time.Now()
	return nil
}

// TransitionToWaitingSettlement transitions the auction from active to waiting_settlement.
// Used when auction ends with a winner but order not yet created.
// Sets the settlement deadline to 24 hours from now.
func (a *Auction) TransitionToWaitingSettlement() error {
	if !canTransition(a.Status, StatusWaitingSettlement) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusWaitingSettlement}
	}
	a.Status = StatusWaitingSettlement
	// Set settlement deadline to 24 hours from now
	deadline := time.Now().Add(24 * time.Hour)
	a.SettlementDeadline = &deadline
	a.UpdatedAt = time.Now()
	return nil
}

// TransitionToExpiredBNR transitions the auction from waiting_settlement to expired_bnr.
// Used when the winner fails to claim the auction within the settlement deadline.
// The auction remains without an order (order_id remains NULL).
func (a *Auction) TransitionToExpiredBNR() error {
	if !canTransition(a.Status, StatusExpiredBNR) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusExpiredBNR}
	}
	a.Status = StatusExpiredBNR
	a.UpdatedAt = time.Now()
	return nil
}

// Settle transitions the auction from waiting_settlement to ended.
// Used after order is created via claim flow.
func (a *Auction) Settle() error {
	if !canTransition(a.Status, StatusEnded) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusEnded}
	}
	a.Status = StatusEnded
	a.UpdatedAt = time.Now()
	return nil
}

// ErrOrderBindingMismatch is returned by ReleaseUnpaidOrder when the auction
// is currently bound to a DIFFERENT order than the one being released.
// Callers must never blindly clear another order's binding.
var ErrOrderBindingMismatch = errors.New("auction: order binding mismatch")

// ReleaseUnpaidOrder clears the auction's OrderID binding after its bound
// order was cancelled or expired before payment succeeded (PASS_20B).
//
// Both settlement paths (buy-now via End(), bid-win via Settle()) transition
// the auction to StatusEnded and set OrderID immediately at order-creation
// time — before payment succeeds — mirroring how ForSale reserves
// stock at order-creation via ReduceQuantity, not at payment success. If
// that order is later cancelled/expired unpaid, this releases the binding
// so the auction's own bookkeeping stays honest (no order is actually live
// against it anymore).
//
// SCOPE: this does NOT reopen the auction for further bids/buy-now. Ended is
// a deliberate terminal state with no valid outgoing transition (see
// transitionAllowed) — by design, once an auction has been through
// settlement, its lifecycle is over. A seller who wants to resell the same
// physical item after an unpaid settlement must create a new listing or
// auction. Making an Ended auction literally reopen for bidding would
// require extending the state machine, which is a distinct product/design
// decision left to a future pass if the business actually requires it —
// releasing the binding (this method) is the safe, minimal, non-inventive
// fix for "don't leave the order/auction pair silently stuck."
//
// Idempotent: a no-op if OrderID is already nil (already released, e.g. a
// retried worker call after a prior partial failure). Returns
// ErrOrderBindingMismatch if bound to a different order.
func (a *Auction) ReleaseUnpaidOrder(orderID uuid.UUID) error {
	if a.OrderID == nil {
		return nil
	}
	if *a.OrderID != orderID {
		return ErrOrderBindingMismatch
	}
	a.OrderID = nil
	a.UpdatedAt = time.Now()
	return nil
}

// Cancel transitions the auction to cancelled state.
// Can only be cancelled from draft, scheduled, or active (with no bids).
func (a *Auction) Cancel() error {
	if !canTransition(a.Status, StatusCancelled) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusCancelled}
	}
	a.Status = StatusCancelled
	a.UpdatedAt = time.Now()
	return nil
}

// UpdateDraft updates draft auction fields.
// Product content (title, description, koi attributes) is updated via
// Product entity — this method only updates surface-specific fields.
func (a *Auction) UpdateDraft(
	startPrice, bidIncrement int64,
	buyNowPrice *int64,
	startAt, endAt time.Time,
) error {
	if a.Status != StatusDraft {
		return &InvalidOperationError{
			Status: a.Status,
			Reason: "can only update draft auctions",
		}
	}

	if err := ValidateAuctionTiming(startAt, endAt); err != nil {
		return err
	}

	a.StartPrice = startPrice
	a.BidIncrement = bidIncrement
	a.BuyNowPrice = buyNowPrice
	a.StartAt = startAt
	a.EndAt = endAt
	a.UpdatedAt = time.Now()
	return nil
}

// UpdateScheduled updates scheduled auction timing.
// Product content (title, description) is updated via Product entity.
func (a *Auction) UpdateScheduled(
	startAt, endAt time.Time,
) error {
	if a.Status != StatusScheduled {
		return &InvalidOperationError{
			Status: a.Status,
			Reason: "can only update scheduled auctions",
		}
	}

	if err := ValidateAuctionTiming(startAt, endAt); err != nil {
		return err
	}
	if err := RequireFutureScheduledStart(startAt, time.Now()); err != nil {
		return err
	}

	a.StartAt = startAt
	a.EndAt = endAt
	a.UpdatedAt = time.Now()
	return nil
}

// MinimumBid returns the minimum acceptable bid amount.
// If no current bid, minimum is start_price.
// If there's a current bid, minimum is current_bid + bid_increment.
func (a *Auction) MinimumBid() int64 {
	if a.CurrentBid == nil {
		return a.StartPrice
	}
	return *a.CurrentBid + a.BidIncrement
}

// PlaceBid validates and updates the auction with a new bid.
// Does NOT persist - must be called within transaction with repository.
//
// Validation rules:
// - Auction must be active
// - Current time must be before end_at
// - Bidder must not be the seller
// - Bid amount must be >= minimum bid
func (a *Auction) PlaceBid(bidderID uuid.UUID, amount int64, now time.Time) error {
	// Must be active
	if a.Status != StatusActive {
		return &AuctionNotActiveError{AuctionID: a.ID, Status: a.Status}
	}

	// Must not have ended
	if !now.Before(a.EndAt) {
		return &AuctionEndedError{AuctionID: a.ID, EndAt: a.EndAt}
	}

	// Cannot bid on own auction
	if bidderID == a.SellerID {
		return &SelfBidError{BidderID: bidderID, SellerID: a.SellerID}
	}

	// Bid must be at least minimum
	minimum := a.MinimumBid()
	if amount < minimum {
		return &BidTooLowError{MinimumBid: minimum, OfferedBid: amount}
	}

	// Update current bid and winner
	a.CurrentBid = &amount
	winnerID := bidderID
	a.CurrentWinnerID = &winnerID

	// Soft-close: a valid bid landing in the closing window extends EndAt,
	// atomically with bid acceptance, up to the cumulative cap.
	a.applyAntiSnipingExtension(now)

	a.UpdatedAt = time.Now()

	return nil
}

// applyAntiSnipingExtension extends EndAt by AntiSnipingExtension when a bid
// lands within AntiSnipingWindow of the current end, subject to the
// cumulative MaxAntiSnipingTotalExtension cap. Returns true if EndAt was
// extended. No-op once the cap is reached — the auction still ends normally.
func (a *Auction) applyAntiSnipingExtension(now time.Time) bool {
	if a.EndAt.Sub(now) > AntiSnipingWindow {
		return false
	}
	remaining := MaxAntiSnipingTotalExtension - a.AntiSnipeExtensionTotal
	if remaining <= 0 {
		return false
	}
	extension := AntiSnipingExtension
	if extension > remaining {
		extension = remaining
	}
	a.EndAt = a.EndAt.Add(extension)
	a.AntiSnipeExtensionTotal += extension
	return true
}

// CanCancel returns true if the auction can be cancelled.
// Active auctions can only be cancelled if there are no bids.
func (a *Auction) CanCancel() bool {
	switch a.Status {
	case StatusDraft, StatusScheduled:
		return true
	case StatusActive:
		return a.CurrentBid == nil // Can cancel active auction only if no bids
	default:
		return false // Ended and Cancelled are terminal
	}
}

// HasWinner returns true if the auction has a winner.
func (a *Auction) HasWinner() bool {
	return a.CurrentWinnerID != nil
}

// WinnerID returns the winner's ID. Returns nil if no winner.
func (a *Auction) WinnerID() *uuid.UUID {
	return a.CurrentWinnerID
}

// WinningBid returns the winning bid amount. Returns nil if no bids.
func (a *Auction) WinningBid() *int64 {
	return a.CurrentBid
}



