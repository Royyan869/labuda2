package entity

// Status represents the order status as a strict state machine.
// Order lifecycle: Pending -> Paid -> Shipped -> Completed
// B4A: Buyer "Terima Barang" calls Complete() directly from Shipped.
// StatusDelivered preserved for internal/worker use but not in buyer-facing flow.
// Branches: Can be Cancelled from Pending, or Refunded from Paid/Shipped
type Status string

const (
	// StatusPending is the initial state when order is created.
	StatusPending Status = "pending_payment"
	// StatusPaid is when buyer payment is confirmed.
	StatusPaid Status = "paid"
	// StatusShipped is when seller ships the item.
	StatusShipped Status = "shipped"
	// StatusDelivered is when buyer confirms receipt.
	StatusDelivered Status = "delivered"
	// StatusCompleted is when order is fully settled and escrow released.
	StatusCompleted Status = "completed"
	// StatusCancelled is when order is cancelled before payment.
	StatusCancelled Status = "cancelled"
	// StatusCancelledTimeout is when order is auto-cancelled due to shipment timeout.
	StatusCancelledTimeout Status = "cancelled_timeout"
	// StatusRefunded is when order is refunded.
	StatusRefunded Status = "refunded"
	// StatusPartiallyRefunded is when order is partially refunded (buyer gets partial, seller gets remainder).
	StatusPartiallyRefunded Status = "partially_refunded"
	// StatusDisputeOpen is when a dispute is active and escrow is frozen.
	StatusDisputeOpen Status = "dispute_open"
	// StatusExpired is when payment expires and order is terminated.
	StatusExpired Status = "expired"
)

// transitionAllowed defines valid state transitions.
//
// BUSINESS RULE:
// Auto-complete timer starts when seller marks order as shipped.
// Buyer has 5 days to confirm or dispute.
// Buyer may extend once (+3 days) near expiry.
//
// Valid transitions:
// - shipped -> completed (buyer "Terima Barang" or auto-complete timer)
// - shipped -> delivered (internal checkpoint — not in buyer-facing flow)
// - delivered -> completed (auto-complete from internal checkpoint)
// - pending -> expired is allowed for payment expiry.
// - shipped -> dispute_open for dispute escalation (before acceptance).
// - dispute_open -> completed/refunded for dispute resolution.
// - shipped -> partially_refunded for partial refund.
var transitionAllowed = map[Status][]Status{
	StatusPending:           {StatusPaid, StatusCancelled, StatusExpired},
	StatusPaid:              {StatusShipped, StatusRefunded, StatusCancelled, StatusCancelledTimeout},                       // Allow timeout cancellation
	StatusShipped:           {StatusCompleted, StatusDelivered, StatusRefunded, StatusDisputeOpen, StatusPartiallyRefunded}, // B4A: shipped→completed is canonical buyer path
	StatusDelivered:         {StatusCompleted, StatusRefunded, StatusDisputeOpen, StatusPartiallyRefunded},                  // Internal checkpoint
	StatusDisputeOpen:       {StatusCompleted, StatusRefunded, StatusPartiallyRefunded}, // Dispute resolution outcomes
	StatusCompleted:         {},                                                         // Terminal state
	StatusCancelled:         {},                                                         // Terminal state
	StatusCancelledTimeout:  {},                                                         // Terminal state (auto-cancelled due to timeout)
	StatusRefunded:          {},                                                         // Terminal state
	StatusPartiallyRefunded: {},                                                         // Terminal state
	StatusExpired:           {},                                                         // Terminal state
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


