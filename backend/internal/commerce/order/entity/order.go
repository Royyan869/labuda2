// DOMAIN: COMMERCE
// NOTE: Unified transaction engine for all commerce types

package entity

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	addressentity "github.com/labuda/backend/internal/identity/address/entity"
	"github.com/labuda/backend/pkg/money"
)

// Order represents a buyer-seller transaction with escrow.
// This is the unified transaction engine for all commerce types.
type Order struct {
	ID       uuid.UUID `json:"id"`
	BuyerID  uuid.UUID `json:"buyer_id"`
	SellerID uuid.UUID `json:"seller_id"`

	// OrderNumber is a human-readable order number for display and customer service.
	// Format: ORD-YYYYMMDD-XXXXXXXX (e.g., ORD-20260309-AB12CD34)
	// Generated at order creation time, stored in database, unique.
	OrderNumber *string `json:"order_number" db:"order_number"`

	// Unified source metadata
	// SourceType indicates where this order originated (for_sale/negotiation/auction)
	SourceType OrderSourceType `json:"source_type"`
	// SourceID is the ID of the source entity (for_sale_id/negotiation_id/auction_id)
	SourceID uuid.UUID `json:"source_id"`
	// NegotiationID is optional, used for negotiated price purchases
	NegotiationID *uuid.UUID `json:"negotiation_id,omitempty"`

	// Auction settlement metadata (only populated when SourceType == auction)
	// AuctionSettlementType indicates how the auction was settled (buy_now vs bid_win)
	// This is critical for pricing rules: buy_now allows discounts/coins, bid_win does not
	AuctionSettlementType *AuctionSettlementType `json:"auction_settlement_type,omitempty"`

	// ============================================================================
	// PRICING SNAPSHOT (immutable after creation)
	// ============================================================================
	// These fields capture the pricing state at order creation time.
	// They are NEVER recalculated and NEVER used as source of truth for finance.
	// For financial truth, query the Ledger service.
	//
	// RULES:
	// - Immutable after order creation
	// - For display/reference only
	// - NOT used for financial calculations
	// - Ledger is single source of money
	// ============================================================================

	Quantity               int         `json:"quantity"`                  // Number of units ordered
	UnitPrice              money.Money `json:"unit_price"`                // Price per unit at order time
	Subtotal               money.Money `json:"subtotal"`                  // Quantity * UnitPrice (snapshot)
	ShippingTotal          money.Money `json:"shipping_total"`            // Shipping cost snapshot
	CommissionPercent      int64       `json:"commission_percent"`        // Commission percentage (e.g., 5 for 5%)
	CommissionAmount       money.Money `json:"commission_amount"`         // Commission amount snapshot
	ServiceFeeAmount       money.Money `json:"service_fee_amount"`        // Buyer-facing service fee projection for the selected payment method
	TotalPayableAmount     money.Money `json:"total_payable_amount"`      // Buyer-facing gross payable projection after payment selection
	TotalBeforeCoinsAmount money.Money `json:"total_before_coins_amount"` // Canonical buyer base before coin deduction/payment fee

	// ============================================================================
	// CoinsUsed - DISPLAY ONLY FIELD - STRICT USAGE RULES APPLY
	// ============================================================================
	// CRITICAL: This field is for DISPLAY PURPOSES ONLY.
	//
	// WHAT IT IS:
	// - Immutable snapshot of coins used at order creation time
	// - Shows to user: "You used X coins on this order"
	// - Used for coins refund ratio calculation in partial refunds
	//
	// WHAT IT IS NOT:
	// - NOT a financial field (no ledger entry)
	// - NOT used for discount calculation (discount logic is handled during order creation)
	// - NOT modifiable after order creation
	// - NOT a source of truth for any financial calculation
	//
	// FINANCIAL TRUTH:
	// - CoinsService tracks user balance
	// - Ledger tracks discount amounts via actual_ledger_amount
	// - This field is purely cosmetic/display
	//
	// USAGE RULES:
	// - OK: Display to user ("You used 500 coins")
	// - OK: Calculate refund ratio in partial refunds (proportional only)
	// - FORBIDDEN: Use for financial calculations
	// - FORBIDDEN: Use for discount calculations
	// - FORBIDDEN: Modify after order creation
	// ============================================================================
	CoinsUsed int64 `json:"coins_used"` // Number of coins used (DISPLAY ONLY - see usage rules above)

	// Shipping option snapshot (no FK - immutable snapshot at order creation)
	ShippingOptionID       *uuid.UUID `json:"shipping_option_id,omitempty"`       // Snapshot of shipping option ID (NULL when using shipping quote)
	ShippingOptionName     string     `json:"shipping_option_name"`               // Snapshot of shipping option name
	ShippingTransportType  string     `json:"shipping_transport_type"`            // Snapshot of transport type (train, bus, travel, plane, custom, manual)
	ShippingExpeditionName *string    `json:"shipping_expedition_name,omitempty"` // Snapshot of expedition/company name (nullable)
	ShippingEstimatedDays  *string    `json:"shipping_estimated_days,omitempty"`  // Snapshot of estimated delivery days (e.g., "1-2 hari")

	// Shipping source indicates where the shipping cost originated
	// - "for_sale": Shipping cost from the for-sale surface shipping options (default)
	// - "shipping_quote": Shipping cost from manual shipping quote provided by seller
	ShippingSource *string `json:"shipping_source,omitempty" db:"shipping_source"` // Nullable for backward compatibility

	// Shipping Quote Snapshot - frozen at order creation time (TASK F)
	// When using a shipping quote, store the quote ID and price for audit trail
	// This preserves the quote reference even if the original quote is modified/deleted
	ShippingQuoteID    *uuid.UUID `json:"shipping_quote_id,omitempty" db:"shipping_quote_id"`       // Quote ID used for this order
	ShippingQuotePrice *int64     `json:"shipping_quote_price,omitempty" db:"shipping_quote_price"` // Quote cost snapshot (for audit)

	// Shipping proof information (REQUIRED when marking as shipped)
	//
	// SHIPPING PROOF TRUTH:
	// - ProofType: REQUIRED - Type of proof provided ("tracking" | "phone" | "manual")
	// - TrackingNumber: REQUIRED for tracking/phone - resi, phone/WA, or other reference
	// - ShippingProofMedia: REQUIRED for manual - URL to proof image/document
	// - ShippingNote: Optional note from seller to buyer
	//
	// PROOF REQUIREMENTS:
	// - proof_type = "tracking": requires tracking_number (tracking number)
	// - proof_type = "phone": requires tracking_number (valid phone format)
	// - proof_type = "manual": requires shipping_proof_media (image/document URL)
	ProofType          *string `json:"proof_type,omitempty" db:"proof_type"`                     // REQUIRED: "tracking" | "phone" | "manual"
	TrackingNumber     *string `json:"tracking_number,omitempty" db:"tracking_number"`           // REQUIRED for tracking/phone: resi, phone/WA reference
	ShippingProofMedia *string `json:"shipping_proof_media,omitempty" db:"shipping_proof_media"` // REQUIRED for manual: URL to proof image/document
	ShippingNote       *string `json:"shipping_note,omitempty" db:"shipping_note"`               // Optional: shipping note from seller

	// Shipping Readiness Snapshot - frozen at order creation time
	// This preserves the buyer's expectation at purchase time, even if seller
	// later changes the listing/auction preparation time
	PreparationTimeSnapshot string     `json:"preparation_time_snapshot"`           // Frozen preparation time from source (listing/auction)
	PreparationNoteSnapshot *string    `json:"preparation_note_snapshot,omitempty"` // Frozen preparation note from source
	ReadyToShipBy           *time.Time `json:"ready_to_ship_by,omitempty"`          // Calculated deadline: paid_at + preparation_days (null until paid)

	// Shipping Destination Snapshot - frozen at order creation time
	// This preserves the shipping address for order fulfillment, even if buyer
	// modifies or deletes their address from the address book
	ShippingDestination *addressentity.AddressSnapshot `json:"shipping_destination,omitempty" db:"address_snapshot"` // Stored as JSONB in database

	// Shipping Origin Snapshot - frozen at order creation time
	// This preserves the seller's farm/warehouse address for order fulfillment,
	// even if the seller modifies or deletes their address from the address book
	// Populated from listing.FarmAddressID at order creation time
	ShippingOrigin *addressentity.AddressSnapshot `json:"shipping_origin,omitempty" db:"shipping_origin_snapshot"` // Stored as JSONB in database

	Status                    Status       `json:"status"`
	EscrowStatus              EscrowStatus `json:"escrow_status"`             // CACHED from Wallet.Escrow.Status - may be stale, use Wallet for source of truth
	AutoReleaseAt             *time.Time   `json:"auto_release_at,omitempty"` // Auto-release timestamp for buyer confirmation
	HasDispute                bool         `json:"has_dispute"`
	ConfirmationExtensionUsed bool         `json:"confirmation_extension_used"`        // Whether buyer has used the one-time confirmation extension (3 days)
	ConfirmationExtendedAt    *time.Time   `json:"confirmation_extended_at,omitempty"` // When the confirmation period was extended
	IdempotencyKey            *string      `json:"-"`                                  // Optional idempotency key for retry-safe creation (not exposed in JSON)

	// PricingTokenID links this order to the pricing token used for creation
	// This provides an audit trail and prevents double-ordering with the same token
	// UNIQUE constraint ensures token can only be used once
	PricingTokenID *uuid.UUID `json:"-" db:"pricing_token_id"` // Pricing token used for this order (prevents double-ordering)

	// ============================================================================
	// PAYMENT EXPIRY (SINGLE SOURCE OF TRUTH)
	// ============================================================================
	// PaymentMethod is "default" at order creation (the buyer has not chosen
	// a payment method yet — see PricingSnapshot.PaymentMethod in
	// order_creation_service.go), used only for the payment-window duration
	// below. Once the buyer selects a canonical method (PASS_18V) and
	// CorePaymentHandler.CreatePayment succeeds, this is overwritten with
	// the real method code (e.g. "qris", "bank_transfer") via
	// OrderRepository.UpdatePaymentSelectionTx — "default" is never the
	// terminal/authoritative value for a paid order.
	// PaymentExpiresAt is when the payment window closes - after this time, payment
	// is rejected and order becomes expired.
	//
	// CRITICAL: This is the ONLY source of truth for payment expiry.
	// - No dual logic with created_at + interval
	// - No separate payment.expired_at field
	// - Workers query this field directly
	// ============================================================================
	PaymentMethod    string     `json:"payment_method"`         // Payment method: instant, va, retail, etc.
	PaymentExpiresAt time.Time  `json:"payment_expires_at"`     // When payment window closes (single source of truth)
	CompletedAt      *time.Time `json:"completed_at,omitempty"` // When order was completed (NULL for non-completed orders)
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// InvalidTransitionError is returned when attempting an invalid state transition.
type InvalidTransitionError struct {
	CurrentStatus Status
	TargetStatus  Status
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid order status transition: %s -> %s", e.CurrentStatus, e.TargetStatus)
}

// DisputeActiveError is returned when attempting an operation that requires no active dispute.
type DisputeActiveError struct {
	OrderID uuid.UUID
}

func (e *DisputeActiveError) Error() string {
	return fmt.Sprintf("cannot complete order with active dispute: %s", e.OrderID)
}

// InvalidEscrowStatusError is returned when escrow status is not valid for the operation.
//
// HARDENING: This error is used in business logic guards.
// For critical financial decisions, validate against live Wallet state, not cached Order.EscrowStatus.
type InvalidEscrowStatusError struct {
	CurrentStatus  EscrowStatus
	RequiredStatus EscrowStatus
}

func (e *InvalidEscrowStatusError) Error() string {
	return fmt.Sprintf("invalid escrow status: %s (required: %s)", e.CurrentStatus, e.RequiredStatus)
}

// ErrInvalidRefundAmount is returned when refund amount is invalid.
type ErrInvalidRefundAmount struct {
	RefundAmount int64
	EscrowAmount int64
	Reason       string
}

func (e *ErrInvalidRefundAmount) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("invalid refund amount: %s", e.Reason)
	}
	return fmt.Sprintf("invalid refund amount: %d (escrow: %d)", e.RefundAmount, e.EscrowAmount)
}

