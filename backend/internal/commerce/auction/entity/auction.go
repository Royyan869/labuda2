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
// Auction lifecycle follows explicit states with clear boundaries.
// THERE IS NO DRAFT STATE: create = publish (owner decision, Oct 2026) — a
// created auction is validated against the market-entry gate and lands
// directly in scheduled (or active for an immediate start).
//
// PRE-MARKET COMMITMENT (scheduled):
//   - Seller has committed auction for future market run
//   - Limited editing allowed (title, description, timing)
//   - Market authority checked at create/schedule/relist time
//   - Can transition to: active, cancelled, lapsed
//   - Market authority RE-CHECKED at activation time: if it expired the
//     auction lapses (scheduled -> lapsed), it does NOT cancel
//
// LIVE MARKET (active):
//   - Auction is currently accepting bids
//   - Immutable except for bid updates
//   - Can be cancelled only if no bids
//   - Can transition to: ended, cancelled
//
// HISTORICAL (ended, cancelled):
//   - ended: Normal completion (time expired or buy now, or settlement success)
//   - cancelled: seller/admin/moderation cancellation — subscription expiry
//     never cancels: a scheduled auction whose authority lapsed LAPSES
//   - cancelled is terminal with no further transitions
//   - ended allows exactly one further transition, ended -> scheduled, used
//     ONLY by Relist() for an auction that ended with no bids
//     (owner business truth: relist is REPUBLISH — no draft detour)
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
	// StatusScheduled is the INITIAL state of a created auction (create =
	// publish; there is no draft state) and the state of an auction that is
	// scheduled but not yet started. Editable with restricted fields.
	StatusScheduled Status = "scheduled"

	// StatusActive is when auction is running.
	// Immutable except for bids and cancellation (if no bids).
	StatusActive Status = "active"

	// StatusWaitingSettlement is when auction has ended but order not yet created.
	// The winner completes the shared Checkout (POST /orders bid-win) to
	// create the order inside the settlement window.
	StatusWaitingSettlement Status = "waiting_settlement"

	// StatusEnded is when auction completes normally (time expires, buy now,
	// or payment succeeds after settlement).
	//
	// NOT terminal: an ended auction that carries no bid, winner, or order can
	// be republished through Relist() (ended -> scheduled) so the seller can
	// run a new lifecycle. An ended auction that carries an outcome never can.
	StatusEnded Status = "ended"

	// StatusCancelled is when auction is cancelled.
	// Terminal state.
	StatusCancelled Status = "cancelled"

	// StatusLapsed is a SCHEDULED auction whose seller's market authority
	// (subscription) expired before it could go live. Owner decision
	// (Oct 2026): this is NOT a cancellation — the seller never chose to
	// stop, the system held the auction back. A lapsed auction never went
	// live, is hidden from every viewer surface by the read-side
	// market-authority filters, and becomes relistable (lapsed -> scheduled)
	// after renewal. Reached only via Lapse().
	StatusLapsed Status = "lapsed"
)

