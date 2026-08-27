package mapper

import (
	"fmt"
)

// Internal Payment Status (Gateway-Agnostic)
//
// These statuses represent the canonical internal state of a payment.
// They are NOT coupled to any specific payment gateway.
const (
	// PaymentStatusPending is the initial state awaiting payment
	PaymentStatusPending = "pending"

	// PaymentStatusSettlement indicates payment is in settlement process
	// Maps from: Midtrans settlement
	PaymentStatusSettlement = "settlement"

	// PaymentStatusCapture indicates payment capture phase
	// Maps from: Midtrans capture
	PaymentStatusCapture = "capture"

	// PaymentStatusPaid indicates successful payment completion
	// Maps from: Midtrans capture, settlement
	PaymentStatusPaid = "paid"

	// PaymentStatusFailed indicates payment was rejected or failed
	// Maps from: Midtrans deny
	PaymentStatusFailed = "failed"

	// PaymentStatusCancelled indicates payment was cancelled
	// Maps from: Midtrans cancel
	PaymentStatusCancelled = "cancelled"

	// PaymentStatusExpired indicates payment expired without completion
	// Maps from: Midtrans expire
	//
	// NOTE: Must match database enum value "expire" (without 'd')
	PaymentStatusExpired = "expire"
)

// IsTerminalStatus returns true if the status is a terminal state
// Terminal statuses cannot transition to other states
func IsTerminalStatus(status string) bool {
	switch status {
	case PaymentStatusPaid, PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired:
		return true
	default:
		return false
	}
}

// IsSuccessfulStatus returns true if the payment was successful
func IsSuccessfulStatus(status string) bool {
	return status == PaymentStatusPaid
}

// IsFailedStatus returns true if the payment failed
func IsFailedStatus(status string) bool {
	switch status {
	case PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired:
		return true
	default:
		return false
	}
}

// MidtransToInternalStatus maps Midtrans transaction status to internal payment status
//
// This mapping decouples the payment domain from Midtrans-specific statuses.
// If we switch to another gateway in the future, only this mapping changes.
func MidtransToInternalStatus(midtransStatus string) (string, error) {
	// Midtrans success statuses
	switch midtransStatus {
	case "capture", "settlement":
		return PaymentStatusPaid, nil
	case "pending":
		return PaymentStatusPending, nil
	case "deny":
		return PaymentStatusFailed, nil
	case "cancel":
		return PaymentStatusCancelled, nil
	case "expire":
		return PaymentStatusExpired, nil
	default:
		return "", fmt.Errorf("unknown Midtrans status: %s", midtransStatus)
	}
}

// MidtransToInternalStatusSafe maps Midtrans status to internal status.
// Returns "pending" for unknown statuses instead of error.
func MidtransToInternalStatusSafe(midtransStatus string) string {
	status, err := MidtransToInternalStatus(midtransStatus)
	if err != nil {
		// Log warning but default to pending for unknown statuses
		return PaymentStatusPending
	}
	return status
}


