package orchestrator

import (
	"context"

	"github.com/google/uuid"
)

// PaymentReferenceHandler defines the contract for handling payment finalization
// for different reference types (order, billing, seller_subscription, etc.)
//
// Each reference type has its own domain logic for handling:
// - Payment completed: successful payment, release goods/service
// - Payment expired: payment window closed, cleanup
// - Payment refunded: payment reversed, revert goods/service
type PaymentReferenceHandler interface {
	// ReferenceType returns the reference type this handler supports
	// Examples: "order", "billing", "seller_subscription"
	ReferenceType() string

	// HandlePaymentCompleted handles successful payment finalization
	// This is called when payment is confirmed as completed (e.g., Midtrans settlement)
	HandlePaymentCompleted(ctx context.Context, paymentID uuid.UUID) error

	// HandlePaymentExpired handles expired payment finalization
	// This is called when payment expires without completion
	HandlePaymentExpired(ctx context.Context, paymentID uuid.UUID) error

	// HandlePaymentRefunded handles refunded payment finalization
	// This is called when payment is refunded (full or partial)
	HandlePaymentRefunded(ctx context.Context, paymentID uuid.UUID) error
}