// transitionAllowed defines valid state transitions.
// The state machine enforces business rules at the entity level.
//
// This map answers ONLY "which target states are structurally reachable".
// Business preconditions are enforced by the method that performs the
// transition — the same way CanCancel() gates active -> cancelled on
// CurrentBid == nil. In particular, ended -> scheduled is reachable only
// through Relist(), which rejects any auction carrying a bid, winner, or order.
var transitionAllowed = map[Status][]Status{
	// scheduled -> lapsed: seller market authority expired before
	// activation (Lapse()). Not a cancellation — relistable after renewal.
	StatusScheduled: {StatusActive, StatusCancelled, StatusLapsed},
	// lapsed -> scheduled: the OWNER-APPROVED RELIST path after the seller
	// renews (Relist with the create-like republish payload).
	StatusLapsed: {StatusScheduled},
	StatusActive: {StatusWaitingSettlement, StatusEnded, StatusCancelled},
	// settlement failure auto-reschedules (RescheduleAfterSettlementFailure):
	// the seller did nothing wrong, so the run re-enters the market at
	// start=now. Market authority is re-checked at activation, which lapses
	// the auction if the seller's subscription is still expired.
	StatusWaitingSettlement: {StatusEnded, StatusScheduled, StatusCancelled}, // After payment success OR settlement failure OR moderation enforcement
	// ended -> scheduled is the OWNER-APPROVED RELIST of an auction that ended
	// with no bids. Relist is republish: no draft detour. See Relist() for the
	// business gate.
	StatusEnded:     {StatusScheduled},
	StatusCancelled: {}, // Terminal state
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
// Closed states (ended, cancelled, waiting_settlement) and unknown
// statuses are not repostable.
//
// Note: "ended" is not terminal for the seller — Relist() republishes it to
// scheduled — but ended itself stays non-repostable: repostability follows the
// CURRENT status, and a republished run earns it back like any scheduled run.
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
//	unavailable  — not currently buyable (scheduled / terminal states)
//	removed      — reserved for moderation/hard-delete; Status does not model
//	               these today so this method never returns "removed".
//
// Internal enum values (waiting_settlement, scheduled, …) MUST NOT
// cross the public boundary. Public surfaces should call this method and emit
// the result instead of the raw enum text.
func (s Status) PublicLifecycle() string {
	switch s {
	case StatusActive, StatusWaitingSettlement:
		return "active"
	case StatusScheduled, StatusEnded, StatusCancelled, StatusLapsed:
		return "unavailable"
	default:
		return "unavailable"
	}
}

// PublicPhase returns the public PHASE vocabulary for this auction status.
//
// The vocabulary is closed: {scheduled, active, waiting_settlement, ended,
// cancelled}. Unlike PublicLifecycle (2-value coarse card vocabulary), the
// phase keeps the marketplace timing granularity the public UI needs
// (countdown on scheduled, live bidding on active, winner settlement
// window, terminal states).
//
// There is no draft in the lifecycle (create = publish). The defensive
// default mapping below is deliberately conservative — any unknown or
// non-public internal state (e.g. lapsed, which the read-side
// market-authority filter excludes from discovery) renders as cancelled
// (dead), never as bid-able or upcoming. Owner surfaces must read the
// separate owner-only `seller_status` wire field for the exact internal
// state.
func (s Status) PublicPhase() string {
	switch s {
	case StatusScheduled:
		return "scheduled"
	case StatusActive:
		return "active"
	case StatusWaitingSettlement:
		return "waiting_settlement"
	case StatusEnded:
		return "ended"
	case StatusCancelled:
		return "cancelled"
	case StatusLapsed:
		// Lapsed never reaches public discovery (the read-side
		// market-authority filter excludes it); if one ever leaked through a
		// bug it must render dead.
		return "cancelled"
	default: // unknown — conservative defensive mapping
		return "cancelled"
	}
}

// String returns the string representation of the auction status.
func (s Status) String() string {
	return string(s)
}

// IsPublicDiscoverable returns true when this auction status is eligible to
// appear in anonymous public discovery (browse/search). Only pre-sale and
// live-sale surfaces qualify: cancelled, waiting_settlement, lapsed and
// ended (settled/no-winner) are non-public/historical states.
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

// ErrAlreadySettled is returned when attempting to create an order for an
// auction that already has an order_id set (prevents double settlement).
var ErrAlreadySettled = fmt.Errorf("auction already settled")

// ErrSettlementDeadlinePassed is returned when the settlement window
// (auction.end_at + 24h) has expired before the winner completed checkout.
var ErrSettlementDeadlinePassed = fmt.Errorf("auction settlement deadline has passed")

// ErrNotWinner is returned when the caller is not the auction winner.
var ErrNotWinner = fmt.Errorf("caller is not the auction winner")

// ErrShippingAlreadyResolved is returned when a shipping resolution is
// attempted after shipping has already been resolved for this settlement.
// First-resolution-wins: a settled auction's shipping facts are immutable.
var ErrShippingAlreadyResolved = fmt.Errorf("auction shipping already resolved")

// Auction represents an auction for a single product.
// This is a Commerce Entry Layer — it owns auction lifecycle and settlement
// eligibility; auction-sourced ORDERS are created only by POST /orders.
//
// STATE MACHINE:
//   - Scheduled: initial state at create (create = publish); limited
//     editing, can activate, relist-window edit, or cancel
//   - Active: Immutable except bid updates, can end or cancel (if no bids)
//   - WaitingSettlement: winner determined; order created and bound but payment
//     not yet settled; settles to ended on payment success, or auto-
//     reschedules to scheduled on settlement failure
//   - Ended: order created + paid, or a run that produced no winner;
//     a no-winner ended auction relists to scheduled (republish)
//   - Cancelled: Terminal, no order created
//
// SETTLEMENT SAFETY:
//   - OrderID is set atomically when order is created
//   - Once OrderID is set, no further order creation is possible
//   - This prevents double settlement (multiple orders for same auction)
//   - Settlement failure AUTO-RESCHEDULES the auction (scheduled, start=now)
//     with all settlement context (OrderID, ShippingResolvedAt, CurrentBid,
//     CurrentWinnerID, seller flags) cleared; bid history in auction_bids
//     remains intact.
type Auction struct {
	ID uuid.UUID

	// Relations
	SellerID  uuid.UUID
	ProductID uuid.UUID

	// Settlement
	OrderID *uuid.UUID // Set atomically when order is created; prevents double settlement

	// ShippingResolvedAt marks the moment shipping was resolved for the current
	// settlement (canonical payment-deadline anchor: shipping_resolved_at + 24h).
	// Cleared when the auction auto-reschedules after a settlement failure.
	ShippingResolvedAt *time.Time

	// SellerActionRequired is set at auction end when the seller must provide a
	// private shipping quote before the winner can resolve shipping (winner
	// destination outside all selected shipping setups' coverage).
	SellerActionRequired bool

	// SellerQuoteProvided is set once the seller has supplied a valid private
	// quote for the current settlement. Cleared on settlement-failure
	// reschedule so an old quote never becomes the authority for a relist.
	SellerQuoteProvided bool

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

// NewScheduled creates an auction directly in its INITIAL market state
// (scheduled) — CREATE = PUBLISH. There is no draft stage: the service layer
// runs the market-entry gate (ownership, restriction, market authority,
// shipping coverage) before persisting, and an immediate start progresses
// scheduled -> active in the same transaction.
// Product identity (title, description, media, koi attributes, preparation)
// is owned by the Product entity — Auction only carries surface-specific
// configuration (pricing, timing, bid state).
func NewScheduled(
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
		Status:          StatusScheduled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// Lapse moves a scheduled auction to lapsed because the seller's market
// authority (subscription) expired before the auction could go live.
//
// Owner decision (Oct 2026): this is deliberately NOT Cancel() — the seller
// took no action, so the cancellation vocabulary (seller/moderation/admin)
// must not absorb it. Only scheduled auctions lapse: a running auction
// continues to completion (buyer fairness), and its bidless outcome ends as
// 'ended' like any other no-bid run.
func (a *Auction) Lapse() error {
	if !canTransition(a.Status, StatusLapsed) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusLapsed}
	}
	a.Status = StatusLapsed
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
// Used when auction ends with a winner but settlement has not completed.
//
// Deadline authority is DERIVED (auction.end_at + 24h) — no deadline is stored
// on the entity. The caller is responsible for setting SellerActionRequired
// (from the winner-destination coverage check) before persisting.
func (a *Auction) TransitionToWaitingSettlement() error {
	if !canTransition(a.Status, StatusWaitingSettlement) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusWaitingSettlement}
	}
	a.Status = StatusWaitingSettlement
	a.UpdatedAt = time.Now()
	return nil
}