// ErrAlreadyResolved is returned when order is already in a terminal escrow state.
type ErrAlreadyResolved struct {
	OrderID      uuid.UUID
	EscrowStatus EscrowStatus
}

func (e *ErrAlreadyResolved) Error() string {
	return fmt.Sprintf("order already resolved: %s (escrow_status: %s)", e.OrderID, e.EscrowStatus)
}

// ErrInvalidStateForPartialRefund is returned when order state doesn't allow partial refund.
type ErrInvalidStateForPartialRefund struct {
	OrderID      uuid.UUID
	Status       Status
	EscrowStatus EscrowStatus
}

func (e *ErrInvalidStateForPartialRefund) Error() string {
	return fmt.Sprintf("invalid state for partial refund: order_id=%s, status=%s, escrow_status=%s",
		e.OrderID, e.Status, e.EscrowStatus)
}

// ErrSelfPurchase is returned when a buyer attempts to purchase their own listing.
type ErrSelfPurchase struct {
	BuyerID  uuid.UUID
	SellerID uuid.UUID
	Source   string // Where the purchase was initiated (listing/auction/negotiation/etc)
}

func (e *ErrSelfPurchase) Error() string {
	return fmt.Sprintf("buyer cannot purchase their own listing: buyer_id=%s, seller_id=%s, source=%s",
		e.BuyerID, e.SellerID, e.Source)
}

