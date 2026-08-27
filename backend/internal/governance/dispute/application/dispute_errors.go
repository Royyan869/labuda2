package application

import "errors"

var (
	// ErrDisputeResolutionCapabilityRequired is returned when an admin caller
	// lacks the finance.dispute.resolve capability.
	ErrDisputeResolutionCapabilityRequired = errors.New("forbidden: missing capability finance.dispute.resolve")

	// ErrDisputeOpenAlreadyHasActive is returned when an order already has an
	// active dispute and a new dispute cannot be opened.
	ErrDisputeOpenAlreadyHasActive = errors.New("cannot open dispute: order already has an active dispute")

	// ErrDisputeOpenAfterCompletion is returned when a dispute is attempted
	// after the order has already reached completed + released finality.
	ErrDisputeOpenAfterCompletion = errors.New("cannot open dispute after order completion")

	// ErrDisputeOpenNoEscrow is returned when no escrow row exists for the order.
	ErrDisputeOpenNoEscrow = errors.New("cannot open dispute: no escrow found for order")

	// ErrDisputeOpenInvalidEscrowState is returned when the escrow is in an
	// unexpected non-terminal state for dispute opening.
	ErrDisputeOpenInvalidEscrowState = errors.New("cannot open dispute: invalid escrow state")

	// ErrDisputeOpenPreShipNotEligible is returned when a paid order is not
	// eligible for pre-ship dispute opening.
	ErrDisputeOpenPreShipNotEligible = errors.New("cannot open pre-ship dispute: order not eligible")

	// ErrDisputeOpenPostShipWindowExpired is returned when the post-ship
	// dispute window has expired.
	ErrDisputeOpenPostShipWindowExpired = errors.New("cannot open dispute: post-ship dispute window expired")

	// ErrDisputeResolveInvalidState is returned when a dispute cannot be
	// resolved in its current state.
	ErrDisputeResolveInvalidState = errors.New("cannot resolve dispute: current status is")

	// ErrDisputeResolveAfterCompletion is returned when a dispute is already
	// final because the order completed and escrow was released.
	ErrDisputeResolveAfterCompletion = errors.New("cannot resolve dispute after order completion")
)