// SettlementDeadline returns the canonical settlement shipping deadline:
// auction.end_at + 24h. There is NO extension and NO second deadline authority.
func (a *Auction) SettlementDeadline() time.Time {
	return a.EndAt.Add(24 * time.Hour)
}

// SettlementDeadlinePassed reports whether the canonical settlement window
// (end_at + 24h) has passed as of now. This is the SINGLE deadline predicate
// for every enforcement point — the advisory pricing-preview check and the
// authoritative POST /orders bid-win re-check under the row lock.
func (a *Auction) SettlementDeadlinePassed(now time.Time) bool {
	return now.After(a.SettlementDeadline())
}

// RescheduleAfterSettlementFailure returns the auction from
// waiting_settlement to SCHEDULED after a settlement failure (buyer shipping
// timeout, seller quote default, or payment expiry).
//
// OWNER DECISION (Oct 2026): settlement failure auto-reschedules — the run is
// over and the seller did nothing wrong, so there is no draft detour: the
// auction immediately re-enters the market at start=now, end=now+previous
// duration (clamped to the canonical 1-7 day bounds). Market authority is NOT
// re-checked here — ActivateScheduledAuction is the single re-check and
// lapses the auction if the seller's subscription is still expired.
//
// Relist model: the same auction record is reused. All current settlement
// context is cleared so no stale settlement state carries into the new run:
//   - OrderID            = nil (old order stays historical/terminal; the next
//     settlement must bind a NEW order — no order reuse)
//   - ShippingResolvedAt = nil
//   - SellerActionRequired = false
//   - SellerQuoteProvided  = false (an old quote is historical only and must
//     never become the current settlement authority for a relist)
//   - CurrentWinnerID    = nil
//   - CurrentBid         = nil (MinimumBid() returns StartPrice again)
//   - AntiSnipeExtensionTotal = 0 (anti-sniping budget is per-lifecycle;
//     a new run must start with a fresh extension budget — otherwise
//     accumulated extension would reduce or exhaust the new run's soft-close cap)
//
// Historical auction_bids rows are intentionally preserved (never deleted).
func (a *Auction) RescheduleAfterSettlementFailure() error {
	// PRECONDITION: settlement-failure path only. Status is checked explicitly
	// (not via the transition map alone) so a mistaken caller cannot route an
	// ended or relisted auction through the settlement-failure path.
	if a.Status != StatusWaitingSettlement {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusScheduled}
	}
	// New run keeps the previous run's duration, clamped to the canonical
	// bounds (anti-snipe extensions may have pushed end_at past MaxAuctionDuration).
	duration := a.EndAt.Sub(a.StartAt)
	if duration < MinAuctionDuration {
		duration = MinAuctionDuration
	}
	if duration > MaxAuctionDuration {
		duration = MaxAuctionDuration
	}
	now := time.Now()
	startAt, endAt, err := ResolveAuctionTiming(StartModeNow, nil, duration, now)
	if err != nil {
		return err
	}
	a.resetForRelist()
	a.StartAt = startAt
	a.EndAt = endAt
	a.Status = StatusScheduled
	a.UpdatedAt = now
	return nil
}