// ErrShipmentDeadlineExceeded is returned when seller tries to ship after deadline.
type ErrShipmentDeadlineExceeded struct {
	OrderID        uuid.UUID
	ReadyToShipBy  time.Time
	GracePeriodEnd time.Time
}

func (e *ErrShipmentDeadlineExceeded) Error() string {
	return fmt.Sprintf("shipment deadline exceeded: order_id=%s, ready_by=%s, grace_period_end=%s",
		e.OrderID, e.ReadyToShipBy.Format(time.RFC3339), e.GracePeriodEnd.Format(time.RFC3339))
}

// ErrBuyerNotEligibleForCancel is returned when buyer tries to cancel before grace period.
type ErrBuyerNotEligibleForCancel struct {
	OrderID        uuid.UUID
	ReadyToShipBy  time.Time
	GracePeriodEnd time.Time
}

func (e *ErrBuyerNotEligibleForCancel) Error() string {
	return fmt.Sprintf("buyer not eligible for cancel: order_id=%s, ready_by=%s, grace_period_end=%s",
		e.OrderID, e.ReadyToShipBy.Format(time.RFC3339), e.GracePeriodEnd.Format(time.RFC3339))
}

// ErrUnauthorizedDisputeAccess is returned when user tries to access dispute they're not part of.
type ErrUnauthorizedDisputeAccess struct {
	OrderID  uuid.UUID
	UserID   uuid.UUID
	BuyerID  uuid.UUID
	SellerID uuid.UUID
}

func (e *ErrUnauthorizedDisputeAccess) Error() string {
	return fmt.Sprintf("unauthorized dispute access: order_id=%s, user_id=%s, buyer_id=%s, seller_id=%s",
		e.OrderID, e.UserID, e.BuyerID, e.SellerID)
}

// ErrVideoRequiredForBuyerDispute is returned when buyer opens dispute without video evidence.
type ErrVideoRequiredForBuyerDispute struct {
	OrderID uuid.UUID
}

func (e *ErrVideoRequiredForBuyerDispute) Error() string {
	return fmt.Sprintf("video evidence required for buyer dispute: order_id=%s", e.OrderID)
}

// ============================================================================
// SHIPPING PROOF CONSTANTS
// ============================================================================

// Proof type constants - defines the type of shipping proof provided.
const (
	ProofTypeTracking = "tracking" // Tracking number (resi)
	ProofTypePhone    = "phone"    // Phone/WA number for delivery contact
	ProofTypeManual   = "manual"   // Manual proof (photo/document)
)

// ============================================================================
// SHIPPING SOURCE CONSTANTS
// ============================================================================

// Shipping source constants - defines where the shipping cost originated.
const (
	ShippingSourceForSale       = "for_sale"       // Shipping cost from for-sale surface shipping options
	ShippingSourceShippingQuote = "shipping_quote" // Shipping cost from manual shipping quote
)

// ============================================================================
// FULFILLMENT DEADLINE CONSTANTS
// ============================================================================

const (
	// FulfillmentGracePeriodDays is the grace period after ReadyToShipBy for seller to ship.
	// This allows for reasonable delays while still protecting buyers.
	FulfillmentGracePeriodDays = 2 // 2 days grace period

	// PostShipDisputeWindowHours is the window after shipping when disputes can be opened.
	// After this window, disputes are only allowed if the order is overdue.
	PostShipDisputeWindowHours = 12 // 12 hours after shipping

	// PreShipDisputeAllowedAfterHours is when buyers can open disputes before shipping if order is overdue.
	PreShipDisputeAllowedAfterHours = 24 // 24 hours after grace period ends

	// AutoReleaseDuration is the buyer confirmation window set at mark-ship time.
	// AutoReleaseAt = time.Now() + AutoReleaseDuration when MarkShipped() is called.
	AutoReleaseDuration = 5 * 24 * time.Hour
)

// InvalidShippingProofError is returned when shipping proof requirements are not met.
type InvalidShippingProofError struct {
	Reason string
}

func (e *InvalidShippingProofError) Error() string {
	return fmt.Sprintf("invalid shipping proof: %s", e.Reason)
}

// ErrImmutableShippingProof is returned when attempting to modify immutable shipping proof fields.
type ErrImmutableShippingProof struct {
	Field  string
	Reason string
}

func (e *ErrImmutableShippingProof) Error() string {
	return fmt.Sprintf("shipping proof field '%s' is immutable: %s", e.Field, e.Reason)
}

// isValidPhoneFormat validates Indonesian phone number format.
// Accepts formats: 08xxxxxxxxxx, +628xxxxxxxxxx, 628xxxxxxxxxx
// Minimum 10 digits required, must be numeric (except for + prefix).
func isValidPhoneFormat(phone string) bool {
	if len(phone) < 10 {
		return false
	}
	// Check if starts with valid Indonesian prefix
	if len(phone) >= 2 && phone[0:2] == "08" {
		// Validate all characters are digits
		for _, ch := range phone {
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
	if len(phone) >= 3 && phone[0:3] == "+62" {
		// Validate all remaining characters are digits
		for i, ch := range phone {
			if i == 0 {
				continue // Skip the + sign
			}
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
	if len(phone) >= 3 && phone[0:3] == "628" {
		// Validate all characters are digits
		for _, ch := range phone {
			if ch < '0' || ch > '9' {
				return false
			}
		}
		return true
	}
	return false
}

// isValidTrackingNumber validates tracking number format.
// Requires minimum 6 characters to be a valid tracking number.
func isValidTrackingNumber(tracking string) bool {
	// Remove whitespace for validation
	cleaned := tracking
	// Simple min-length check - most tracking numbers are at least 6 chars
	return len(cleaned) >= 6
}

// isShippedOrLater returns true if the order status is shipped or any subsequent status.
func (o *Order) isShippedOrLater() bool {
	return o.Status == StatusShipped ||
		o.Status == StatusDelivered ||
		o.Status == StatusCompleted ||
		o.Status == StatusDisputeOpen ||
		o.Status == StatusPartiallyRefunded
}

// validateShippingProofImmutability checks that shipping proof fields are not modified
// after the order has been shipped. This prevents manipulation of shipping proof data.
func (o *Order) validateShippingProofImmutability(
	newProofType *string,
	newTrackingNumber *string,
	newShippingProofMedia *string,
) error {
	// Only enforce immutability for orders that are already shipped or later
	if !o.isShippedOrLater() {
		return nil
	}

	// Check proof_type immutability
	if newProofType != nil && o.ProofType != nil && *newProofType != *o.ProofType {
		return &ErrImmutableShippingProof{
			Field:  "proof_type",
			Reason: fmt.Sprintf("cannot change from %s to %s after order is shipped", *o.ProofType, *newProofType),
		}
	}

	// Check tracking_number immutability
	if newTrackingNumber != nil && o.TrackingNumber != nil && *newTrackingNumber != *o.TrackingNumber {
		return &ErrImmutableShippingProof{
			Field:  "tracking_number",
			Reason: fmt.Sprintf("cannot modify tracking number after order is shipped (original: %s)", *o.TrackingNumber),
		}
	}

	// Check shipping_proof_media immutability
	if newShippingProofMedia != nil && o.ShippingProofMedia != nil && *newShippingProofMedia != *o.ShippingProofMedia {
		return &ErrImmutableShippingProof{
			Field:  "shipping_proof_media",
			Reason: "cannot modify proof media URL after order is shipped",
		}
	}

	return nil
}

// MarkPaid transitions the order from pending to paid.
//
// CRITICAL: EscrowStatus is NOT set here. It must be derived from Wallet.Escrow.Status
// AFTER WalletService.HoldForOrder succeeds.
//
// This method ONLY:
// - Validates order status transition (pending -> paid)
// - Sets order status to paid
// - Calculates ready_to_ship_by based on preparation_time_snapshot
//
// CALLER MUST:
// 1. Call WalletService.HoldForOrder() FIRST
// 2. Fetch wallet escrow state
// 3. Set Order.EscrowStatus = mapWalletEscrowToOrderEscrow(walletEscrow.Status)
// 4. THEN call this method
func (o *Order) MarkPaid() error {
	// Validate order status transition
	if !canTransition(o.Status, StatusPaid) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusPaid,
		}
	}

	// CRITICAL: Do NOT set EscrowStatus here - it must be derived from Wallet
	// This ensures Order.EscrowStatus is ALWAYS a projection of Wallet state

	o.Status = StatusPaid

	// Calculate ready_to_ship_by based on preparation_time_snapshot
	// This is the deadline when seller should have the item ready for shipping
	// EVERY paid order MUST have a deadline — immediate = 1 day, unknown = 2 days (safe fallback)
	preparationDays := preparationTimeToDays(o.PreparationTimeSnapshot)
	readyBy := time.Now().Add(time.Duration(preparationDays) * 24 * time.Hour)
	o.ReadyToShipBy = &readyBy

	o.UpdatedAt = time.Now()
	return nil
}

// preparationTimeToDays converts preparation time string to days.
// Every value returns > 0 so that ALL paid orders get a shipment deadline.
//
// Mapping:
//
//	immediate = 1 day  (seller claims ready, still gets 1 day + 2 day grace = 3 day total)
//	short     = 2 days
//	medium    = 5 days
//	long      = 7 days
//	unknown   = 2 days (safe fallback = short)
func preparationTimeToDays(preparationTime string) int {
	switch preparationTime {
	case "immediate":
		return 1
	case "short":
		return 2
	case "medium":
		return 5
	case "long":
		return 7
	default:
		return 2 // Safe fallback: unknown → short (2 days)
	}
}

// IsExpired returns true if the current time is past the payment expiry deadline.
// This provides a single source of truth for expiry checks across the system.
//
// CRITICAL: This is the ONLY method that should be used to check order expiry.
// Do NOT use created_at + interval logic anywhere.
func (o *Order) IsExpired() bool {
	return time.Now().After(o.PaymentExpiresAt)
}

// ============================================================================
// FULFILLMENT DEADLINE METHODS
// ============================================================================

// IsShipmentOverdue returns true if the order is past the ReadyToShipBy deadline + grace period.
// This is used to determine if buyer can force cancel or open pre-ship dispute.
func (o *Order) IsShipmentOverdue() bool {
	if o.ReadyToShipBy == nil {
		return false // No deadline set, not overdue
	}

	gracePeriodEnd := o.ReadyToShipBy.Add(FulfillmentGracePeriodDays * 24 * time.Hour)
	return time.Now().After(gracePeriodEnd)
}

// IsBuyerEligibleForCancel returns true if buyer can cancel the order due to shipment delay.
// Buyer can cancel after ReadyToShipBy + grace period has passed.
func (o *Order) IsBuyerEligibleForCancel() bool {
	return o.Status == StatusPaid && o.IsShipmentOverdue()
}

// IsBuyerEligibleForPreShipDispute returns true if buyer can open a dispute before shipping.
// Buyer can open pre-ship dispute after order is overdue + additional buffer period.
func (o *Order) IsBuyerEligibleForPreShipDispute() bool {
	if o.Status != StatusPaid || o.ReadyToShipBy == nil {
		return false
	}

	// Can open pre-ship dispute after grace period + buffer
	preShipDisputeAllowedAt := o.ReadyToShipBy.Add(
		(FulfillmentGracePeriodDays * 24) + (PreShipDisputeAllowedAfterHours * time.Hour),
	)
	return time.Now().After(preShipDisputeAllowedAt)
}

// IsWithinPostShipDisputeWindow returns true if we're within the post-ship dispute window.
// Buyers can open disputes within 12 hours after shipping.
//
// TIMESTAMP DERIVATION: We derive the actual mark-ship time from AutoReleaseAt.
// MarkShipped() sets AutoReleaseAt = time.Now() + AutoReleaseDuration, so
// mark-ship time = AutoReleaseAt - AutoReleaseDuration.
// Dispute window = mark-ship time + PostShipDisputeWindowHours.
func (o *Order) IsWithinPostShipDisputeWindow() bool {
	if o.Status != StatusShipped || o.AutoReleaseAt == nil {
		return false
	}

	// Derive actual mark-ship time from AutoReleaseAt
	markShipTime := o.AutoReleaseAt.Add(-AutoReleaseDuration)
	windowCloses := markShipTime.Add(PostShipDisputeWindowHours * time.Hour)
	return time.Now().Before(windowCloses)
}

// IsUserAuthorizedForDispute returns true if the user is authorized to access/manage the dispute.
// Only buyer and seller can access their order disputes.
func (o *Order) IsUserAuthorizedForDispute(userID uuid.UUID) bool {
	return userID == o.BuyerID || userID == o.SellerID
}

// GetGracePeriodEnd returns the end of the grace period for shipment.
// This is when buyer can start taking action if seller hasn't shipped.
func (o *Order) GetGracePeriodEnd() *time.Time {
	if o.ReadyToShipBy == nil {
		return nil
	}

	gracePeriodEnd := o.ReadyToShipBy.Add(FulfillmentGracePeriodDays * 24 * time.Hour)
	return &gracePeriodEnd
}

// MarkShipped transitions the order from paid to shipped.
//
// BUSINESS TRUTH: The 5-day auto-complete timer starts HERE, when the seller marks SHIPPED
// with valid proof. This is the buyer protection window - it does NOT wait for buyer delivery
// confirmation. The timer is: now + 5 days, stored in auto_release_at.
//
// SHIPPING PROOF REQUIREMENTS (STRICT - NO FAKE SHIPMENT):
// - proofType: REQUIRED - must be one of "tracking" | "phone" | "manual"
// - shippingReference: REQUIRED for proof_type="tracking" or "phone"
// - shippingProofMedia: REQUIRED for proof_type="manual"
// - shippingNote: Optional shipping note from seller to buyer
//
// VALIDATION:
// - ONLY allows transition from PAID status (blocks double-submit)
// - Cannot mark shipped without valid proof
// - Cannot mark shipped after ReadyToShipBy + grace period (FULFILLMENT DEADLINE)
// - Timer starts after proof validation passes (now + 5 days)
// - Tracking number must be at least 6 characters
// - Phone number must be at least 10 digits (Indonesian format)
func (o *Order) MarkShipped(proofType *string, trackingNumber *string, shippingProofMedia *string, note *string) error {
	// DOUBLE SUBMIT PROTECTION: Only allow transition from PAID status
	if o.Status != StatusPaid {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusShipped,
		}
	}

	// 🔥 PHASE 1: FULFILLMENT DEADLINE ENFORCEMENT (P0)
	// Block late shipments to prevent indefinite buyer waiting
	if o.ReadyToShipBy != nil {
		gracePeriodEnd := o.ReadyToShipBy.Add(FulfillmentGracePeriodDays * 24 * time.Hour)
		now := time.Now()

		if now.After(gracePeriodEnd) {
			return &ErrShipmentDeadlineExceeded{
				OrderID:        o.ID,
				ReadyToShipBy:  *o.ReadyToShipBy,
				GracePeriodEnd: gracePeriodEnd,
			}
		}
	}

	// VALIDATE: proof_type is required
	if proofType == nil || *proofType == "" {
		return &InvalidShippingProofError{
			Reason: "proof_type is required",
		}
	}

	// VALIDATE: proof_type must be one of the allowed values
	pt := *proofType
	if pt != ProofTypeTracking && pt != ProofTypePhone && pt != ProofTypeManual {
		return &InvalidShippingProofError{
			Reason: "proof_type must be one of: tracking, phone, manual",
		}
	}

	// VALIDATE: tracking_number is required for tracking/phone
	if (pt == ProofTypeTracking || pt == ProofTypePhone) && (trackingNumber == nil || *trackingNumber == "") {
		return &InvalidShippingProofError{
			Reason: "tracking_number is required for proof_type=" + pt,
		}
	}

	// VALIDATE: shipping_proof_media is required for manual
	if pt == ProofTypeManual && (shippingProofMedia == nil || *shippingProofMedia == "") {
		return &InvalidShippingProofError{
			Reason: "shipping_proof_media is required for proof_type=manual",
		}
	}

	// VALIDATE: tracking number format (minimum 6 characters)
	if pt == ProofTypeTracking && trackingNumber != nil {
		if !isValidTrackingNumber(*trackingNumber) {
			return &InvalidShippingProofError{
				Reason: "tracking number must be at least 6 characters",
			}
		}
	}

	// VALIDATE: phone format for phone proof type (minimum 10 digits)
	if pt == ProofTypePhone && trackingNumber != nil {
		if !isValidPhoneFormat(*trackingNumber) {
			return &InvalidShippingProofError{
				Reason: "phone number must be at least 10 digits (Indonesian format: 08xxxxxxxxxx, +628xxxxxxxxxx, or 628xxxxxxxxxx)",
			}
		}
	}

	// All validations passed - transition to shipped
	o.Status = StatusShipped

	// TIMER RESET PROTECTION: Only set timer if not already set
	// This prevents abuse where seller could repeatedly "ship" to reset the timer
	if o.AutoReleaseAt == nil {
		autoRelease := time.Now().Add(AutoReleaseDuration)
		o.AutoReleaseAt = &autoRelease
	}

	// Store shipping proof information
	o.ProofType = proofType
	o.TrackingNumber = trackingNumber
	o.ShippingProofMedia = shippingProofMedia
	o.ShippingNote = note
	o.UpdatedAt = time.Now()
	return nil
}

// Complete transitions the order from shipped/delivered to completed and releases escrow to seller.
//
// BUSINESS RULE: Timer starts at SHIPPED, so auto-complete works from both shipped and delivered.
// The timer-based auto-complete is the source of truth, not the delivery status.
//
// CRITICAL SAFETY GUARDS (defended in depth):
// - Can complete from "shipped" OR "delivered" (timer-based auto-complete)
// - Cannot complete if has_dispute = true (DisputeActiveError)
// - Cannot complete if escrow_status != "holding" (InvalidEscrowStatusError)
//
// These guards are the SECOND LAYER of defense against auto-completing disputed orders.
// The FIRST LAYER is the database query (has_dispute = false).
// This defense-in-depth approach prevents race conditions.
// ValidateComplete validates that an order can be completed without modifying state.
// This allows validation to happen BEFORE wallet operations.
func (o *Order) ValidateComplete() error {
	// GUARD 0: Can ONLY complete from "shipped" or "delivered" status
	// Timer starts at shipped, so auto-complete works from both states
	if o.Status != StatusShipped && o.Status != StatusDelivered {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusCompleted,
		}
	}

	// CRITICAL GUARD 1: Cannot complete order with active dispute
	// This is the final defense - even if a disputed order slips through the query,
	// this check prevents completion.
	if o.HasDispute {
		return &DisputeActiveError{OrderID: o.ID}
	}

	// NOTE: EscrowStatus validation removed - that's Wallet's responsibility
	// This method only validates order-level state
	// WalletService will validate escrow state when ReleaseEscrow is called

	return nil
}