// resetForRelist clears every field that belongs to the finished lifecycle.
// It is the SINGLE clearing authority for both ways of restarting a run:
//   - RescheduleAfterSettlementFailure — waiting_settlement -> scheduled
//   - Relist — ended/lapsed -> scheduled (republish)
//
// Status is deliberately NOT touched here — each transition sets its own
// target state after this reset.
//
// Reset semantics:
//   - OrderID            = nil (old order stays historical; the next
//     settlement must bind a NEW order — no order reuse)
//   - ShippingResolvedAt = nil
//   - SellerActionRequired = false
//   - SellerQuoteProvided  = false (an old quote is historical only and must
//     never become the current settlement authority for a relist)
//   - CurrentWinnerID    = nil
//   - CurrentBid         = nil (MinimumBid() returns StartPrice again)
//   - AntiSnipeExtensionTotal = 0 (anti-sniping budget is per-lifecycle;
//     otherwise accumulated extension from the previous lifecycle would
//     reduce or exhaust the new lifecycle's soft-close cap)
//
// Historical auction_bids rows are intentionally preserved (never deleted).
func (a *Auction) resetForRelist() {
	a.OrderID = nil
	a.ShippingResolvedAt = nil
	a.SellerActionRequired = false
	a.SellerQuoteProvided = false
	a.CurrentWinnerID = nil
	a.CurrentBid = nil
	a.AntiSnipeExtensionTotal = 0
	a.UpdatedAt = time.Now()
}

// ErrAuctionNotRelistable is returned when a seller tries to relist an
// auction that cannot start a new lifecycle: it carries the outcome of the
// finished run (a bid, a winner, or a bound order).
var ErrAuctionNotRelistable = errors.New("auction is not relistable: only an ended auction with no bids, or a lapsed auction, can be republished")