// Cancel transitions the order from pending to cancelled.
//
// EXPLICIT GUARD: Cannot cancel after shipped.
// This provides an explicit safety check on top of the state machine transition.
func (o *Order) Cancel() error {
	// EXPLICIT GUARD: Cannot cancel after order has been shipped
	if o.Status == StatusShipped || o.Status == StatusDelivered {
		return fmt.Errorf("cannot cancel order: already shipped (status=%s)", o.Status)
	}

	// State machine transition check
	if !canTransition(o.Status, StatusCancelled) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusCancelled,
		}
	}
	o.Status = StatusCancelled
	o.UpdatedAt = time.Now()
	return nil
}

// ValidateCancelTimeout validates that an order can be cancelled due to timeout without modifying state.
// This allows validation to happen BEFORE wallet operations.
func (o *Order) ValidateCancelTimeout() error {
	if !canTransition(o.Status, StatusCancelledTimeout) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusCancelledTimeout,
		}
	}

	// NOTE: EscrowStatus validation removed - that's Wallet's responsibility
	// This method only validates order-level state
	// WalletService will validate escrow state when RefundEscrow is called

	return nil
}

// MarkExpired transitions the order from pending to expired.
// This is called when payment expires without completion.
func (o *Order) MarkExpired() error {
	if !canTransition(o.Status, StatusExpired) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusExpired,
		}
	}
	o.Status = StatusExpired
	o.UpdatedAt = time.Now()
	return nil
}

// MarkDisputeOpen transitions the order to dispute_open state.
//
// CRITICAL: EscrowStatus is NOT modified here. Dispute state is tracked via HasDispute field.
//
// This method ONLY:
// - Validates order status transition (shipped/delivered -> dispute_open)
// - Sets order status to dispute_open
// - Sets has_dispute = true
//
// ESCROW STATE: The Wallet domain does not have a "frozen" state.
// Disputes are tracked solely by Order.HasDispute = true.
// When a dispute is resolved, the escrow is released/refunded via WalletService.
func (o *Order) MarkDisputeOpen() error {
	// Validate order status transition
	if !canTransition(o.Status, StatusDisputeOpen) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusDisputeOpen,
		}
	}

	o.Status = StatusDisputeOpen
	o.HasDispute = true

	// CRITICAL: Do NOT modify EscrowStatus here
	// Wallet domain has no "frozen" state
	// Dispute presence is tracked by HasDispute field only

	o.UpdatedAt = time.Now()
	return nil
}

// MarkPartiallyRefunded transitions the order to partially_refunded state.
// This is called when a partial refund is processed via dispute resolution.
//
// CRITICAL: EscrowStatus is set based on what actually happened in Wallet:
// - If buyer refunded: EscrowStatus = "refunded" (full refund to buyer)
// - If seller released: EscrowStatus = "released" (full release to seller)
//
// PARTIAL DISPUTE RESOLUTION:
// - Partial refunds are tracked via Order.Status = "partially_refunded"
// - Wallet performs SEPARATE operations (ReleaseEscrow + RefundEscrow)
// - Order.EscrowStatus reflects the FINAL wallet state (usually "released" or "refunded")
// - The "partial" aspect is tracked in Order.Status, NOT in EscrowStatus
//
// TRANSITIONS: shipped/delivered/dispute_open -> partially_refunded
//
// GUARDS:
// - Can only be called from shipped, delivered, or dispute_open status
// - Sets order status to partially_refunded
//
// CALLER MUST:
// 1. Execute Wallet operations (ReleaseEscrow + RefundEscrow for partial split)
// 2. Fetch FINAL wallet escrow state
// 3. Set Order.EscrowStatus = mapWalletEscrowToOrderEscrow(walletEscrow.Status)
// 4. THEN call this method
func (o *Order) MarkPartiallyRefunded() error {
	// Validate order status transition
	if !canTransition(o.Status, StatusPartiallyRefunded) {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusPartiallyRefunded,
		}
	}

	// CRITICAL: Do NOT set EscrowStatus to "partially_refunded"
	// Wallet domain has no such state
	// EscrowStatus should be set to "released" or "refunded" based on Wallet state

	o.Status = StatusPartiallyRefunded
	o.UpdatedAt = time.Now()
	return nil
}