// Relist REPUBLISHES a finished auction for a new lifecycle: ended (with no
// bid/winner/order) or lapsed (never went live) -> scheduled, with the caller-
// supplied run timing and pricing committed in the same step.
//
// OWNER BUSINESS TRUTH: relist IS republish. There is no draft detour — the
// seller comes from the create form (autofilled, duration re-chosen), so the
// same auction record is reused, resetForRelist() clears the previous
// lifecycle, and the new run is market-visible immediately after the service
// applies the schedule gates (market authority, restriction, shipping
// coverage). Historical auction_bids rows are kept.
//
// timing must already come from ResolveAuctionTiming (create authority);
// RequireFutureAuctionEnd is re-checked here as belt-and-suspenders so a stale
// end_at can never commit a run that would end the instant it starts.
func (a *Auction) Relist(startAt, endAt time.Time, startPrice, bidIncrement int64, buyNowPrice *int64) error {
	switch a.Status {
	case StatusEnded:
		if a.CurrentBid != nil || a.CurrentWinnerID != nil || a.OrderID != nil {
			return ErrAuctionNotRelistable
		}
	case StatusLapsed:
		// A lapsed auction never went live, so it carries no bid, winner, or
		// order by construction — checked anyway so the gate is one rule.
		if a.CurrentBid != nil || a.CurrentWinnerID != nil || a.OrderID != nil {
			return ErrAuctionNotRelistable
		}
	default:
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusScheduled}
	}
	if err := RequireFutureAuctionEnd(endAt, time.Now()); err != nil {
		return err
	}
	a.resetForRelist()
	a.StartPrice = startPrice
	a.BidIncrement = bidIncrement
	a.BuyNowPrice = buyNowPrice
	a.StartAt = startAt
	a.EndAt = endAt
	a.Status = StatusScheduled
	a.UpdatedAt = time.Now()
	return nil
}

// Settle transitions the auction from waiting_settlement to ended.
// Used when settlement completes successfully — the order is bound and
// payment has succeeded (payment success settles the auction to ENDED).
func (a *Auction) Settle() error {
	if !canTransition(a.Status, StatusEnded) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusEnded}
	}
	a.Status = StatusEnded
	a.UpdatedAt = time.Now()
	return nil
}

// ResolveShipping marks shipping as resolved for the current settlement.
// First-resolution-wins: once set, shipping_resolved_at is never overwritten
// (ErrShippingAlreadyResolved on subsequent attempts).
func (a *Auction) ResolveShipping(now time.Time) error {
	if a.ShippingResolvedAt != nil {
		return ErrShippingAlreadyResolved
	}
	a.ShippingResolvedAt = &now
	a.UpdatedAt = now
	return nil
}

// ErrOrderBindingMismatch is returned by ReleaseUnpaidOrder when the auction
// is currently bound to a DIFFERENT order than the one being released.
// Callers must never blindly clear another order's binding.
var ErrOrderBindingMismatch = errors.New("auction: order binding mismatch")

// ReleaseUnpaidOrder clears the auction's OrderID binding after its bound
// order was cancelled or expired before payment succeeded.
//
// Settlement path: a bid-win auction stays in waiting_settlement with OrderID
// bound until payment succeeds (Settle → ended). A buy-now auction ends
// immediately at order creation (End → ended). If the bound order is later
// cancelled/expired unpaid, this releases the binding so the auction's own
// bookkeeping stays honest (no order is actually live against it anymore).
//
// For bid-win auctions still in waiting_settlement, the caller (order expiry/
// cancel rollback) is responsible for the full settlement-failure path:
// release the binding, record the buyer violation, apply the restriction, and
// call RescheduleAfterSettlementFailure() so the auction re-enters the market.
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
// Can only be cancelled from scheduled or active (with no bids).
func (a *Auction) Cancel() error {
	if !canTransition(a.Status, StatusCancelled) {
		return &InvalidTransitionError{CurrentStatus: a.Status, TargetStatus: StatusCancelled}
	}
	a.Status = StatusCancelled
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
	now := time.Now()
	if err := RequireFutureScheduledStart(startAt, now); err != nil {
		return err
	}
	if err := RequireScheduledStartWithinHorizon(startAt, now); err != nil {
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
	case StatusScheduled:
		return true
	case StatusActive:
		return a.CurrentBid == nil // Can cancel active auction only if no bids
	default:
		return false // Ended/Lapsed cannot be cancelled (they only allow relist); Cancelled is terminal
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