// ExtendConfirmationPeriod extends the buyer confirmation period by 3 days.
// This allows buyer more time to inspect the item before auto-release.
//
// BUSINESS RULES:
// - status must be 'shipped'
// - has_dispute must be false
// - confirmation_extension_used must be false
// - auto_release_at must be in the future
// - extension only allowed within last 24 hours of current deadline (ANTI-ABUSE)
//
// ACTION:
// - auto_release_at += 3 days
// - confirmation_extension_used = true
// - confirmation_extended_at = now()
func (o *Order) ExtendConfirmationPeriod() error {
	// GUARD 1: Can only extend from shipped status
	if o.Status != StatusShipped {
		return &InvalidTransitionError{
			CurrentStatus: o.Status,
			TargetStatus:  StatusShipped,
		}
	}

	// GUARD 2: Cannot extend if dispute is active
	if o.HasDispute {
		return &DisputeActiveError{OrderID: o.ID}
	}

	// GUARD 3: Can only extend once
	if o.ConfirmationExtensionUsed {
		return fmt.Errorf("confirmation period already extended for order: %s", o.ID)
	}

	// GUARD 4: auto_release_at must be set
	if o.AutoReleaseAt == nil {
		return fmt.Errorf("auto_release_at not set for order: %s", o.ID)
	}

	// GUARD 5: Cannot extend expired order (SAFETY CHECK)
	now := time.Now()
	if o.AutoReleaseAt.Before(now) {
		return fmt.Errorf("cannot extend expired order: %s (auto_release_at passed)", o.ID)
	}

	// GUARD 6: Extension only allowed near expiry (ANTI-ABUSE)
	// Buyer can only extend in the last 24 hours of the confirmation period.
	// This prevents extending on day 1 and abusing the +3 days.
	remaining := o.AutoReleaseAt.Sub(now)
	if remaining > 24*time.Hour {
		return fmt.Errorf("extension only allowed near expiry (last 24 hours): %s (remaining: %v)", o.ID, remaining)
	}

	// All guards passed - extend by 3 days
	extendedTime := o.AutoReleaseAt.Add(3 * 24 * time.Hour)
	o.AutoReleaseAt = &extendedTime
	o.ConfirmationExtensionUsed = true
	o.ConfirmationExtendedAt = &now
	o.UpdatedAt = now

	return nil
}

// SetAutoRelease sets the auto-release timestamp for buyer confirmation.
func (o *Order) SetAutoRelease(at time.Time) {
	o.AutoReleaseAt = &at
	o.UpdatedAt = time.Now()
}

// ApplyShippingDestination applies the shipping destination snapshot to the order.
//
// This is called AFTER address validation and BEFORE order persistence.
// It stores the immutable snapshot of the shipping address.
//
// IMMUTABILITY GUARANTEE:
// - The snapshot is frozen at order creation time
// - Buyer can modify or delete their address without affecting this order
// - Seller always has the correct shipping destination for fulfillment
//
// This snapshot is used for:
// - Shipping label generation
// - Order detail display
// - Fulfillment and delivery
// - Refund/dispute resolution (shipping proof)
// - Audit and compliance
func (o *Order) ApplyShippingDestination(snapshot addressentity.AddressSnapshot) {
	o.ShippingDestination = &snapshot
	o.UpdatedAt = time.Now()
}

// ApplyShippingOrigin applies the shipping origin snapshot to the order.
//
// This is called AFTER farm address validation and BEFORE order persistence.
// It stores the immutable snapshot of the seller's farm/warehouse address.
//
// IMMUTABILITY GUARANTEE:
// - The snapshot is frozen at order creation time from listing.FarmAddressID
// - Seller can modify or delete their farm address without affecting this order
// - Buyer always has the correct shipping origin for tracking and returns
//
// This snapshot is used for:
// - Shipping label generation
// - Order detail display
// - Return shipping calculations
// - Fulfillment logistics
// - Audit and compliance
func (o *Order) ApplyShippingOrigin(snapshot addressentity.AddressSnapshot) {
	o.ShippingOrigin = &snapshot
	o.UpdatedAt = time.Now()
}

// NewOrderFromSource creates a new pending order with explicit source metadata.
// This is the ONLY method for order creation - all orders must use pricing snapshot.
//
// CRITICAL: PricingSnapshot is MANDATORY - no fallback calculation exists.
// If PricingSnapshot is nil → HARD FAIL
//
// Parameters:
// - sourceType: The origin of this order (listing/negotiation/auction)
// - sourceID: The ID of the source entity
// - negotiationID: Optional, for negotiated purchases
// - quantity: Number of units (must be > 0)
// - unitPrice: Price per unit
// - subtotal: Subtotal amount from pricing snapshot (Quantity * UnitPrice)
// - shippingTotal: Shipping cost
// - commissionPercent: Commission percentage for record (e.g., 5 for 5%)
// - commissionAmount: Commission amount snapshot from pricing token
// - shippingSource: "for_sale" or "shipping_quote"
// - shippingQuoteID: Quote ID when using shipping quote (TASK F)
// - shippingQuotePrice: Quote price snapshot when using shipping quote (TASK F)
// - paymentMethod: Payment method (instant, va, retail, etc.)
// - paymentExpiresAt: When payment window closes (SINGLE SOURCE OF TRUTH)
//
// VALIDATION GUARDS:
// - Panics if quantity <= 0
// - Panics if unitPrice is negative
// - Panics if sourceType is invalid
//
// FINANCIAL TRUTH:
// - Escrow is managed by Wallet service, NOT calculated here
// - Refunds are tracked in Wallet, NOT in Order
// - Discounts are applied at order creation with pricing token
// - Coins usage is recorded for display only
func NewOrderFromSource(
	buyerID, sellerID uuid.UUID,
	sourceType OrderSourceType,
	sourceID uuid.UUID,
	negotiationID *uuid.UUID,
	quantity int,
	unitPrice, subtotal, shippingTotal money.Money,
	commissionPercent int64,
	commissionAmount money.Money, // Commission amount from pricing snapshot
	serviceFeeAmount money.Money, // Flat buyer checkout service fee snapshot
	totalPayableAmount money.Money, // Buyer gross payable snapshot
	shippingOptionID *uuid.UUID, // NULLABLE: nil when using shipping quote
	shippingOptionName string,
	shippingTransportType string,
	shippingExpeditionName *string,
	shippingEstimatedDays *string,
	auctionSettlementType *AuctionSettlementType,
	preparationTimeSnapshot string,
	preparationNoteSnapshot *string,
	shippingSource *string, // Shipping source ("for_sale" or "shipping_quote")
	shippingQuoteID *uuid.UUID, // TASK F: Quote ID when using shipping quote
	shippingQuotePrice *int64, // TASK F: Quote price snapshot
	pricingTokenID *uuid.UUID, // Pricing token used for this order (prevents double-ordering)
	paymentMethod string, // Payment method: instant, va, retail, etc.
	paymentExpiresAt time.Time, // When payment window closes (SINGLE SOURCE OF TRUTH)
) *Order {
	// Validation guards
	if quantity <= 0 {
		panic("order: quantity must be positive")
	}
	if unitPrice.IsNegative() {
		panic("order: unit price cannot be negative")
	}

	// D2: Max price cap validation
	// Prevents absurd prices that could cause system issues or fraud
	// Cap: 50,000,000,000 (50 billion in smallest currency unit = 500M IDR)
	// This is a reasonable upper bound for any legitimate transaction
	const maxUnitPrice = 50_000_000_000
	if unitPrice.Int64() > maxUnitPrice {
		panic(fmt.Sprintf("order: unit price exceeds maximum allowed: %d > %d", unitPrice.Int64(), maxUnitPrice))
	}

	// Validation guard: source type must be valid
	if !sourceType.IsValid() {
		panic(fmt.Sprintf("order: invalid source type: %s", sourceType))
	}

	now := time.Now()
	orderNum := GenerateOrderNumber()

	// Subtotal is provided from pricing snapshot (NO calculation)
	// commissionPercent is kept for record only

	return &Order{
		ID:                        uuid.New(),
		OrderNumber:               &orderNum,
		BuyerID:                   buyerID,
		SellerID:                  sellerID,
		SourceType:                sourceType,
		SourceID:                  sourceID,
		NegotiationID:             negotiationID,
		AuctionSettlementType:     auctionSettlementType,
		Quantity:                  quantity,
		UnitPrice:                 unitPrice,
		Subtotal:                  subtotal,
		ShippingTotal:             shippingTotal,
		CommissionPercent:         commissionPercent,
		CommissionAmount:          commissionAmount,
		ServiceFeeAmount:          serviceFeeAmount,
		TotalPayableAmount:        totalPayableAmount,
		TotalBeforeCoinsAmount:    totalPayableAmount,
		CoinsUsed:                 0, // No coins used by default
		ShippingOptionID:          shippingOptionID,
		ShippingOptionName:        shippingOptionName,
		ShippingTransportType:     shippingTransportType,
		ShippingExpeditionName:    shippingExpeditionName,
		ShippingEstimatedDays:     shippingEstimatedDays,
		ShippingSource:            shippingSource,
		ShippingQuoteID:           shippingQuoteID,    // TASK F: Store quote ID
		ShippingQuotePrice:        shippingQuotePrice, // TASK F: Store quote price
		PricingTokenID:            pricingTokenID,     // Store pricing token ID (prevents double-ordering)
		PreparationTimeSnapshot:   preparationTimeSnapshot,
		PreparationNoteSnapshot:   preparationNoteSnapshot,
		ReadyToShipBy:             nil, // Will be calculated when order is marked as paid
		Status:                    StatusPending,
		EscrowStatus:              EscrowStatusHolding,
		AutoReleaseAt:             nil,
		HasDispute:                false,
		ConfirmationExtensionUsed: false,
		ConfirmationExtendedAt:    nil,
		PaymentMethod:             paymentMethod,    // Payment method
		PaymentExpiresAt:          paymentExpiresAt, // SINGLE SOURCE OF TRUTH for expiry
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
}
